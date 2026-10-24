package push_test

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	_ "github.com/kalfasyan/plasto/migrations"
	"github.com/kalfasyan/plasto/push"
)

func newApp(t testing.TB) *tests.TestApp {
	a, err := tests.NewTestAppWithConfig(core.BaseAppConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	a.OnServe().BindFunc(func(se *core.ServeEvent) error {
		push.RegisterRoutes(se)
		return se.Next()
	})
	return a
}

func addUser(t testing.TB, a core.App, email string) (*core.Record, string) {
	users, _ := a.FindCollectionByNameOrId("users")
	u := core.NewRecord(users)
	u.SetEmail(email)
	u.SetPassword("password123")
	if err := a.Save(u); err != nil {
		t.Fatal(err)
	}
	token, _ := u.NewAuthToken()
	return u, token
}

// subscribe saves a device for user whose push service is the given URL, with
// real browser-style keys so messages can be encrypted for it.
func subscribe(t testing.TB, a core.App, user *core.Record, endpoint string) *core.Record {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	col, _ := a.FindCollectionByNameOrId(push.Collection)
	s := core.NewRecord(col)
	s.Set("user", user.Id)
	s.Set("endpoint", endpoint)
	s.Set("p256dh", base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()))
	s.Set("auth", base64.RawURLEncoding.EncodeToString(secret))
	if err := a.Save(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDelivery(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, _ := addUser(t, a, "ada@example.com")
	bob, _ := addUser(t, a, "bob@example.com")

	hits := make(chan *http.Request, 10)
	var status atomic.Int32
	status.Store(http.StatusCreated)
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits <- r
		w.WriteHeader(int(status.Load()))
	}))
	defer service.Close()

	subscribe(t, a, ada, service.URL+"/ada")
	gone := subscribe(t, a, bob, service.URL+"/bob")

	push.ToUser(a, ada.Id, push.Message{Title: "Hi"})
	select {
	case r := <-hits:
		if r.URL.Path != "/ada" || !strings.HasPrefix(r.Header.Get("Authorization"), "vapid ") || r.Header.Get("Content-Encoding") != "aes128gcm" {
			t.Errorf("unexpected push request: %s %v", r.URL.Path, r.Header)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no push was sent")
	}

	// a push service answering 410 means the device unsubscribed: forget it
	status.Store(http.StatusGone)
	push.ToEveryone(a, ada.Id, push.Message{Title: "Hi"})
	if r := <-hits; r.URL.Path != "/bob" {
		t.Fatalf("pushed to %s, want only /bob", r.URL.Path)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := a.FindRecordById(push.Collection, gone.Id); err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the gone subscription was kept")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRoutes(t *testing.T) {
	const sub = `{"endpoint":"https://push.example.com/1","keys":{"p256dh":"key","auth":"secret"}}`
	var adaToken, bobToken string
	withUsers := func(t testing.TB) *tests.TestApp {
		a := newApp(t)
		_, adaToken = addUser(t, a, "ada@example.com")
		_, bobToken = addUser(t, a, "bob@example.com")
		return a
	}
	countFor := func(email string, want int) func(testing.TB, *tests.TestApp, *http.Response) {
		return func(t testing.TB, a *tests.TestApp, _ *http.Response) {
			u, _ := a.FindAuthRecordByEmail("users", email)
			recs, _ := a.FindRecordsByFilter(push.Collection, "user = {:u}", "", 0, 0, map[string]any{"u": u.Id})
			if len(recs) != want {
				t.Errorf("%s has %d subscriptions, want %d", email, len(recs), want)
			}
		}
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "signed-out visitors get no key",
			Method:          http.MethodGet,
			URL:             "/api/app/push/key",
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{"authorization token"},
			TestAppFactory:  withUsers,
		},
		{
			Name:            "signed-in users get the public key",
			Method:          http.MethodGet,
			URL:             "/api/app/push/key",
			Headers:         map[string]string{"Authorization": "{ada}"},
			ExpectedStatus:  http.StatusOK,
			ExpectedContent: []string{`"key":"B`},
			TestAppFactory:  withUsers,
		},
		{
			Name:           "subscribing saves the device",
			Method:         http.MethodPost,
			URL:            "/api/app/push/subscribe",
			Body:           strings.NewReader(sub),
			Headers:        map[string]string{"Authorization": "{ada}"},
			ExpectedStatus: http.StatusNoContent,
			TestAppFactory: withUsers,
			AfterTestFunc:  countFor("ada@example.com", 1),
		},
		{
			Name:            "a subscription needs its keys",
			Method:          http.MethodPost,
			URL:             "/api/app/push/subscribe",
			Body:            strings.NewReader(`{"endpoint":"https://push.example.com/1"}`),
			Headers:         map[string]string{"Authorization": "{ada}"},
			ExpectedStatus:  http.StatusBadRequest,
			ExpectedContent: []string{"Missing subscription"},
			TestAppFactory:  withUsers,
		},
		{
			Name:           "a device someone else used moves to the new user",
			Method:         http.MethodPost,
			URL:            "/api/app/push/subscribe",
			Body:           strings.NewReader(sub),
			Headers:        map[string]string{"Authorization": "{bob}"},
			ExpectedStatus: http.StatusNoContent,
			TestAppFactory: func(t testing.TB) *tests.TestApp {
				a := withUsers(t)
				ada, _ := a.FindAuthRecordByEmail("users", "ada@example.com")
				subscribe(t, a, ada, "https://push.example.com/1")
				return a
			},
			AfterTestFunc: func(t testing.TB, a *tests.TestApp, r *http.Response) {
				countFor("ada@example.com", 0)(t, a, r)
				countFor("bob@example.com", 1)(t, a, r)
			},
		},
		{
			Name:           "nobody can unsubscribe someone else's device",
			Method:         http.MethodPost,
			URL:            "/api/app/push/unsubscribe",
			Body:           strings.NewReader(sub),
			Headers:        map[string]string{"Authorization": "{bob}"},
			ExpectedStatus: http.StatusNoContent,
			TestAppFactory: func(t testing.TB) *tests.TestApp {
				a := withUsers(t)
				ada, _ := a.FindAuthRecordByEmail("users", "ada@example.com")
				subscribe(t, a, ada, "https://push.example.com/1")
				return a
			},
			AfterTestFunc: countFor("ada@example.com", 1),
		},
		{
			Name:            "the API doesn't expose subscriptions",
			Method:          http.MethodGet,
			URL:             "/api/collections/push_subscriptions/records",
			Headers:         map[string]string{"Authorization": "{ada}"},
			ExpectedStatus:  http.StatusForbidden,
			ExpectedContent: []string{"Only superusers"},
			TestAppFactory:  withUsers,
		},
	}

	for _, s := range scenarios {
		// tokens exist only once the factory has run, so fill them in then
		factory := s.TestAppFactory
		headers := s.Headers
		s.TestAppFactory = func(t testing.TB) *tests.TestApp {
			a := factory(t)
			for k, v := range headers {
				headers[k] = strings.NewReplacer("{ada}", adaToken, "{bob}", bobToken).Replace(v)
			}
			return a
		}
		s.Test(t)
	}
}
