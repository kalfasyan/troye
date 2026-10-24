package app

// DEMO: the items list that shows every part of plasto working together.
// scripts/strip-demo.sh removes it.

import (
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/kalfasyan/plasto/push"
)

// registerDemo records who added each item and tells everyone else about it.
func registerDemo(app core.App) {
	app.OnRecordCreateRequest("items").BindFunc(func(e *core.RecordRequestEvent) error {
		by := "Someone"
		if e.Auth != nil {
			by = e.Auth.GetString("name")
			if by == "" {
				by = strings.Split(e.Auth.Email(), "@")[0]
			}
		}
		e.Record.Set("by", by)
		if err := e.Next(); err != nil {
			return err
		}

		except := ""
		if e.Auth != nil {
			except = e.Auth.Id
		}
		push.ToEveryone(e.App, except, push.Message{
			Title: "New item from " + by,
			Body:  e.Record.GetString("title"),
			Tag:   "items",
		})
		return nil
	})
}
