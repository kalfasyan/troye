package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// The food diary: entries get their totals from their food, and barcodes are
// looked up in Open Food Facts the first time someone scans them. The product
// is then kept as a food, so the next scan works without the internet.

// OpenFoodFactsURL is where products are looked up; tests point it elsewhere.
var OpenFoodFactsURL = "https://world.openfoodfacts.org"

var barcodePattern = regexp.MustCompile(`^\d{6,14}$`)

func registerDiary(app core.App) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/app/barcode/{code}", lookupBarcode).Bind(apis.RequireAuth("users"))
		return se.Next()
	})

	app.OnRecordCreate("entries").BindFunc(func(e *core.RecordEvent) error {
		if err := fillEntry(e.App, e.Record); err != nil {
			return err
		}
		return e.Next()
	})
	app.OnRecordUpdate("entries").BindFunc(func(e *core.RecordEvent) error {
		if err := fillEntry(e.App, e.Record); err != nil {
			return err
		}
		return e.Next()
	})
}

// fillEntry works out an entry's totals from its food and grams. An entry
// without a food is a quick add, and keeps the calories it was given.
func fillEntry(app core.App, entry *core.Record) error {
	foodID := entry.GetString("food")
	if foodID == "" {
		return nil
	}
	food, err := app.FindRecordById("foods", foodID)
	if err != nil {
		return errors.New("that food no longer exists")
	}
	if owner := food.GetString("owner"); owner != "" && owner != entry.GetString("user") {
		return errors.New("that food belongs to someone else")
	}
	if entry.GetString("name") == "" {
		entry.Set("name", food.GetString("name"))
	}
	factor := entry.GetFloat("grams") / 100
	for _, field := range []string{"kcal", "protein", "carbs", "fat"} {
		v := food.GetFloat(field) * factor
		if field == "kcal" {
			v = math.Round(v)
		} else {
			v = math.Round(v*10) / 10
		}
		entry.Set(field, v)
	}
	return nil
}

// offProduct is the part of an Open Food Facts product that Troye uses.
type offProduct struct {
	Status  int `json:"status"`
	Product struct {
		Name   string `json:"product_name"`
		Brands string `json:"brands"`
		// numbers, and also units as strings
		Nutriments map[string]any `json:"nutriments"`
		// a number, or a number in a string
		ServingQuantity any    `json:"serving_quantity"`
		ServingSize     string `json:"serving_size"`
	} `json:"product"`
}

var offClient = &http.Client{Timeout: 10 * time.Second}

func lookupBarcode(e *core.RequestEvent) error {
	code := e.Request.PathValue("code")
	if !barcodePattern.MatchString(code) {
		return e.BadRequestError("That doesn't look like a barcode.", nil)
	}
	if food, err := e.App.FindFirstRecordByFilter("foods",
		"barcode = {:code} && (owner = '' || owner = {:user})",
		map[string]any{"code": code, "user": e.Auth.Id}); err == nil {
		return e.JSON(http.StatusOK, food)
	}

	food, err := fetchProduct(e.App, code)
	if errors.Is(err, errUnknownProduct) {
		return e.NotFoundError("We don't know this product yet. Add it as your own food, with the values from the label.", nil)
	}
	if err != nil {
		e.App.Logger().Warn("Open Food Facts lookup failed", "code", code, "error", err)
		return e.Error(http.StatusBadGateway, "Couldn't reach Open Food Facts. Try again, or add the food yourself.", nil)
	}
	return e.JSON(http.StatusOK, food)
}

var errUnknownProduct = errors.New("unknown product")

// fetchProduct looks a barcode up in Open Food Facts and saves the product as
// a shared food.
func fetchProduct(app core.App, code string) (*core.Record, error) {
	url := fmt.Sprintf("%s/api/v2/product/%s.json?fields=product_name,brands,nutriments,serving_quantity,serving_size", OpenFoodFactsURL, code)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// Open Food Facts asks apps to say who they are
	req.Header.Set("User-Agent", "Troye/"+Version+" (self-hosted calorie tracker)")
	res, err := offClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, errUnknownProduct
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", res.StatusCode)
	}
	var p offProduct
	if err := json.NewDecoder(res.Body).Decode(&p); err != nil {
		return nil, err
	}
	nutrient := func(key string) (float64, bool) {
		v, ok := number(p.Product.Nutriments[key])
		return math.Max(v, 0), ok
	}
	kcal, hasKcal := nutrient("energy-kcal_100g")
	if !hasKcal {
		if kj, ok := nutrient("energy_100g"); ok {
			kcal, hasKcal = kj/4.184, true
		}
	}
	per100g := func(key string) float64 {
		v, _ := nutrient(key)
		return math.Min(v, 1000)
	}
	name := strings.TrimSpace(p.Product.Name)
	if p.Status != 1 || name == "" || !hasKcal {
		return nil, errUnknownProduct
	}

	foods, err := app.FindCollectionByNameOrId("foods")
	if err != nil {
		return nil, err
	}
	food := core.NewRecord(foods)
	brand, _, _ := strings.Cut(p.Product.Brands, ",")
	food.Load(map[string]any{
		"name":      truncate(name, 200),
		"brand":     truncate(strings.TrimSpace(brand), 200),
		"source":    "off",
		"source_id": code,
		"barcode":   code,
		"kcal":      math.Min(math.Round(kcal), 1000),
		"protein":   per100g("proteins_100g"),
		"carbs":     per100g("carbohydrates_100g"),
		"fat":       per100g("fat_100g"),
		"fiber":     per100g("fiber_100g"),
		"sugar":     per100g("sugars_100g"),
		"salt":      per100g("salt_100g"),
	})
	if g, ok := number(p.Product.ServingQuantity); ok && g > 0 && g <= 5000 {
		food.Set("serving_g", g)
		food.Set("serving_label", truncate(p.Product.ServingSize, 50))
	}
	if err := app.Save(food); err != nil {
		return nil, err
	}
	return food, nil
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

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
