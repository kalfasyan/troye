package foods

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// OFFAttribution credits Open Food Facts, whose data is under the Open
// Database Licence.
const OFFAttribution = "Open Food Facts, https://world.openfoodfacts.org (Open Database Licence)"

// Where products are looked up and the monthly export comes from; tests point
// them elsewhere.
var (
	OpenFoodFactsURL = "https://world.openfoodfacts.org"
	OFFExportURL     = "https://static.openfoodfacts.org/data/en.openfoodfacts.org.products.csv.gz"
)

// UserAgent says who is asking, as Open Food Facts asks apps to.
var UserAgent = "Troye/dev (self-hosted calorie tracker)"

// The countries whose products are kept from the export.
var offCountries = []string{"en:belgium", "en:netherlands"}

// ErrUnknownProduct means Open Food Facts doesn't have the barcode, or has it
// without a name or calories.
var ErrUnknownProduct = errors.New("unknown product")

var offClient = &http.Client{Timeout: 10 * time.Second}

// FindBarcode returns the food with this barcode: one of the person's own
// foods, a product already kept, or else one looked up in Open Food Facts and
// kept for next time.
func FindBarcode(app core.App, code, userID string) (*core.Record, error) {
	codes := []any{code}
	if len(code) == 12 {
		// a UPC-A code is the EAN-13 code with a leading zero
		codes = append(codes, "0"+code)
	} else if len(code) == 13 && code[0] == '0' {
		codes = append(codes, code[1:])
	}
	var found []*core.Record
	err := app.RecordQuery("foods").
		AndWhere(dbx.In("barcode", codes...)).
		AndWhere(dbx.Or(dbx.HashExp{"owner": ""}, dbx.HashExp{"owner": userID})).
		// your own version of a product first
		OrderBy("owner DESC").
		Limit(1).
		All(&found)
	if err != nil {
		return nil, err
	}
	if len(found) > 0 {
		return found[0], nil
	}

	f, err := fetchProduct(code)
	if err != nil {
		return nil, err
	}
	if _, err := Store(app, "off", []Food{f}); err != nil {
		return nil, err
	}
	return app.FindFirstRecordByFilter("foods", "source = 'off' && source_id = {:code}", dbx.Params{"code": f.SourceID})
}

// fetchProduct looks a barcode up in Open Food Facts.
func fetchProduct(code string) (Food, error) {
	url := fmt.Sprintf("%s/api/v2/product/%s.json?fields=%s", OpenFoodFactsURL, code, strings.Join(offFields, ","))
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Food{}, err
	}
	req.Header.Set("User-Agent", UserAgent)
	res, err := offClient.Do(req)
	if err != nil {
		return Food{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return Food{}, ErrUnknownProduct
	}
	if res.StatusCode != http.StatusOK {
		return Food{}, fmt.Errorf("status %d", res.StatusCode)
	}
	var body struct {
		Status  int            `json:"status"`
		Code    string         `json:"code"`
		Product map[string]any `json:"product"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return Food{}, err
	}
	if body.Status != 1 {
		return Food{}, ErrUnknownProduct
	}
	p := body.Product
	nutriments, _ := p["nutriments"].(map[string]any)
	field := func(key string) string {
		if s, ok := p[key].(string); ok {
			return s
		}
		if v, ok := number(p[key]); ok {
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
		return ""
	}
	nutrient := func(key string) string {
		if v, ok := number(nutriments[key]); ok {
			return strconv.FormatFloat(v, 'f', -1, 64)
		}
		return ""
	}
	if body.Code == "" {
		body.Code = code
	}
	f, ok := offFood(body.Code, field, nutrient)
	if !ok {
		return Food{}, ErrUnknownProduct
	}
	return f, nil
}

// The product fields Troye asks for.
var offFields = []string{
	"code", "product_name", "product_name_en", "product_name_nl", "product_name_fr", "generic_name",
	"brands", "nutriments", "serving_quantity", "serving_size",
}

// offFood makes a food from a product, whether it came from the API or the
// export, given the product's fields and its nutrients per 100 g. It reports
// false for a product without a name or calories.
func offFood(code string, field, nutrient func(string) string) (Food, bool) {
	num := func(s string) (float64, bool) {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
	}
	per100g := func(key string) float64 {
		v, _ := num(nutrient(key + "_100g"))
		return math.Max(v, 0)
	}

	f := Food{SourceID: code, Barcode: code, NameNL: field("product_name_nl"), NameFR: field("product_name_fr")}
	// the English name, else Dutch or French, else the name in the product's
	// own language, which stays searchable as an alias
	for _, name := range []string{field("product_name_en"), f.NameNL, f.NameFR, field("product_name"), field("generic_name")} {
		if name = strings.TrimSpace(name); name != "" {
			f.Name = name
			break
		}
	}
	if own := strings.TrimSpace(field("product_name")); own != "" && own != f.Name && own != f.NameNL && own != f.NameFR {
		f.Aliases = own
	}
	kcal, ok := num(nutrient("energy-kcal_100g"))
	if !ok {
		// energy_100g is in kJ
		for _, key := range []string{"energy-kj_100g", "energy_100g"} {
			if kj, ok2 := num(nutrient(key)); ok2 {
				kcal, ok = kj/4.184, true
				break
			}
		}
	}
	if f.Name == "" || !ok || kcal < 0 {
		return f, false
	}
	f.Kcal = kcal
	f.Protein = per100g("proteins")
	f.Carbs = per100g("carbohydrates")
	f.Fat = per100g("fat")
	f.Fiber = per100g("fiber")
	f.Sugar = per100g("sugars")
	f.Salt = per100g("salt")
	if f.Salt == 0 {
		f.Salt = per100g("sodium") * 2.5
	}
	brand, _, _ := strings.Cut(field("brands"), ",")
	f.Brand = strings.TrimSpace(brand)
	if g, ok := num(field("serving_quantity")); ok && g > 0 {
		f.ServingG, f.ServingLabel = g, field("serving_size")
	}
	return f, true
}

// plausible leaves out products whose values can't be right, which happens in
// a crowd-sourced database: more energy than pure fat, or more than 100 g of
// macronutrients in 100 g.
func plausible(f Food) bool {
	return f.Kcal <= 900 && f.Protein+f.Carbs+f.Fat <= 105
}

// SyncOFF downloads the Open Food Facts export (about a gigabyte, read as it
// streams in) and keeps the products sold in Belgium and the Netherlands that
// have a name and calories.
func SyncOFF(app core.App) error {
	req, err := http.NewRequest(http.MethodGet, OFFExportURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	client := &http.Client{Timeout: 3 * time.Hour}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading the Open Food Facts export: status %d", res.StatusCode)
	}
	var body io.Reader = res.Body
	if strings.HasSuffix(OFFExportURL, ".gz") {
		zr, err := gzip.NewReader(res.Body)
		if err != nil {
			return err
		}
		body = zr
	}
	version := time.Now().Format("2006-01-02")
	return importTable(app, "off", version, func(add func(Food) error) error {
		return readOFFExport(body, add)
	})
}

// readOFFExport reads the tab-separated export and hands add the products sold
// in the kept countries.
func readOFFExport(r io.Reader, add func(Food) error) error {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 1<<20), 64<<20)
	if !s.Scan() {
		if err := s.Err(); err != nil {
			return err
		}
		return errors.New("the Open Food Facts export is empty")
	}
	col := map[string]int{}
	for i, h := range strings.Split(s.Text(), "\t") {
		col[h] = i
	}
	for _, need := range []string{"code", "product_name", "countries_tags", "energy-kcal_100g"} {
		if _, ok := col[need]; !ok {
			return fmt.Errorf("the Open Food Facts export has no %s column", need)
		}
	}
	countries := col["countries_tags"]
	for s.Scan() {
		line := s.Text()
		if !containsAny(line, offCountries) {
			continue
		}
		cells := strings.Split(line, "\t")
		cell := func(key string) string {
			if i, ok := col[key]; ok && i < len(cells) {
				return cells[i]
			}
			return ""
		}
		if countries >= len(cells) || !containsAny(cells[countries], offCountries) {
			continue
		}
		code := cell("code")
		if !barcodePattern.MatchString(code) {
			continue
		}
		f, ok := offFood(code, cell, cell)
		if !ok || !plausible(f) {
			continue
		}
		if err := add(f); err != nil {
			return err
		}
	}
	return s.Err()
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// number reads a JSON value that is a number, or a number in a string.
func number(v any) (float64, bool) {
	switch v := v.(type) {
	case float64:
		return v, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	}
	return 0, false
}
