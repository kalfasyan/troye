package foods

import (
	"os"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Pâté, de campagne":    "pate de campagne",
		"Crème brûlée":         "creme brulee",
		"Kaas Goudse 48+ jong": "kaas goudse 48 jong",
		"  Œuf  dur ":          "oeuf dur",
		"Vol-au-vent (kip)":    "vol au vent kip",
		"Würstchen; Straße":    "wurstchen strasse",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReadCiqual(t *testing.T) {
	tbl, err := ReadCiqual("testdata/ciqual.zip")
	if err != nil {
		t.Fatal(err)
	}
	if tbl.Version != "2025-11-03" || len(tbl.Foods) != 3 {
		t.Fatalf("version %q, %d foods", tbl.Version, len(tbl.Foods))
	}
	bread := tbl.Foods[0]
	if bread.SourceID != "7001" || bread.Name != "Bread, wholemeal" || bread.NameFR != "Pain complet ou intégral (à la farine T150)" ||
		bread.Group != "Cereal products" || bread.Kcal != 241 || bread.Protein != 9.59 || bread.Fiber != 7.5 || bread.Salt != 1.06 {
		t.Errorf("bread %+v", bread)
	}
	// traces and "< 0,01" count as 0, "-" is unknown
	apple := tbl.Foods[1]
	if apple.Fat != 0 || apple.Salt != 0 || apple.Kcal != 53.6 {
		t.Errorf("apple %+v", apple)
	}
}

func TestBundledCiqual(t *testing.T) {
	tbl, err := BundledCiqual()
	if err != nil {
		t.Fatal(err)
	}
	if tbl.Version == "" || len(tbl.Foods) == 0 {
		t.Fatalf("version %q, %d foods", tbl.Version, len(tbl.Foods))
	}
	for _, f := range tbl.Foods {
		if f.SourceID == "" || f.Name == "" || f.NameFR == "" {
			t.Fatalf("incomplete food %+v", f)
		}
	}
}

func TestReadNEVO(t *testing.T) {
	csv := "Voedingsmiddelgroep|Food group|NEVO-code|Voedingsmiddelnaam/Dutch food name|Engelse naam/Food name|Synoniem|Hoeveelheid/Quantity|ENERCJ|ENERCC|PROT|CHO|SUGAR|FAT|FIBT|NA\n" +
		"Kaas|Cheese|449|Kaas Goudse 48+ jong|Cheese Gouda 48+ young|Jonge kaas|per 100g|1479|356|23,5|0|0|29|0|820\n" +
		"Groenten|Vegetables|1|Witlof rauw|Chicory raw|Chicon/Brussels lof|per 100g|71|17|1,2|2,3|2,3|0,2|1,2|3\n" +
		"Overig|Other|2|Zonder energie|No energy||per 100g||||||||\n"
	xlsx, err := os.ReadFile("testdata/NEVO2025_v9.0.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []struct {
		name string
		data []byte
	}{
		{"NEVO2025_v9.0.csv", []byte(csv)},
		{"NEVO2025_v9.0.csv", []byte(strings.ReplaceAll(strings.ReplaceAll(csv, ",", "."), "|", ";"))},
		{"NEVO2025_v9.0.xlsx", xlsx},
	} {
		tbl, err := ReadNEVO(file.name, file.data)
		if err != nil {
			t.Fatalf("%s: %v", file.name, err)
		}
		if tbl.Version != "2025/9.0" || len(tbl.Foods) != 2 {
			t.Fatalf("%s: version %q, %d foods", file.name, tbl.Version, len(tbl.Foods))
		}
		cheese, chicory := tbl.Foods[0], tbl.Foods[1]
		if cheese.SourceID != "449" || cheese.Name != "Cheese Gouda 48+ young" || cheese.NameNL != "Kaas Goudse 48+ jong" ||
			cheese.Aliases != "Jonge kaas" || cheese.Group != "Cheese" || cheese.Kcal != 356 || cheese.Protein != 23.5 || cheese.Fat != 29 {
			t.Errorf("%s: cheese %+v", file.name, cheese)
		}
		// sodium in mg becomes salt in g
		if cheese.Salt < 2.049 || cheese.Salt > 2.051 {
			t.Errorf("%s: salt %v", file.name, cheese.Salt)
		}
		if chicory.Aliases != "Chicon; Brussels lof" {
			t.Errorf("%s: aliases %q", file.name, chicory.Aliases)
		}
	}

	if _, err := ReadNEVO("other.csv", []byte("name,kcal\nApple,52\n")); err == nil || !strings.Contains(err.Error(), "NEVO table") {
		t.Errorf("another CSV: %v", err)
	}
}

func TestReadOFFExport(t *testing.T) {
	export := strings.Join([]string{
		"code\tproduct_name\tgeneric_name\tbrands\tcountries_tags\tenergy-kcal_100g\tenergy_100g\tproteins_100g\tcarbohydrates_100g\tfat_100g\tsugars_100g\tsalt_100g\tsodium_100g\tserving_quantity\tserving_size",
		"5410000000001\tSpeculoos\t\tLotus,Lotus Bakeries\ten:belgium,en:france\t484\t\t4.9\t72.6\t19\t38\t0.9\t\t6.25\t1 biscuit",
		"8710000000001\t\tKarnemelk\tAH\ten:netherlands\t\t159\t3.5\t4\t0.5\t\t\t0.04\t\t",
		"3000000000001\tCroissant\t\t\ten:france\t400\t\t8\t45\t21\t\t1\t\t\t",
		"5410000000002\tNo calories\t\t\ten:belgium\t\t\t\t\t\t\t\t\t\t",
		"5410000000003\tImpossible\t\t\ten:belgium\t2000\t\t\t\t\t\t\t\t\t",
		"short\tline",
	}, "\n")
	var got []Food
	err := readOFFExport(strings.NewReader(export), func(f Food) error {
		got = append(got, f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("kept %+v", got)
	}
	lotus, milk := got[0], got[1]
	if lotus.Barcode != "5410000000001" || lotus.Name != "Speculoos" || lotus.Brand != "Lotus" || lotus.Kcal != 484 || lotus.ServingG != 6.25 {
		t.Errorf("speculoos %+v", lotus)
	}
	// the generic name stands in, kJ become kcal and sodium becomes salt
	if milk.Name != "Karnemelk" || milk.Kcal < 37.9 || milk.Kcal > 38.1 || milk.Salt != 0.1 {
		t.Errorf("karnemelk %+v", milk)
	}
}

func TestScore(t *testing.T) {
	// whole words beat the start of a word, which beats a word inside another
	words := []string{"melk"}
	if !(fieldScore("melk halfvolle", words) > fieldScore("melkchocolade", words) &&
		fieldScore("melkchocolade", words) > fieldScore("chocomelk", words)) {
		t.Error("melk ranks wrong")
	}
	if got := stemmed([]string{"appels", "kip"}); len(got) != 2 || got[0] != "appel" || got[1] != "kip" {
		t.Errorf("stemmed %q", got)
	}
	if stemmed([]string{"kip", "rijst"}) != nil {
		t.Error("stemmed words without plurals")
	}
}
