package app

import (
	"errors"
	"math"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/selfhostit/troye/foods"
)

// The food diary: entries get their totals from their food, and add-ons (milk
// in a coffee) follow the entry they belong to. Food search, barcodes and the
// food tables are in backend/foods.

func registerDiary(app core.App) {
	app.OnRecordCreate("entries").BindFunc(func(e *core.RecordEvent) error {
		if err := fitAddOn(e.App, e.Record); err != nil {
			return err
		}
		if err := fillEntry(e.App, e.Record); err != nil {
			return err
		}
		return e.Next()
	})
	app.OnRecordUpdate("entries").BindFunc(func(e *core.RecordEvent) error {
		if err := fitAddOn(e.App, e.Record); err != nil {
			return err
		}
		if err := fillEntry(e.App, e.Record); err != nil {
			return err
		}
		if err := e.Next(); err != nil {
			return err
		}
		return moveAddOns(e.App, e.Record)
	})
}

// fitAddOn checks that an add-on hangs off one of the same person's entries,
// one level deep, and puts it on its parent's day and meal.
func fitAddOn(app core.App, entry *core.Record) error {
	parentID := entry.GetString("parent")
	if parentID == "" {
		return nil
	}
	if parentID == entry.Id {
		return errors.New("an entry can't be its own add-on")
	}
	parent, err := app.FindRecordById("entries", parentID)
	if err != nil || parent.GetString("user") != entry.GetString("user") {
		return errors.New("that entry isn't in your diary")
	}
	if parent.GetString("parent") != "" {
		return errors.New("an add-on can't have add-ons of its own")
	}
	if !entry.IsNew() {
		n, err := app.CountRecords("entries", dbx.HashExp{"parent": entry.Id})
		if err != nil {
			return err
		}
		if n > 0 {
			return errors.New("an entry with add-ons can't be an add-on")
		}
	}
	entry.Set("date", parent.GetString("date"))
	entry.Set("meal", parent.GetString("meal"))
	return nil
}

// moveAddOns keeps an entry's add-ons on its day and meal when it moves.
func moveAddOns(app core.App, entry *core.Record) error {
	addOns, err := app.FindAllRecords("entries", dbx.HashExp{"parent": entry.Id})
	if err != nil {
		return err
	}
	for _, a := range addOns {
		if a.GetString("date") == entry.GetString("date") && a.GetString("meal") == entry.GetString("meal") {
			continue
		}
		a.Set("date", entry.GetString("date"))
		a.Set("meal", entry.GetString("meal"))
		if err := app.Save(a); err != nil {
			return err
		}
	}
	return nil
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
		lang := ""
		if p, err := app.FindFirstRecordByFilter("profiles", "user = {:user}", dbx.Params{"user": entry.GetString("user")}); err == nil {
			lang = p.GetString("food_language")
		}
		entry.Set("name", foods.Name(food, lang))
	}
	factor := entry.GetFloat("grams") / 100
	for _, field := range []string{"kcal", "protein", "carbs", "fat", "fiber", "sugar", "salt"} {
		v := food.GetFloat(field) * factor
		switch field {
		case "kcal":
			v = math.Round(v)
		case "salt":
			v = math.Round(v*100) / 100
		default:
			v = math.Round(v*10) / 10
		}
		entry.Set(field, v)
	}
	return nil
}
