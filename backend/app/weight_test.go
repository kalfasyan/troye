package app_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/kalfasyan/troye/app"
)

// addProfile saves an onboarded profile losing from 80 to 70 kg.
func addProfile(t testing.TB, a core.App, user string) *core.Record {
	profiles, err := a.FindCollectionByNameOrId("profiles")
	if err != nil {
		t.Fatal(err)
	}
	p := core.NewRecord(profiles)
	p.Load(map[string]any{
		"user": user, "goal": "lose", "sex": "female", "birth_date": "1990-06-15", "height_cm": 168,
		"weight_kg": 80, "goal_weight_kg": 70, "activity": "light", "pace_kg": 0.5, "diet": "balanced", "onboarded": true,
	})
	if err := a.Save(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func weigh(t testing.TB, a core.App, user, day string, kg float64) *core.Record {
	weighins, _ := a.FindCollectionByNameOrId("weighins")
	w, err := a.FindFirstRecordByData("weighins", "date", day)
	if err != nil || w.GetString("user") != user {
		w = core.NewRecord(weighins)
		w.Load(map[string]any{"user": user, "date": day})
	}
	w.Set("weight_kg", kg)
	if err := a.Save(w); err != nil {
		t.Fatal(err)
	}
	return w
}

func weighins(t testing.TB, a core.App, user string) []*core.Record {
	list, err := a.FindRecordsByFilter("weighins", "user = '"+user+"'", "date", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func reload(t testing.TB, a core.App, r *core.Record) *core.Record {
	r, err := a.FindRecordById(r.Collection().Name, r.Id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestWeighinsMoveThePlan(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, _ := addUser(t, a, "ada@example.com", "owner")
	today := time.Now().Format(time.DateOnly)

	// onboarding logs the first weigh-in
	p := addProfile(t, a, ada.Id)
	list := weighins(t, a, ada.Id)
	if len(list) != 1 || list[0].GetString("date") != today || list[0].GetFloat("weight_kg") != 80 {
		t.Fatalf("weigh-ins after onboarding: %v", list)
	}
	budget := p.GetInt("calorie_budget")

	// a lighter weigh-in becomes the profile's weight and lowers the budget
	tomorrow := time.Now().AddDate(0, 0, 1).Format(time.DateOnly)
	w := weigh(t, a, ada.Id, tomorrow, 78)
	p = reload(t, a, p)
	if p.GetFloat("weight_kg") != 78 || p.GetInt("calorie_budget") >= budget {
		t.Errorf("profile weight %v, budget %d (was %d)", p.GetFloat("weight_kg"), p.GetInt("calorie_budget"), budget)
	}

	// an older weigh-in doesn't change the current weight
	weigh(t, a, ada.Id, "2026-01-01", 85)
	if p = reload(t, a, p); p.GetFloat("weight_kg") != 78 {
		t.Errorf("an old weigh-in moved the weight to %v", p.GetFloat("weight_kg"))
	}

	// removing the latest goes back to the one before, without logging it again
	if err := a.Delete(w); err != nil {
		t.Fatal(err)
	}
	if p = reload(t, a, p); p.GetFloat("weight_kg") != 80 {
		t.Errorf("weight %v after removing the latest", p.GetFloat("weight_kg"))
	}
	if n := len(weighins(t, a, ada.Id)); n != 2 {
		t.Errorf("%d weigh-ins, want 2", n)
	}

	// a new weight typed into the plan is today's weigh-in
	p.Set("weight_kg", 79.4)
	if err := a.Save(p); err != nil {
		t.Fatal(err)
	}
	list = weighins(t, a, ada.Id)
	if last := list[len(list)-1]; len(list) != 2 || last.GetString("date") != today || last.GetFloat("weight_kg") != 79.4 {
		t.Errorf("weigh-ins after changing the plan: %v", list)
	}
}

func TestReachingTheGoal(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, _ := addUser(t, a, "ada@example.com", "owner")
	p := addProfile(t, a, ada.Id)
	weigh(t, a, ada.Id, time.Now().AddDate(0, 0, 1).Format(time.DateOnly), 69.8)

	// the budget switches to staying at the new weight instead of going to zero
	p = reload(t, a, p)
	want, _ := app.ComputePlan(app.PlanInput{
		Sex: "female", BirthDate: "1990-06-15", HeightCm: 168, WeightKg: 69.8,
		Goal: "maintain", Activity: "light", Diet: "balanced",
	}, time.Now())
	if p.GetInt("calorie_budget") != want.Budget || p.GetString("goal_date") != "" {
		t.Errorf("budget %d, goal date %q; want %d and none", p.GetInt("calorie_budget"), p.GetString("goal_date"), want.Budget)
	}
}

func TestWeighinRules(t *testing.T) {
	scenarios := []struct {
		name, as, method, url, body string
		status                      int
		content                     string
	}{
		{"you log your weight", "ada", http.MethodPost, "/api/collections/weighins/records", `{"user":"{ada}","date":"2026-10-20","weight_kg":79.5}`, http.StatusOK, `"weight_kg":79.5`},
		{"once a day", "ada", http.MethodPost, "/api/collections/weighins/records", `{"user":"{ada}","date":"{today}","weight_kg":79.5}`, http.StatusBadRequest, ""},
		{"not for someone else", "bob", http.MethodPost, "/api/collections/weighins/records", `{"user":"{ada}","date":"2026-10-20","weight_kg":79.5}`, http.StatusBadRequest, ""},
		{"others can't see your weight", "bob", http.MethodGet, "/api/collections/weighins/records", "", http.StatusOK, `"totalItems":0`},
		{"you see yours", "ada", http.MethodGet, "/api/collections/weighins/records", "", http.StatusOK, `"totalItems":1`},
	}
	for _, sc := range scenarios {
		a := newApp(t)
		ada, adaToken := addUser(t, a, "ada@example.com", "owner")
		_, bobToken := addUser(t, a, "bob@example.com", "member")
		addProfile(t, a, ada.Id)
		token := adaToken
		if sc.as == "bob" {
			token = bobToken
		}
		replace := strings.NewReplacer("{ada}", ada.Id, "{today}", time.Now().Format(time.DateOnly))
		s := tests.ApiScenario{
			Name:           sc.name,
			Method:         sc.method,
			URL:            sc.url,
			Body:           strings.NewReader(replace.Replace(sc.body)),
			Headers:        map[string]string{"Authorization": token},
			ExpectedStatus: sc.status,
			TestAppFactory: func(testing.TB) *tests.TestApp { return a },
		}
		if sc.content != "" {
			s.ExpectedContent = []string{sc.content}
		} else {
			s.NotExpectedContent = []string{`"id":`}
		}
		s.Test(t)
	}
}
