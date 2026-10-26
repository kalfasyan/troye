package migrations

import (
	"slices"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// What a Lose It! import brings that Troye didn't keep (backend/app/loseit.go):
//
//   - entries get fiber, sugar, saturated fat, salt and cholesterol, and the
//     amount as it was logged ("2 Tablespoons") when it wasn't in grams
//   - exercises can be an activity tracker's extra calories, without minutes
//   - fasting can be on some days of the week only
//   - imported entries and exercises remember the row they came from, so
//     importing a newer export only adds what's new
func init() {
	m.Register(func(app core.App) error {
		entries, err := app.FindCollectionByNameOrId("entries")
		if err != nil {
			return err
		}
		grams := func(name string) *core.NumberField {
			return &core.NumberField{Name: name, Min: types.Pointer(0.0), Max: types.Pointer(5000.0)}
		}
		entries.Fields.Add(
			&core.TextField{Name: "amount", Max: 50},
			grams("fiber"),
			grams("sugar"),
			grams("sat_fat"),
			grams("salt"),
			// milligrams
			&core.NumberField{Name: "cholesterol", Min: types.Pointer(0.0), Max: types.Pointer(50000.0)},
			&core.TextField{Name: "import_key", Max: 64},
		)
		entries.AddIndex("idx_entries_user_import", true, "user, import_key", "import_key != ''")
		if err := app.Save(entries); err != nil {
			return err
		}

		exercises, err := app.FindCollectionByNameOrId("exercises")
		if err != nil {
			return err
		}
		kind := exercises.Fields.GetByName("kind").(*core.SelectField)
		kind.Values = append(slices.Clone(kind.Values), "tracker")
		minutes := exercises.Fields.GetByName("minutes").(*core.NumberField)
		minutes.Required, minutes.Min = false, types.Pointer(0.0)
		exercises.Fields.Add(&core.TextField{Name: "import_key", Max: 64})
		exercises.AddIndex("idx_exercises_user_import", true, "user, import_key", "import_key != ''")
		if err := app.Save(exercises); err != nil {
			return err
		}

		profiles, err := app.FindCollectionByNameOrId("profiles")
		if err != nil {
			return err
		}
		// the days a fast starts on; none chosen is every day
		profiles.Fields.Add(&core.SelectField{Name: "fasting_days", Values: weekdays, MaxSelect: len(weekdays)})
		return app.Save(profiles)
	}, func(app core.App) error {
		entries, err := app.FindCollectionByNameOrId("entries")
		if err != nil {
			return err
		}
		entries.RemoveIndex("idx_entries_user_import")
		for _, name := range []string{"amount", "fiber", "sugar", "sat_fat", "salt", "cholesterol", "import_key"} {
			entries.Fields.RemoveByName(name)
		}
		if err := app.Save(entries); err != nil {
			return err
		}

		exercises, err := app.FindCollectionByNameOrId("exercises")
		if err != nil {
			return err
		}
		exercises.RemoveIndex("idx_exercises_user_import")
		exercises.Fields.RemoveByName("import_key")
		exercises.Fields.GetByName("kind").(*core.SelectField).Values = exerciseKinds
		minutes := exercises.Fields.GetByName("minutes").(*core.NumberField)
		minutes.Required, minutes.Min = true, types.Pointer(1.0)
		if err := app.Save(exercises); err != nil {
			return err
		}

		profiles, err := app.FindCollectionByNameOrId("profiles")
		if err != nil {
			return err
		}
		profiles.Fields.RemoveByName("fasting_days")
		return app.Save(profiles)
	})
}

var weekdays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
