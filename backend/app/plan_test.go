package app_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/selfhostit/troye/app"
)

var planDay = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func planFor(goal string) app.PlanInput {
	return app.PlanInput{
		Sex: "female", BirthDate: "1990-06-15", HeightCm: 168, WeightKg: 80,
		Goal: goal, GoalWeightKg: 70, Activity: "light", PaceKg: 0.5, Diet: "balanced",
	}
}

func TestComputePlan(t *testing.T) {
	// 36 years old: BMR 10*80 + 6.25*168 - 5*36 - 161 = 1509, TDEE 1509 * 1.375 = 2074.875
	p, err := app.ComputePlan(planFor("lose"), planDay)
	if err != nil {
		t.Fatal(err)
	}
	if p.Age != 36 || p.BMR != 1509 || p.TDEE != 2075 {
		t.Errorf("age %d, BMR %d, TDEE %d", p.Age, p.BMR, p.TDEE)
	}
	// 0.5 kg a week is 550 kcal a day
	if p.Budget != 1520 || p.Floored {
		t.Errorf("budget %d floored %v", p.Budget, p.Floored)
	}
	if p.BMI != 28.3 {
		t.Errorf("BMI %v", p.BMI)
	}
	// 10 kg at about 0.5 kg a week is about 20 weeks
	if p.GoalDate < "2027-03-10" || p.GoalDate > "2027-03-16" {
		t.Errorf("goal date %s", p.GoalDate)
	}
	// 50% carbs, 20% protein, 30% fat
	if p.CarbsG != 190 || p.ProteinG != 76 || p.FatG != 51 {
		t.Errorf("macros %d/%d/%d", p.CarbsG, p.ProteinG, p.FatG)
	}
}

func TestComputePlanFloorsTheBudget(t *testing.T) {
	in := planFor("lose")
	in.Activity, in.PaceKg = "sedentary", 1
	p, err := app.ComputePlan(in, planDay)
	if err != nil {
		t.Fatal(err)
	}
	if p.Budget != 1200 || !p.Floored {
		t.Errorf("budget %d floored %v", p.Budget, p.Floored)
	}
	// TDEE 1811 - 1200 is about 0.56 kg a week, not the 1 kg asked for
	if p.PaceKg < 0.5 || p.PaceKg > 0.6 {
		t.Errorf("pace %v", p.PaceKg)
	}
}

func TestComputePlanMaintainAndGain(t *testing.T) {
	p, err := app.ComputePlan(planFor("maintain"), planDay)
	if err != nil {
		t.Fatal(err)
	}
	if p.Budget != 2070 || p.GoalDate != "" || p.PaceKg != 0 {
		t.Errorf("maintain: budget %d goal date %q pace %v", p.Budget, p.GoalDate, p.PaceKg)
	}

	in := planFor("gain")
	in.GoalWeightKg, in.PaceKg = 85, 0.25
	p, err = app.ComputePlan(in, planDay)
	if err != nil {
		t.Fatal(err)
	}
	if p.Budget != 2350 {
		t.Errorf("gain: budget %d", p.Budget)
	}
}

func TestComputePlanRefusesBadAnswers(t *testing.T) {
	cases := map[string]func(*app.PlanInput){
		"adults":        func(in *app.PlanInput) { in.BirthDate = "2010-01-01" },
		"goal weight":   func(in *app.PlanInput) { in.GoalWeightKg = 90 },
		"pace":          func(in *app.PlanInput) { in.PaceKg = 2 },
		"active":        func(in *app.PlanInput) { in.Activity = "" },
		"height":        func(in *app.PlanInput) { in.HeightCm = 20 },
		"date of birth": func(in *app.PlanInput) { in.BirthDate = "" },
	}
	for want, change := range cases {
		in := planFor("lose")
		change(&in)
		if _, err := app.ComputePlan(in, planDay); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v", want, err)
		}
	}
}

func TestPlanRoute(t *testing.T) {
	body := `{"sex":"male","birth_date":"1985-01-01","height_cm":180,"weight_kg":95,"goal":"lose","goal_weight_kg":85,"activity":"moderate","pace_kg":0.5,"diet":"high_protein"}`
	scenarios := []tests.ApiScenario{
		{
			Name:            "signed-out visitors can't use it",
			Method:          http.MethodPost,
			URL:             "/api/app/plan",
			Body:            strings.NewReader(body),
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"data":{}`},
			TestAppFactory:  newApp,
		},
		{
			Name:            "it previews the plan",
			Method:          http.MethodPost,
			URL:             "/api/app/plan",
			Body:            strings.NewReader(body),
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"budget":`, `"protein_g":`, `"goal_date":"20`},
		},
		{
			Name:            "it explains a bad answer",
			Method:          http.MethodPost,
			URL:             "/api/app/plan",
			Body:            strings.NewReader(strings.Replace(body, `"goal_weight_kg":85`, `"goal_weight_kg":99`, 1)),
			ExpectedStatus:  http.StatusBadRequest,
			ExpectedContent: []string{"below your current weight"},
		},
	}
	for _, s := range scenarios {
		if s.TestAppFactory == nil {
			a := newApp(t)
			_, token := addUser(t, a, "ada@example.com", "owner")
			s.Headers = map[string]string{"Authorization": token}
			s.TestAppFactory = func(testing.TB) *tests.TestApp { return a }
		}
		s.Test(t)
	}
}

func TestProfiles(t *testing.T) {
	profile := func(user string) string {
		return `{"user":"` + user + `","goal":"lose","sex":"female","birth_date":"1990-06-15","height_cm":168,"weight_kg":80,"goal_weight_kg":70,"activity":"light","pace_kg":0.5,"diet":"balanced","calorie_budget":5000,"onboarded":true}`
	}
	scenarios := []struct {
		name, as, method, url string
		status                int
		content, missing      string
	}{
		{"someone else can't create your profile", "bob", http.MethodPost, "/api/collections/profiles/records", http.StatusBadRequest, "Failed to create", ""},
		// whatever budget the client sent is replaced (it is about 1,500 kcal)
		{"the server works out the budget", "ada", http.MethodPost, "/api/collections/profiles/records", http.StatusOK, `"calorie_budget":1`, `"calorie_budget":5000`},
		{"others can't see it", "bob", http.MethodGet, "/api/collections/profiles/records?seed=1", http.StatusOK, `"totalItems":0`, ""},
		{"you see your own", "ada", http.MethodGet, "/api/collections/profiles/records?seed=1", http.StatusOK, `"totalItems":1`, ""},
	}
	for _, sc := range scenarios {
		a := newApp(t)
		ada, adaToken := addUser(t, a, "ada@example.com", "owner")
		_, bobToken := addUser(t, a, "bob@example.com", "member")
		if strings.Contains(sc.url, "seed") {
			saveProfile(t, a, ada.Id)
		}
		token := adaToken
		if sc.as == "bob" {
			token = bobToken
		}
		s := tests.ApiScenario{
			Name:            sc.name,
			Method:          sc.method,
			URL:             sc.url,
			Body:            strings.NewReader(profile(ada.Id)),
			Headers:         map[string]string{"Authorization": token},
			ExpectedStatus:  sc.status,
			ExpectedContent: []string{sc.content},
			TestAppFactory:  func(testing.TB) *tests.TestApp { return a },
		}
		if sc.missing != "" {
			s.NotExpectedContent = []string{sc.missing}
		}
		s.Test(t)
	}
}

func saveProfile(t testing.TB, a core.App, user string) *core.Record {
	profiles, err := a.FindCollectionByNameOrId("profiles")
	if err != nil {
		t.Fatal(err)
	}
	r := core.NewRecord(profiles)
	in := planFor("lose")
	r.Load(map[string]any{
		"user": user, "goal": in.Goal, "sex": in.Sex, "birth_date": in.BirthDate,
		"height_cm": in.HeightCm, "weight_kg": in.WeightKg, "goal_weight_kg": in.GoalWeightKg,
		"activity": in.Activity, "pace_kg": in.PaceKg, "diet": in.Diet,
	})
	if err := a.Save(r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestProfileBudgetFollowsTheAnswers(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, _ := addUser(t, a, "ada@example.com", "owner")
	r := saveProfile(t, a, ada.Id)
	before := r.GetInt("calorie_budget")
	if before == 0 {
		t.Fatal("no budget")
	}

	// a new weight moves the budget
	r.Set("weight_kg", 75)
	if err := a.Save(r); err != nil {
		t.Fatal(err)
	}
	if after := r.GetInt("calorie_budget"); after >= before {
		t.Errorf("budget %d after losing weight, %d before", after, before)
	}

	// incomplete answers clear the targets
	r.Set("activity", "")
	if err := a.Save(r); err != nil {
		t.Fatal(err)
	}
	if r.GetInt("calorie_budget") != 0 || r.GetString("goal_date") != "" {
		t.Errorf("budget %d with incomplete answers", r.GetInt("calorie_budget"))
	}
}
