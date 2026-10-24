package app

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/kalfasyan/troye/jobs"
	"github.com/kalfasyan/troye/push"
)

// Exercise, water and the weekly summary.
//
// Exercise calories are what the activity burns on top of resting, since the
// daily budget already counts resting and everyday activity: (MET - 1) x kg x
// hours, with METs from the Compendium of Physical Activities. They are added
// to that day's budget.

// mets is how hard each kind of exercise is, in METs.
var mets = map[string]float64{
	"walk":     3.5,
	"dog_walk": 3.0,
	"run":      9.8,
	"cycle":    7.5,
	"swim":     7.0,
	"strength": 5.0,
	"yoga":     2.5,
	"hike":     6.0,
	"sport":    7.0,
	"other":    4.5,
}

// ExerciseKcal is the calories an exercise burns above resting.
func ExerciseKcal(kind string, minutes, kg float64) int {
	met, ok := mets[kind]
	if !ok {
		met = mets["other"]
	}
	if kg <= 0 {
		kg = 70
	}
	return int(math.Round((met - 1) * kg * minutes / 60))
}

func registerExtras(app core.App) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/app/week", weekRoute).Bind(apis.RequireAuth("users"))
		return se.Next()
	})

	app.OnRecordCreate("exercises").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetInt("kcal") == 0 {
			fillExercise(e.App, e.Record)
		}
		return e.Next()
	})
	app.OnRecordUpdate("exercises").BindFunc(func(e *core.RecordEvent) error {
		old := e.Record.Original()
		changed := old.GetString("kind") != e.Record.GetString("kind") || old.GetInt("minutes") != e.Record.GetInt("minutes")
		// new minutes move the calories, unless the person typed their own
		if e.Record.GetInt("kcal") == 0 || changed && e.Record.GetInt("kcal") == old.GetInt("kcal") {
			fillExercise(e.App, e.Record)
		}
		return e.Next()
	})

	// Monday morning: how last week went
	jobs.MustAdd(app, "weekly-summary", "0 9 * * 1", func(app core.App) error {
		return sendWeeklySummaries(app, time.Now())
	})
}

func fillExercise(app core.App, r *core.Record) {
	kg := 0.0
	if p, err := app.FindFirstRecordByFilter("profiles", "user = {:user}", dbx.Params{"user": r.GetString("user")}); err == nil {
		kg = p.GetFloat("weight_kg")
	}
	r.Set("kcal", ExerciseKcal(r.GetString("kind"), r.GetFloat("minutes"), kg))
}

// Day is one day of a week's summary.
type Day struct {
	Date     string  `json:"date"`
	Kcal     float64 `json:"kcal"`
	Exercise int     `json:"exercise"`
	// the calorie budget plus that day's exercise
	Budget  int  `json:"budget"`
	WaterML int  `json:"water_ml"`
	Logged  bool `json:"logged"`
}

// Week sums up seven days from a Monday.
type Week struct {
	Start      string `json:"start"`
	Days       []Day  `json:"days"`
	DaysLogged int    `json:"days_logged"`
	// the average over the days with food logged
	AvgKcal     int `json:"avg_kcal"`
	UnderBudget int `json:"under_budget"`
	// the latest weigh-in of the week against the one before the week; 0 without both
	WeightChange float64 `json:"weight_change"`
	HasWeight    bool    `json:"has_weight"`
	// days in a row with food logged, up to today
	Streak int `json:"streak"`
}

// Monday is the Monday of the week a day is in.
func Monday(day time.Time) time.Time {
	offset := (int(day.Weekday()) + 6) % 7
	return time.Date(day.Year(), day.Month(), day.Day()-offset, 0, 0, 0, 0, day.Location())
}

// WeekSummary sums up a person's week from its Monday.
func WeekSummary(app core.App, user string, monday, today time.Time) (Week, error) {
	start := monday.Format(time.DateOnly)
	end := monday.AddDate(0, 0, 6).Format(time.DateOnly)
	w := Week{Start: start}

	budget := 0
	if p, err := app.FindFirstRecordByFilter("profiles", "user = {:user}", dbx.Params{"user": user}); err == nil {
		budget = p.GetInt("calorie_budget")
	}
	byDay := map[string]*Day{}
	for i := range 7 {
		d := monday.AddDate(0, 0, i).Format(time.DateOnly)
		w.Days = append(w.Days, Day{Date: d, Budget: budget})
	}
	for i := range w.Days {
		byDay[w.Days[i].Date] = &w.Days[i]
	}

	inWeek := "user = {:user} && date >= {:start} && date <= {:end}"
	params := dbx.Params{"user": user, "start": start, "end": end}
	entries, err := app.FindRecordsByFilter("entries", inWeek, "", 0, 0, params)
	if err != nil {
		return w, err
	}
	for _, e := range entries {
		d := byDay[e.GetString("date")]
		d.Kcal += e.GetFloat("kcal")
		d.Logged = true
	}
	exercises, err := app.FindRecordsByFilter("exercises", inWeek, "", 0, 0, params)
	if err != nil {
		return w, err
	}
	for _, x := range exercises {
		d := byDay[x.GetString("date")]
		d.Exercise += x.GetInt("kcal")
		d.Budget += x.GetInt("kcal")
	}
	water, err := app.FindRecordsByFilter("water", inWeek, "", 0, 0, params)
	if err != nil {
		return w, err
	}
	for _, x := range water {
		byDay[x.GetString("date")].WaterML = x.GetInt("ml")
	}

	var total float64
	for _, d := range w.Days {
		if !d.Logged {
			continue
		}
		w.DaysLogged++
		total += d.Kcal
		if d.Kcal <= float64(d.Budget) {
			w.UnderBudget++
		}
	}
	if w.DaysLogged > 0 {
		w.AvgKcal = int(math.Round(total / float64(w.DaysLogged)))
	}

	last, err := app.FindRecordsByFilter("weighins", inWeek, "-date", 1, 0, params)
	if err != nil {
		return w, err
	}
	before, err := app.FindRecordsByFilter("weighins", "user = {:user} && date < {:start}", "-date", 1, 0, params)
	if err != nil {
		return w, err
	}
	if len(last) == 1 && len(before) == 1 {
		w.HasWeight = true
		w.WeightChange = math.Round((last[0].GetFloat("weight_kg")-before[0].GetFloat("weight_kg"))*10) / 10
	}

	w.Streak, err = streak(app, user, today)
	return w, err
}

// streak counts the days in a row with food logged, ending today, or
// yesterday while today is still empty.
func streak(app core.App, user string, today time.Time) (int, error) {
	since := today.AddDate(0, 0, -400).Format(time.DateOnly)
	var dates []string
	err := app.RecordQuery("entries").
		Select("date").
		Distinct(true).
		AndWhere(dbx.HashExp{"user": user}).
		AndWhere(dbx.NewExp("date >= {:since}", dbx.Params{"since": since})).
		Column(&dates)
	if err != nil {
		return 0, err
	}
	logged := map[string]bool{}
	for _, d := range dates {
		logged[d] = true
	}
	day := today
	if !logged[day.Format(time.DateOnly)] {
		day = day.AddDate(0, 0, -1)
	}
	n := 0
	for logged[day.Format(time.DateOnly)] {
		n++
		day = day.AddDate(0, 0, -1)
	}
	return n, nil
}

func weekRoute(e *core.RequestEvent) error {
	now := time.Now()
	day := now
	if s := e.Request.URL.Query().Get("day"); s != "" {
		d, err := time.ParseInLocation(time.DateOnly, s, now.Location())
		if err != nil {
			return e.BadRequestError("Give the day as YYYY-MM-DD.", nil)
		}
		day = d
	}
	w, err := WeekSummary(e.App, e.Auth.Id, Monday(day), now)
	if err != nil {
		return e.InternalServerError("", err)
	}
	return e.JSON(http.StatusOK, w)
}

// sendWeeklySummaries tells everyone who logged last week how it went.
func sendWeeklySummaries(app core.App, now time.Time) error {
	profiles, err := app.FindRecordsByFilter("profiles", "onboarded = true", "", 0, 0)
	if err != nil {
		return err
	}
	lastMonday := Monday(now).AddDate(0, 0, -7)
	for _, p := range profiles {
		w, err := WeekSummary(app, p.GetString("user"), lastMonday, now)
		if err != nil {
			return err
		}
		if msg, ok := WeeklyMessage(w, p.GetString("units")); ok {
			push.ToUser(app, p.GetString("user"), msg)
		}
	}
	return nil
}

// WeeklyMessage is the Monday notification for a week, if anything was logged.
func WeeklyMessage(w Week, units string) (push.Message, bool) {
	if w.DaysLogged == 0 {
		return push.Message{}, false
	}
	parts := []string{
		fmt.Sprintf("%d of 7 days logged", w.DaysLogged),
		fmt.Sprintf("%s kcal a day", thousands(w.AvgKcal)),
	}
	if w.HasWeight {
		parts = append(parts, signedWeight(w.WeightChange, units))
	}
	title := "Your week in Troye"
	switch {
	case w.UnderBudget >= 5:
		title = fmt.Sprintf("Great week: %d days within budget", w.UnderBudget)
	case w.DaysLogged == 7:
		title = "You logged every day last week"
	}
	return push.Message{Title: title, Body: strings.Join(parts, " · "), URL: "/#week", Tag: "weekly-" + w.Start}, true
}

func thousands(n int) string {
	s := fmt.Sprint(n)
	if len(s) <= 3 {
		return s
	}
	return s[:len(s)-3] + "," + s[len(s)-3:]
}

func signedWeight(kg float64, units string) string {
	unit := "kg"
	if units == "imperial" {
		kg, unit = kg/0.45359237, "lb"
	}
	kg = math.Round(kg*10) / 10
	switch {
	case kg < 0:
		return fmt.Sprintf("−%.1f %s", -kg, unit)
	case kg > 0:
		return fmt.Sprintf("+%.1f %s", kg, unit)
	}
	return "weight steady"
}
