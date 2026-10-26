package app_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"slices"
	"testing"

	"github.com/pocketbase/pocketbase/tests"
)

// loseItZip is a made-up Lose It! export.
func loseItZip(t testing.TB) []byte {
	t.Helper()
	var photo bytes.Buffer
	if err := jpeg.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"food-logs.csv": "Date,Name,Icon,Meal,Quantity,Units,Calories,Deleted,Fat (g),Protein (g),Carbohydrates (g),Saturated Fat (g),Sugars (g),Fiber (g),Cholesterol (mg),Sodium (mg)\n" +
			"03/02/2026,Porridge,Oatmeal,Breakfast,1.0,Cup,150,0,3,5,27,0.5,1,4,0,10\n" +
			"03/02/2026,Banana,Banana,Breakfast,1.0,Each,105,0,0.4,1.3,27,n/a,14,3.1,n/a,1\n" +
			"03/02/2026,Banana,Banana,Breakfast,1.0,Each,105,0,0.4,1.3,27,n/a,14,3.1,n/a,1\n" +
			"03/02/2026,Tomato soup,Soup,Lunch,250.0,Grams,90,0,2,3,15,0.3,9,2,5,800\n",
		"weights.csv": "Date,Weight,Last Updated,Deleted\n" +
			"03/01/2026,82.5,2026-03-01T07:00:00+0100,false\n" +
			"03/08/2026,81.9,2026-03-08T07:00:00+0100,false\n",
		"exercise-logs.csv": "Date,Name,Icon,Type,Quantity,Units,Calories,Deleted\n" +
			"03/02/2026,Walking,Walking,Exercise,30,minutes,-120.4,0\n" +
			"03/02/2026,Garmin Adjustment,Garmin,Exercise,0,minutes,-250.0,0\n",
		"profile.csv": "Name,Value\nBirthday,05/20/1990\nGender,Female\nHeight,170.0\nCurrent Weight,81.9\n" +
			"Goal Weight,75.0\nPlan,3/4 kg per week\nActivity Level,Sedentary\n",
		"fasting-schedules.csv": "Day,Scheduled start,Duration,Deleted\nMonday,20:00:00,960,false\nThursday,20:00:00,960,false\n",
		"custom-foods.csv": "Name,UniqueId,Brand,Image,Quantity,Measure,Calories,Fat (g),Protein (g),Carbohydrates (g),Saturated Fat (g),Sugars (g),Fiber (g),Cholesterol (mg),Sodium (mg)\n" +
			"Granola,bbbb0000000000000000000000000001,Home,,50,Grams,220,8,6,30,1,9,4,0,40\n",
		"food-photos/food-photos.csv": "Date,Meal Type,Visibility,Token,Deleted,Last Updated\n03/02/2026,Lunch,Private,0123456789abcdef0123456789abcdef,false,1772440000000\n",
		"food-photos/Lunch(2026-03-02)-0123456789abcdef0123456789abcdef-photo.jpg": photo.String(),
		"achievements.csv": "Tag,Level,Earned On,Deleted\nFOOD_LOGGED,1,2026-03-02T08:00:00+0100,false\n",
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for name, content := range files {
		w, _ := z.Create(name)
		_, _ = w.Write([]byte(content))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func importBody(t testing.TB, data []byte, plan bool) (*bytes.Buffer, string) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, _ := w.CreateFormFile("file", "loseit-export.zip")
	_, _ = part.Write(data)
	if plan {
		_ = w.WriteField("plan", "true")
	}
	_ = w.Close()
	return &body, w.FormDataContentType()
}

func TestImportLoseIt(t *testing.T) {
	a := newApp(t)
	ada, token := addUser(t, a, "ada@example.com", "member")
	bob, bobToken := addUser(t, a, "bob@example.com", "member")
	addProfile(t, a, ada.Id)
	// Troye's own weigh-in for a day wins over the export's
	weigh(t, a, ada.Id, "2026-03-01", 82)
	factory := func(testing.TB) *tests.TestApp { return a }
	zipped := loseItZip(t)

	first, firstType := importBody(t, zipped, true)
	again, againType := importBody(t, zipped, false)
	other, otherType := importBody(t, []byte("not a zip"), false)
	bobs, bobsType := importBody(t, zipped, false)

	var result struct {
		Entries, Weighins, Exercises, Foods, Photos struct{ Added, Existing int }
		Plan                                        bool
		PlanError                                   string `json:"plan_error"`
		Unused                                      []string
	}
	decode := func(t testing.TB, res *http.Response) {
		result = struct {
			Entries, Weighins, Exercises, Foods, Photos struct{ Added, Existing int }
			Plan                                        bool
			PlanError                                   string `json:"plan_error"`
			Unused                                      []string
		}{}
		if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "signed out",
			Method:          http.MethodPost,
			URL:             "/api/app/import/loseit",
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:            "another file says what's wrong",
			Method:          http.MethodPost,
			URL:             "/api/app/import/loseit",
			Body:            other,
			Headers:         map[string]string{"Authorization": token, "Content-Type": otherType},
			ExpectedStatus:  http.StatusBadRequest,
			ExpectedContent: []string{"doesn't look like a Lose It! export"},
		},
		{
			Name:            "the export and the plan come in",
			Method:          http.MethodPost,
			URL:             "/api/app/import/loseit",
			Body:            first,
			Headers:         map[string]string{"Authorization": token, "Content-Type": firstType},
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"entries":`},
			AfterTestFunc: func(t testing.TB, a *tests.TestApp, res *http.Response) {
				decode(t, res)
				if result.Entries.Added != 4 || result.Weighins.Added != 1 || result.Weighins.Existing != 1 ||
					result.Exercises.Added != 2 || result.Foods.Added != 1 || result.Photos.Added != 1 || !result.Plan {
					t.Errorf("result %+v", result)
				}
				if !slices.Equal(result.Unused, []string{"achievements (1)"}) {
					t.Errorf("unused %v", result.Unused)
				}

				soup, err := a.FindFirstRecordByData("entries", "name", "Tomato soup")
				if err != nil {
					t.Fatal(err)
				}
				if soup.GetString("user") != ada.Id || soup.GetString("date") != "2026-03-02" || soup.GetString("meal") != "lunch" ||
					soup.GetFloat("grams") != 250 || soup.GetFloat("kcal") != 90 || soup.GetFloat("salt") != 2 ||
					soup.GetFloat("sugar") != 9 || soup.GetString("amount") != "250 Grams" {
					t.Errorf("soup %v", soup.FieldsData())
				}

				walk, err := a.FindFirstRecordByData("exercises", "kind", "walk")
				if err != nil || walk.GetInt("minutes") != 30 || walk.GetInt("kcal") != 120 {
					t.Errorf("walk %v %v", walk, err)
				}
				garmin, err := a.FindFirstRecordByData("exercises", "kind", "tracker")
				if err != nil || garmin.GetInt("minutes") != 0 || garmin.GetInt("kcal") != 250 {
					t.Errorf("tracker %v %v", garmin, err)
				}

				granola, err := a.FindFirstRecordByData("foods", "name", "Granola")
				if err != nil || granola.GetString("owner") != ada.Id || granola.GetFloat("kcal") != 440 {
					t.Errorf("granola %v %v", granola, err)
				}

				photo, err := a.FindFirstRecordByData("meal_photos", "user", ada.Id)
				if err != nil || photo.GetString("status") != "logged" || photo.GetString("meal") != "lunch" || photo.GetString("photo") == "" {
					t.Errorf("photo %v %v", photo, err)
				}

				p, _ := a.FindFirstRecordByData("profiles", "user", ada.Id)
				// the weight stays the latest weigh-in's, today's from setting up the plan
				if p.GetFloat("weight_kg") != 80 || p.GetFloat("goal_weight_kg") != 75 || p.GetFloat("pace_kg") != 0.75 ||
					p.GetFloat("height_cm") != 170 || p.GetString("activity") != "sedentary" || p.GetString("fasting") != "16_8" ||
					p.GetString("eating_start") != "12:00" || !slices.Equal(p.GetStringSlice("fasting_days"), []string{"mon", "thu"}) {
					t.Errorf("profile %v", p.FieldsData())
				}
				weighins, _ := a.FindRecordsByFilter("weighins", "user = {:u}", "date", 0, 0, map[string]any{"u": ada.Id})
				var got []float64
				for _, w := range weighins {
					got = append(got, w.GetFloat("weight_kg"))
				}
				if !slices.Equal(got, []float64{82, 81.9, 80}) {
					t.Errorf("weigh-ins %v", got)
				}
			},
		},
		{
			Name:            "importing again adds nothing",
			Method:          http.MethodPost,
			URL:             "/api/app/import/loseit",
			Body:            again,
			Headers:         map[string]string{"Authorization": token, "Content-Type": againType},
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"plan":false`},
			AfterTestFunc: func(t testing.TB, a *tests.TestApp, res *http.Response) {
				decode(t, res)
				if result.Entries.Added != 0 || result.Entries.Existing != 4 || result.Weighins.Added != 0 ||
					result.Exercises.Existing != 2 || result.Foods.Existing != 1 || result.Photos.Existing != 1 {
					t.Errorf("result %+v", result)
				}
				if n, _ := a.CountRecords("entries"); n != 4 {
					t.Errorf("%d entries", n)
				}
			},
		},
		{
			Name:            "someone else's import is theirs",
			Method:          http.MethodPost,
			URL:             "/api/app/import/loseit",
			Body:            bobs,
			Headers:         map[string]string{"Authorization": bobToken, "Content-Type": bobsType},
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"entries":{"added":4,"existing":0}`},
			AfterTestFunc: func(t testing.TB, a *tests.TestApp, res *http.Response) {
				n, _ := a.CountRecords("entries")
				mine, _ := a.FindRecordsByFilter("entries", "user = {:u}", "", 0, 0, map[string]any{"u": bob.Id})
				if n != 8 || len(mine) != 4 {
					t.Errorf("%d entries, %d of Bob's", n, len(mine))
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
