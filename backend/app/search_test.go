package app_test

import (
	"strings"
	"testing"

	"github.com/selfhostit/troye/foods"
)

// The bundled USDA table answers everyday searches with the plain food.
func TestSearchBundledFoods(t *testing.T) {
	a := newApp(t)
	defer a.Cleanup()
	for q, want := range map[string]string{
		"apple":      "Apples, raw",
		"banana":     "Bananas, raw",
		"white rice": "Rice, white,",
		"broccoli":   "Broccoli, raw",
		"cheddar":    "Cheese, cheddar",
	} {
		found, err := foods.Search(a, "", q, 3)
		if err != nil || len(found) == 0 || !strings.HasPrefix(found[0].GetString("name"), want) {
			t.Errorf("%s: %v %v", q, found, err)
		}
		if q == "white rice" && len(found) > 0 && !strings.Contains(found[0].GetString("name"), "cooked") {
			t.Errorf("white rice: %q isn't cooked", found[0].GetString("name"))
		}
	}
}
