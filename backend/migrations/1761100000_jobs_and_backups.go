package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// The last run of each scheduled job (backend/jobs), readable by superusers in
// the admin UI, and nightly backups turned on unless already configured.
func init() {
	m.Register(func(app core.App) error {
		jobs := core.NewBaseCollection("jobs")
		jobs.Fields.Add(
			&core.TextField{Name: "name", Required: true, Max: 100},
			&core.DateField{Name: "last_run"},
			&core.NumberField{Name: "last_duration_ms", OnlyInt: true},
			&core.TextField{Name: "last_error", Max: 2000},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		jobs.AddIndex("idx_jobs_name", true, "name", "")
		if err := app.Save(jobs); err != nil {
			return err
		}

		// a backup of pb_data at 03:00 every night, keeping the last 7; change
		// it in the admin UI under Settings > Backups
		settings := app.Settings()
		if settings.Backups.Cron == "" {
			settings.Backups.Cron = "0 3 * * *"
			settings.Backups.CronMaxKeep = 7
			return app.Save(settings)
		}
		return nil
	}, func(app core.App) error {
		jobs, err := app.FindCollectionByNameOrId("jobs")
		if err != nil {
			return err
		}
		return app.Delete(jobs)
	})
}
