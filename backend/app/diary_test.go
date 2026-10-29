package app_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// addFood saves a food with the given kcal and protein per 100 g; owner may be empty.
func addFood(t testing.TB, a core.App, name, owner string, kcal, protein float64) *core.Record {
	foods, err := a.FindCollectionByNameOrId("foods")
	if err != nil {
		t.Fatal(err)
	}
	f := core.NewRecord(foods)
	source := "usda"
	if owner != "" {
		source = "custom"
	}
	f.Load(map[string]any{"name": name, "source": source, "owner": owner, "kcal": kcal, "protein": protein})
	if err := a.Save(f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestEntryTotals(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, _ := addUser(t, a, "ada@example.com", "owner")
	bob, _ := addUser(t, a, "bob@example.com", "member")
	rice := addFood(t, a, "Rice, cooked", "", 130, 2.7)

	entries, _ := a.FindCollectionByNameOrId("entries")
	e := core.NewRecord(entries)
	e.Load(map[string]any{"user": ada.Id, "date": "2026-10-24", "meal": "lunch", "food": rice.Id, "grams": 250})
	if err := a.Save(e); err != nil {
		t.Fatal(err)
	}
	if e.GetString("name") != "Rice, cooked" || e.GetFloat("kcal") != 325 || e.GetFloat("protein") != 6.8 {
		t.Errorf("entry = %v", e.FieldsData())
	}

	// a new amount moves the totals
	e.Set("grams", 100)
	if err := a.Save(e); err != nil {
		t.Fatal(err)
	}
	if e.GetFloat("kcal") != 130 {
		t.Errorf("kcal %v after changing the grams", e.GetFloat("kcal"))
	}

	// a quick add keeps its calories
	q := core.NewRecord(entries)
	q.Load(map[string]any{"user": ada.Id, "date": "2026-10-24", "meal": "snacks", "name": "Cake at work", "kcal": 400})
	if err := a.Save(q); err != nil || q.GetFloat("kcal") != 400 {
		t.Errorf("quick add: %v, kcal %v", err, q.GetFloat("kcal"))
	}

	// someone who chose Dutch food names gets the Dutch name, or the English
	// one when the food has none
	p := addProfile(t, a, bob.Id)
	p.Set("food_language", "nl")
	if err := a.Save(p); err != nil {
		t.Fatal(err)
	}
	rice.Set("name_nl", "Rijst, gekookt")
	if err := a.Save(rice); err != nil {
		t.Fatal(err)
	}
	pasta := addFood(t, a, "Pasta, cooked", "", 150, 5)
	for food, want := range map[string]string{rice.Id: "Rijst, gekookt", pasta.Id: "Pasta, cooked"} {
		d := core.NewRecord(entries)
		d.Load(map[string]any{"user": bob.Id, "date": "2026-10-24", "meal": "dinner", "food": food, "grams": 100})
		if err := a.Save(d); err != nil || d.GetString("name") != want {
			t.Errorf("Dutch entry name %q, want %q (%v)", d.GetString("name"), want, err)
		}
	}

	// someone else's custom food can't be logged
	secret := addFood(t, a, "Bob's granola", bob.Id, 450, 10)
	x := core.NewRecord(entries)
	x.Load(map[string]any{"user": ada.Id, "date": "2026-10-24", "meal": "breakfast", "food": secret.Id, "grams": 50})
	if err := a.Save(x); err == nil {
		t.Error("logged someone else's food")
	}
}

func TestDiaryRules(t *testing.T) {
	scenarios := []struct {
		name, as, method, url, body string
		status                      int
		content                     string
	}{
		// the bundled USDA foods have a source id, the test's foods don't
		{"you see shared foods and your own", "ada", http.MethodGet, "/api/collections/foods/records?filter=source_id%3D%27%27", "", http.StatusOK, `"totalItems":2`},
		{"you don't see others' custom foods", "bob", http.MethodGet, "/api/collections/foods/records?filter=source_id%3D%27%27", "", http.StatusOK, `"totalItems":1`},
		{"you add a custom food", "ada", http.MethodPost, "/api/collections/foods/records", `{"name":"Soup","source":"custom","owner":"{ada}","kcal":40}`, http.StatusOK, `"name":"Soup"`},
		{"you can't add a shared food", "ada", http.MethodPost, "/api/collections/foods/records", `{"name":"Soup","source":"usda","owner":"{ada}","kcal":40}`, http.StatusBadRequest, ""},
		{"you can't change a shared food", "ada", http.MethodPatch, "/api/collections/foods/records/{rice}", `{"kcal":1}`, http.StatusNotFound, ""},
		{"you log to your own diary", "ada", http.MethodPost, "/api/collections/entries/records", `{"user":"{ada}","date":"2026-10-24","meal":"lunch","food":"{rice}","grams":200}`, http.StatusOK, `"kcal":260`},
		{"not to someone else's", "bob", http.MethodPost, "/api/collections/entries/records", `{"user":"{ada}","date":"2026-10-24","meal":"lunch","food":"{rice}","grams":200}`, http.StatusBadRequest, ""},
		{"others can't see your diary", "bob", http.MethodGet, "/api/collections/entries/records", "", http.StatusOK, `"totalItems":0`},
	}
	for _, sc := range scenarios {
		a := newApp(t)
		ada, adaToken := addUser(t, a, "ada@example.com", "owner")
		_, bobToken := addUser(t, a, "bob@example.com", "member")
		rice := addFood(t, a, "Rice, cooked", "", 130, 2.7)
		addFood(t, a, "Ada's soup", ada.Id, 40, 2)
		entries, _ := a.FindCollectionByNameOrId("entries")
		e := core.NewRecord(entries)
		e.Load(map[string]any{"user": ada.Id, "date": "2026-10-23", "meal": "dinner", "food": rice.Id, "grams": 100})
		if err := a.Save(e); err != nil {
			t.Fatal(err)
		}
		token := adaToken
		if sc.as == "bob" {
			token = bobToken
		}
		replace := strings.NewReplacer("{ada}", ada.Id, "{rice}", rice.Id)
		s := tests.ApiScenario{
			Name:           sc.name,
			Method:         sc.method,
			URL:            replace.Replace(sc.url),
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

func TestAddOns(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, _ := addUser(t, a, "ada@example.com", "owner")
	bob, _ := addUser(t, a, "bob@example.com", "member")
	coffee := addFood(t, a, "Coffee", "", 2, 0.1)
	milk := addFood(t, a, "Milk, semi-skimmed", "", 46, 3.4)
	entries, _ := a.FindCollectionByNameOrId("entries")
	save := func(values map[string]any) (*core.Record, error) {
		r := core.NewRecord(entries)
		r.Load(values)
		return r, a.Save(r)
	}

	cup, err := save(map[string]any{"user": ada.Id, "date": "2026-10-24", "meal": "breakfast", "food": coffee.Id, "grams": 200})
	if err != nil {
		t.Fatal(err)
	}

	// an add-on gets its own totals and its parent's day and meal
	splash, err := save(map[string]any{"user": ada.Id, "date": "2026-01-01", "meal": "snacks", "food": milk.Id, "grams": 50, "parent": cup.Id})
	if err != nil {
		t.Fatal(err)
	}
	if splash.GetFloat("kcal") != 23 || splash.GetString("date") != "2026-10-24" || splash.GetString("meal") != "breakfast" {
		t.Errorf("add-on = %v", splash.FieldsData())
	}

	// moving the coffee moves the milk
	cup.Set("meal", "snacks")
	cup.Set("date", "2026-10-25")
	if err := a.Save(cup); err != nil {
		t.Fatal(err)
	}
	moved, _ := a.FindRecordById("entries", splash.Id)
	if moved.GetString("meal") != "snacks" || moved.GetString("date") != "2026-10-25" {
		t.Errorf("add-on after moving its parent = %v", moved.FieldsData())
	}

	// no add-ons on add-ons, on someone else's entry, or on an entry that has add-ons
	if _, err := save(map[string]any{"user": ada.Id, "date": "2026-10-25", "meal": "snacks", "food": milk.Id, "grams": 10, "parent": splash.Id}); err == nil {
		t.Error("added an add-on to an add-on")
	}
	if _, err := save(map[string]any{"user": bob.Id, "date": "2026-10-25", "meal": "snacks", "food": milk.Id, "grams": 10, "parent": cup.Id}); err == nil {
		t.Error("added an add-on to someone else's entry")
	}
	other, _ := save(map[string]any{"user": ada.Id, "date": "2026-10-25", "meal": "snacks", "name": "Biscuit", "kcal": 40})
	cup.Set("parent", other.Id)
	if err := a.Save(cup); err == nil {
		t.Error("made an entry with add-ons an add-on")
	}

	// removing the coffee removes the milk
	cup, _ = a.FindRecordById("entries", cup.Id)
	if err := a.Delete(cup); err != nil {
		t.Fatal(err)
	}
	if _, err := a.FindRecordById("entries", splash.Id); err == nil {
		t.Error("the add-on outlived its parent")
	}
}
