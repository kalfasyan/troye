package app_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"

	"github.com/selfhostit/troye/app"
)

func pngBytes(t testing.TB) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// addPhoto queues a meal photo for a user, as the phone would.
func addPhoto(t testing.TB, a core.App, user, hash string) *core.Record {
	c, _ := a.FindCollectionByNameOrId("meal_photos")
	f, err := filesystem.NewFileFromBytes(pngBytes(t), "meal.png")
	if err != nil {
		t.Fatal(err)
	}
	p := core.NewRecord(c)
	p.Load(map[string]any{"user": user, "date": "2026-10-24", "meal": "lunch", "status": "queued", "hash": hash})
	p.Set("photo", f)
	if err := a.Save(p); err != nil {
		t.Fatal(err)
	}
	return p
}

// call sends a request to the app's router and decodes the JSON answer.
func call(t testing.TB, a *tests.TestApp, method, url, token, body string) (int, map[string]any) {
	t.Helper()
	r, err := apis.NewRouter(a)
	if err != nil {
		t.Fatal(err)
	}
	se := &core.ServeEvent{App: a, Router: r}
	if err := a.OnServe().Trigger(se, func(e *core.ServeEvent) error { return e.Next() }); err != nil {
		t.Fatal(err)
	}
	mux, err := se.Router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out map[string]any
	if b, _ := io.ReadAll(rec.Body); len(b) > 0 {
		_ = json.Unmarshal(b, &out)
	}
	return rec.Code, out
}

func TestPhotoFlow(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, adaToken := addUser(t, a, "ada@example.com", "owner")
	_, bobToken := addUser(t, a, "bob@example.com", "member")
	_, worker := addWorker(t, a, "analyser")
	rice := addFood(t, a, "Rice, white, cooked", "", 130, 2.7)
	addFood(t, a, "Rice pudding with caramel", "", 140, 3)
	chicken := addFood(t, a, "Chicken breast, roasted", "", 165, 31)

	// the analyser is off, and not set up, until it asks for work
	if code, s := call(t, a, "GET", "/api/app/analyser", adaToken, ""); code != 200 || s["online"] != false || s["set_up"] != false {
		t.Fatalf("status %d %v", code, s)
	}

	photo := addPhoto(t, a, ada.Id, "abc")
	code, claim := call(t, a, "POST", "/api/app/photos/claim", worker, "")
	if code != 200 || claim["id"] != photo.Id || !strings.Contains(claim["url"].(string), "token=") {
		t.Fatalf("claim %d %v", code, claim)
	}
	if code, _ := call(t, a, "POST", "/api/app/photos/claim", worker, ""); code != http.StatusNoContent {
		t.Errorf("second claim %d, want nothing to do", code)
	}
	if _, s := call(t, a, "GET", "/api/app/analyser", adaToken, ""); s["online"] != true || s["set_up"] != true || s["waiting"] != 1.0 {
		t.Errorf("status after a claim: %v", s)
	}
	// the download link works without other credentials
	if code, _ := call(t, a, "GET", claim["url"].(string), "", ""); code != 200 {
		t.Errorf("photo download %d", code)
	}

	// people can't post drafts
	if code, _ := call(t, a, "POST", "/api/app/photos/"+photo.Id+"/draft", adaToken, `{"items":[]}`); code != 401 && code != 403 {
		t.Errorf("a person posted a draft: %d", code)
	}
	code, _ = call(t, a, "POST", "/api/app/photos/"+photo.Id+"/draft", worker,
		`{"model":"qwen2.5vl:7b","items":[{"name":"white rice","grams":180,"confidence":0.7},{"name":"grilled chicken breast","grams":120,"confidence":0.9},{"name":"unicorn steak","grams":50,"confidence":0.2}]}`)
	if code != 200 {
		t.Fatalf("draft %d", code)
	}
	photo = reload(t, a, photo)
	var items []app.DraftItem
	if err := photo.UnmarshalJSONField("items", &items); err != nil {
		t.Fatal(err)
	}
	if photo.GetString("status") != "ready" || len(items) != 3 {
		t.Fatalf("photo %s with %d items", photo.GetString("status"), len(items))
	}
	if items[0].Food != rice.Id || items[0].Kcal != 234 || len(items[0].Alternatives) == 0 {
		t.Errorf("rice matched to %q (%v kcal), alternatives %v", items[0].FoodName, items[0].Kcal, items[0].Alternatives)
	}
	if name := strings.ToLower(items[1].FoodName); !strings.Contains(name, "chicken") || !strings.Contains(name, "breast") {
		t.Errorf("chicken matched to %q", items[1].FoodName)
	}
	if items[2].Food != "" {
		t.Errorf("unicorn steak matched to %q", items[2].FoodName)
	}

	// only the owner sees it, and can't fake the draft
	if code, _ := call(t, a, "GET", "/api/collections/meal_photos/records/"+photo.Id, bobToken, ""); code != 404 {
		t.Errorf("someone else saw the photo: %d", code)
	}
	if code, _ := call(t, a, "PATCH", "/api/collections/meal_photos/records/"+photo.Id, adaToken, `{"status":"logged"}`); code != 400 {
		t.Errorf("a person set the status: %d", code)
	}
	if code, _ := call(t, a, "POST", "/api/app/photos/"+photo.Id+"/log", bobToken, `{"items":[{"food":"`+rice.Id+`","grams":100}]}`); code != 404 {
		t.Errorf("someone else logged the photo: %d", code)
	}

	// logging it: rice was really 150 g, the chicken right, the steak dropped
	code, _ = call(t, a, "POST", "/api/app/photos/"+photo.Id+"/log", adaToken,
		`{"meal":"dinner","items":[{"name":"white rice","food":"`+rice.Id+`","grams":150},{"name":"grilled chicken breast","food":"`+chicken.Id+`","grams":120},{"name":"sauce","kcal":80}]}`)
	if code != 200 {
		t.Fatalf("log %d", code)
	}
	entries, _ := a.FindRecordsByFilter("entries", "user = '"+ada.Id+"'", "created", 0, 0)
	if len(entries) != 3 || entries[0].GetFloat("kcal") != 195 || entries[0].GetString("meal") != "dinner" || entries[2].GetFloat("kcal") != 80 {
		t.Errorf("entries %v", entries)
	}
	if photo = reload(t, a, photo); photo.GetString("status") != "logged" {
		t.Errorf("status %s after logging", photo.GetString("status"))
	}
	if code, _ := call(t, a, "POST", "/api/app/photos/"+photo.Id+"/log", adaToken, `{"items":[{"food":"`+rice.Id+`","grams":100}]}`); code != 400 {
		t.Errorf("logged twice: %d", code)
	}

	// the next photo with white rice starts from what was logged
	next := addPhoto(t, a, ada.Id, "def")
	call(t, a, "POST", "/api/app/photos/claim", worker, "")
	call(t, a, "POST", "/api/app/photos/"+next.Id+"/draft", worker, `{"items":[{"name":"White rice","grams":200,"confidence":0.5}]}`)
	next = reload(t, a, next)
	items = nil
	_ = next.UnmarshalJSONField("items", &items)
	if len(items) != 1 || items[0].Food != rice.Id || items[0].Grams != 150 {
		t.Errorf("second draft %+v", items)
	}

	// scoring a model matches without what was learned, and saves nothing
	code, out := call(t, a, "POST", "/api/app/photos/match", worker,
		`{"user":"`+ada.Id+`","items":[{"name":"White rice","grams":200,"confidence":0.5}]}`)
	got, _ := out["items"].([]any)
	if code != 200 || len(got) != 1 || got[0].(map[string]any)["grams"] != 200.0 || got[0].(map[string]any)["food"] != rice.Id {
		t.Errorf("match %d %v", code, out)
	}
	if code, _ := call(t, a, "POST", "/api/app/photos/match", adaToken, `{"items":[]}`); code != 401 && code != 403 {
		t.Errorf("a person used the match route: %d", code)
	}
}

func TestPhotoFailsAndRetries(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, adaToken := addUser(t, a, "ada@example.com", "owner")
	_, worker := addWorker(t, a, "analyser")
	photo := addPhoto(t, a, ada.Id, "")
	call(t, a, "POST", "/api/app/photos/claim", worker, "")
	if code, _ := call(t, a, "POST", "/api/app/photos/"+photo.Id+"/draft", worker, `{"error":"The model didn't answer."}`); code != 200 {
		t.Fatalf("draft %d", code)
	}
	if photo = reload(t, a, photo); photo.GetString("status") != "failed" || photo.GetString("error") == "" {
		t.Fatalf("status %s", photo.GetString("status"))
	}
	// back to the queue with a note
	code, out := call(t, a, "PATCH", "/api/collections/meal_photos/records/"+photo.Id, adaToken, `{"status":"queued","note":"pasta with pesto"}`)
	if code != 200 || out["status"] != "queued" || out["error"] != "" {
		t.Errorf("retry %d %v", code, out)
	}
}

func TestPhotoUpload(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	ada, adaToken := addUser(t, a, "ada@example.com", "owner")

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for k, v := range map[string]string{"user": ada.Id, "date": "2026-10-24", "meal": "lunch", "status": "ready", "hash": "x1", "items": `[{"name":"fake"}]`} {
		_ = w.WriteField(k, v)
	}
	fw, _ := w.CreateFormFile("photo", "meal.png")
	_, _ = fw.Write(pngBytes(t))
	_ = w.Close()

	(&tests.ApiScenario{
		Name:            "a new photo is queued with an empty draft",
		Method:          http.MethodPost,
		URL:             "/api/collections/meal_photos/records",
		Body:            &body,
		Headers:         map[string]string{"Authorization": adaToken, "Content-Type": w.FormDataContentType()},
		ExpectedStatus:  200,
		ExpectedContent: []string{`"status":"queued"`, `"items":null`},
		TestAppFactory:  func(testing.TB) *tests.TestApp { return a },
	}).Test(t)
}

func TestMagicPacket(t *testing.T) {
	p, err := app.MagicPacket("aa:bb:cc:dd:ee:ff")
	if err != nil || len(p) != 102 || p[0] != 0xff || p[6] != 0xaa || p[101] != 0xff || p[96] != 0xaa {
		t.Errorf("packet %x, %v", p, err)
	}
	if _, err := app.MagicPacket("not a mac"); err == nil {
		t.Error("accepted a bad MAC address")
	}
}
