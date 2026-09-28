package main

import (
	"database/sql"
	"errors"
	"os"

	"github.com/maurolnl/bolsa-de-trabajo-back/sql/schema"
	"github.com/pressly/goose/v3"
)

// migrateCommand es el argumento con el que el binario aplica las migraciones pendientes
// y termina, en vez de levantar la API. Railway lo ejecuta como preDeployCommand: si una
// migración falla, el deploy no sale y la versión anterior sigue atendiendo.
const migrateCommand = "migrate"

func runMigrations() error {
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		return errors.New("DB_URL is required to run migrations")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	goose.SetBaseFS(schema.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	return goose.Up(db, ".")
}
