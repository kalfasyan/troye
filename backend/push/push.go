// Package push sends Web Push notifications from this server straight to
// people's phones and browsers, with no third-party account. The keys that
// sign messages are made on first use and kept in pb_data, so they move with
// backups.
//
// From a hook or route:
//
//	push.ToUser(app, userID, push.Message{Title: "Done", Body: "Your export is ready", URL: "/exports"})
package push

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Collection holds one record per subscribed device. Only the server reads it.
const Collection = "push_subscriptions"

// Message is what the service worker (web/public/sw.js) shows.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	// URL opens when the notification is tapped; it defaults to the app's home.
	URL string `json:"url,omitempty"`
	// Tag makes a newer notification replace an older one with the same tag.
	Tag string `json:"tag,omitempty"`
}

type keyPair struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

var (
	keysMu sync.Mutex
	keys   = map[string]*keyPair{} // by data dir, so tests can run several apps
)

// PublicKey returns the key browsers need to subscribe, creating the key pair
// on first use.
func PublicKey(app core.App) (string, error) {
	k, err := loadKeys(app)
	if err != nil {
		return "", err
	}
	return k.Public, nil
}

func loadKeys(app core.App) (*keyPair, error) {
	keysMu.Lock()
	defer keysMu.Unlock()
	if k := keys[app.DataDir()]; k != nil {
		return k, nil
	}

	path := filepath.Join(app.DataDir(), "push_keys.json")
	k := &keyPair{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, k); err != nil {
			return nil, err
		}
	} else if errors.Is(err, os.ErrNotExist) {
		priv, pub, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			return nil, err
		}
		k = &keyPair{Public: pub, Private: priv}
		b, _ := json.Marshal(k)
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	keys[app.DataDir()] = k
	return k, nil
}

// contact goes in every message so push services know whom to reach about
// problems: PLASTO_PUSH_CONTACT if set, otherwise the owner's email.
func contact(app core.App) string {
	if c := os.Getenv("PLASTO_PUSH_CONTACT"); c != "" {
		return c
	}
	owner, err := app.FindFirstRecordByFilter("users", "role = 'owner'")
	if err == nil && owner.Email() != "" {
		return "mailto:" + owner.Email()
	}
	return "https://github.com/kalfasyan/plasto"
}

var client = &http.Client{Timeout: 15 * time.Second}

// ToUser sends msg to every device of one user, in the background.
func ToUser(app core.App, userID string, msg Message) {
	send(app, dbx.HashExp{"user": userID}, msg)
}

// ToEveryone sends msg to every device of every user except the one given
// (usually whoever caused it); pass "" to include everyone.
func ToEveryone(app core.App, exceptUserID string, msg Message) {
	send(app, dbx.Not(dbx.HashExp{"user": exceptUserID}), msg)
}

func send(app core.App, where dbx.Expression, msg Message) {
	subs, err := app.FindAllRecords(Collection, where)
	if err != nil || len(subs) == 0 {
		return
	}
	go deliver(app, subs, msg)
}

// deliver sends msg to each subscription and forgets the devices that
// unsubscribed or reinstalled the app.
func deliver(app core.App, subs []*core.Record, msg Message) {
	k, err := loadKeys(app)
	if err != nil {
		app.Logger().Error("Push keys unavailable", "error", err)
		return
	}
	if msg.URL == "" {
		msg.URL = "/"
	}
	payload, _ := json.Marshal(msg)
	from := contact(app)

	for _, s := range subs {
		res, err := webpush.SendNotification(payload, &webpush.Subscription{
			Endpoint: s.GetString("endpoint"),
			Keys:     webpush.Keys{P256dh: s.GetString("p256dh"), Auth: s.GetString("auth")},
		}, &webpush.Options{
			HTTPClient:      client,
			Subscriber:      from,
			VAPIDPublicKey:  k.Public,
			VAPIDPrivateKey: k.Private,
			TTL:             24 * 60 * 60,
			Urgency:         webpush.UrgencyNormal,
		})
		if err != nil {
			app.Logger().Warn("Push failed", "error", err)
			continue
		}
		res.Body.Close()
		switch {
		case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
			if err := app.Delete(s); err != nil {
				app.Logger().Warn("Could not forget a push subscription", "error", err)
			}
		case res.StatusCode >= 400:
			app.Logger().Warn("Push rejected", "status", res.StatusCode)
		}
	}
}
