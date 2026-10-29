package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// This imported the Ciqual table, which Troye no longer ships; USDA
// FoodData Central took its place in 1762200000_usda.go.
func init() {
	m.Register(func(app core.App) error { return nil }, nil)
}
