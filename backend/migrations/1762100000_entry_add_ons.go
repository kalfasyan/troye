package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Add-ons: an entry can hang off another one, like milk in a coffee or an egg
// on a noodle soup. It is an ordinary entry with its own food and amount, and
// it goes with its parent: removing the coffee removes the milk, and moving it
// to another meal moves the milk too (backend/app/diary.go).
func init() {
	m.Register(func(app core.App) error {
		entries, err := app.FindCollectionByNameOrId("entries")
		if err != nil {
			return err
		}
		entries.Fields.Add(&core.RelationField{Name: "parent", CollectionId: entries.Id, MaxSelect: 1, CascadeDelete: true})
		entries.AddIndex("idx_entries_parent", false, "parent", "parent != ''")
		return app.Save(entries)
	}, func(app core.App) error {
		entries, err := app.FindCollectionByNameOrId("entries")
		if err != nil {
			return err
		}
		entries.RemoveIndex("idx_entries_parent")
		entries.Fields.RemoveByName("parent")
		return app.Save(entries)
	})
}
