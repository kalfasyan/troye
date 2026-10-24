package migrations

// DEMO: the items list that shows every part of plasto working together.
// scripts/strip-demo.sh removes it; keep your own migrations in other files.

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		items := core.NewBaseCollection("items")
		items.Fields.Add(
			&core.TextField{Name: "title", Required: true, Max: 200},
			// who added it, set by the server (backend/app/demo.go)
			&core.TextField{Name: "by", Max: 100},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		signedIn := types.Pointer(`@request.auth.id != ""`)
		items.ListRule, items.ViewRule, items.CreateRule = signedIn, signedIn, signedIn
		items.DeleteRule = types.Pointer(`@request.auth.collectionName = "users"`)
		return app.Save(items)
	}, func(app core.App) error {
		items, err := app.FindCollectionByNameOrId("items")
		if err != nil {
			return err
		}
		return app.Delete(items)
	})
}
