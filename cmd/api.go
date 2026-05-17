package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

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
	psqlDB := database.New(db)
	employeeRepo := employee.NewRepository(psqlDB)
	employeeService := employee.NewService(employeeRepo)
	employeeHandler := employee.NewHandler(employeeService)

	userRepo := user.NewRepository(psqlDB)
	userService := user.NewService(userRepo, app.config.secretKey)
	userHandler := user.NewHandler(userService)

	mux.HandleFunc("POST /employees", app.authenticatedUserMiddleWare(employeeHandler.CreateEmployee))
	mux.HandleFunc("GET /employees/{employeeID}", app.authenticatedUserMiddleWare(employeeHandler.GetEmployee))

	mux.HandleFunc("POST /auth/register", userHandler.RegisterUser)
	mux.HandleFunc("POST /auth/login", userHandler.Login)

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
