package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	// El flag del consumidor se resuelve aparte del transporte: son dos decisiones de
	// despliegue distintas, y con el transporte apagado LoadConfig no lee ninguna otra
	// variable.
	workerCfg, err := queue.LoadWorkerConfig(os.LookupEnv)
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
		queueCfg:  queueCfg,
		workerCfg: workerCfg,
	}

	api := application{
		config: cfg,
	}

	// El contexto del proceso termina con la señal de apagado. Es lo que corta la recepción
	// en curso del worker, que de otro modo seguiría bloqueada en el long polling de la cola.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := api.mountQueue(ctx); err != nil {
		logErrorAndFail(err)
	}

	h := api.mount()
	api.startWorker(ctx)
	if err := api.run(ctx, h); err != nil {
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
