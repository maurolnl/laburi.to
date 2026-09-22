package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue"
)

func main() {
	loadEnv()
	port := getPort()

	secretKey := os.Getenv("SECRET_KEY")
	s3Bucket := os.Getenv("AWS_S3_BUCKET")

	// La configuración de la cola se resuelve y valida antes de montar nada: un valor
	// obligatorio faltante o fuera de rango debe abortar acá y no en la primera llamada a
	// AWS, ya en producción.
	queueCfg, err := queue.LoadConfig(os.LookupEnv)
	if err != nil {
		logErrorAndFail(err)
	}

	cfg := appConfig{
		addr:      ":" + port,
		db:        dbConfig{},
		secretKey: secretKey,
		s3Cfg: s3Config{
			bucket: s3Bucket,
		},
		queueCfg: queueCfg,
	}

	api := application{
		config: cfg,
	}

	if err := api.mountQueue(context.Background()); err != nil {
		logErrorAndFail(err)
	}

	h := api.mount()
	if err := api.run(h); err != nil {
		logErrorAndFail(err)
	}
}

func loadEnv() {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		logErrorAndFail(err)
	}
}

func getPort() string {
	port := "8080"

	if envPort, exists := os.LookupEnv("APP_PORT"); exists {
		port = envPort
	}

	return port
}

func logErrorAndFail(err error) {
	log.Printf("Server has failed to start, err: %s", err)
	os.Exit(1)
}
