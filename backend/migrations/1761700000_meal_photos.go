package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Meal photos waiting for, or done with, the photo analyser. The phone uploads
// a shrunken copy without its location; the analyser on the GPU PC claims it,
// names what it sees and how many grams, and backend/app/photos.go matches each
// item to a food so the person can check the draft and log it.
//
// status: queued -> analysing -> ready -> logged, or failed.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}

		photos := core.NewBaseCollection("meal_photos")
		photos.Fields.Add(
			&core.RelationField{Name: "user", CollectionId: users.Id, Required: true, MaxSelect: 1, CascadeDelete: true},
			&core.TextField{Name: "date", Required: true, Pattern: `^\d{4}-\d{2}-\d{2}$`},
			&core.SelectField{Name: "meal", Required: true, Values: []string{"breakfast", "lunch", "dinner", "snacks"}, MaxSelect: 1},
			// only with a file token, so a leaked link doesn't show anyone's meal
			&core.FileField{Name: "photo", Required: true, MaxSelect: 1, MaxSize: 8 << 20, Protected: true,
				MimeTypes: []string{"image/jpeg", "image/webp", "image/png"}, Thumbs: []string{"480x480f"}},
			// the SHA-256 of the picked file, so the same photo isn't queued twice
			&core.TextField{Name: "hash", Max: 64},
			// a hint for the analyser, such as "no dressing"
			&core.TextField{Name: "note", Max: 200},
			&core.SelectField{Name: "status", Required: true, Values: []string{"queued", "analysing", "ready", "logged", "failed"}, MaxSelect: 1},
			// the analyser's draft: [{name, grams, confidence, food, kcal, ...}]
			&core.JSONField{Name: "items", MaxSize: 64 << 10},
			// what the person logged in the end, to score the analyser on
			&core.JSONField{Name: "confirmed", MaxSize: 64 << 10},
			&core.TextField{Name: "model", Max: 100},
			&core.TextField{Name: "error", Max: 500},
			&core.DateField{Name: "claimed_at"},
			&core.DateField{Name: "analysed_at"},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		photos.AddIndex("idx_meal_photos_user_hash", true, "user, hash", "hash != ''")
		photos.AddIndex("idx_meal_photos_status", false, "status, created", "")
		own := `user = @request.auth.id`
		photos.ListRule = types.Pointer(own + ` || @request.auth.collectionName = "workers"`)
		photos.ViewRule = photos.ListRule
		photos.CreateRule = types.Pointer(`@request.auth.collectionName = "users" && @request.body.user = @request.auth.id`)
		// people change their own photos (the status is guarded in photos.go);
		// the analyser writes drafts through /api/app/photos
		photos.UpdateRule = types.Pointer(own + ` && (@request.body.user:isset = false || @request.body.user = @request.auth.id)`)
		photos.DeleteRule = types.Pointer(own)
		if err := app.Save(photos); err != nil {
			return err
		}

		// What the analyser's names turned out to be: "fries" logged as a
		// particular food, and how much. The next photo starts from it.
		foods, err := app.FindCollectionByNameOrId("foods")
		if err != nil {
			return err
		}
		aliases := core.NewBaseCollection("food_aliases")
		aliases.Fields.Add(
			&core.RelationField{Name: "user", CollectionId: users.Id, Required: true, MaxSelect: 1, CascadeDelete: true},
			// the analyser's name, lower case
			&core.TextField{Name: "alias", Required: true, Max: 200},
			&core.RelationField{Name: "food", CollectionId: foods.Id, Required: true, MaxSelect: 1, CascadeDelete: true},
			// the grams logged last time
			&core.NumberField{Name: "grams", Min: types.Pointer(0.0), Max: types.Pointer(5000.0)},
			&core.NumberField{Name: "uses", OnlyInt: true},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		aliases.AddIndex("idx_food_aliases_user_alias", true, "user, alias", "")
		// kept by the server; people can see and remove theirs
		aliases.ListRule = types.Pointer(own)
		aliases.ViewRule = aliases.ListRule
		aliases.DeleteRule = aliases.ListRule
		return app.Save(aliases)
	}, func(app core.App) error {
		for _, name := range []string{"food_aliases", "meal_photos"} {
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
