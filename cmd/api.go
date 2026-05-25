package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/employee"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type application struct {
	config appConfig
}

type s3Config struct {
	// uploader *transfermanager.Client
	bucket string
}

type appConfig struct {
	addr      string
	db        dbConfig
	s3Cfg     s3Config
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

func (app *application) mountFeatureRoutes(mux *http.ServeMux, psqlDB *sql.DB) {
	bucket := app.config.s3Cfg.bucket
	uploaderService := uploader.NewService(bucket, certificationsKeyPrefix)

	validator := validator.New(validator.WithRequiredStructEnabled())
	employeeRepo := employee.NewRepository(psqlDB)
	employeeHandler := employee.BuildHandlers(employeeRepo, validator, uploaderService)

	middleware := employee.MountEmployee{Middleware: app.authenticatedUserMiddleWare}
	employee.RegisterRoutes(mux, employeeHandler, middleware)

	userHandler := user.BuildHandlers(database.New(psqlDB), app.config.secretKey, validator)
	user.RegisterRoutes(mux, userHandler)
}

func (app *application) mountDB() *sql.DB {
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	return db
}

func (app *application) run(h http.Handler) error {
	server := &http.Server{
		Addr:    app.config.addr,
		Handler: h,
	}

	log.Printf("Server listening on %s \n", app.config.addr)

	return server.ListenAndServe()
}
