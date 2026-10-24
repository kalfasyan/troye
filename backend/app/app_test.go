package app_test

import (
	"net/http"
	"testing"

	"github.com/pocketbase/pocketbase/tests"

	"github.com/kalfasyan/plasto/app"
)

func TestRoutes(t *testing.T) {
	setup := func(t testing.TB) *tests.TestApp {
		testApp, err := tests.NewTestApp()
		if err != nil {
			t.Fatal(err)
		}
		app.Register(testApp, app.Config{PublicDir: "testdata"})
		return testApp
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "info reports the version",
			Method:          http.MethodGet,
			URL:             "/api/app/info",
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"version":"dev"`},
			TestAppFactory:  setup,
		},
		{
			Name:            "unknown paths fall back to the web app",
			Method:          http.MethodGet,
			URL:             "/some/client/route",
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{"plasto test page"},
			TestAppFactory:  setup,
		},
	}

	for _, s := range scenarios {
		s.Test(t)
	}
}
