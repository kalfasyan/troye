package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// The food diary. Foods hold nutrition values per 100 g: shared ones come from
// the food tables and Open Food Facts, and custom ones belong to the person who
// made them. Entries are what someone ate on a day, one per food and meal, and
// only they can see them. backend/app/diary.go works out each entry's totals.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}

		per100g := func(name string) *core.NumberField {
			return &core.NumberField{Name: name, Min: types.Pointer(0.0), Max: types.Pointer(1000.0)}
		}
		foods := core.NewBaseCollection("foods")
		foods.Fields.Add(
			&core.TextField{Name: "name", Required: true, Max: 200},
			&core.TextField{Name: "brand", Max: 200},
			&core.SelectField{Name: "source", Required: true, Values: []string{"custom", "off", "ciqual", "nevo", "usda"}, MaxSelect: 1},
			// the food's id in its source table, or its barcode for Open Food Facts
			&core.TextField{Name: "source_id", Max: 50},
			&core.TextField{Name: "barcode", Max: 32, Pattern: `^\d{6,14}$`},
			per100g("kcal"),
			per100g("protein"),
			per100g("carbs"),
			per100g("fat"),
			per100g("fiber"),
			per100g("sugar"),
			per100g("salt"),
			// a typical serving, such as "1 slice" of 30 g
			&core.NumberField{Name: "serving_g", Min: types.Pointer(0.0), Max: types.Pointer(5000.0)},
			&core.TextField{Name: "serving_label", Max: 50},
			// set for custom foods only
			&core.RelationField{Name: "owner", CollectionId: users.Id, MaxSelect: 1, CascadeDelete: true},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		foods.AddIndex("idx_foods_source", true, "source, source_id", "source_id != ''")
		foods.AddIndex("idx_foods_barcode", false, "barcode", "")
		foods.AddIndex("idx_foods_name", false, "name", "")
		foods.ListRule = types.Pointer(`@request.auth.id != "" && (owner = "" || owner = @request.auth.id)`)
		foods.ViewRule = foods.ListRule
		foods.CreateRule = types.Pointer(`@request.auth.collectionName = "workers" || (@request.auth.collectionName = "users" && @request.body.owner = @request.auth.id && @request.body.source = "custom")`)
		foods.UpdateRule = types.Pointer(`@request.auth.collectionName = "workers" || (owner = @request.auth.id && (@request.body.owner:isset = false || @request.body.owner = @request.auth.id) && (@request.body.source:isset = false || @request.body.source = "custom"))`)
		foods.DeleteRule = types.Pointer(`owner = @request.auth.id`)
		if err := app.Save(foods); err != nil {
			return err
		}

		entries := core.NewBaseCollection("entries")
		entries.Fields.Add(
			&core.RelationField{Name: "user", CollectionId: users.Id, Required: true, MaxSelect: 1, CascadeDelete: true},
			&core.TextField{Name: "date", Required: true, Pattern: `^\d{4}-\d{2}-\d{2}$`},
			&core.SelectField{Name: "meal", Required: true, Values: []string{"breakfast", "lunch", "dinner", "snacks"}, MaxSelect: 1},
			// empty for a quick add of just calories
			&core.RelationField{Name: "food", CollectionId: foods.Id, MaxSelect: 1},
			// a copy of the food's name, so the diary keeps it if the food changes
			&core.TextField{Name: "name", Required: true, Max: 200},
			&core.NumberField{Name: "grams", Min: types.Pointer(0.0), Max: types.Pointer(5000.0)},
			// totals for this entry, worked out from the food and the grams
			&core.NumberField{Name: "kcal", Min: types.Pointer(0.0), Max: types.Pointer(20000.0)},
			&core.NumberField{Name: "protein", Min: types.Pointer(0.0), Max: types.Pointer(5000.0)},
			&core.NumberField{Name: "carbs", Min: types.Pointer(0.0), Max: types.Pointer(5000.0)},
			&core.NumberField{Name: "fat", Min: types.Pointer(0.0), Max: types.Pointer(5000.0)},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		entries.AddIndex("idx_entries_user_date", false, "user, date", "")
		own := types.Pointer(`user = @request.auth.id`)
		entries.ListRule, entries.ViewRule, entries.DeleteRule = own, own, own
		entries.CreateRule = types.Pointer(`@request.auth.collectionName = "users" && @request.body.user = @request.auth.id`)
		entries.UpdateRule = types.Pointer(`user = @request.auth.id && (@request.body.user:isset = false || @request.body.user = @request.auth.id)`)
		return app.Save(entries)
	}, func(app core.App) error {
		for _, name := range []string{"entries", "foods"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			if err := app.Delete(c); err != nil {
				return err
			}
		}
		return nil
	})
}
