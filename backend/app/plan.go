package app

import (
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// A person's daily targets, worked out from their answers to the onboarding
// interview. The web app previews them with POST /api/app/plan while the
// interview runs, and a hook stores them on the profile whenever it is saved,
// so a new weigh-in also moves the budget.

// PlanInput holds the answers the plan depends on.
type PlanInput struct {
	Sex          string  `json:"sex"`
	BirthDate    string  `json:"birth_date"`
	HeightCm     float64 `json:"height_cm"`
	WeightKg     float64 `json:"weight_kg"`
	Goal         string  `json:"goal"`
	GoalWeightKg float64 `json:"goal_weight_kg"`
	Activity     string  `json:"activity"`
	PaceKg       float64 `json:"pace_kg"`
	Diet         string  `json:"diet"`
}

// Plan is what the answers work out to.
type Plan struct {
	Age  int     `json:"age"`
	BMI  float64 `json:"bmi"`
	BMR  int     `json:"bmr"`
	TDEE int     `json:"tdee"`
	// the daily calorie budget, rounded to 10 kcal
	Budget int `json:"budget"`
	// true when the budget was raised to the safe minimum, which also slows the pace
	Floored  bool    `json:"floored"`
	PaceKg   float64 `json:"pace_kg"`
	ProteinG int     `json:"protein_g"`
	CarbsG   int     `json:"carbs_g"`
	FatG     int     `json:"fat_g"`
	// when the goal weight is reached at this pace, YYYY-MM-DD; empty when maintaining
	GoalDate string `json:"goal_date"`
}

// kcalPerKg is the energy in a kilogram of body weight, the usual rule of thumb.
const kcalPerKg = 7700

// activityFactors multiply the resting energy (BMR) into a day's total.
var activityFactors = map[string]float64{
	"sedentary":   1.2,
	"light":       1.375,
	"moderate":    1.55,
	"active":      1.725,
	"very_active": 1.9,
}

// macroSplits are the shares of calories from carbs, protein and fat.
var macroSplits = map[string][3]float64{
	"balanced":     {0.50, 0.20, 0.30},
	"high_protein": {0.40, 0.30, 0.30},
	"low_carb":     {0.25, 0.35, 0.40},
}

// minBudget keeps a weight-loss budget from going dangerously low.
var minBudget = map[string]int{"female": 1200, "male": 1500}

// ComputePlan works out the daily targets for someone on the given day.
// The error is a sentence to show them.
func ComputePlan(in PlanInput, today time.Time) (Plan, error) {
	var p Plan
	if in.Sex != "female" && in.Sex != "male" {
		return p, errors.New("Choose the sex to use for the calculation.")
	}
	born, err := time.ParseInLocation(time.DateOnly, in.BirthDate, today.Location())
	if err != nil {
		return p, errors.New("Enter your date of birth.")
	}
	p.Age = age(born, today)
	if p.Age < 18 || p.Age > 110 {
		return p, errors.New("Troye is made for adults, 18 and over.")
	}
	if in.HeightCm < 100 || in.HeightCm > 250 {
		return p, errors.New("Enter a height between 100 and 250 cm.")
	}
	if in.WeightKg < 30 || in.WeightKg > 350 {
		return p, errors.New("Enter a weight between 30 and 350 kg.")
	}
	factor, ok := activityFactors[in.Activity]
	if !ok {
		return p, errors.New("Choose how active you are.")
	}
	split, ok := macroSplits[in.Diet]
	if !ok {
		split = macroSplits["balanced"]
	}

	switch in.Goal {
	case "maintain":
		in.PaceKg = 0
	case "lose", "gain":
		if in.Goal == "lose" && in.GoalWeightKg >= in.WeightKg {
			return p, errors.New("To lose weight, the goal weight has to be below your current weight.")
		}
		if in.Goal == "gain" && in.GoalWeightKg <= in.WeightKg {
			return p, errors.New("To gain weight, the goal weight has to be above your current weight.")
		}
		if in.PaceKg <= 0 || in.PaceKg > 1 {
			return p, errors.New("Choose a pace of up to 1 kg a week.")
		}
	default:
		return p, errors.New("Choose your goal.")
	}

	h := in.HeightCm / 100
	p.BMI = math.Round(in.WeightKg/(h*h)*10) / 10

	// Mifflin-St Jeor
	bmr := 10*in.WeightKg + 6.25*in.HeightCm - 5*float64(p.Age)
	if in.Sex == "male" {
		bmr += 5
	} else {
		bmr -= 161
	}
	tdee := bmr * factor
	p.BMR, p.TDEE = int(math.Round(bmr)), int(math.Round(tdee))

	daily := in.PaceKg * kcalPerKg / 7
	budget := tdee
	switch in.Goal {
	case "lose":
		budget -= daily
		if floor := float64(minBudget[in.Sex]); budget < floor {
			budget, p.Floored = floor, true
		}
	case "gain":
		budget += daily
	}
	p.Budget = int(math.Round(budget/10) * 10)

	// the pace this budget really gives, which is slower when it was floored
	p.PaceKg = math.Round(math.Abs(tdee-float64(p.Budget))*7/kcalPerKg*100) / 100
	if in.Goal != "maintain" && p.PaceKg > 0 {
		weeks := math.Abs(in.WeightKg-in.GoalWeightKg) / p.PaceKg
		p.GoalDate = today.AddDate(0, 0, int(math.Ceil(weeks*7))).Format(time.DateOnly)
	}

	b := float64(p.Budget)
	p.CarbsG = int(math.Round(b * split[0] / 4))
	p.ProteinG = int(math.Round(b * split[1] / 4))
	p.FatG = int(math.Round(b * split[2] / 9))
	return p, nil
}

// age is the number of whole years from born to today.
func age(born, today time.Time) int {
	years := today.Year() - born.Year()
	if today.Month() < born.Month() || today.Month() == born.Month() && today.Day() < born.Day() {
		years--
	}
	return years
}

func planInput(r *core.Record) PlanInput {
	return PlanInput{
		Sex:          r.GetString("sex"),
		BirthDate:    r.GetString("birth_date"),
		HeightCm:     r.GetFloat("height_cm"),
		WeightKg:     r.GetFloat("weight_kg"),
		Goal:         r.GetString("goal"),
		GoalWeightKg: r.GetFloat("goal_weight_kg"),
		Activity:     r.GetString("activity"),
		PaceKg:       r.GetFloat("pace_kg"),
		Diet:         r.GetString("diet"),
	}
}

// GoalReached tells whether the weight has got to the goal weight.
func GoalReached(in PlanInput) bool {
	return in.Goal == "lose" && in.WeightKg <= in.GoalWeightKg ||
		in.Goal == "gain" && in.WeightKg >= in.GoalWeightKg
}

// storePlan sets a profile's targets from its answers, or clears them while
// the answers are incomplete. Once a weigh-in reaches the goal, the budget
// becomes the one for staying there, until a new goal is set.
func storePlan(r *core.Record) {
	in := planInput(r)
	if in.WeightKg > 0 && GoalReached(in) {
		in.Goal = "maintain"
	}
	p, err := ComputePlan(in, time.Now())
	if err != nil {
		p = Plan{}
	}
	r.Set("calorie_budget", p.Budget)
	r.Set("protein_g", p.ProteinG)
	r.Set("carbs_g", p.CarbsG)
	r.Set("fat_g", p.FatG)
	r.Set("goal_date", p.GoalDate)
}

func registerPlan(app core.App) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/app/plan", func(e *core.RequestEvent) error {
			var in PlanInput
			if err := e.BindBody(&in); err != nil {
				return e.BadRequestError("Could not read your answers.", err)
			}
			p, err := ComputePlan(in, time.Now())
			if err != nil {
				return e.BadRequestError(err.Error(), nil)
			}
			return e.JSON(http.StatusOK, p)
		}).Bind(apis.RequireAuth("users"))
		return se.Next()
	})

	app.OnRecordCreate("profiles").BindFunc(func(e *core.RecordEvent) error {
		storePlan(e.Record)
		return e.Next()
	})
	app.OnRecordUpdate("profiles").BindFunc(func(e *core.RecordEvent) error {
		storePlan(e.Record)
		return e.Next()
	})
}
