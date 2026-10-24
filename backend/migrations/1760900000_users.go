package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Users get a role. The owner (created by first-run setup) manages everyone
// else; members can see each other and edit themselves, but not their role.
// Nobody can sign themselves up.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		users.Fields.Add(&core.SelectField{
			Name:      "role",
			Values:    []string{"owner", "member"},
			MaxSelect: 1,
		})
		users.ListRule = types.Pointer(`@request.auth.id != ""`)
		users.ViewRule = types.Pointer(`@request.auth.id != ""`)
		users.CreateRule = types.Pointer(`@request.auth.role = "owner"`)
		users.UpdateRule = types.Pointer(`(id = @request.auth.id && @request.body.role:isset = false) || @request.auth.role = "owner"`)
		users.DeleteRule = types.Pointer(`@request.auth.role = "owner" && id != @request.auth.id`)
		return app.Save(users)
	}, func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		users.Fields.RemoveByName("role")
		self := types.Pointer("id = @request.auth.id")
		users.ListRule, users.ViewRule, users.UpdateRule, users.DeleteRule = self, self, self, self
		users.CreateRule = types.Pointer("")
		return app.Save(users)
	})
}
