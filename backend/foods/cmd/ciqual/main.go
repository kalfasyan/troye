// Command ciqual turns the Ciqual XML download from ciqual.anses.fr into the
// table that ships with Troye, backend/foods/data/ciqual.json.gz.
//
//	go run ./foods/cmd/ciqual Ciqual_2025_XML.zip
package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/selfhostit/troye/foods"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./foods/cmd/ciqual <Ciqual XML zip>")
		os.Exit(2)
	}
	t, err := foods.ReadCiqual(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_, here, _, _ := runtime.Caller(0)
	out := filepath.Join(filepath.Dir(here), "..", "..", "data", "ciqual.json.gz")
	if err := write(out, t); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Ciqual %s: %d foods written to %s\n", t.Version, len(t.Foods), out)
}

func write(path string, t foods.Table) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw, _ := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err := json.NewEncoder(zw).Encode(t); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}
