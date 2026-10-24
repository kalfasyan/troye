package app_test

import (
	"bytes"
	"compress/gzip"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/selfhostit/troye/foods"
)

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
		case "/api/v2/product/0012345678905.json":
			// no name in the product's own language, only in Dutch
			_, _ = w.Write([]byte(`{"status":1,"code":"0012345678905","product":{"product_name":"","product_name_nl":"Halfvolle melk",
				"nutriments":{"energy-kj_100g":192,"proteins_100g":3.4,"carbohydrates_100g":4.7,"fat_100g":1.5,"sodium_100g":0.04}}}`))
		case "/api/v2/product/5411188112709.json":
			// named in German first, with English and French names too
			_, _ = w.Write([]byte(`{"status":1,"product":{"product_name":"Geröstete Mandel","product_name_en":"Roasted almond drink",
				"product_name_fr":"Lait amande grillée","brands":"Alpro","nutriments":{"energy-kcal_100g":14}}}`))
		case "/api/v2/product/1234567890123.json":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status":0,"status_verbose":"product not found"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer off.Close()
	foods.OpenFoodFactsURL = off.URL
	defer func() { foods.OpenFoodFactsURL = "https://world.openfoodfacts.org" }()

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
			Name:            "a product named only in Dutch, with energy in kJ and sodium instead of salt",
			Method:          http.MethodGet,
			URL:             "/api/app/barcode/0012345678905",
			Headers:         auth,
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"name":"Halfvolle melk"`, `"name_nl":"Halfvolle melk"`, `"kcal":46`, `"salt":0.1`},
		},
		{
			Name:            "its 12-digit UPC code finds it too",
			Method:          http.MethodGet,
			URL:             "/api/app/barcode/012345678905",
			Headers:         auth,
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"name":"Halfvolle melk"`},
			AfterTestFunc: func(t testing.TB, _ *tests.TestApp, _ *http.Response) {
				if calls.Load() != 2 {
					t.Errorf("%d calls to Open Food Facts", calls.Load())
				}
			},
		},
		{
			Name:            "the English name comes first, and the product's own name stays searchable",
			Method:          http.MethodGet,
			URL:             "/api/app/barcode/5411188112709",
			Headers:         auth,
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"name":"Roasted almond drink"`, `"name_fr":"Lait amande grillée"`, `"aliases":"Geröstete Mandel"`},
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

// storeFoods adds foods to a shared table.
func storeFoods(t testing.TB, a core.App, source string, list ...foods.Food) {
	if _, err := foods.Store(a, source, list); err != nil {
		t.Fatal(err)
	}
}

func TestFoodSearch(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, token := addUser(t, a, "ada@example.com", "owner")
	bob, _ := addUser(t, a, "bob@example.com", "member")
	storeFoods(t, a, "nevo",
		foods.Food{SourceID: "1", Name: "Cheese Gouda 48+ young", NameNL: "Kaas Goudse 48+ jong", Aliases: "jonge kaas", Kcal: 356},
		foods.Food{SourceID: "2", Name: "Milk semi-skimmed", NameNL: "Melk halfvolle", Kcal: 46},
	)
	storeFoods(t, a, "off",
		foods.Food{SourceID: "5400000000001", Barcode: "5400000000001", Name: "Jonge kaas plakken", Brand: "Delhaize", Kcal: 360},
		foods.Food{SourceID: "5400000000002", Barcode: "5400000000002", Name: "Chocomelk", Brand: "Boni", Kcal: 70},
	)
	addFood(t, a, "Bob's cheese", bob.Id, 400, 20)
	mine := addFood(t, a, "Kaas van de markt", ada.Id, 380, 24)

	search := func(q string) []string {
		t.Helper()
		found, err := foods.Search(a, ada.Id, q, 10)
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, len(found))
		for i, f := range found {
			names[i] = f.GetString("name")
		}
		return names
	}

	// Dutch finds the NEVO food by its Dutch name, in any word order, and
	// the generic table comes before the branded product
	if got := search("jonge kaas"); len(got) < 2 || got[0] != "Cheese Gouda 48+ young" || got[1] != "Jonge kaas plakken" {
		t.Errorf("jonge kaas: %q", got)
	}
	if got := search("kaas jong"); len(got) == 0 || got[0] != "Cheese Gouda 48+ young" {
		t.Errorf("kaas jong: %q", got)
	}
	// someone else's foods never show
	for _, name := range search("cheese") {
		if name == "Bob's cheese" {
			t.Error("found Bob's own food")
		}
	}
	// a brand and a word inside a compound word
	if got := search("boni melk"); len(got) != 1 || got[0] != "Chocomelk" {
		t.Errorf("boni melk: %q", got)
	}
	// plurals
	if got := search("kazen"); len(got) != 0 {
		t.Errorf("kazen: %q", got)
	}
	if got := search("cheeses"); len(got) == 0 {
		t.Errorf("cheeses found nothing")
	}

	// what you log often comes first
	entries, _ := a.FindCollectionByNameOrId("entries")
	for range 3 {
		e := core.NewRecord(entries)
		e.Load(map[string]any{"user": ada.Id, "date": time.Now().Format("2006-01-02"), "meal": "lunch", "food": mine.Id, "grams": 30})
		if err := a.Save(e); err != nil {
			t.Fatal(err)
		}
	}
	if got := search("kaas"); len(got) == 0 || got[0] != "Kaas van de markt" {
		t.Errorf("kaas after logging: %q", got)
	}

	s := tests.ApiScenario{
		Method:          http.MethodGet,
		URL:             "/api/app/foods/search?q=p%C3%A2t%C3%A9",
		Headers:         map[string]string{"Authorization": token},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"name":"Country-style pâté"`, `"source":"ciqual"`, `"name_fr":"Pâté de campagne"`},
		// the search field stays on the server
		NotExpectedContent:    []string{`"search"`},
		TestAppFactory:        func(testing.TB) *tests.TestApp { return a },
		DisableTestAppCleanup: true,
	}
	s.Test(t)
	s.Name, s.URL, s.ExpectedContent, s.NotExpectedContent = "pate without accents", "/api/app/foods/search?q=pate", []string{`"name":"Country-style pâté"`}, nil
	s.Test(t)
}

func TestCustomFoodsAreSearchable(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, _ := addUser(t, a, "ada@example.com", "owner")
	f := addFood(t, a, "Stoofvlees van oma", ada.Id, 180, 20)
	if f.GetString("search") != "stoofvlees van oma" {
		t.Errorf("search %q", f.GetString("search"))
	}
	f.Set("name", "Vol-au-vent")
	if err := a.Save(f); err != nil {
		t.Fatal(err)
	}
	if found, _ := foods.Search(a, ada.Id, "vol au vent", 5); len(found) != 1 {
		t.Errorf("found %d after renaming", len(found))
	}
}

func multipartFile(t testing.TB, name string, content []byte) (*bytes.Buffer, string) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	_ = w.Close()
	return &body, w.FormDataContentType()
}

const nevoCSV = "Voedingsmiddelgroep|Food group|NEVO-code|Voedingsmiddelnaam/Dutch food name|Engelse naam/Food name|Synoniem|Hoeveelheid/Quantity|ENERCJ|ENERCC|PROT|CHO|SUGAR|FAT|FIBT|NA\n" +
	"Brood|Bread|1|Brood volkoren|Bread wholemeal|Volkorenbrood|per 100g|1000|238|9,8|39,5|2,8|2,5|7,4|430\n"

func TestNEVOUpload(t *testing.T) {
	a := newApp(t)
	_, owner := addUser(t, a, "ada@example.com", "owner")
	_, member := addUser(t, a, "bob@example.com", "member")
	factory := func(testing.TB) *tests.TestApp { return a }

	upload := func(name string, content []byte, token string) (*bytes.Buffer, map[string]string) {
		body, contentType := multipartFile(t, name, content)
		return body, map[string]string{"Authorization": token, "Content-Type": contentType}
	}
	memberBody, memberHeaders := upload("NEVO2025_v9.0.csv", []byte(nevoCSV), member)
	wrongBody, wrongHeaders := upload("holiday.csv", []byte("a,b\n1,2\n"), owner)
	body, headers := upload("NEVO2025_v9.0.csv", []byte(nevoCSV), owner)

	scenarios := []tests.ApiScenario{
		{
			Name:            "a member can't import",
			Method:          http.MethodPost,
			URL:             "/api/app/foods/nevo",
			Body:            memberBody,
			Headers:         memberHeaders,
			ExpectedStatus:  http.StatusForbidden,
			ExpectedContent: []string{"Only the owner"},
		},
		{
			Name:            "another file says what's wrong",
			Method:          http.MethodPost,
			URL:             "/api/app/foods/nevo",
			Body:            wrongBody,
			Headers:         wrongHeaders,
			ExpectedStatus:  http.StatusBadRequest,
			ExpectedContent: []string{"doesn't look like the NEVO table", "Choose the CSV or Excel file"},
		},
		{
			Name:            "the owner imports NEVO",
			Method:          http.MethodPost,
			URL:             "/api/app/foods/nevo",
			Body:            body,
			Headers:         headers,
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"name":"nevo"`, `"version":"2025/9.0"`, `"foods":1`},
			AfterTestFunc: func(t testing.TB, a *tests.TestApp, _ *http.Response) {
				f, err := a.FindFirstRecordByFilter("foods", "source = 'nevo' && source_id = '1'")
				if err != nil {
					t.Fatal(err)
				}
				if f.GetString("name_nl") != "Brood volkoren" || f.GetFloat("kcal") != 238 || f.GetFloat("salt") != 1.08 {
					t.Errorf("food %v", f.FieldsData())
				}
			},
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

func TestOFFSync(t *testing.T) {
	export := "code\tproduct_name\tbrands\tcountries_tags\tenergy-kcal_100g\tproteins_100g\tcarbohydrates_100g\tfat_100g\tsalt_100g\n" +
		"5410000000001\tSpeculoos\tLotus\ten:belgium,en:france\t484\t4.9\t72.6\t19\t0.9\n" +
		"3000000000001\tCroissant\t\ten:france\t400\t8\t45\t21\t1\n"
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte(export))
	_ = zw.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(gz.Bytes())
	}))
	defer srv.Close()
	foods.OFFExportURL = srv.URL + "/products.csv.gz"
	defer func() {
		foods.OFFExportURL = "https://static.openfoodfacts.org/data/en.openfoodfacts.org.products.csv.gz"
	}()

	a := newApp(t)
	_, owner := addUser(t, a, "ada@example.com", "owner")
	_, member := addUser(t, a, "bob@example.com", "member")
	factory := func(testing.TB) *tests.TestApp { return a }
	var done atomic.Bool

	scenarios := []tests.ApiScenario{
		{
			Name:            "a member can't start it",
			Method:          http.MethodPost,
			URL:             "/api/app/foods/off",
			Headers:         map[string]string{"Authorization": member},
			ExpectedStatus:  http.StatusForbidden,
			ExpectedContent: []string{"Only the owner"},
		},
		{
			Name:            "the owner starts it",
			Method:          http.MethodPost,
			URL:             "/api/app/foods/off",
			Headers:         map[string]string{"Authorization": owner},
			ExpectedStatus:  http.StatusAccepted,
			ExpectedContent: []string{},
			AfterTestFunc: func(t testing.TB, a *tests.TestApp, _ *http.Response) {
				for range 100 {
					if status, err := a.FindFirstRecordByData("food_tables", "name", "off"); err == nil && status.GetInt("foods") > 0 {
						done.Store(true)
						break
					}
					time.Sleep(50 * time.Millisecond)
				}
			},
		},
	}
	for i := range scenarios {
		s := scenarios[i]
		s.TestAppFactory = factory
		s.DisableTestAppCleanup = true
		s.Test(t)
	}
	if !done.Load() {
		t.Fatal("the download didn't finish")
	}
	if _, err := a.FindFirstRecordByFilter("foods", "barcode = '5410000000001' && name = 'Speculoos'"); err != nil {
		t.Error("the Belgian product is missing")
	}
	if _, err := a.FindFirstRecordByFilter("foods", "barcode = '3000000000001'"); err == nil {
		t.Error("kept a product only sold in France")
	}
	a.Cleanup()
}
