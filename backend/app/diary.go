package app

import (
	"errors"
	"math"

	"github.com/pocketbase/pocketbase/core"
)

// The food diary: entries get their totals from their food. Food search,
// barcodes and the food tables are in backend/foods.

func registerDiary(app core.App) {
	app.OnRecordCreate("entries").BindFunc(func(e *core.RecordEvent) error {
		if err := fillEntry(e.App, e.Record); err != nil {
			return err
		}
		return e.Next()
	})
	app.OnRecordUpdate("entries").BindFunc(func(e *core.RecordEvent) error {
		if err := fillEntry(e.App, e.Record); err != nil {
			return err
		}
		return e.Next()
	})
}

// fillEntry works out an entry's totals from its food and grams. An entry
// without a food is a quick add, and keeps the calories it was given.
func fillEntry(app core.App, entry *core.Record) error {
	foodID := entry.GetString("food")
	if foodID == "" {
		return nil
	}
	food, err := app.FindRecordById("foods", foodID)
	if err != nil {
		return errors.New("that food no longer exists")
	}
	if owner := food.GetString("owner"); owner != "" && owner != entry.GetString("user") {
		return errors.New("that food belongs to someone else")
	}
	if entry.GetString("name") == "" {
		entry.Set("name", food.GetString("name"))
	}
	factor := entry.GetFloat("grams") / 100
	for _, field := range []string{"kcal", "protein", "carbs", "fat"} {
		v := food.GetFloat(field) * factor
		if field == "kcal" {
			v = math.Round(v)
		} else {
			v = math.Round(v*10) / 10
		}
		entry.Set(field, v)
	}
	return nil
}
