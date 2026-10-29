package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"

	"github.com/selfhostit/troye/foods"
)

// USDA FoodData Central replaces Ciqual as the generic food table that ships
// with the app. Ciqual foods nobody logged or taught the photo analyser go;
// the others stay so the diary keeps them. A new USDA edition gets a migration like this one, which
// updates the foods it already has and adds the new ones.
func init() {
	m.Register(func(app core.App) error {
		if _, err := app.DB().NewQuery(`DELETE FROM foods WHERE source = 'ciqual'
			AND id NOT IN (SELECT food FROM entries WHERE food != '')
			AND id NOT IN (SELECT food FROM food_aliases)`).Execute(); err != nil {
			return err
		}
		if _, err := app.DB().Delete(foods.TablesCollection, dbx.HashExp{"name": "ciqual"}).Execute(); err != nil {
			return err
		}
		tables, err := app.FindCollectionByNameOrId(foods.TablesCollection)
		if err != nil {
			return err
		}
		tables.Fields.GetByName("name").(*core.SelectField).Values = []string{"usda", "nevo", "off"}
		if err := app.Save(tables); err != nil {
			return err
		}
		return foods.ImportUSDA(app)
	}, nil)
}
