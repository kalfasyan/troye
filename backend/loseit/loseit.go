// Package loseit reads the export Lose It! makes of someone's account: a zip
// of CSV files with their food log, weigh-ins, exercise, own foods, plan and
// meal photos. Read turns it into values Troye can save (backend/app/loseit.go
// saves them), already in Troye's units and choices.
package loseit

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ErrNotExport means the file isn't a Lose It! export.
var ErrNotExport = errors.New("this doesn't look like a Lose It! export")

// Export is what an export holds that Troye can use.
type Export struct {
	Foods       []FoodLog
	Weights     []Weight
	Exercises   []Exercise
	CustomFoods []CustomFood
	Photos      []Photo
	// nil when the export has no profile
	Profile *Profile
	// the weekly fasting schedule, nil without one
	Fasting *Fasting
	// own foods and recipes left out because nothing says what a serving weighs
	Unweighed int
	// files with data Troye has no place for, such as "achievements (20)"
	Unused []string
}

// FoodLog is one food logged, with its totals.
type FoodLog struct {
	Date string
	Meal string
	Name string
	// what Lose It! showed, such as "2 Tablespoons"
	Amount string
	// set when the amount was a weight
	Grams       float64
	Kcal        float64
	Protein     float64
	Carbs       float64
	Fat         float64
	SatFat      float64
	Sugar       float64
	Fiber       float64
	Salt        float64
	Cholesterol float64
	// the same for the same row of the same export, so importing again skips it
	Key string
}

// Weight is a weigh-in.
type Weight struct {
	Date string
	Kg   float64
}

// Exercise is one exercise logged.
type Exercise struct {
	Date string
	// one of Troye's exercise kinds
	Kind    string
	Name    string
	Minutes int
	Kcal    int
	Key     string
}

// CustomFood is a food or recipe the person made, per 100 g.
type CustomFood struct {
	ID           string
	Name         string
	Brand        string
	Kcal         float64
	Protein      float64
	Carbs        float64
	Fat          float64
	Fiber        float64
	Sugar        float64
	Salt         float64
	ServingG     float64
	ServingLabel string
}

// Photo is a meal photo.
type Photo struct {
	Date string
	Meal string
	File *zip.File
}

// Profile is the person's plan, in Troye's choices and metric units.
type Profile struct {
	BirthDate    string
	Sex          string
	HeightCm     float64
	WeightKg     float64
	GoalWeightKg float64
	Activity     string
	Goal         string
	PaceKg       float64
}

// Fasting is a fasting schedule as Troye keeps it: the same window on some
// days of the week.
type Fasting struct {
	// one of Troye's fasting plans, such as "16_8"
	Plan string
	// when the eating window opens, HH:MM
	EatingStart string
	// the days a fast starts on: "mon" to "sun"
	Days []string
}

const kgPerLb = 0.45359237

// Read reads an export.
func Read(r io.ReaderAt, size int64) (*Export, error) {
	z, err := zip.NewReader(r, size)
	if err != nil {
		return nil, ErrNotExport
	}
	files := map[string]*zip.File{}
	photos := map[string]*zip.File{}
	for _, f := range z.File {
		name := path.Base(f.Name)
		if strings.HasSuffix(name, ".csv") {
			files[strings.TrimSuffix(name, ".csv")] = f
		} else if m := photoName.FindStringSubmatch(name); m != nil {
			photos[m[1]] = f
		}
	}
	if files["food-logs"] == nil && files["weights"] == nil {
		return nil, ErrNotExport
	}

	x := &Export{}
	table := func(name string) ([]map[string]string, error) {
		f := files[name]
		if f == nil {
			return nil, nil
		}
		delete(files, name)
		rows, err := readCSV(f)
		if err != nil {
			return nil, fmt.Errorf("%s.csv: %w", name, err)
		}
		return rows, nil
	}

	// the profile first: it says whether weights are in pounds
	rows, err := table("profile")
	if err != nil {
		return nil, err
	}
	imperial := false
	if len(rows) > 0 {
		values := map[string]string{}
		for _, r := range rows {
			values[r["Name"]] = r["Value"]
		}
		// Lose It! writes the height in inches for people who use pounds
		if h := number(values["Height"]); h > 0 && h < 100 {
			imperial = true
		}
		x.Profile = readProfile(values, imperial)
	}
	kg := func(v float64) float64 {
		if imperial {
			v *= kgPerLb
		}
		return math.Round(v*100) / 100
	}

	if rows, err = table("food-logs"); err != nil {
		return nil, err
	}
	seen := map[string]int{}
	for _, r := range rows {
		date, meal := day(r["Date"]), meals[strings.ToLower(r["Meal"])]
		if deleted(r["Deleted"]) || date == "" || r["Name"] == "" {
			continue
		}
		if meal == "" {
			meal = "snacks"
		}
		qty, units := number(r["Quantity"]), strings.TrimSpace(r["Units"])
		f := FoodLog{
			Date:        date,
			Meal:        meal,
			Name:        clip(r["Name"], 200),
			Amount:      clip(strings.TrimSpace(trim(qty)+" "+units), 50),
			Kcal:        math.Max(0, number(r["Calories"])),
			Protein:     grams(r["Protein (g)"]),
			Carbs:       grams(r["Carbohydrates (g)"]),
			Fat:         grams(r["Fat (g)"]),
			SatFat:      grams(r["Saturated Fat (g)"]),
			Sugar:       grams(r["Sugars (g)"]),
			Fiber:       grams(r["Fiber (g)"]),
			Salt:        round(number(r["Sodium (mg)"])*2.5/1000, 2),
			Cholesterol: round(number(r["Cholesterol (mg)"]), 0),
		}
		if g, ok := massGrams[unit(units)]; ok {
			f.Grams = round(qty*g, 1)
		}
		f.Key = key(seen, "food", r["Date"], r["Meal"], r["Name"], r["Quantity"], r["Units"], r["Calories"])
		x.Foods = append(x.Foods, f)
	}

	if rows, err = table("weights"); err != nil {
		return nil, err
	}
	for _, r := range rows {
		if w := number(r["Weight"]); !deleted(r["Deleted"]) && day(r["Date"]) != "" && w > 0 {
			x.Weights = append(x.Weights, Weight{Date: day(r["Date"]), Kg: kg(w)})
		}
	}

	if rows, err = table("exercise-logs"); err != nil {
		return nil, err
	}
	for _, r := range rows {
		date := day(r["Date"])
		if deleted(r["Deleted"]) || date == "" {
			continue
		}
		e := Exercise{Date: date, Kcal: int(math.Round(math.Abs(number(r["Calories"]))))}
		e.Kind, e.Name = exerciseKind(r["Name"])
		qty := number(r["Quantity"])
		switch unit(r["Units"]) {
		case "minute", "min":
			e.Minutes = int(math.Round(qty))
		case "hour":
			e.Minutes = int(math.Round(qty * 60))
		}
		e.Minutes = min(e.Minutes, 1440)
		if e.Minutes == 0 && e.Kcal == 0 {
			continue
		}
		e.Key = key(seen, "exercise", r["Date"], r["Name"], r["Quantity"], r["Units"], r["Calories"])
		x.Exercises = append(x.Exercises, e)
	}

	// the measures each food was last logged in, which say what a serving weighs
	if rows, err = table("active-food-servings"); err != nil {
		return nil, err
	}
	servings := map[string][]map[string]string{}
	for _, r := range rows {
		if !deleted(r["Deleted"]) {
			servings[r["FoodUniqueId"]] = append(servings[r["FoodUniqueId"]], r)
		}
	}
	for _, name := range []string{"custom-foods", "recipes"} {
		if rows, err = table(name); err != nil {
			return nil, err
		}
		for _, r := range rows {
			if deleted(r["Deleted"]) {
				continue
			}
			if f, ok := customFood(r, servings[r["UniqueId"]]); ok {
				x.CustomFoods = append(x.CustomFoods, f)
			} else if r["Name"] != "" {
				x.Unweighed++
			}
		}
	}

	if rows, err = table("food-photos"); err != nil {
		return nil, err
	}
	for _, r := range rows {
		f := photos[strings.ToLower(r["Token"])]
		if deleted(r["Deleted"]) || f == nil || day(r["Date"]) == "" {
			continue
		}
		meal := meals[strings.ToLower(r["Meal Type"])]
		if meal == "" {
			meal = "snacks"
		}
		x.Photos = append(x.Photos, Photo{Date: day(r["Date"]), Meal: meal, File: f})
	}

	if rows, err = table("fasting-schedules"); err != nil {
		return nil, err
	}
	x.Fasting = readFasting(rows)

	// What's left is either worked out from the rest (the daily totals) or
	// has no place in Troye. Say which of those have data.
	for _, name := range derived {
		delete(files, name)
	}
	var unused []string
	for name, f := range files {
		rows, err := readCSV(f)
		if err == nil && len(rows) > 0 {
			unused = append(unused, fmt.Sprintf("%s (%d)", name, len(rows)))
		}
	}
	slices.Sort(unused)
	x.Unused = unused
	return x, nil
}

// derived are files of daily totals, which Troye works out from the food log.
var derived = []string{
	"daily-calorie-summary", "carbohydrates(g)", "protein(g)", "fiber", "sugar",
	"saturated-fats(g)", "fat(g)", "sodium(mg)", "cholesterol(mg)", "daily-values",
}

// photoName is how a photo is named in food-photos/: Lunch(2025-08-19)-<token>-photo.jpg
var photoName = regexp.MustCompile(`(?i)-([0-9a-f]{32})-photo\.(?:jpe?g|png|webp)$`)

var meals = map[string]string{
	"breakfast":        "breakfast",
	"lunch":            "lunch",
	"dinner":           "dinner",
	"snacks":           "snacks",
	"snack":            "snacks",
	"morning snacks":   "snacks",
	"afternoon snacks": "snacks",
	"evening snacks":   "snacks",
}

// massGrams are the grams in each unit of weight; a millilitre counts as a gram.
var massGrams = map[string]float64{
	"gram": 1, "g": 1, "kilogram": 1000, "kg": 1000, "milligram": 0.001,
	"ounce": 28.3495, "oz": 28.3495, "pound": 453.592, "lb": 453.592,
	"milliliter": 1, "millilitre": 1, "ml": 1,
}

func readCSV(f *zip.File) ([]map[string]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	cr := csv.NewReader(rc)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	var rows []map[string]string
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		row := make(map[string]string, len(header))
		for i, h := range header {
			if i < len(rec) {
				row[strings.TrimSpace(h)] = strings.TrimSpace(rec[i])
			}
		}
		rows = append(rows, row)
	}
}

// day turns Lose It!'s MM/DD/YYYY into YYYY-MM-DD, or "" when it isn't a date.
func day(s string) string {
	for _, layout := range []string{"01/02/2006", "1/2/2006", time.DateOnly} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(time.DateOnly)
		}
	}
	return ""
}

func deleted(s string) bool {
	return s == "1" || strings.EqualFold(s, "true")
}

// number reads "1,000.5"; "n/a" and other text is 0.
func number(s string) float64 {
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", ""), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func grams(s string) float64 {
	return round(math.Max(0, number(s)), 1)
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

// trim writes a quantity without trailing zeros: 2, 0.5, 1.25.
func trim(v float64) string {
	return strconv.FormatFloat(round(v, 2), 'f', -1, 64)
}

// unit is a measure's name in the singular, lower case: "Tablespoons" is "tablespoon".
func unit(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) > 3 && strings.HasSuffix(s, "s") {
		s = strings.TrimSuffix(s, "s")
	}
	return s
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// key identifies a row by its values and how many identical rows came before
// it, since the same food can be logged twice in a meal.
func key(seen map[string]int, parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	base := hex.EncodeToString(h[:12])
	seen[base]++
	return fmt.Sprintf("loseit:%s:%d", base, seen[base])
}

// exerciseKind picks the closest of Troye's kinds for a Lose It! exercise,
// and the name to keep with it.
func exerciseKind(name string) (string, string) {
	n := strings.ToLower(name)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(n, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("garmin", "fitbit", "apple watch", "apple health", "google fit", "samsung", "adjustment", "steps"):
		return "tracker", clip(name, 100)
	case has("dog"):
		return "dog_walk", ""
	case has("hik"):
		return "hike", ""
	case has("walk"):
		return "walk", ""
	case has("run", "jog"):
		return "run", ""
	case has("cycl", "bik", "spinning"):
		return "cycle", ""
	case has("swim"):
		return "swim", ""
	case has("yoga", "pilates", "stretch"):
		return "yoga", ""
	case has("kettlebell", "weight", "strength", "lifting", "crossfit", "circuit", "push", "squat"):
		return "strength", clip(name, 100)
	case has("tennis", "football", "soccer", "basketball", "volleyball", "badminton", "squash", "padel", "hockey", "golf"):
		return "sport", clip(name, 100)
	}
	return "other", clip(name, 100)
}

// customFood turns a food or recipe, given per serving, into one per 100 g.
// That needs the serving's weight: from its measure, or from another measure
// the food was logged in. Without it, the food can't be used in Troye.
func customFood(r map[string]string, servings []map[string]string) (CustomFood, bool) {
	qty, measure := number(r["Quantity"]), unit(r["Measure"])
	if r["Name"] == "" || qty <= 0 {
		return CustomFood{}, false
	}
	weight := 0.0
	if g, ok := massGrams[measure]; ok {
		weight = qty * g
	} else {
		// servings logged at the same moment are the same amount in different measures
		byTime := map[string]map[string]float64{}
		for _, s := range servings {
			t := s["Created"]
			if byTime[t] == nil {
				byTime[t] = map[string]float64{}
			}
			byTime[t][unit(s["Measure"])] = number(s["Quantity"])
		}
		for _, m := range byTime {
			// a weight before a volume, which is only about right
			for _, u := range []string{"gram", "kilogram", "ounce", "pound", "milliliter"} {
				if m[u] > 0 && m[measure] > 0 {
					weight = qty / m[measure] * m[u] * massGrams[u]
					break
				}
			}
			if weight > 0 {
				break
			}
		}
	}
	if weight <= 0 {
		return CustomFood{}, false
	}
	per100 := func(field string) float64 {
		return round(math.Max(0, number(r[field]))*100/weight, 1)
	}
	f := CustomFood{
		ID:           r["UniqueId"],
		Name:         clip(r["Name"], 200),
		Brand:        clip(r["Brand"], 200),
		Kcal:         round(math.Max(0, number(r["Calories"]))*100/weight, 0),
		Protein:      per100("Protein (g)"),
		Carbs:        per100("Carbohydrates (g)"),
		Fat:          per100("Fat (g)"),
		Fiber:        per100("Fiber (g)"),
		Sugar:        per100("Sugars (g)"),
		Salt:         round(math.Max(0, number(r["Sodium (mg)"]))*2.5/1000*100/weight, 2),
		ServingG:     round(weight, 1),
		ServingLabel: clip(strings.TrimSpace(trim(qty)+" "+strings.TrimSpace(r["Measure"])), 50),
	}
	if f.Kcal > 1000 || f.ServingG > 5000 {
		return CustomFood{}, false
	}
	return f, true
}

var activities = map[string]string{
	"sedentary":       "sedentary",
	"somewhat active": "light",
	"lightly active":  "light",
	"active":          "moderate",
	"very active":     "active",
}

// pace reads a plan such as "3/4 kg per week" or "Lose 1 lb per week".
var pace = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)(?:\s*/\s*(\d+))?\s*(kg|kilo|lb|pound)`)

func readProfile(v map[string]string, imperial bool) *Profile {
	kg := func(s string) float64 {
		w := number(s)
		if imperial {
			w *= kgPerLb
		}
		return round(w, 1)
	}
	p := &Profile{
		BirthDate:    day(v["Birthday"]),
		Sex:          strings.ToLower(v["Gender"]),
		HeightCm:     number(v["Height"]),
		WeightKg:     kg(v["Current Weight"]),
		GoalWeightKg: kg(v["Goal Weight"]),
		Activity:     activities[strings.ToLower(v["Activity Level"])],
	}
	if p.Sex != "male" && p.Sex != "female" {
		p.Sex = ""
	}
	if imperial {
		p.HeightCm *= 2.54
	}
	p.HeightCm = round(p.HeightCm, 0)

	plan := strings.ToLower(v["Plan"])
	if m := pace.FindStringSubmatch(plan); m != nil {
		n := number(m[1])
		if m[2] != "" {
			n /= number(m[2])
		}
		if strings.HasPrefix(m[3], "lb") || strings.HasPrefix(m[3], "pound") {
			n *= kgPerLb
		}
		// Troye's paces go in quarter kilos up to 1 kg a week
		p.PaceKg = math.Min(1, math.Max(0.25, math.Round(n*4)/4))
	}
	switch {
	case strings.Contains(plan, "maintain") || p.PaceKg == 0:
		p.Goal, p.PaceKg = "maintain", 0
	case strings.Contains(plan, "gain") || p.GoalWeightKg > p.WeightKg:
		p.Goal = "gain"
	default:
		p.Goal = "lose"
	}
	return p
}

var weekdays = map[string]string{
	"monday": "mon", "tuesday": "tue", "wednesday": "wed", "thursday": "thu",
	"friday": "fri", "saturday": "sat", "sunday": "sun",
}

var weekOrder = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// fastingPlans are Troye's fasting plans by the hours of fasting.
var fastingPlans = []struct {
	hours int
	id    string
}{{12, "12_12"}, {14, "14_10"}, {16, "16_8"}, {18, "18_6"}, {20, "20_4"}, {23, "omad"}}

// readFasting turns Lose It!'s fasts, one row per weekday with when it starts
// and its minutes, into one window on some days. When the days differ, the
// most common start and length win.
func readFasting(rows []map[string]string) *Fasting {
	type window struct {
		start   string
		minutes int
	}
	count := map[window]int{}
	days := map[window][]string{}
	for _, r := range rows {
		d, ok := weekdays[strings.ToLower(r["Day"])]
		t, err := time.Parse("15:04:05", r["Scheduled start"])
		if deleted(r["Deleted"]) || !ok || err != nil {
			continue
		}
		w := window{t.Format("15:04"), int(number(r["Duration"]))}
		if w.minutes <= 0 {
			continue
		}
		count[w]++
		days[w] = append(days[w], d)
	}
	var best window
	for w, n := range count {
		if n > count[best] || n == count[best] && w.start < best.start {
			best = w
		}
	}
	if count[best] == 0 {
		return nil
	}
	f := &Fasting{Plan: fastingPlans[0].id}
	hours := float64(best.minutes) / 60
	for _, p := range fastingPlans {
		if math.Abs(float64(p.hours)-hours) < math.Abs(float64(planHours(f.Plan))-hours) {
			f.Plan = p.id
		}
	}
	start, _ := time.Parse("15:04", best.start)
	f.EatingStart = start.Add(time.Duration(best.minutes) * time.Minute).Format("15:04")
	chosen := map[string]bool{}
	for _, d := range days[best] {
		chosen[d] = true
	}
	for _, d := range weekOrder {
		if chosen[d] {
			f.Days = append(f.Days, d)
		}
	}
	return f
}

func planHours(id string) int {
	for _, p := range fastingPlans {
		if p.id == id {
			return p.hours
		}
	}
	return 0
}
