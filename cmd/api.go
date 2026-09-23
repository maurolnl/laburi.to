package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

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
	recommendationTrigger := app.recommendationTrigger(psqlDB)

	employeeRepo := employee.NewRepository(psqlDB)
	employeeHandler := employee.BuildHandlers(employeeRepo, validator, uploaderService, recommendationTrigger)

	employee.RegisterRoutes(mux, employeeHandler, employeeRepo, app.config.secretKey)

	employerRepo := employer.NewRepository(psqlDB)
	employerHandler := employer.BuildHandlers(employerRepo, validator)
	employer.RegisterRoutes(mux, employerHandler, app.config.secretKey)

	jobPositionRepo := jobposition.NewRepository(psqlDB)
	jobPositionHandler := jobposition.BuildHandlers(jobPositionRepo, recommendationTrigger, validator)
	jobposition.RegisterRoutes(mux, jobPositionHandler, app.config.secretKey)

	userHandler := user.BuildHandlers(database.New(psqlDB), app.config.secretKey, validator)
	user.RegisterRoutes(mux, userHandler, app.config.secretKey)

	// El repositorio de recomendaciones satisface los dos puertos que la consulta necesita —el
	// conjunto vigente y la propiedad del sujeto—, así que el borde de lectura se arma con una
	// sola instancia y sin depender del interruptor de la cola: consultar no emite trabajo.
	recommendationRepo := recommendation.NewRepository(psqlDB)
	recommendationQueryHandler := recommendation.BuildQueryHandlers(recommendationRepo, recommendationRepo)
	recommendation.RegisterQueryRoutes(mux, recommendationQueryHandler, app.config.secretKey)

	tzRepo := timezone.NewRepository(psqlDB)
	tzService := timezone.NewService(tzRepo)
	tzHandler := timezone.NewHandler(tzService)

	tzHandler.RegisterRoutes(mux, app.config.secretKey)
}

// recommendationTrigger arma el disparador que los bordes de escritura de employee y
// jobposition usan para solicitar la regeneración. El mismo valor satisface los puertos de los
// dos paquetes, que reciben solo el identificador del sujeto.
//
// El interruptor de la cola decide sobre qué productor se arma, con el mismo criterio que
// startWorker usa para consumir. Con el transporte deshabilitado, QueueJobPublisher abriría un
// batch y lo dejaría en failed en cada alta y cada edición, porque queue.Disabled falla en vez
// de simular éxito: un apagado deliberado quedaría convertido en un rastro de fallos que nadie
// va a atender. NoopJobPublisher no abre nada.
func (app *application) recommendationTrigger(psqlDB *sql.DB) recommendation.Trigger {
	if !app.config.queueCfg.Enabled {
		return recommendation.NewTrigger(recommendation.NoopJobPublisher{})
	}

	repo := recommendation.NewRepository(psqlDB)

	return recommendation.NewTrigger(recommendation.NewQueueJobPublisher(repo, app.queueClient))
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

	// Las advertencias distinguen un apagado deliberado de uno causado por un interruptor mal
	// escrito. Sin ellas los dos se verían igual en el log de arriba.
	app.logQueueWarnings()

	return nil
}

func (app *application) logQueueWarnings() {
	for _, warning := range append(app.config.queueCfg.Warnings, app.config.workerCfg.Warnings...) {
		log.Printf("Recommendation configuration warning: %s \n", warning)
	}
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

// shutdownGrace es lo que se le da a las peticiones en curso para terminar antes de cerrar.
// Acotado a propósito: un plazo largo retrasa cada deploy sin beneficio, porque los handlers
// de esta API son cortos.
const shutdownGrace = 10 * time.Second

// run atiende hasta que el contexto se cancela y recién entonces cierra el servidor.
//
// El apagado explícito es obligatorio y no un lujo: atender la señal con signal.NotifyContext
// desactiva la terminación por defecto del proceso, y ListenAndServe no mira el contexto. Sin
// este cierre, un SIGTERM cancelaría el worker y dejaría el proceso sirviendo para siempre,
// hasta que el orquestador lo matara al vencer su propio plazo.
func (app *application) run(ctx context.Context, h http.Handler) error {
	server := &http.Server{
		Addr:    app.config.addr,
		Handler: h,
	}

	closed := make(chan error, 1)
	go func() {
		<-ctx.Done()

		// El plazo de gracia se desprende de la cancelación que lo disparó: usar el contexto
		// ya cancelado abortaría el cierre de inmediato, que es lo contrario de ordenado.
		grace, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownGrace)
		defer cancel()

		log.Printf("Shutting down, draining requests for up to %s \n", shutdownGrace)
		closed <- server.Shutdown(grace)
	}()

	log.Printf("Server listening on %s \n", app.config.addr)

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-closed
}
