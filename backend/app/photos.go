package app

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/kalfasyan/troye/push"
)

// Photo logging. The phone queues a meal photo; the analyser (workers/analyser.py
// on the GPU PC) claims it, names the items it sees with their grams, and posts
// that back. The server matches each item to a food, so calories come from the
// food tables and never from the model, and tells the person their meal is
// ready to check. Logging it turns the draft into diary entries and remembers
// what each name turned out to be, so the next draft starts from that.
//
// The analyser polls for work every half minute, which is also how the app
// knows it is on. When it is off, the Wake PC button sends a Wake-on-LAN packet
// to TROYE_WAKE_MAC (to TROYE_WAKE_ADDR, 255.255.255.255:9 by default).

// staleClaim is how long a claimed photo waits before another round may claim it.
const staleClaim = 10 * time.Minute

// analyserTimeout is how long after its last poll the analyser counts as off.
const analyserTimeout = 90 * time.Second

const analyserSeenKey = "analyser_seen"

// DraftItem is one thing the analyser saw, with the food it was matched to.
type DraftItem struct {
	Name       string  `json:"name"`
	Grams      float64 `json:"grams"`
	Confidence float64 `json:"confidence"`
	// the matched food, empty when nothing matched
	Food     string  `json:"food"`
	FoodName string  `json:"food_name"`
	Kcal     float64 `json:"kcal"`
	// the food's values per 100 g, so the phone can redo the sums as grams change
	Per100       Nutrients     `json:"per100"`
	Alternatives []Alternative `json:"alternatives"`
}

// Nutrients per 100 g.
type Nutrients struct {
	Kcal    float64 `json:"kcal"`
	Protein float64 `json:"protein"`
	Carbs   float64 `json:"carbs"`
	Fat     float64 `json:"fat"`
}

// Alternative is another food the item could be.
type Alternative struct {
	Food   string    `json:"food"`
	Name   string    `json:"name"`
	Per100 Nutrients `json:"per100"`
}

func registerPhotos(app core.App) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.POST("/api/app/photos/claim", claimPhoto).Bind(apis.RequireAuth("workers"))
		se.Router.POST("/api/app/photos/{id}/draft", postDraft).Bind(apis.RequireAuth("workers"))
		se.Router.POST("/api/app/photos/{id}/log", logPhoto).Bind(apis.RequireAuth("users"))
		se.Router.GET("/api/app/analyser", analyserStatus).Bind(apis.RequireAuth("users"))
		se.Router.POST("/api/app/analyser/wake", wakeAnalyser).Bind(apis.RequireAuth("users"))
		return se.Next()
	})

	// a new photo always starts in the queue with an empty draft
	app.OnRecordCreateRequest("meal_photos").BindFunc(func(e *core.RecordRequestEvent) error {
		e.Record.Set("status", "queued")
		for _, f := range []string{"items", "confirmed", "model", "error", "claimed_at", "analysed_at"} {
			e.Record.Set(f, nil)
		}
		return e.Next()
	})
	// people may change the meal, day or note, and send a photo back to the
	// queue; the draft and the rest of the status are the server's
	app.OnRecordUpdateRequest("meal_photos").BindFunc(func(e *core.RecordRequestEvent) error {
		old := e.Record.Original()
		for _, f := range []string{"items", "confirmed", "model", "error", "claimed_at", "analysed_at", "hash", "photo"} {
			e.Record.Set(f, old.Get(f))
		}
		if status := e.Record.GetString("status"); status != old.GetString("status") {
			if status != "queued" || old.GetString("status") == "logged" {
				return e.BadRequestError("A photo can only go back to the queue.", nil)
			}
			e.Record.Set("error", "")
		}
		return e.Next()
	})
}

// claimPhoto hands the analyser the oldest photo waiting, with a link to
// download it, or 204 when there is nothing to do.
func claimPhoto(e *core.RequestEvent) error {
	e.App.Store().Set(analyserSeenKey, time.Now())

	var claimed *core.Record
	err := e.App.RunInTransaction(func(tx core.App) error {
		stale := time.Now().Add(-staleClaim).UTC().Format(types.DefaultDateLayout)
		list, err := tx.FindRecordsByFilter("meal_photos",
			`status = "queued" || (status = "analysing" && claimed_at < {:stale})`,
			"created", 1, 0, dbx.Params{"stale": stale})
		if err != nil || len(list) == 0 {
			return err
		}
		claimed = list[0]
		claimed.Set("status", "analysing")
		claimed.Set("claimed_at", time.Now())
		return tx.Save(claimed)
	})
	if err != nil {
		return e.InternalServerError("", err)
	}
	if claimed == nil {
		return e.NoContent(http.StatusNoContent)
	}
	token, err := e.Auth.NewFileToken()
	if err != nil {
		return e.InternalServerError("", err)
	}
	return e.JSON(http.StatusOK, map[string]any{
		"id":   claimed.Id,
		"meal": claimed.GetString("meal"),
		"note": claimed.GetString("note"),
		"url": fmt.Sprintf("/api/files/%s/%s/%s?token=%s",
			claimed.Collection().Id, claimed.Id, claimed.GetString("photo"), token),
	})
}

// postDraft takes what the analyser saw, or why it couldn't.
func postDraft(e *core.RequestEvent) error {
	var body struct {
		Items []struct {
			Name       string  `json:"name"`
			Grams      float64 `json:"grams"`
			Confidence float64 `json:"confidence"`
		} `json:"items"`
		Model string `json:"model"`
		Error string `json:"error"`
	}
	if err := e.BindBody(&body); err != nil {
		return e.BadRequestError("Could not read the draft.", err)
	}
	photo, err := e.App.FindRecordById("meal_photos", e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("", err)
	}
	if photo.GetString("status") != "analysing" {
		return e.BadRequestError("This photo isn't being analysed any more.", nil)
	}
	user := photo.GetString("user")

	photo.Set("model", clip(body.Model, 100))
	photo.Set("analysed_at", time.Now())
	if body.Error != "" || len(body.Items) == 0 {
		msg := body.Error
		if msg == "" {
			msg = "No food found in the photo."
		}
		photo.Set("status", "failed")
		photo.Set("error", clip(msg, 500))
	} else {
		items := make([]DraftItem, 0, len(body.Items))
		for _, it := range body.Items {
			name := strings.TrimSpace(it.Name)
			if name == "" || len(items) == 20 {
				continue
			}
			item := DraftItem{
				Name:       clip(name, 200),
				Grams:      math.Round(math.Min(math.Max(it.Grams, 1), 3000)),
				Confidence: math.Min(math.Max(it.Confidence, 0), 1),
			}
			if err := matchItem(e.App, user, &item); err != nil {
				return e.InternalServerError("", err)
			}
			items = append(items, item)
		}
		photo.Set("status", "ready")
		photo.Set("items", items)
		photo.Set("error", "")
	}
	if err := e.App.Save(photo); err != nil {
		return e.InternalServerError("", err)
	}

	meal := photo.GetString("meal")
	if photo.GetString("status") == "ready" {
		push.ToUser(e.App, user, push.Message{
			Title: fmt.Sprintf("Your %s is ready to check", mealName(meal)),
			Body:  draftSummary(photo),
			URL:   "/#photo/" + photo.Id,
			Tag:   "photo-" + photo.Id,
		})
	} else {
		push.ToUser(e.App, user, push.Message{
			Title: fmt.Sprintf("Couldn't read your %s photo", mealName(meal)),
			Body:  "Add it by hand, or try again with a note.",
			URL:   "/#photo/" + photo.Id,
			Tag:   "photo-" + photo.Id,
		})
	}
	return e.JSON(http.StatusOK, photo)
}

func mealName(meal string) string {
	if meal == "snacks" {
		return "snack"
	}
	return meal
}

// draftSummary is "Rice, chicken and 1 more · about 640 kcal".
func draftSummary(photo *core.Record) string {
	var items []DraftItem
	if err := photo.UnmarshalJSONField("items", &items); err != nil || len(items) == 0 {
		return ""
	}
	var kcal float64
	names := make([]string, 0, 2)
	for i, it := range items {
		kcal += it.Kcal
		if i < 2 {
			names = append(names, strings.ToLower(it.Name))
		}
	}
	first := []rune(names[0])
	names[0] = strings.ToUpper(string(first[:1])) + string(first[1:])
	s := strings.Join(names, ", ")
	if len(items) > 2 {
		s += fmt.Sprintf(" and %d more", len(items)-2)
	}
	return fmt.Sprintf("%s · about %d kcal", s, int(math.Round(kcal/10)*10))
}

// matchItem finds the food for an item: what the person logged for that name
// before, or else the best match in the food tables.
func matchItem(app core.App, user string, item *DraftItem) error {
	alias, err := app.FindFirstRecordByFilter("food_aliases", "user = {:user} && alias = {:alias}",
		dbx.Params{"user": user, "alias": strings.ToLower(item.Name)})
	if err == nil {
		if food, err := app.FindRecordById("foods", alias.GetString("food")); err == nil {
			setFood(item, food)
			// the grams they logged last time are a better guess than the model's
			if g := alias.GetFloat("grams"); g > 0 && item.Confidence < 0.8 {
				item.Grams = g
				item.Kcal = math.Round(item.Per100.Kcal * g / 100)
			}
		}
	}

	matches, err := searchFoodsFor(app, user, item.Name, 4)
	if err != nil {
		return err
	}
	for _, f := range matches {
		if item.Food == "" {
			setFood(item, f)
			continue
		}
		if f.Id != item.Food && len(item.Alternatives) < 3 {
			item.Alternatives = append(item.Alternatives, Alternative{Food: f.Id, Name: f.GetString("name"), Per100: per100(f)})
		}
	}
	if item.Alternatives == nil {
		item.Alternatives = []Alternative{}
	}
	return nil
}

func setFood(item *DraftItem, food *core.Record) {
	item.Food = food.Id
	item.FoodName = food.GetString("name")
	item.Per100 = per100(food)
	item.Kcal = math.Round(item.Per100.Kcal * item.Grams / 100)
}

func per100(food *core.Record) Nutrients {
	return Nutrients{
		Kcal:    food.GetFloat("kcal"),
		Protein: food.GetFloat("protein"),
		Carbs:   food.GetFloat("carbs"),
		Fat:     food.GetFloat("fat"),
	}
}

// words the analyser adds that don't help find a food
var fillerWords = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "with": true, "and": true, "some": true,
	"slice": true, "slices": true, "piece": true, "pieces": true, "portion": true, "serving": true,
	"plate": true, "bowl": true, "cup": true, "glass": true, "small": true, "large": true, "fresh": true,
}

// searchFoodsFor finds foods matching a name, best first: those with every
// word of the name, then those with all but one ("grilled chicken breast"
// finds "Chicken breast, roasted"), and so on down to single words.
func searchFoodsFor(app core.App, user, name string, limit int) ([]*core.Record, error) {
	var words []string
	for _, w := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127)
	}) {
		if !fillerWords[w] && len(w) > 1 {
			words = append(words, w)
		}
	}
	if len(words) > 4 {
		words = words[:4]
	}
	var found []*core.Record
	seen := map[string]bool{}
	for n := len(words); n > 0 && len(found) < limit; n-- {
		var batch []*core.Record
		for _, subset := range combinations(words, n) {
			params := dbx.Params{"user": user}
			conds := []string{`(owner = "" || owner = {:user})`}
			for i, w := range subset {
				key := fmt.Sprintf("w%d", i)
				params[key] = w
				conds = append(conds, fmt.Sprintf("name ~ {:%s}", key))
			}
			list, err := app.FindRecordsByFilter("foods", strings.Join(conds, " && "), "", 30, 0, params)
			if err != nil {
				return nil, err
			}
			for _, f := range list {
				if !seen[f.Id] {
					seen[f.Id] = true
					batch = append(batch, f)
				}
			}
		}
		rankFoods(batch, words, user)
		found = append(found, batch...)
	}
	if len(found) > limit {
		found = found[:limit]
	}
	return found, nil
}

// combinations lists the subsets of n words, keeping their order.
func combinations(words []string, n int) [][]string {
	if n == 0 {
		return [][]string{{}}
	}
	var out [][]string
	for i := 0; i+n <= len(words); i++ {
		for _, rest := range combinations(words[i+1:], n-1) {
			out = append(out, append([]string{words[i]}, rest...))
		}
	}
	return out
}

// source order when two foods match as well: your own, then the generic tables
var sourceRank = map[string]int{"custom": 0, "ciqual": 1, "nevo": 2, "usda": 3, "off": 4}

// rankFoods puts the plainest match first: names that start with one of the
// words, then shorter names, so "Rice, white, cooked" beats "Rice pudding
// with caramel" for "rice".
func rankFoods(list []*core.Record, words []string, user string) {
	score := func(f *core.Record) (int, int, int) {
		name := strings.ToLower(f.GetString("name"))
		// names that start with one of the words, and have the fewest other words
		lead := 1
		for _, w := range words {
			if strings.HasPrefix(name, w) {
				lead = 0
			}
		}
		extra := len(strings.Fields(name))
		src := sourceRank[f.GetString("source")]
		if f.GetString("owner") == user {
			src = -1
		}
		return lead, src, extra
	}
	sort.SliceStable(list, func(i, j int) bool {
		li, si, ei := score(list[i])
		lj, sj, ej := score(list[j])
		if li != lj {
			return li < lj
		}
		if ei != ej {
			return ei < ej
		}
		return si < sj
	})
}

// logPhoto turns a checked draft into diary entries, and remembers each name.
func logPhoto(e *core.RequestEvent) error {
	var body struct {
		Meal  string `json:"meal"`
		Date  string `json:"date"`
		Items []struct {
			Name  string  `json:"name"`
			Food  string  `json:"food"`
			Grams float64 `json:"grams"`
			// for an item with no food: just calories
			Kcal float64 `json:"kcal"`
		} `json:"items"`
	}
	if err := e.BindBody(&body); err != nil {
		return e.BadRequestError("Could not read what to log.", err)
	}
	photo, err := e.App.FindRecordById("meal_photos", e.Request.PathValue("id"))
	if err != nil || photo.GetString("user") != e.Auth.Id {
		return e.NotFoundError("", err)
	}
	if photo.GetString("status") == "logged" {
		return e.BadRequestError("This photo is already in your diary.", nil)
	}
	if len(body.Items) == 0 {
		return e.BadRequestError("Keep at least one item to log.", nil)
	}
	if body.Meal == "" {
		body.Meal = photo.GetString("meal")
	}
	if body.Date == "" {
		body.Date = photo.GetString("date")
	}

	err = e.App.RunInTransaction(func(tx core.App) error {
		entries, err := tx.FindCollectionByNameOrId("entries")
		if err != nil {
			return err
		}
		confirmed := make([]map[string]any, 0, len(body.Items))
		for _, it := range body.Items {
			entry := core.NewRecord(entries)
			entry.Set("user", e.Auth.Id)
			entry.Set("date", body.Date)
			entry.Set("meal", body.Meal)
			if it.Food != "" {
				entry.Set("food", it.Food)
				entry.Set("grams", it.Grams)
			} else {
				entry.Set("name", it.Name)
				entry.Set("kcal", it.Kcal)
			}
			if err := tx.Save(entry); err != nil {
				return err
			}
			confirmed = append(confirmed, map[string]any{
				"name": it.Name, "food": it.Food, "grams": it.Grams, "kcal": entry.GetFloat("kcal"), "entry": entry.Id,
			})
			if it.Food != "" && it.Name != "" {
				if err := rememberAlias(tx, e.Auth.Id, it.Name, it.Food, it.Grams); err != nil {
					return err
				}
			}
		}
		photo.Set("status", "logged")
		photo.Set("meal", body.Meal)
		photo.Set("date", body.Date)
		photo.Set("confirmed", confirmed)
		return tx.Save(photo)
	})
	if err != nil {
		return e.BadRequestError("Some items couldn't be logged. Check the foods and amounts.", err)
	}
	return e.JSON(http.StatusOK, photo)
}

func rememberAlias(app core.App, user, name, food string, grams float64) error {
	alias := strings.ToLower(strings.TrimSpace(name))
	r, err := app.FindFirstRecordByFilter("food_aliases", "user = {:user} && alias = {:alias}",
		dbx.Params{"user": user, "alias": alias})
	if err != nil {
		c, err := app.FindCollectionByNameOrId("food_aliases")
		if err != nil {
			return err
		}
		r = core.NewRecord(c)
		r.Set("user", user)
		r.Set("alias", clip(alias, 200))
	}
	r.Set("food", food)
	r.Set("grams", grams)
	r.Set("uses", r.GetInt("uses")+1)
	return app.Save(r)
}

// analyserStatus tells the phone whether the analyser is on and how many of
// the person's photos wait for it.
func analyserStatus(e *core.RequestEvent) error {
	seen, _ := e.App.Store().Get(analyserSeenKey).(time.Time)
	waiting, err := e.App.CountRecords("meal_photos", dbx.HashExp{"user": e.Auth.Id, "status": []any{"queued", "analysing"}})
	if err != nil {
		return e.InternalServerError("", err)
	}
	res := map[string]any{
		"online":   !seen.IsZero() && time.Since(seen) < analyserTimeout,
		"waiting":  waiting,
		"can_wake": os.Getenv("TROYE_WAKE_MAC") != "",
		"seen":     "",
	}
	if !seen.IsZero() {
		res["seen"] = seen.UTC().Format(time.RFC3339)
	}
	return e.JSON(http.StatusOK, res)
}

// wakeAnalyser sends the Wake-on-LAN packet to the GPU PC.
func wakeAnalyser(e *core.RequestEvent) error {
	mac := os.Getenv("TROYE_WAKE_MAC")
	if mac == "" {
		return e.BadRequestError("Waking the PC isn't set up. Set TROYE_WAKE_MAC on the server.", nil)
	}
	addr := os.Getenv("TROYE_WAKE_ADDR")
	if addr == "" {
		addr = "255.255.255.255:9"
	}
	if err := sendMagicPacket(mac, addr); err != nil {
		e.App.Logger().Error("couldn't send the wake packet", "error", err)
		return e.BadRequestError("Couldn't send the wake signal. Check TROYE_WAKE_MAC and TROYE_WAKE_ADDR.", nil)
	}
	return e.NoContent(http.StatusNoContent)
}

// MagicPacket is the Wake-on-LAN packet for a MAC address: six 0xFF bytes,
// then the address sixteen times.
func MagicPacket(mac string) ([]byte, error) {
	hw, err := net.ParseMAC(mac)
	if err != nil || len(hw) != 6 {
		return nil, fmt.Errorf("not a MAC address: %q", mac)
	}
	p := make([]byte, 0, 102)
	for range 6 {
		p = append(p, 0xff)
	}
	for range 16 {
		p = append(p, hw...)
	}
	return p, nil
}

func sendMagicPacket(mac, addr string) error {
	packet, err := MagicPacket(mac)
	if err != nil {
		return err
	}
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Write(packet)
	return err
}

// clip cuts s to at most n characters.
func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
