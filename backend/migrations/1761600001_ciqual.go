package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"

	"github.com/selfhostit/troye/foods"
)

// The Ciqual foods that ship with the app. A new edition gets a migration like
// this one, which updates the foods it already has and adds the new ones.
func init() {
	m.Register(func(app core.App) error {
		return foods.ImportCiqual(app)
	}, nil)
}
