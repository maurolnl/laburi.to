package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/cmd/middleware"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/employee"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/employer"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/jobposition"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/recommendation"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/scoring"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/timezone"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type application struct {
	config appConfig
	// queueClient es el transporte de recomendaciones, y db la conexión que comparten las
	// rutas y el worker. Viven acá, y no como globales, para que quien las necesite las
	// reciba inyectadas.
	queueClient queue.Client
	db          *sql.DB
}

type s3Config struct {
	bucket string
}

type appConfig struct {
	addr      string
	db        dbConfig
	s3Cfg     s3Config
	secretKey string
	queueCfg  queue.Config
	workerCfg queue.WorkerConfig
}

type dbConfig struct {
	dsn string
}

func (app *application) mount() http.Handler {
	mux := http.NewServeMux()

	psqlDB := app.mountDB()
	app.db = psqlDB

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("laburito!"))
	})

	app.mountFeatureRoutes(mux, psqlDB)

	spaURL := os.Getenv("SPA_URL")

	stack := middleware.CreateStack(
		middleware.Logger,
		middleware.CORS([]string{"http://localhost:5173", spaURL}),
	)

	return stack(mux)
}

func (app *application) mountFeatureRoutes(mux *http.ServeMux, psqlDB *sql.DB) {
	bucket := app.config.s3Cfg.bucket
	uploaderService := uploader.NewService(bucket, certificationsKeyPrefix)

	validator := validator.New(validator.WithRequiredStructEnabled())
	employeeRepo := employee.NewRepository(psqlDB)
	employeeHandler := employee.BuildHandlers(employeeRepo, validator, uploaderService)

	employee.RegisterRoutes(mux, employeeHandler, employeeRepo, app.config.secretKey)

	employerRepo := employer.NewRepository(psqlDB)
	employerHandler := employer.BuildHandlers(employerRepo, validator)
	employer.RegisterRoutes(mux, employerHandler, app.config.secretKey)

	jobPositionRepo := jobposition.NewRepository(psqlDB)
	jobPositionHandler := jobposition.BuildHandlers(jobPositionRepo, jobposition.NoopEventPublisher{}, validator)
	jobposition.RegisterRoutes(mux, jobPositionHandler, app.config.secretKey)

	userHandler := user.BuildHandlers(database.New(psqlDB), app.config.secretKey, validator)
	user.RegisterRoutes(mux, userHandler, app.config.secretKey)

	tzRepo := timezone.NewRepository(psqlDB)
	tzService := timezone.NewService(tzRepo)
	tzHandler := timezone.NewHandler(tzService)

	tzHandler.RegisterRoutes(mux, app.config.secretKey)
}

// mountQueue construye el transporte de recomendaciones y lo deja disponible en la
// aplicación. Devuelve error en vez de abortar: quien decide terminar el proceso es main.
func (app *application) mountQueue(ctx context.Context) error {
	client, err := queue.New(ctx, app.config.queueCfg)
	if err != nil {
		return err
	}

	app.queueClient = client

	// Una única línea de diagnóstico, sin URL de cola, región ni credenciales. Con el
	// transporte deshabilitado nadie recibe recomendaciones, y eso debe ser visible en el
	// arranque en vez de descubrirse por ausencia de resultados.
	log.Printf("Recommendation queue enabled: %t \n", app.config.queueCfg.Enabled)

	return nil
}

// startWorker arranca el consumidor de recomendaciones si esta instancia debe consumir.
//
// El worker es una goroutine de este mismo proceso y no un binario aparte: la separación que
// importa es que el ciclo de consumo no comparta el camino de request, y la lógica vive en
// internal/recommendation, así que extraerlo a su propio main más adelante es mover un
// entrypoint y no rediseñar.
//
// Se le inyecta scoring.Unavailable: el algoritmo de indicadores no existe y la épica prohíbe
// simularlo, así que todo batch con candidatos va a terminar en failed. Eso es el
// comportamiento pedido y no un defecto.
//
// El contexto es el del proceso: cancelarlo corta la recepción en curso y termina el ciclo.
func (app *application) startWorker(ctx context.Context) {
	should, reason := app.config.workerCfg.ShouldConsume(app.config.queueCfg)
	log.Printf("%s \n", reason)
	if !should {
		return
	}

	repo := recommendation.NewRepository(app.db)
	worker := recommendation.NewWorker(
		repo,
		repo,
		app.queueClient,
		scoring.Unavailable{},
		app.config.queueCfg.VisibilityTimeoutSeconds,
	)

	go worker.Run(ctx)
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
