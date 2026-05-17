package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/employee"
)

type application struct {
	config config
}

type config struct {
	addr string
	db   dbConfig
}

type dbConfig struct {
	dsn string
}

func (app *application) mount() http.Handler {
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	//Handlers
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("laburito!"))
	})

	employeeRepo := employee.NewRepository(database.New(db))
	employeeService := employee.NewService(employeeRepo)
	employeeHandler := employee.NewHandler(employeeService)

	mux.HandleFunc("POST /employees", employeeHandler.CreateEmployee)
	mux.HandleFunc("GET /employees/{employeeID}", employeeHandler.GetEmployee)

	return mux
}

func (app *application) run(h http.Handler) error {
	server := &http.Server{
		Addr:    app.config.addr,
		Handler: h,
	}

	log.Printf("Server listening on %s \n", app.config.addr)

	return server.ListenAndServe()
}
