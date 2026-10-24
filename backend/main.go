// Command plasto runs the app: the PocketBase API, the database and the web
// app built into ./pb_public, in one process.
package main

import (
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	"github.com/kalfasyan/plasto/app"
	_ "github.com/kalfasyan/plasto/migrations"
)

func main() {
	pb := pocketbase.New()

	var publicDir string
	pb.RootCmd.PersistentFlags().StringVar(&publicDir, "publicDir", "./pb_public", "the directory with the built web app")
	_ = pb.RootCmd.ParseFlags(os.Args[1:])

	// Collections are defined in Go migrations under ./migrations. Edits made in
	// the admin UI are never turned into migrations automatically.
	migratecmd.MustRegister(pb, pb.RootCmd, migratecmd.Config{
		TemplateLang: migratecmd.TemplateLangGo,
		Automigrate:  false,
	})

	app.Register(pb, app.Config{PublicDir: publicDir})

	if err := pb.Start(); err != nil {
		log.Fatal(err)
	}
}
