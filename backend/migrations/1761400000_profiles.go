package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// The choices the onboarding interview offers. The web app shows them in this
// order (web/src/lib/profile.ts has their labels).
var (
	activities  = []string{"sedentary", "light", "moderate", "active", "very_active"}
	fastings    = []string{"none", "12_12", "14_10", "16_8", "18_6", "20_4", "omad"}
	motivations = []string{"health", "energy", "confidence", "fitness", "event", "mood", "family", "longevity"}
)

// Each person's plan: their answers to the onboarding interview and the daily
// targets worked out from them (backend/app/plan.go). It is kept apart from the
// users record because members can see each other, and this is health data
// only its owner should see.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}

		profiles := core.NewBaseCollection("profiles")
		profiles.Fields.Add(
			&core.RelationField{Name: "user", CollectionId: users.Id, Required: true, MaxSelect: 1, CascadeDelete: true},
			&core.SelectField{Name: "goal", Values: []string{"lose", "maintain", "gain"}, MaxSelect: 1},
			&core.SelectField{Name: "sex", Values: []string{"female", "male"}, MaxSelect: 1},
			&core.TextField{Name: "birth_date", Pattern: `^\d{4}-\d{2}-\d{2}$`},
			&core.NumberField{Name: "height_cm", Min: types.Pointer(100.0), Max: types.Pointer(250.0)},
			// the latest weight; weigh-ins keep it up to date
			&core.NumberField{Name: "weight_kg", Min: types.Pointer(30.0), Max: types.Pointer(350.0)},
			&core.NumberField{Name: "goal_weight_kg", Min: types.Pointer(30.0), Max: types.Pointer(350.0)},
			&core.SelectField{Name: "activity", Values: activities, MaxSelect: 1},
			// kilograms to lose or gain a week
			&core.NumberField{Name: "pace_kg", Min: types.Pointer(0.0), Max: types.Pointer(1.0)},
			&core.SelectField{Name: "fasting", Values: fastings, MaxSelect: 1},
			// when the eating window opens, HH:MM
			&core.TextField{Name: "eating_start", Pattern: `^([01]\d|2[0-3]):[0-5]\d$`},
			&core.SelectField{Name: "motivations", Values: motivations, MaxSelect: len(motivations)},
			&core.TextField{Name: "why", Max: 500},
			&core.SelectField{Name: "diet", Values: []string{"balanced", "high_protein", "low_carb"}, MaxSelect: 1},
			&core.SelectField{Name: "units", Values: []string{"metric", "imperial"}, MaxSelect: 1},
			&core.BoolField{Name: "onboarded"},
			// worked out by the server whenever the answers change
			&core.NumberField{Name: "calorie_budget", OnlyInt: true},
			&core.NumberField{Name: "protein_g", OnlyInt: true},
			&core.NumberField{Name: "carbs_g", OnlyInt: true},
			&core.NumberField{Name: "fat_g", OnlyInt: true},
			&core.TextField{Name: "goal_date"},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		profiles.AddIndex("idx_profiles_user", true, "user", "")
		own := types.Pointer(`user = @request.auth.id`)
		profiles.ListRule, profiles.ViewRule, profiles.DeleteRule = own, own, own
		profiles.CreateRule = types.Pointer(`@request.auth.collectionName = "users" && @request.body.user = @request.auth.id`)
		profiles.UpdateRule = types.Pointer(`user = @request.auth.id && (@request.body.user:isset = false || @request.body.user = @request.auth.id)`)
		return app.Save(profiles)
	}, func(app core.App) error {
		profiles, err := app.FindCollectionByNameOrId("profiles")
		if err != nil {
			return err
		}
		return app.Delete(profiles)
	})
}
