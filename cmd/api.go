package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/employee"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type application struct {
	config config
}

type config struct {
	addr      string
	db        dbConfig
	secretKey string
}

type dbConfig struct {
	dsn string
}

func (app *application) mount() http.Handler {
	mux := http.NewServeMux()

	psqlDB := app.mountDB()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("laburito!"))
	})

	app.mountFeatureRoutes(mux, psqlDB)

	return mux
}

func (app *application) mountFeatureRoutes(mux *http.ServeMux, psqlDB *database.Queries) {
	validator := validator.New(validator.WithRequiredStructEnabled())

	employeeHandler := employee.BuildHandlers(psqlDB, validator)
	middleware := employee.MountEmployee{Middleware: app.authenticatedUserMiddleWare}
	employee.RegisterRoutes(mux, employeeHandler, middleware)

	userHandler := user.BuildHandlers(psqlDB, app.config.secretKey, validator)
	user.RegisterRoutes(mux, userHandler)
}

func (app *application) mountDB() *database.Queries {
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	return database.New(db)
}

func (app *application) run(h http.Handler) error {
	server := &http.Server{
		Addr:    app.config.addr,
		Handler: h,
	}

	log.Printf("Server listening on %s \n", app.config.addr)

	return server.ListenAndServe()
}
