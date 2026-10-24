package app

import (
	"database/sql"
	"errors"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Weigh-ins and the profile's weight stay in step both ways: the latest
// weigh-in is the profile's weight (which moves the daily budget), and a new
// weight typed into the plan, as in onboarding, is logged as today's weigh-in.

func registerWeight(app core.App) {
	sync := func(e *core.RecordEvent) error {
		if err := syncProfileWeight(e.App, e.Record.GetString("user")); err != nil {
			e.App.Logger().Error("couldn't update the profile's weight", "error", err)
		}
		return e.Next()
	}
	app.OnRecordAfterCreateSuccess("weighins").BindFunc(sync)
	app.OnRecordAfterUpdateSuccess("weighins").BindFunc(sync)
	app.OnRecordAfterDeleteSuccess("weighins").BindFunc(sync)

	logWeight := func(e *core.RecordEvent) error {
		if err := logProfileWeight(e.App, e.Record, time.Now()); err != nil {
			e.App.Logger().Error("couldn't log the plan's weight", "error", err)
		}
		return e.Next()
	}
	app.OnRecordAfterCreateSuccess("profiles").BindFunc(logWeight)
	app.OnRecordAfterUpdateSuccess("profiles").BindFunc(logWeight)
}

// latestWeighin is the person's newest weigh-in, or nil when they have none.
func latestWeighin(app core.App, user string) (*core.Record, error) {
	list, err := app.FindRecordsByFilter("weighins", "user = {:user}", "-date", 1, 0, dbx.Params{"user": user})
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return list[0], nil
}

// syncProfileWeight sets the profile's weight to the latest weigh-in.
func syncProfileWeight(app core.App, user string) error {
	latest, err := latestWeighin(app, user)
	if err != nil || latest == nil {
		return err
	}
	p, err := app.FindFirstRecordByFilter("profiles", "user = {:user}", dbx.Params{"user": user})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if p.GetFloat("weight_kg") == latest.GetFloat("weight_kg") {
		return nil
	}
	p.Set("weight_kg", latest.GetFloat("weight_kg"))
	return app.Save(p)
}

// logProfileWeight records the profile's weight as a weigh-in on the given
// day, unless it is already the latest weigh-in.
func logProfileWeight(app core.App, profile *core.Record, now time.Time) error {
	kg := profile.GetFloat("weight_kg")
	if kg <= 0 {
		return nil
	}
	user := profile.GetString("user")
	latest, err := latestWeighin(app, user)
	if err != nil {
		return err
	}
	if latest != nil && latest.GetFloat("weight_kg") == kg {
		return nil
	}
	day := now.Format(time.DateOnly)
	w, err := app.FindFirstRecordByFilter("weighins", "user = {:user} && date = {:day}", dbx.Params{"user": user, "day": day})
	if errors.Is(err, sql.ErrNoRows) {
		weighins, err := app.FindCollectionByNameOrId("weighins")
		if err != nil {
			return err
		}
		w = core.NewRecord(weighins)
		w.Set("user", user)
		w.Set("date", day)
	} else if err != nil {
		return err
	}
	w.Set("weight_kg", kg)
	return app.Save(w)
}
