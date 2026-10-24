package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// The language a person sees food names in: English unless they choose Dutch
// or French. A food without a name in their language shows its English name.
func init() {
	m.Register(func(app core.App) error {
		profiles, err := app.FindCollectionByNameOrId("profiles")
		if err != nil {
			return err
		}
		profiles.Fields.Add(&core.SelectField{Name: "food_language", Values: []string{"en", "nl", "fr"}, MaxSelect: 1})
		return app.Save(profiles)
	}, func(app core.App) error {
		profiles, err := app.FindCollectionByNameOrId("profiles")
		if err != nil {
			return err
		}
		profiles.Fields.RemoveByName("food_language")
		return app.Save(profiles)
	})
}
