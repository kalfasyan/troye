package foods

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"golang.org/x/text/encoding/charmap"
)

// CiqualAttribution is shown wherever Ciqual foods are, as its licence asks.
const CiqualAttribution = "Anses. Ciqual French food composition table. https://ciqual.anses.fr/ (Licence Ouverte / Open Licence 2.0)"

// The Ciqual table ships with the app as data/ciqual.json.gz, made from the
// XML download on ciqual.anses.fr by: go run ./foods/cmd/ciqual <the zip>
//
//go:embed data/ciqual.json.gz
var bundledCiqual []byte

// Table is a food table: its edition and its foods.
type Table struct {
	Version string `json:"version"`
	Foods   []Food `json:"foods"`
}

// BundledCiqual reads the Ciqual table that ships with the app.
func BundledCiqual() (Table, error) {
	var t Table
	zr, err := gzip.NewReader(bytes.NewReader(bundledCiqual))
	if err != nil {
		return t, err
	}
	err = json.NewDecoder(zr).Decode(&t)
	return t, err
}

// ImportCiqual stores the bundled Ciqual table, unless that edition is
// already in.
func ImportCiqual(app core.App) error {
	t, err := BundledCiqual()
	if err != nil {
		return fmt.Errorf("reading the bundled Ciqual table: %w", err)
	}
	if status, err := app.FindFirstRecordByData(TablesCollection, "name", "ciqual"); err == nil &&
		status.GetString("version") == t.Version && status.GetInt("foods") > 0 {
		return nil
	}
	return importTable(app, "ciqual", t.Version, each(t.Foods))
}

// The Ciqual constituents Troye keeps, by their code in the table.
var ciqualConstituents = map[string]func(*Food) *float64{
	"328":   func(f *Food) *float64 { return &f.Kcal },    // Energy, Regulation EU No 1169/2011 (kcal/100 g)
	"25000": func(f *Food) *float64 { return &f.Protein }, // Protein, N x Jones' factor (g/100 g)
	"31000": func(f *Food) *float64 { return &f.Carbs },   // Carbohydrate (g/100 g)
	"40000": func(f *Food) *float64 { return &f.Fat },     // Fat (g/100 g)
	"34100": func(f *Food) *float64 { return &f.Fiber },   // Fibres (g/100 g)
	"32000": func(f *Food) *float64 { return &f.Sugar },   // Sugars (g/100 g)
	"10004": func(f *Food) *float64 { return &f.Salt },    // Salt (g/100 g)
}

// ReadCiqual reads the Ciqual XML download: a zip with the alim, alim_grp and
// compo files (and others it doesn't need). Foods without an energy value are
// left out.
func ReadCiqual(zipPath string) (Table, error) {
	var t Table
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return t, err
	}
	defer zr.Close()

	files := map[string]*zip.File{}
	for _, f := range zr.File {
		base := strings.ToLower(path.Base(f.Name))
		if !strings.HasSuffix(base, ".xml") {
			continue
		}
		for _, kind := range []string{"alim_grp", "alim", "compo"} {
			if strings.HasPrefix(base, kind+"_") && files[kind] == nil {
				files[kind] = f
				if kind == "alim" {
					// alim_2025_11_03.xml gives the edition
					t.Version = strings.TrimSuffix(strings.TrimPrefix(base, "alim_"), ".xml")
				}
				break
			}
		}
	}
	for _, kind := range []string{"alim", "alim_grp", "compo"} {
		if files[kind] == nil {
			return t, fmt.Errorf("the zip has no %s_*.xml file", kind)
		}
	}
	if d, err := time.Parse("2006_01_02", t.Version); err == nil {
		t.Version = d.Format("2006-01-02")
	}

	groups := map[string]string{}
	err = eachXMLRow(files["alim_grp"], func(row map[string]string) {
		groups[row["alim_grp_code"]] = capitalize(row["alim_grp_nom_eng"])
	})
	if err != nil {
		return t, err
	}

	byCode := map[string]*Food{}
	var order []string
	err = eachXMLRow(files["alim"], func(row map[string]string) {
		code := row["alim_code"]
		if code == "" || byCode[code] != nil {
			return
		}
		f := &Food{SourceID: code, Name: row["alim_nom_eng"], NameFR: row["alim_nom_fr"], Group: groups[row["alim_grp_code"]]}
		if f.Name == "" {
			f.Name = f.NameFR
		}
		byCode[code] = f
		order = append(order, code)
	})
	if err != nil {
		return t, err
	}

	hasKcal := map[string]bool{}
	err = eachXMLRow(files["compo"], func(row map[string]string) {
		f, field := byCode[row["alim_code"]], ciqualConstituents[row["const_code"]]
		if f == nil || field == nil {
			return
		}
		v, ok := ciqualValue(row["teneur"])
		if !ok {
			return
		}
		*field(f) = v
		if row["const_code"] == "328" {
			hasKcal[f.SourceID] = true
		}
	})
	if err != nil {
		return t, err
	}

	for _, code := range order {
		if hasKcal[code] {
			t.Foods = append(t.Foods, *byCode[code])
		}
	}
	if len(t.Foods) == 0 {
		return t, errors.New("no foods with an energy value in the zip")
	}
	return t, nil
}

// ciqualValue reads a value as Ciqual writes it: "12,5", "traces", "< 0,5",
// or "-" when it is unknown. Traces and values below the detection limit
// count as 0.
func ciqualValue(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	switch {
	case s == "" || s == "-":
		return 0, false
	case strings.EqualFold(s, "traces"), strings.HasPrefix(s, "<"):
		return 0, true
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	return v, err == nil
}

// eachXMLRow calls fn with each row of a Ciqual XML file: the children of the
// root's children, by their lowercased names, with their text trimmed. The
// files are in windows-1252.
func eachXMLRow(f *zip.File, fn func(map[string]string)) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	d := xml.NewDecoder(r)
	d.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		switch strings.ToLower(label) {
		case "windows-1252", "cp1252", "iso-8859-1", "latin1":
			return charmap.Windows1252.NewDecoder().Reader(input), nil
		case "utf-8", "":
			return input, nil
		}
		return nil, fmt.Errorf("unknown encoding %q", label)
	}

	depth := 0
	var row map[string]string
	var field string
	var text strings.Builder
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
			switch depth {
			case 2:
				row = map[string]string{}
			case 3:
				field = strings.ToLower(tok.Name.Local)
				text.Reset()
			}
		case xml.CharData:
			if depth == 3 {
				text.Write(tok)
			}
		case xml.EndElement:
			switch depth {
			case 3:
				row[field] = strings.TrimSpace(text.String())
			case 2:
				fn(row)
			}
			depth--
		}
	}
}

func capitalize(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}
