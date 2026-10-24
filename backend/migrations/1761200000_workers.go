package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Workers are accounts for programs, such as the Python workers in workers/.
// The owner adds one in the admin UI with a name and password, and the worker
// signs in with them. Change the password to rotate it; delete it to revoke it.
// API rules can tell workers apart with @request.auth.collectionName = "workers".
func init() {
	m.Register(func(app core.App) error {
		workers := core.NewAuthCollection("workers")
		workers.Fields.Add(
			&core.TextField{Name: "name", Required: true, Max: 100, Pattern: `^[a-z0-9][a-z0-9_-]*$`},
			&core.DateField{Name: "last_seen"},
		)
		if email, ok := workers.Fields.GetByName("email").(*core.EmailField); ok {
			email.Required = false
		}
		workers.AddIndex("idx_workers_name", true, "name", "")
		workers.PasswordAuth.IdentityFields = []string{"name"}
		// only superusers manage workers; a worker can't even read itself
		workers.ListRule, workers.ViewRule, workers.CreateRule, workers.UpdateRule, workers.DeleteRule = nil, nil, nil, nil, nil
		return app.Save(workers)
	}, func(app core.App) error {
		workers, err := app.FindCollectionByNameOrId("workers")
		if err != nil {
			return err
		}
		return app.Delete(workers)
	})
}
