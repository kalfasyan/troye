package foods

import (
	"errors"
	"sync"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// TablesCollection records the edition and size of each imported table.
const TablesCollection = "food_tables"

// one import at a time per table
var importing sync.Map // table name -> *sync.Mutex

// ErrBusy is returned when the table is already being imported.
var ErrBusy = errors.New("this table is already being imported")

// importTable runs fill, which hands it foods one by one, and stores them in
// batches. It records the run in food_tables: running while it goes, then the
// edition, the count and the date, or the error.
func importTable(app core.App, name, version string, fill func(add func(Food) error) error) error {
	mu, _ := importing.LoadOrStore(name, &sync.Mutex{})
	if !mu.(*sync.Mutex).TryLock() {
		return ErrBusy
	}
	defer mu.(*sync.Mutex).Unlock()

	status, err := tableStatus(app, name)
	if err != nil {
		return err
	}
	status.Set("running", true)
	status.Set("error", "")
	if err := app.Save(status); err != nil {
		return err
	}

	var batch []Food
	flush := func() error {
		_, err := storeBatch(app, name, batch)
		batch = batch[:0]
		return err
	}
	err = fill(func(f Food) error {
		batch = append(batch, f)
		if len(batch) < batchSize {
			return nil
		}
		return flush()
	})
	if err == nil {
		err = flush()
	}

	status.Set("running", false)
	if err != nil {
		status.Set("error", truncate(err.Error(), 500))
	} else {
		if version != "" {
			status.Set("version", version)
		}
		status.Set("foods", countFoods(app, name))
		status.Set("imported", types.NowDateTime())
	}
	if saveErr := app.Save(status); saveErr != nil && err == nil {
		err = saveErr
	}
	return err
}

// each hands a table's foods to add.
func each(foods []Food) func(add func(Food) error) error {
	return func(add func(Food) error) error {
		for _, f := range foods {
			if err := add(f); err != nil {
				return err
			}
		}
		return nil
	}
}

func tableStatus(app core.App, name string) (*core.Record, error) {
	if r, err := app.FindFirstRecordByData(TablesCollection, "name", name); err == nil {
		return r, nil
	}
	col, err := app.FindCollectionByNameOrId(TablesCollection)
	if err != nil {
		return nil, err
	}
	r := core.NewRecord(col)
	r.Set("name", name)
	return r, nil
}

func countFoods(app core.App, source string) int {
	n, _ := app.CountRecords("foods", dbx.HashExp{"source": source})
	return int(n)
}
