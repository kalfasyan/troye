package app_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/selfhostit/troye/app"
)

// addFood saves a food with the given kcal and protein per 100 g; owner may be empty.
func addFood(t testing.TB, a core.App, name, owner string, kcal, protein float64) *core.Record {
	foods, err := a.FindCollectionByNameOrId("foods")
	if err != nil {
		t.Fatal(err)
	}
	f := core.NewRecord(foods)
	source := "ciqual"
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
		{"you see shared foods and your own", "ada", http.MethodGet, "/api/collections/foods/records", "", http.StatusOK, `"totalItems":2`},
		{"you don't see others' custom foods", "bob", http.MethodGet, "/api/collections/foods/records", "", http.StatusOK, `"totalItems":1`},
		{"you add a custom food", "ada", http.MethodPost, "/api/collections/foods/records", `{"name":"Soup","source":"custom","owner":"{ada}","kcal":40}`, http.StatusOK, `"name":"Soup"`},
		{"you can't add a shared food", "ada", http.MethodPost, "/api/collections/foods/records", `{"name":"Soup","source":"ciqual","owner":"{ada}","kcal":40}`, http.StatusBadRequest, ""},
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

func TestBarcode(t *testing.T) {
	var calls atomic.Int32
	off := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.UserAgent() == "" || !strings.HasPrefix(r.UserAgent(), "Troye/") {
			t.Errorf("user agent %q", r.UserAgent())
		}
		switch r.URL.Path {
		case "/api/v2/product/5400113690252.json":
			_, _ = w.Write([]byte(`{"status":1,"product":{"product_name":"Speculoos","brands":"Lotus, Lotus Bakeries",
				"nutriments":{"energy-kcal_100g":484,"energy-kcal_unit":"kcal","proteins_100g":"4.9","carbohydrates_100g":72.6,"fat_100g":19},
				"serving_quantity":"6.25","serving_size":"1 biscuit (6.25 g)"}}`))
		case "/api/v2/product/1234567890123.json":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status":0,"status_verbose":"product not found"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer off.Close()
	app.OpenFoodFactsURL = off.URL
	defer func() { app.OpenFoodFactsURL = "https://world.openfoodfacts.org" }()

	a := newApp(t)
	_, token := addUser(t, a, "ada@example.com", "owner")
	factory := func(testing.TB) *tests.TestApp { return a }
	auth := map[string]string{"Authorization": token}

	scenarios := []tests.ApiScenario{
		{
			Name:            "a new product comes from Open Food Facts",
			Method:          http.MethodGet,
			URL:             "/api/app/barcode/5400113690252",
			Headers:         auth,
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"name":"Speculoos"`, `"brand":"Lotus"`, `"kcal":484`, `"protein":4.9`, `"serving_g":6.25`, `"source":"off"`},
		},
		{
			Name:            "the second scan is answered locally",
			Method:          http.MethodGet,
			URL:             "/api/app/barcode/5400113690252",
			Headers:         auth,
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"name":"Speculoos"`},
			AfterTestFunc: func(t testing.TB, _ *tests.TestApp, _ *http.Response) {
				if calls.Load() != 1 {
					t.Errorf("%d calls to Open Food Facts", calls.Load())
				}
			},
		},
		{
			Name:            "an unknown product asks for a custom food",
			Method:          http.MethodGet,
			URL:             "/api/app/barcode/1234567890123",
			Headers:         auth,
			ExpectedStatus:  http.StatusNotFound,
			ExpectedContent: []string{"Add it as your own food"},
		},
		{
			Name:            "an outage says so",
			Method:          http.MethodGet,
			URL:             "/api/app/barcode/9999999999999",
			Headers:         auth,
			ExpectedStatus:  http.StatusBadGateway,
			ExpectedContent: []string{"Couldn't reach Open Food Facts"},
		},
		{
			Name:            "it isn't open to signed-out visitors",
			Method:          http.MethodGet,
			URL:             "/api/app/barcode/5400113690252",
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{"authorization token"},
		},
	}
	for i := range scenarios {
		s := scenarios[i]
		s.TestAppFactory = factory
		s.DisableTestAppCleanup = true
		s.Test(t)
	}
	a.Cleanup()
}
