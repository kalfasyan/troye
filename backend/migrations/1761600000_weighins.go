package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Weigh-ins: one weight a day for each person, which only they can see.
// Weighing again the same day replaces it. backend/app/weight.go keeps the
// profile's weight on the latest one, so the daily budget follows.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}

		weighins := core.NewBaseCollection("weighins")
		weighins.Fields.Add(
			&core.RelationField{Name: "user", CollectionId: users.Id, Required: true, MaxSelect: 1, CascadeDelete: true},
			&core.TextField{Name: "date", Required: true, Pattern: `^\d{4}-\d{2}-\d{2}$`},
			&core.NumberField{Name: "weight_kg", Required: true, Min: types.Pointer(30.0), Max: types.Pointer(350.0)},
			&core.TextField{Name: "note", Max: 200},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		weighins.AddIndex("idx_weighins_user_date", true, "user, date", "")
		own := types.Pointer(`user = @request.auth.id`)
		weighins.ListRule, weighins.ViewRule, weighins.DeleteRule = own, own, own
		weighins.CreateRule = types.Pointer(`@request.auth.collectionName = "users" && @request.body.user = @request.auth.id`)
		weighins.UpdateRule = types.Pointer(`user = @request.auth.id && (@request.body.user:isset = false || @request.body.user = @request.auth.id)`)
		return app.Save(weighins)
	}, func(app core.App) error {
		weighins, err := app.FindCollectionByNameOrId("weighins")
		if err != nil {
			return err
		}
		return app.Delete(weighins)
	})
}
