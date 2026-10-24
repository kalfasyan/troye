package app

import (
	"net/http"
	"strings"
	"sync"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// First-run setup: until a user exists, the web app shows a setup screen
// instead of the sign-in form, so nobody needs a terminal or the admin UI to
// get started. That first user becomes the owner and also gets an admin UI
// account. Once anyone exists the endpoint refuses, so it only ever runs once.

type setupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

var setupMu sync.Mutex

func setupNeeded(app core.App) (bool, error) {
	n, err := app.CountRecords("users")
	return n == 0, err
}

func registerSetupRoutes(se *core.ServeEvent) {
	se.Router.GET("/api/app/setup", func(e *core.RequestEvent) error {
		needed, err := setupNeeded(e.App)
		if err != nil {
			return e.InternalServerError("", err)
		}
		return e.JSON(http.StatusOK, map[string]bool{"needed": needed})
	})

	se.Router.POST("/api/app/setup", func(e *core.RequestEvent) error {
		var body setupRequest
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Fill in your name, email and password.", err)
		}
		body.Name = strings.TrimSpace(body.Name)
		body.Email = strings.ToLower(strings.TrimSpace(body.Email))
		if body.Name == "" || body.Email == "" {
			return e.BadRequestError("Fill in your name and email.", nil)
		}
		if len(body.Password) < 8 {
			return e.BadRequestError("The password needs at least 8 characters.", nil)
		}

		setupMu.Lock()
		defer setupMu.Unlock()
		needed, err := setupNeeded(e.App)
		if err != nil {
			return e.InternalServerError("", err)
		}
		if !needed {
			return e.ForbiddenError("This app is already set up.", nil)
		}

		if err := e.App.RunInTransaction(func(tx core.App) error { return createOwner(tx, body) }); err != nil {
			return e.BadRequestError("Could not save: "+err.Error(), err)
		}
		e.App.Logger().Info("First-run setup done", "owner", body.Email)
		return e.NoContent(http.StatusNoContent)
	})
}

// createOwner adds the owner and, unless a real one exists already, an admin
// UI account with the same email and password.
func createOwner(tx core.App, req setupRequest) error {
	users, err := tx.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}
	owner := core.NewRecord(users)
	owner.Set("name", req.Name)
	owner.Set("role", "owner")
	owner.SetEmail(req.Email)
	owner.SetPassword(req.Password)
	owner.SetVerified(true)
	if err := tx.Save(owner); err != nil {
		return err
	}

	// PocketBase keeps a placeholder installer account until a real admin
	// exists; saving one removes it.
	admins, err := tx.CountRecords(core.CollectionNameSuperusers,
		dbx.Not(dbx.HashExp{"email": core.DefaultInstallerEmail}))
	if err != nil || admins > 0 {
		return err
	}
	superusers, err := tx.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		return err
	}
	admin := core.NewRecord(superusers)
	admin.SetEmail(req.Email)
	admin.SetPassword(req.Password)
	return tx.Save(admin)
}
