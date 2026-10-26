package loseit_test

import (
	"archive/zip"
	"bytes"
	"slices"
	"testing"

	"github.com/selfhostit/troye/loseit"
)

// export zips files as Lose It! does, under a folder of its own.
func export(t testing.TB, files map[string]string) *bytes.Reader {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for name, content := range files {
		w, err := z.Create("export/" + name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(content))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b.Bytes())
}

// Made-up data in the layout of a real export.
var sample = map[string]string{
	"food-logs.csv": "Date,Name,Icon,Meal,Quantity,Units,Calories,Deleted,Fat (g),Protein (g),Carbohydrates (g),Saturated Fat (g),Sugars (g),Fiber (g),Cholesterol (mg),Sodium (mg)\n" +
		"03/02/2026,Porridge,Oatmeal,Breakfast,1.0,Cup,150,0,3,5,27,0.5,1,4,0,10\n" +
		"03/02/2026,Banana,Banana,Breakfast,1.0,Each,105,0,0.4,1.3,27,n/a,14,3.1,n/a,1\n" +
		"03/02/2026,Banana,Banana,Breakfast,1.0,Each,105,0,0.4,1.3,27,n/a,14,3.1,n/a,1\n" +
		"03/02/2026,\"Soup, tomato\",Soup,Lunch,250.0,Grams,90,0,2,3,15,0.3,9,2,5,800\n" +
		"03/02/2026,Feast,Default,Dinner,1.0,Serving,\"1,000\",0,40,50,100,10,20,10,100,2000\n" +
		"03/02/2026,Crisps,Chips,Afternoon Snacks,1.5,Ounces,230,0,15,3,22,1.5,0.5,2,0,250\n" +
		"03/02/2026,Cake,Cake,Morning Snacks,1.0,Slice,300,1,15,3,40,8,30,1,50,200\n",
	"weights.csv": "Date,Weight,Last Updated,Deleted\n" +
		"03/01/2026,82.5,2026-03-01T07:00:00+0100,false\n" +
		"03/08/2026,81.9,2026-03-08T07:00:00+0100,false\n" +
		"03/09/2026,90,2026-03-09T07:00:00+0100,true\n",
	"exercise-logs.csv": "Date,Name,Icon,Type,Quantity,Units,Calories,Deleted\n" +
		"03/02/2026,Walking,Walking,Exercise,30,minutes,-120.4,0\n" +
		"03/02/2026,Garmin Adjustment,Garmin,Exercise,0,minutes,-250.0,0\n" +
		"03/03/2026,Garmin Adjustment,Garmin,Exercise,0,minutes,-0.0,0\n" +
		"03/03/2026,Kettlebells,Kettlebell,Exercise,15,minutes,-90,1\n" +
		"03/04/2026,Rowing Machine,Rowing,Exercise,1,hours,-400,0\n",
	"profile.csv": "Name,Value\nBirthday,05/20/1990\nGender,Female\nHeight,168.0\nStart Weight,85.0\n" +
		"Current Weight,81.9\nGoal Weight,72.0\nPlan,1/2 kg per week\nActivity Level,Somewhat Active\n",
	"fasting-schedules.csv": "Day,Scheduled start,Duration,Deleted\n" +
		"Monday,20:00:00,960,false\nTuesday,20:00:00,960,false\nFriday,19:00:00,840,false\nSunday,20:00:00,960,true\n",
	"custom-foods.csv": "Name,UniqueId,Brand,Image,Quantity,Measure,Calories,Fat (g),Protein (g),Carbohydrates (g),Saturated Fat (g),Sugars (g),Fiber (g),Cholesterol (mg),Sodium (mg)\n" +
		"Granola,aaaa0000000000000000000000000001,Home,,50,Grams,220,8,6,30,1,9,4,0,40\n" +
		"Muffin,aaaa0000000000000000000000000002,,,1,Each,350,15,5,45,6,25,2,40,300\n" +
		"Mystery bar,aaaa0000000000000000000000000003,,,1,Each,200,8,3,25,4,15,1,0,90\n",
	"recipes.csv": "Name,UniqueId,Quantity,Measure,Author,Image Name,Calories,Fat (g),Protein (g),Carbohydrates (g),Saturated Fat (g),Sugars (g),Fiber (g),Cholesterol (mg),Sodium (mg)\n",
	"active-food-servings.csv": "FoodUniqueId,Measure,Quantity,BaseMultiplier,Created,LastUpdated,Deleted\n" +
		"aaaa0000000000000000000000000002,Each,2.0,1.0,2026-03-01T10:00:00.000Z,2026-03-01T10:00:00Z,false\n" +
		"aaaa0000000000000000000000000002,Milliliter,500.0,1.0,2026-03-01T10:00:00.000Z,2026-03-01T10:00:00Z,false\n" +
		"aaaa0000000000000000000000000002,Gram,200.0,1.0,2026-03-01T10:00:00.000Z,2026-03-01T10:00:00Z,false\n",
	"food-photos/food-photos.csv": "Date,Meal Type,Visibility,Token,Deleted,Last Updated\n" +
		"03/02/2026,Lunch,Private,0123456789abcdef0123456789abcdef,false,1772440000000\n" +
		"03/02/2026,Dinner,Private,fedcba9876543210fedcba9876543210,false,1772440000000\n",
	"food-photos/Lunch(2026-03-02)-0123456789abcdef0123456789abcdef-photo.jpg": "jpeg",
	"achievements.csv":          "Tag,Level,Earned On,Deleted\nFOOD_LOGGED,1,2026-03-02T08:00:00+0100,false\n",
	"notes.csv":                 "Date,Title,Body\n",
	"daily-calorie-summary.csv": "Date,Food cals,Exercise cals,Budget cals,EER\n03/02/2026,1980.0,120,1700,2300\n",
}

func TestRead(t *testing.T) {
	r := export(t, sample)
	x, err := loseit.Read(r, r.Size())
	if err != nil {
		t.Fatal(err)
	}

	if len(x.Foods) != 6 {
		t.Fatalf("%d foods: %+v", len(x.Foods), x.Foods)
	}
	porridge, banana, banana2, soup, feast, crisps := x.Foods[0], x.Foods[1], x.Foods[2], x.Foods[3], x.Foods[4], x.Foods[5]
	if porridge.Date != "2026-03-02" || porridge.Meal != "breakfast" || porridge.Amount != "1 Cup" || porridge.Grams != 0 || porridge.Kcal != 150 || porridge.Protein != 5 {
		t.Errorf("porridge %+v", porridge)
	}
	if banana.SatFat != 0 || banana.Cholesterol != 0 || banana.Sugar != 14 {
		t.Errorf("n/a values %+v", banana)
	}
	if banana.Key == banana2.Key || banana.Key == "" {
		t.Errorf("the same food twice needs two keys: %q %q", banana.Key, banana2.Key)
	}
	if soup.Name != "Soup, tomato" || soup.Grams != 250 || soup.Salt != 2 {
		t.Errorf("soup %+v", soup)
	}
	if feast.Kcal != 1000 {
		t.Errorf("thousands %+v", feast)
	}
	if crisps.Meal != "snacks" || crisps.Grams != 42.5 || crisps.Amount != "1.5 Ounces" {
		t.Errorf("crisps %+v", crisps)
	}
	again := export(t, sample)
	y, _ := loseit.Read(again, again.Size())
	if y.Foods[1].Key != banana.Key {
		t.Error("keys change between reads")
	}

	if len(x.Weights) != 2 || x.Weights[1] != (loseit.Weight{Date: "2026-03-08", Kg: 81.9}) {
		t.Errorf("weights %+v", x.Weights)
	}

	if len(x.Exercises) != 3 {
		t.Fatalf("exercises %+v", x.Exercises)
	}
	walk, garmin, rowing := x.Exercises[0], x.Exercises[1], x.Exercises[2]
	if walk.Kind != "walk" || walk.Minutes != 30 || walk.Kcal != 120 {
		t.Errorf("walk %+v", walk)
	}
	if garmin.Kind != "tracker" || garmin.Name != "Garmin Adjustment" || garmin.Minutes != 0 || garmin.Kcal != 250 {
		t.Errorf("garmin %+v", garmin)
	}
	if rowing.Kind != "other" || rowing.Name != "Rowing Machine" || rowing.Minutes != 60 {
		t.Errorf("rowing %+v", rowing)
	}

	p := x.Profile
	if p == nil || p.BirthDate != "1990-05-20" || p.Sex != "female" || p.HeightCm != 168 || p.WeightKg != 81.9 ||
		p.GoalWeightKg != 72 || p.Goal != "lose" || p.PaceKg != 0.5 || p.Activity != "light" {
		t.Errorf("profile %+v", p)
	}

	// 16 hours from 20:00 on Mondays and Tuesdays; Friday's shorter fast is the odd one out
	f := x.Fasting
	if f == nil || f.Plan != "16_8" || f.EatingStart != "12:00" || !slices.Equal(f.Days, []string{"mon", "tue"}) {
		t.Errorf("fasting %+v", f)
	}

	if len(x.CustomFoods) != 2 || x.Unweighed != 1 {
		t.Fatalf("custom foods %+v, %d unweighed", x.CustomFoods, x.Unweighed)
	}
	granola, muffin := x.CustomFoods[0], x.CustomFoods[1]
	if granola.Kcal != 440 || granola.Protein != 12 || granola.ServingG != 50 || granola.ServingLabel != "50 Grams" || granola.Salt != 0.2 {
		t.Errorf("granola %+v", granola)
	}
	// two muffins were logged as 200 g, so one is 100 g
	if muffin.Kcal != 350 || muffin.ServingG != 100 || muffin.ServingLabel != "1 Each" {
		t.Errorf("muffin %+v", muffin)
	}

	if len(x.Photos) != 1 || x.Photos[0].Meal != "lunch" || x.Photos[0].Date != "2026-03-02" {
		t.Errorf("photos %+v", x.Photos)
	}

	if !slices.Equal(x.Unused, []string{"achievements (1)"}) {
		t.Errorf("unused %v", x.Unused)
	}
}

func TestReadImperial(t *testing.T) {
	r := export(t, map[string]string{
		"profile.csv": "Name,Value\nBirthday,05/20/1990\nGender,Male\nHeight,70.0\nCurrent Weight,200\nGoal Weight,180\n" +
			"Plan,Lose 1 lb per week\nActivity Level,Active\n",
		"weights.csv": "Date,Weight,Last Updated,Deleted\n03/01/2026,200,2026-03-01T07:00:00+0100,false\n",
	})
	x, err := loseit.Read(r, r.Size())
	if err != nil {
		t.Fatal(err)
	}
	if x.Weights[0].Kg != 90.72 {
		t.Errorf("weights %+v", x.Weights)
	}
	p := x.Profile
	if p.HeightCm != 178 || p.WeightKg != 90.7 || p.GoalWeightKg != 81.6 || p.PaceKg != 0.5 || p.Activity != "moderate" || p.Goal != "lose" {
		t.Errorf("profile %+v", p)
	}
}

func TestReadOther(t *testing.T) {
	r := export(t, map[string]string{"holiday.csv": "a,b\n1,2\n"})
	if _, err := loseit.Read(r, r.Size()); err != loseit.ErrNotExport {
		t.Errorf("another zip: %v", err)
	}
	b := bytes.NewReader([]byte("not a zip"))
	if _, err := loseit.Read(b, b.Size()); err != loseit.ErrNotExport {
		t.Errorf("not a zip: %v", err)
	}
}
