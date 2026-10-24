package app

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/selfhostit/troye/foods"
	"github.com/selfhostit/troye/jobs"
)

// Food search, barcode lookup and the food table imports (backend/foods), and
// the monthly Open Food Facts download.
func registerFoods(app core.App) {
	foods.UserAgent = "Troye/" + Version + " (self-hosted calorie tracker)"

	app.OnRecordCreate("foods").BindFunc(func(e *core.RecordEvent) error {
		foods.SetSearch(e.Record)
		return e.Next()
	})
	app.OnRecordUpdate("foods").BindFunc(func(e *core.RecordEvent) error {
		foods.SetSearch(e.Record)
		return e.Next()
	})

	// on the 3rd of every month, at night
	jobs.MustAdd(app, offJob, "30 4 3 * *", foods.SyncOFF)

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// an import the server stopped in the middle of isn't running anymore
		if _, err := se.App.DB().Update(foods.TablesCollection, dbx.Params{"running": false}, dbx.HashExp{"running": true}).Execute(); err != nil {
			se.App.Logger().Warn("Could not reset the food table imports", "error", err)
		}

		users := apis.RequireAuth("users")
		se.Router.GET("/api/app/foods/search", searchFoods).Bind(users)
		se.Router.GET("/api/app/barcode/{code}", lookupBarcode).Bind(users)
		se.Router.POST("/api/app/foods/nevo", uploadNEVO).Bind(users, apis.BodyLimit(100<<20))
		se.Router.POST("/api/app/foods/off", startOFFSync).Bind(users)
		return se.Next()
	})
}

const offJob = "openfoodfacts"

func searchFoods(e *core.RequestEvent) error {
	limit, err := strconv.Atoi(e.Request.URL.Query().Get("limit"))
	if err != nil || limit <= 0 || limit > 100 {
		limit = 30
	}
	found, err := foods.Search(e.App, e.Auth.Id, e.Request.URL.Query().Get("q"), limit)
	if err != nil {
		return e.InternalServerError("", err)
	}
	return e.JSON(http.StatusOK, found)
}

func lookupBarcode(e *core.RequestEvent) error {
	code := e.Request.PathValue("code")
	if !barcodePattern.MatchString(code) {
		return e.BadRequestError("That doesn't look like a barcode.", nil)
	}
	food, err := foods.FindBarcode(e.App, code, e.Auth.Id)
	if errors.Is(err, foods.ErrUnknownProduct) {
		return e.NotFoundError("We don't know this product yet. Add it as your own food, with the values from the label.", nil)
	}
	if err != nil {
		e.App.Logger().Warn("Open Food Facts lookup failed", "code", code, "error", err)
		return e.Error(http.StatusBadGateway, "Couldn't reach Open Food Facts. Try again, or add the food yourself.", nil)
	}
	return e.JSON(http.StatusOK, food)
}

func isOwner(e *core.RequestEvent) bool {
	return e.Auth != nil && e.Auth.GetString("role") == "owner"
}

var barcodePattern = regexp.MustCompile(`^\d{6,14}$`)

// uploadNEVO imports the NEVO file the owner downloaded from RIVM.
func uploadNEVO(e *core.RequestEvent) error {
	if !isOwner(e) {
		return e.ForbiddenError("Only the owner can import food tables.", nil)
	}
	file, header, err := e.Request.FormFile("file")
	if err != nil {
		return e.BadRequestError("Choose the NEVO file you downloaded.", err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return e.BadRequestError("Couldn't read that file.", err)
	}
	err = foods.ImportNEVO(e.App, header.Filename, data)
	var bad foods.FormatError
	if errors.As(err, &bad) {
		return e.BadRequestError(bad.Error()+" Choose the CSV or Excel file from the NEVO download.", err)
	}
	if err != nil {
		if errors.Is(err, foods.ErrBusy) {
			return e.Error(http.StatusConflict, "NEVO is already being imported.", nil)
		}
		return e.InternalServerError("", err)
	}
	status, err := e.App.FindFirstRecordByData(foods.TablesCollection, "name", "nevo")
	if err != nil {
		return e.InternalServerError("", err)
	}
	return e.JSON(http.StatusOK, status)
}

// startOFFSync starts the Open Food Facts download now, instead of waiting for
// the monthly run. It takes a few minutes; food_tables shows the progress.
func startOFFSync(e *core.RequestEvent) error {
	if !isOwner(e) {
		return e.ForbiddenError("Only the owner can update food tables.", nil)
	}
	if status, err := e.App.FindFirstRecordByData(foods.TablesCollection, "name", "off"); err == nil && status.GetBool("running") {
		return e.Error(http.StatusConflict, "Open Food Facts is already downloading.", nil)
	}
	go jobs.Run(e.App, offJob, foods.SyncOFF)
	return e.NoContent(http.StatusAccepted)
}
