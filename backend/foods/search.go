package foods

import (
	"sort"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// how many matches are ranked; the database orders them roughly first
const candidates = 400

// Search finds foods whose names (in English, Dutch or French), other names or
// brand contain every word of q, in any order and without minding accents. The
// best come first: foods the person logs often, their own foods, names that
// start with the words, whole words, the generic tables before branded
// products, and short names.
func Search(app core.App, userID, q string, limit int) ([]*core.Record, error) {
	words := strings.Fields(Normalize(q))
	if len(words) > 6 {
		words = words[:6]
	}
	if len(words) == 0 {
		return []*core.Record{}, nil
	}
	found, err := matches(app, userID, words)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		// "apples", "pommes" and "eieren" find "apple", "pomme" and "eier"
		if stems := stemmed(words); stems != nil {
			words = stems
			if found, err = matches(app, userID, words); err != nil {
				return nil, err
			}
		}
	}

	uses := usage(app, userID)
	scores := make(map[string]float64, len(found))
	for _, r := range found {
		scores[r.Id] = score(r, words, userID, uses[r.Id])
	}
	sort.SliceStable(found, func(i, j int) bool {
		if scores[found[i].Id] != scores[found[j].Id] {
			return scores[found[i].Id] > scores[found[j].Id]
		}
		return found[i].GetString("name") < found[j].GetString("name")
	})
	if len(found) > limit {
		found = found[:limit]
	}
	return found, nil
}

func matches(app core.App, userID string, words []string) ([]*core.Record, error) {
	query := app.RecordQuery("foods").
		AndWhere(dbx.Or(dbx.HashExp{"owner": ""}, dbx.HashExp{"owner": userID}))
	for _, w := range words {
		query.AndWhere(dbx.Like("search", w))
	}
	// a rough order, so the best matches are among the candidates: the
	// person's own foods, then names that start with the first word, then the
	// generic tables, then short names
	first := dbx.Params{"first": escapeLike(words[0]) + "%"}
	query.OrderBy(
		"(owner != '') DESC",
		"(search LIKE {:first} ESCAPE '\\') DESC",
		"(source IN ('ciqual', 'nevo', 'usda')) DESC",
		"length(name)",
	).Bind(first).Limit(candidates)
	var found []*core.Record
	err := query.All(&found)
	return found, err
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// stemmed drops common plural endings from the words, or returns nil when
// there are none.
func stemmed(words []string) []string {
	out := make([]string, len(words))
	changed := false
	for i, w := range words {
		out[i] = w
		if len(w) < 5 {
			continue
		}
		for _, end := range []string{"eren", "en", "es", "s"} {
			if strings.HasSuffix(w, end) {
				out[i] = strings.TrimSuffix(w, end)
				changed = true
				break
			}
		}
	}
	if !changed {
		return nil
	}
	return out
}

// usage counts how often the person logged each food in the last 90 days.
func usage(app core.App, userID string) map[string]int {
	since := types.NowDateTime().Add(-90 * 24 * time.Hour).String()[:10]
	var rows []struct {
		Food string `db:"food"`
		N    int    `db:"n"`
	}
	err := app.DB().
		Select("food", "count(*) AS n").
		From("entries").
		Where(dbx.HashExp{"user": userID}).
		AndWhere(dbx.NewExp("food != '' AND date >= {:since}", dbx.Params{"since": since})).
		GroupBy("food").
		All(&rows)
	uses := make(map[string]int, len(rows))
	if err != nil {
		return uses
	}
	for _, r := range rows {
		uses[r.Food] = r.N
	}
	return uses
}

// score rates how well a food matches the words.
func score(r *core.Record, words []string, userID string, uses int) float64 {
	best := 0.0
	for _, field := range []struct {
		name   string
		weight float64
	}{{"name", 1}, {"name_nl", 1}, {"name_fr", 1}, {"aliases", 0.9}} {
		for _, text := range strings.Split(r.GetString(field.name), ";") {
			if s := fieldScore(Normalize(text), words) * field.weight; s > best {
				best = s
			}
		}
	}
	// a brand adds to a name match: "boni yoghurt"
	total := best + 0.5*fieldScore(Normalize(r.GetString("brand")), words)

	if uses > 0 {
		total += 4 + float64(min(uses, 10))
	}
	switch source := r.GetString("source"); {
	case r.GetString("owner") == userID:
		total += 3
	case source == "ciqual" || source == "nevo" || source == "usda":
		total += 2
	}
	// "Apple, raw" before "Apple pie, homemade, with puff pastry"
	total -= 0.15 * float64(len(strings.Fields(Normalize(r.GetString("name")))))
	return total
}

// fieldScore rates one name: each word found counts, more at the start of a
// word and more again as a whole word; a name that starts with the first word
// or holds every word counts extra.
func fieldScore(text string, words []string) float64 {
	if text == "" {
		return 0
	}
	padded := " " + text + " "
	s, all := 0.0, true
	for _, w := range words {
		switch {
		case strings.Contains(padded, " "+w+" "):
			s += 4
		case strings.Contains(padded, " "+w):
			s += 3
		case strings.Contains(text, w):
			s += 1
		default:
			all = false
		}
	}
	if strings.HasPrefix(text, words[0]) {
		s += 3
	}
	if all {
		s += 4
	}
	return s
}
