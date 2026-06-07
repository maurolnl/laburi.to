package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	loadEnv()
	port := getPort()

	secretKey := os.Getenv("SECRET_KEY")
	s3Bucket := os.Getenv("AWS_S3_BUCKET")

	cfg := appConfig{
		addr:      ":" + port,
		db:        dbConfig{},
		secretKey: secretKey,
		s3Cfg: s3Config{
			bucket: s3Bucket,
		},
	}

	api := application{
		config: cfg,
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
