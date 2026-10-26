package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"

	"github.com/selfhostit/troye/loseit"
)

// Importing a Lose It! export (backend/loseit reads it) into the account of
// whoever uploads it: their food log, weigh-ins, exercise, own foods and
// recipes, meal photos and, when they ask, their plan and fasting schedule.
//
// Importing is safe to repeat with a newer export: rows already imported are
// skipped, and so are days that already have a weigh-in in Troye.

// Counted is how many of a kind of record were added, and how many were
// already there.
type Counted struct {
	Added    int `json:"added"`
	Existing int `json:"existing"`
}

// ImportResult says what an import brought in.
type ImportResult struct {
	Entries   Counted `json:"entries"`
	Weighins  Counted `json:"weighins"`
	Exercises Counted `json:"exercises"`
	Foods     Counted `json:"foods"`
	Photos    Counted `json:"photos"`
	// whether the plan and fasting schedule were taken
	Plan bool `json:"plan"`
	// why the plan wasn't taken, when it was asked for
	PlanError string `json:"plan_error,omitempty"`
	// own foods left out because nothing says what a serving weighs
	Unweighed int `json:"unweighed"`
	// what the export has that Troye doesn't keep, such as "achievements (20)"
	Unused []string `json:"unused"`
}

// photoMaxSize is the largest meal photo meal_photos takes.
const photoMaxSize = 8 << 20

func registerImport(app core.App) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/app/import/loseit", importLoseIt).Bind(apis.RequireAuth("users"), apis.BodyLimit(1<<30))
		return se.Next()
	})
}

func importLoseIt(e *core.RequestEvent) error {
	file, header, err := e.Request.FormFile("file")
	if err != nil {
		return e.BadRequestError("Choose the zip file from Lose It!.", err)
	}
	defer file.Close()
	ra, ok := file.(io.ReaderAt)
	if !ok {
		return e.BadRequestError("Couldn't read that file.", nil)
	}
	x, err := loseit.Read(ra, header.Size)
	if errors.Is(err, loseit.ErrNotExport) {
		return e.BadRequestError("That doesn't look like a Lose It! export. Choose the zip file Lose It! emailed you.", err)
	}
	if err != nil {
		return e.BadRequestError("Couldn't read the export: "+err.Error(), err)
	}
	result, err := ImportLoseIt(e.App, e.Auth.Id, x, e.Request.FormValue("plan") == "true")
	if err != nil {
		return e.InternalServerError("", err)
	}
	return e.JSON(http.StatusOK, result)
}

// ImportLoseIt saves an export for a user, all of it or nothing. With plan,
// the export's plan and fasting schedule replace the user's.
func ImportLoseIt(app core.App, user string, x *loseit.Export, plan bool) (ImportResult, error) {
	r := ImportResult{Unweighed: x.Unweighed, Unused: x.Unused}
	if r.Unused == nil {
		r.Unused = []string{}
	}
	err := app.RunInTransaction(func(tx core.App) error {
		steps := []func(core.App, string, *loseit.Export, *ImportResult) error{
			importWeighins, importEntries, importExercises, importFoods, importPhotos,
		}
		for _, step := range steps {
			if err := step(tx, user, x, &r); err != nil {
				return err
			}
		}
		if plan {
			return importPlan(tx, user, x, &r)
		}
		return nil
	})
	return r, err
}

// existing is the set of values a user's records have in a field.
func existing(app core.App, collection, field, user string) (map[string]bool, error) {
	var values []string
	err := app.RecordQuery(collection).
		Select(field).
		AndWhere(dbx.HashExp{"user": user}).
		AndWhere(dbx.NewExp(field + " != ''")).
		Column(&values)
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set, err
}

func importWeighins(app core.App, user string, x *loseit.Export, r *ImportResult) error {
	have, err := existing(app, "weighins", "date", user)
	if err != nil {
		return err
	}
	c, err := app.FindCollectionByNameOrId("weighins")
	if err != nil {
		return err
	}
	for _, w := range x.Weights {
		if w.Kg < 30 || w.Kg > 350 {
			continue
		}
		// the weight Troye has for a day wins
		if have[w.Date] {
			r.Weighins.Existing++
			continue
		}
		rec := core.NewRecord(c)
		rec.Load(map[string]any{"user": user, "date": w.Date, "weight_kg": w.Kg})
		if err := app.Save(rec); err != nil {
			return fmt.Errorf("weigh-in of %s: %w", w.Date, err)
		}
		have[w.Date] = true
		r.Weighins.Added++
	}
	return nil
}

func importEntries(app core.App, user string, x *loseit.Export, r *ImportResult) error {
	have, err := existing(app, "entries", "import_key", user)
	if err != nil {
		return err
	}
	c, err := app.FindCollectionByNameOrId("entries")
	if err != nil {
		return err
	}
	for _, f := range x.Foods {
		if have[f.Key] {
			r.Entries.Existing++
			continue
		}
		rec := core.NewRecord(c)
		rec.Load(map[string]any{
			"user": user, "date": f.Date, "meal": f.Meal, "name": f.Name, "amount": f.Amount,
			"grams": math.Min(f.Grams, 5000), "kcal": math.Min(f.Kcal, 20000),
			"protein": f.Protein, "carbs": f.Carbs, "fat": f.Fat, "fiber": f.Fiber, "sugar": f.Sugar,
			"sat_fat": f.SatFat, "salt": f.Salt, "cholesterol": f.Cholesterol, "import_key": f.Key,
		})
		if err := app.Save(rec); err != nil {
			return fmt.Errorf("%s on %s: %w", f.Name, f.Date, err)
		}
		r.Entries.Added++
	}
	return nil
}

func importExercises(app core.App, user string, x *loseit.Export, r *ImportResult) error {
	have, err := existing(app, "exercises", "import_key", user)
	if err != nil {
		return err
	}
	c, err := app.FindCollectionByNameOrId("exercises")
	if err != nil {
		return err
	}
	for _, e := range x.Exercises {
		if have[e.Key] {
			r.Exercises.Existing++
			continue
		}
		rec := core.NewRecord(c)
		rec.Load(map[string]any{
			"user": user, "date": e.Date, "kind": e.Kind, "name": e.Name,
			"minutes": e.Minutes, "kcal": min(e.Kcal, 10000), "import_key": e.Key,
		})
		if err := app.Save(rec); err != nil {
			return fmt.Errorf("exercise on %s: %w", e.Date, err)
		}
		r.Exercises.Added++
	}
	return nil
}

func importFoods(app core.App, user string, x *loseit.Export, r *ImportResult) error {
	c, err := app.FindCollectionByNameOrId("foods")
	if err != nil {
		return err
	}
	for _, f := range x.CustomFoods {
		// source ids are unique across everyone, and two people may share a food
		id := f.ID
		if id == "" {
			id = f.Name + "\x00" + f.Brand
		}
		h := sha256.Sum256([]byte(user + "\x00" + id))
		sourceID := "loseit:" + hex.EncodeToString(h[:16])
		if _, err := app.FindFirstRecordByFilter("foods", "owner = {:user} && source = 'custom' && source_id = {:id}",
			dbx.Params{"user": user, "id": sourceID}); err == nil {
			r.Foods.Existing++
			continue
		}
		rec := core.NewRecord(c)
		rec.Load(map[string]any{
			"name": f.Name, "brand": f.Brand, "source": "custom", "source_id": sourceID, "owner": user,
			"kcal": f.Kcal, "protein": f.Protein, "carbs": f.Carbs, "fat": f.Fat, "fiber": f.Fiber,
			"sugar": f.Sugar, "salt": f.Salt, "serving_g": f.ServingG, "serving_label": f.ServingLabel,
		})
		if err := app.Save(rec); err != nil {
			return fmt.Errorf("food %s: %w", f.Name, err)
		}
		r.Foods.Added++
	}
	return nil
}

// importPhotos keeps the meal photos with the meals they were taken for. They
// are already in the diary, so they skip the photo analyser.
func importPhotos(app core.App, user string, x *loseit.Export, r *ImportResult) error {
	have, err := existing(app, "meal_photos", "hash", user)
	if err != nil {
		return err
	}
	c, err := app.FindCollectionByNameOrId("meal_photos")
	if err != nil {
		return err
	}
	for _, p := range x.Photos {
		if p.File.UncompressedSize64 > photoMaxSize {
			continue
		}
		rc, err := p.File.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(rc, photoMaxSize))
		rc.Close()
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		if have[hash] {
			r.Photos.Existing++
			continue
		}
		f, err := filesystem.NewFileFromBytes(data, "meal.jpg")
		if err != nil {
			return err
		}
		rec := core.NewRecord(c)
		rec.Load(map[string]any{"user": user, "date": p.Date, "meal": p.Meal, "hash": hash, "status": "logged"})
		rec.Set("photo", f)
		if err := app.Save(rec); err != nil {
			return fmt.Errorf("photo of %s: %w", p.Date, err)
		}
		have[hash] = true
		r.Photos.Added++
	}
	return nil
}

// importPlan takes the export's goal, pace, activity and body details, and
// its fasting schedule. The weight stays the latest weigh-in's.
func importPlan(app core.App, user string, x *loseit.Export, r *ImportResult) error {
	if x.Profile == nil && x.Fasting == nil {
		r.PlanError = "The export has no plan."
		return nil
	}
	p, err := app.FindFirstRecordByFilter("profiles", "user = {:user}", dbx.Params{"user": user})
	if err != nil {
		r.PlanError = "Set up your plan in Troye first."
		return nil
	}
	if lp := x.Profile; lp != nil {
		set := func(field string, v any, ok bool) {
			if ok {
				p.Set(field, v)
			}
		}
		set("birth_date", lp.BirthDate, lp.BirthDate != "")
		set("sex", lp.Sex, lp.Sex != "")
		set("height_cm", lp.HeightCm, lp.HeightCm >= 100 && lp.HeightCm <= 250)
		set("activity", lp.Activity, lp.Activity != "")
		set("goal", lp.Goal, lp.Goal != "")
		set("goal_weight_kg", lp.GoalWeightKg, lp.GoalWeightKg >= 30 && lp.GoalWeightKg <= 350)
		set("pace_kg", lp.PaceKg, true)
		latest, err := latestWeighin(app, user)
		if err != nil {
			return err
		}
		if latest != nil {
			p.Set("weight_kg", latest.GetFloat("weight_kg"))
		} else if lp.WeightKg >= 30 && lp.WeightKg <= 350 {
			p.Set("weight_kg", lp.WeightKg)
		}
	}
	if f := x.Fasting; f != nil {
		p.Set("fasting", f.Plan)
		p.Set("eating_start", f.EatingStart)
		days := f.Days
		if len(days) == 7 {
			days = nil
		}
		p.Set("fasting_days", days)
	}
	if err := app.Save(p); err != nil {
		r.PlanError = "The plan from Lose It! doesn't fit: " + err.Error()
		return nil
	}
	r.Plan = true
	return nil
}
