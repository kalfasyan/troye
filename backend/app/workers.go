package app

import (
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// Workers report in every minute, so the admin UI shows when each one was
// last seen (the last_seen field of the workers collection).
func registerWorkerRoutes(se *core.ServeEvent) {
	se.Router.POST("/api/app/workers/heartbeat", func(e *core.RequestEvent) error {
		e.Auth.Set("last_seen", time.Now())
		if err := e.App.Save(e.Auth); err != nil {
			return e.InternalServerError("", err)
		}
		return e.NoContent(http.StatusNoContent)
	}).Bind(apis.RequireAuth("workers"))
}
