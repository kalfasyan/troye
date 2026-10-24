package push

import (
	"net/http"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

type subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// RegisterRoutes adds the endpoints the web app uses to turn notifications on
// and off for the signed-in user, under /api/app/push.
func RegisterRoutes(se *core.ServeEvent) {
	g := se.Router.Group("/api/app/push")
	g.Bind(apis.RequireAuth("users"))

	g.GET("/key", func(e *core.RequestEvent) error {
		key, err := PublicKey(e.App)
		if err != nil {
			return e.InternalServerError("", err)
		}
		return e.JSON(http.StatusOK, map[string]string{"key": key})
	})

	g.POST("/subscribe", func(e *core.RequestEvent) error {
		var body subscription
		if err := e.BindBody(&body); err != nil || body.Endpoint == "" || body.Keys.P256dh == "" || body.Keys.Auth == "" {
			return e.BadRequestError("Missing subscription.", err)
		}
		// a device that was used by someone else before now belongs to this user
		rec, err := e.App.FindFirstRecordByData(Collection, "endpoint", body.Endpoint)
		if err != nil {
			col, err := e.App.FindCollectionByNameOrId(Collection)
			if err != nil {
				return e.InternalServerError("", err)
			}
			rec = core.NewRecord(col)
			rec.Set("endpoint", body.Endpoint)
		}
		rec.Set("user", e.Auth.Id)
		rec.Set("p256dh", body.Keys.P256dh)
		rec.Set("auth", body.Keys.Auth)
		if err := e.App.Save(rec); err != nil {
			return e.BadRequestError("Could not save.", err)
		}
		return e.NoContent(http.StatusNoContent)
	})

	g.POST("/unsubscribe", func(e *core.RequestEvent) error {
		var body subscription
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("", err)
		}
		rec, err := e.App.FindFirstRecordByData(Collection, "endpoint", body.Endpoint)
		if err == nil && rec.GetString("user") == e.Auth.Id {
			if err := e.App.Delete(rec); err != nil {
				return e.InternalServerError("", err)
			}
		}
		return e.NoContent(http.StatusNoContent)
	})

	g.POST("/test", func(e *core.RequestEvent) error {
		ToUser(e.App, e.Auth.Id, Message{
			Title: "Notifications are on",
			Body:  "This device will get notifications from this app.",
			Tag:   "test",
		})
		return e.NoContent(http.StatusNoContent)
	})
}
