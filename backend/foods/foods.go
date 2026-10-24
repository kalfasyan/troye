// Package foods fills the foods collection from the food tables and keeps it
// searchable:
//
//   - Ciqual (ANSES, France), bundled with the app under the Open Licence
//   - NEVO (RIVM, Netherlands), which the owner downloads and uploads once
//   - Open Food Facts products sold in Belgium and the Netherlands, downloaded
//     every month so barcodes work offline, and looked up live otherwise
//
// Imports write straight to the database in batches, because the tables hold
// thousands of foods and Open Food Facts about a hundred thousand products.
// An import adds and updates foods but never deletes one, since diary entries
// point at them.
package foods

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Food is one food from a table, with its values per 100 g.
type Food struct {
	SourceID string `json:"id"`
	Name     string `json:"name"`
	NameNL   string `json:"name_nl,omitempty"`
	NameFR   string `json:"name_fr,omitempty"`
	Aliases  string `json:"aliases,omitempty"`
	Group    string `json:"group,omitempty"`
	Brand    string `json:"brand,omitempty"`
	Barcode  string `json:"barcode,omitempty"`

	Kcal    float64 `json:"kcal"`
	Protein float64 `json:"protein"`
	Carbs   float64 `json:"carbs"`
	Fat     float64 `json:"fat"`
	Fiber   float64 `json:"fiber"`
	Sugar   float64 `json:"sugar"`
	Salt    float64 `json:"salt"`

	ServingG     float64 `json:"serving_g,omitempty"`
	ServingLabel string  `json:"serving_label,omitempty"`
}

var barcodePattern = regexp.MustCompile(`^\d{6,14}$`)

// record is the food as columns of the foods table.
func (f Food) record(source string) dbx.Params {
	value := func(v float64) float64 {
		if math.IsNaN(v) || v < 0 {
			return 0
		}
		return math.Round(math.Min(v, 1000)*100) / 100
	}
	barcode := f.Barcode
	if !barcodePattern.MatchString(barcode) {
		barcode = ""
	}
	serving, label := f.ServingG, truncate(strings.TrimSpace(f.ServingLabel), 50)
	if serving <= 0 || serving > 5000 {
		serving, label = 0, ""
	}
	p := dbx.Params{
		"name":          truncate(strings.TrimSpace(f.Name), 200),
		"name_nl":       truncate(strings.TrimSpace(f.NameNL), 200),
		"name_fr":       truncate(strings.TrimSpace(f.NameFR), 200),
		"aliases":       truncate(strings.TrimSpace(f.Aliases), 1000),
		"food_group":    truncate(strings.TrimSpace(f.Group), 100),
		"brand":         truncate(strings.TrimSpace(f.Brand), 200),
		"source":        source,
		"source_id":     truncate(f.SourceID, 50),
		"barcode":       barcode,
		"kcal":          math.Round(value(f.Kcal)),
		"protein":       value(f.Protein),
		"carbs":         value(f.Carbs),
		"fat":           value(f.Fat),
		"fiber":         value(f.Fiber),
		"sugar":         value(f.Sugar),
		"salt":          value(f.Salt),
		"serving_g":     serving,
		"serving_label": label,
	}
	p["search"] = searchText(p["name"], p["name_nl"], p["name_fr"], p["aliases"], p["brand"])
	return p
}

// importColumns are the columns an import sets; an existing food keeps its id,
// owner and created date.
var importColumns = []string{
	"name", "name_nl", "name_fr", "aliases", "food_group", "brand", "source", "source_id", "barcode",
	"kcal", "protein", "carbs", "fat", "fiber", "sugar", "salt", "serving_g", "serving_label", "search",
}

const batchSize = 500

// Store adds the foods to the foods collection, or updates the ones it already
// has from the same source, matching them by source id. It saves in batches
// and returns how many foods it stored.
func Store(app core.App, source string, foods []Food) (int, error) {
	stored := 0
	for start := 0; start < len(foods); start += batchSize {
		batch := foods[start:min(start+batchSize, len(foods))]
		n, err := storeBatch(app, source, batch)
		stored += n
		if err != nil {
			return stored, err
		}
	}
	return stored, nil
}

func storeBatch(app core.App, source string, batch []Food) (int, error) {
	updates := make([]string, len(importColumns))
	for i, c := range importColumns {
		updates[i] = fmt.Sprintf("[[%s]] = excluded.[[%s]]", c, c)
	}
	cols := append([]string{"id", "owner", "created", "updated"}, importColumns...)
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = "[[" + c + "]]"
	}
	placeholders := make([]string, len(cols))
	for i, c := range cols {
		placeholders[i] = "{:" + c + "}"
	}
	// updated is set by hand: raw SQL skips PocketBase's autodate fields
	sql := fmt.Sprintf(
		"INSERT INTO {{foods}} (%s) VALUES (%s) ON CONFLICT ([[source]], [[source_id]]) WHERE [[source_id]] != '' DO UPDATE SET %s, [[updated]] = excluded.[[updated]]",
		strings.Join(names, ", "), strings.Join(placeholders, ", "), strings.Join(updates, ", "))

	stored := 0
	err := app.RunInTransaction(func(tx core.App) error {
		now := types.NowDateTime().String()
		for _, f := range batch {
			p := f.record(source)
			if p["name"] == "" || p["source_id"] == "" {
				continue
			}
			p["id"] = security.RandomStringWithAlphabet(15, idAlphabet)
			p["owner"], p["created"], p["updated"] = "", now, now
			if _, err := tx.DB().NewQuery(sql).Bind(p).Execute(); err != nil {
				return fmt.Errorf("food %s: %w", f.SourceID, err)
			}
			stored++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return stored, nil
}

// the alphabet of PocketBase's own record ids
const idAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

var fold = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// Normalize lowercases s, drops accents and turns everything but letters and
// digits into single spaces, so "Pâté, de campagne" becomes "pate de campagne".
func Normalize(s string) string {
	folded, _, err := transform.String(fold, s)
	if err != nil {
		folded = s
	}
	folded = strings.NewReplacer("ß", "ss", "æ", "ae", "œ", "oe", "Æ", "ae", "Œ", "oe").Replace(folded)
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(folded) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// searchText joins the normalized names with " | ", which a search word can't
// match across.
func searchText(names ...any) string {
	var parts []string
	for _, n := range names {
		if s := Normalize(fmt.Sprint(n)); s != "" {
			parts = append(parts, s)
		}
	}
	return truncate(strings.Join(parts, " | "), 2000)
}

// SetSearch fills a food record's search field from its names and brand.
func SetSearch(r *core.Record) {
	r.Set("search", searchText(r.GetString("name"), r.GetString("name_nl"), r.GetString("name_fr"),
		r.GetString("aliases"), r.GetString("brand")))
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
