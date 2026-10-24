package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Devices that get push notifications. There are no API rules, so only the
// server reads them (through /api/app/push, see backend/push).
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		subs := core.NewBaseCollection("push_subscriptions")
		subs.Fields.Add(
			&core.RelationField{Name: "user", Required: true, MaxSelect: 1, CollectionId: users.Id, CascadeDelete: true},
			&core.TextField{Name: "endpoint", Required: true, Max: 2000},
			&core.TextField{Name: "p256dh", Required: true, Max: 200},
			&core.TextField{Name: "auth", Required: true, Max: 200},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		subs.AddIndex("idx_push_subscriptions_endpoint", true, "endpoint", "")
		subs.AddIndex("idx_push_subscriptions_user", false, "user", "")
		return app.Save(subs)
	}, func(app core.App) error {
		subs, err := app.FindCollectionByNameOrId("push_subscriptions")
		if err != nil {
			return err
		}
		return app.Delete(subs)
	})
}
