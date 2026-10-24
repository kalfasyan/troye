package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// The kinds of exercise the app offers; backend/app/extras.go has how hard each is.
var exerciseKinds = []string{"walk", "dog_walk", "run", "cycle", "swim", "strength", "yoga", "hike", "sport", "other"}

// Water drunk each day, and exercise done. Both are private to their owner.
// The calories an exercise burns are worked out on the server from its kind,
// minutes and the person's weight, unless they enter them.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		own := types.Pointer(`user = @request.auth.id`)
		create := types.Pointer(`@request.auth.collectionName = "users" && @request.body.user = @request.auth.id`)
		update := types.Pointer(`user = @request.auth.id && (@request.body.user:isset = false || @request.body.user = @request.auth.id)`)

		water := core.NewBaseCollection("water")
		water.Fields.Add(
			&core.RelationField{Name: "user", CollectionId: users.Id, Required: true, MaxSelect: 1, CascadeDelete: true},
			&core.TextField{Name: "date", Required: true, Pattern: `^\d{4}-\d{2}-\d{2}$`},
			&core.NumberField{Name: "ml", Min: types.Pointer(0.0), Max: types.Pointer(20000.0), OnlyInt: true},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		water.AddIndex("idx_water_user_date", true, "user, date", "")
		water.ListRule, water.ViewRule, water.DeleteRule = own, own, own
		water.CreateRule, water.UpdateRule = create, update
		if err := app.Save(water); err != nil {
			return err
		}

		exercises := core.NewBaseCollection("exercises")
		exercises.Fields.Add(
			&core.RelationField{Name: "user", CollectionId: users.Id, Required: true, MaxSelect: 1, CascadeDelete: true},
			&core.TextField{Name: "date", Required: true, Pattern: `^\d{4}-\d{2}-\d{2}$`},
			&core.SelectField{Name: "kind", Required: true, Values: exerciseKinds, MaxSelect: 1},
			// what the person called it, for "other"
			&core.TextField{Name: "name", Max: 100},
			&core.NumberField{Name: "minutes", Required: true, Min: types.Pointer(1.0), Max: types.Pointer(1440.0), OnlyInt: true},
			// calories burned; worked out when left at 0
			&core.NumberField{Name: "kcal", Min: types.Pointer(0.0), Max: types.Pointer(10000.0), OnlyInt: true},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		exercises.AddIndex("idx_exercises_user_date", false, "user, date", "")
		exercises.ListRule, exercises.ViewRule, exercises.DeleteRule = own, own, own
		exercises.CreateRule, exercises.UpdateRule = create, update
		return app.Save(exercises)
	}, func(app core.App) error {
		for _, name := range []string{"exercises", "water"} {
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
