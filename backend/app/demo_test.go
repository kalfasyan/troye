package app_test

// DEMO: tests for the items list in demo.go. scripts/strip-demo.sh removes them.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestDemoItems(t *testing.T) {
	cases := []struct {
		name, as, method, url, body string
		status                      int
		content                     string
	}{
		{
			name: "items record which worker added them", as: "worker", method: http.MethodPost,
			url: "/api/collections/items/records", body: `{"title":"Front door: person","by":"forged"}`,
			status: http.StatusOK, content: `"by":"camera"`,
		},
		{
			name: "items record which person added them", as: "member", method: http.MethodPost,
			url: "/api/collections/items/records", body: `{"title":"Milk"}`,
			status: http.StatusOK, content: `"by":"member"`,
		},
		{
			name: "signed-out visitors can't add items", method: http.MethodPost,
			url: "/api/collections/items/records", body: `{"title":"Spam"}`,
			status: http.StatusBadRequest, content: `"status":400`,
		},
		{
			name: "workers can't delete items", as: "worker", method: http.MethodDelete,
			url: "/api/collections/items/records/{item}", status: http.StatusNotFound, content: `"status":404`,
		},
		{
			name: "people can delete items", as: "member", method: http.MethodDelete,
			url: "/api/collections/items/records/{item}", status: http.StatusNoContent,
		},
	}

	for _, c := range cases {
		a := newApp(t)
		_, memberToken := addUser(t, a, "member@example.com", "member")
		_, workerToken := addWorker(t, a, "camera")
		items, _ := a.FindCollectionByNameOrId("items")
		item := core.NewRecord(items)
		item.Set("title", "Existing")
		if err := a.Save(item); err != nil {
			t.Fatal(err)
		}

		headers := map[string]string{}
		switch c.as {
		case "member":
			headers["Authorization"] = memberToken
		case "worker":
			headers["Authorization"] = workerToken
		}
		s := tests.ApiScenario{
			Name:           c.name,
			Method:         c.method,
			URL:            strings.ReplaceAll(c.url, "{item}", item.Id),
			Body:           strings.NewReader(c.body),
			Headers:        headers,
			ExpectedStatus: c.status,
			TestAppFactory: func(testing.TB) *tests.TestApp { return a },
		}
		if c.content != "" {
			s.ExpectedContent = []string{c.content}
		}
		s.Test(t)
	}
}
