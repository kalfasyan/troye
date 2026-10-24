package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Food tables: foods get their Dutch and French names and other names they are
// known by, so a search in any of the three languages finds them, and
// food_tables records which edition of each table was imported and when
// (backend/foods). Only the server writes food_tables.
func init() {
	m.Register(func(app core.App) error {
		foods, err := app.FindCollectionByNameOrId("foods")
		if err != nil {
			return err
		}
		foods.Fields.Add(
			&core.TextField{Name: "name_nl", Max: 200},
			&core.TextField{Name: "name_fr", Max: 200},
			// synonyms and other names, separated by semicolons
			&core.TextField{Name: "aliases", Max: 1000},
			// the table's food group, such as "Bread and rusks"
			&core.TextField{Name: "food_group", Max: 100},
			// every name and the brand, lowercased without accents, for search
			&core.TextField{Name: "search", Max: 2000, Hidden: true},
		)
		if err := app.Save(foods); err != nil {
			return err
		}

		tables := core.NewBaseCollection("food_tables")
		tables.Fields.Add(
			&core.SelectField{Name: "name", Required: true, Values: []string{"ciqual", "nevo", "off"}, MaxSelect: 1},
			&core.TextField{Name: "version", Max: 100},
			&core.NumberField{Name: "foods", OnlyInt: true},
			&core.DateField{Name: "imported"},
			// set while an import runs, and to the error when one fails
			&core.BoolField{Name: "running"},
			&core.TextField{Name: "error", Max: 500},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		tables.AddIndex("idx_food_tables_name", true, "name", "")
		tables.ListRule = types.Pointer(`@request.auth.collectionName = "users"`)
		tables.ViewRule = tables.ListRule
		return app.Save(tables)
	}, func(app core.App) error {
		tables, err := app.FindCollectionByNameOrId("food_tables")
		if err != nil {
			return err
		}
		if err := app.Delete(tables); err != nil {
			return err
		}
		foods, err := app.FindCollectionByNameOrId("foods")
		if err != nil {
			return err
		}
		for _, name := range []string{"name_nl", "name_fr", "aliases", "food_group", "search"} {
			foods.Fields.RemoveByName(name)
		}
		return app.Save(foods)
	})
}
