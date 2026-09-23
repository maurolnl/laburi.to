package recommendation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue/queuetest"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/scoring"
)

const testVisibilitySeconds = 60

type workerFixture struct {
	worker     *Worker
	store      *workerStore
	candidates *fakeCandidates
	client     *queuetest.Fake
}

func newWorkerFixture(t *testing.T, scorer scoring.Scorer) workerFixture {
	t.Helper()

	store := newWorkerStore(t)
	candidates := newFakeCandidates()
	client := queuetest.New()

	worker := NewWorker(store, candidates, client, scorer, testVisibilitySeconds)
	// Sin esto, cada fallo de recepción que un test ejercita costaría un segundo real.
	worker.receiveBackoff = time.Millisecond
	worker.heartbeatEvery = time.Hour

	return workerFixture{worker: worker, store: store, candidates: candidates, client: client}
}

// publish encola el mensaje que el productor emitiría para un batch ya abierto.
func (f workerFixture) publish(t *testing.T, batch Batch) {
	t.Helper()

	event, err := NewJobRequestedEvent("event-1", batch, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("armar el evento: %v", err)
	}
	body, err := event.Body()
	if err != nil {
		t.Fatalf("serializar el evento: %v", err)
	}
	if err := f.client.Send(context.Background(), body); err != nil {
		t.Fatalf("encolar el mensaje: %v", err)
	}
}

// drain recibe lo que haya en la cola y lo procesa, como una vuelta del ciclo.
func (f workerFixture) drain(t *testing.T) int {
	t.Helper()

	messages, err := f.client.Receive(context.Background())
	if err != nil {
		t.Fatalf("recibir: %v", err)
	}
	for _, message := range messages {
		f.worker.processMessage(context.Background(), message)
	}

	return len(messages)
}

func employeeBatch(id, employeeID int32, status BatchStatus) Batch {
	return Batch{ID: id, SubjectType: SubjectEmployee, EmployeeID: &employeeID, Status: status}
}

func jobBatch(id, jobPositionID int32, status BatchStatus) Batch {
	return Batch{ID: id, SubjectType: SubjectJobPosition, JobPositionID: &jobPositionID, Status: status}
}

// ── Ciclo de vida ────────────────────────────────────────────────────────────

func TestRunTerminaAlCancelarElContexto(t *testing.T) {
	fixture := newWorkerFixture(t, &scoringStub{t: t})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		fixture.worker.Run(ctx)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run no terminó tras cancelar el contexto")
	}
}

func TestRunNoProcesaNadaNuevoTrasLaCancelacion(t *testing.T) {
	fixture := newWorkerFixture(t, &scoringStub{t: t})

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.publish(t, batch)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fixture.worker.Run(ctx)

	if got, _ := fixture.store.batch(1); got.Status != BatchPending {
		t.Fatalf("un ciclo ya cancelado no debe iniciar procesamiento: el batch quedó %q", got.Status)
	}
	if fixture.client.Pending() != 1 {
		t.Fatal("el mensaje debe seguir en la cola")
	}
}

// Un fallo de recepción es de infraestructura: se registra y se reintenta, no apaga el ciclo.
func TestRunSobreviveAUnFalloDeRecepcion(t *testing.T) {
	fixture := newWorkerFixture(t, &scoringStub{t: t})

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.candidates.employees[10] = []scoring.Pair{testPair(10, 100)}
	fixture.publish(t, batch)

	fixture.client.FailReceive(errors.New("red caída"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		fixture.worker.Run(ctx)
	}()

	// Se restaura el transporte: si el fallo hubiera terminado el ciclo, el batch no se
	// resolvería nunca.
	time.Sleep(20 * time.Millisecond)
	fixture.client.FailReceive(nil)

	deadline := time.After(5 * time.Second)
	for {
		if got, ok := fixture.store.batch(1); ok && got.Status == BatchCompleted {
			break
		}
		select {
		case <-deadline:
			t.Fatal("el ciclo no se recuperó del fallo de recepción")
		case <-time.After(5 * time.Millisecond):
		}
	}

	cancel()
	<-done
}

// Cancelar mientras un mensaje se procesa no puede dejar el batch a medias: sería el estado
// zombi que bloquea al sujeto por el índice único parcial.
func TestElMensajeEnCursoSeTerminaAunqueSeCanceleElCiclo(t *testing.T) {
	fixture := newWorkerFixture(t, &scoringStub{t: t})

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.candidates.employees[10] = []scoring.Pair{testPair(10, 100)}
	fixture.publish(t, batch)

	messages, err := fixture.client.Receive(context.Background())
	if err != nil {
		t.Fatalf("recibir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fixture.worker.processMessage(ctx, messages[0])

	got, _ := fixture.store.batch(1)
	if got.Status != BatchCompleted {
		t.Fatalf("el batch quedó %q, se esperaba un estado terminal pese a la cancelación", got.Status)
	}
	if fixture.client.Pending() != 0 {
		t.Fatal("el mensaje debía reconocerse pese a la cancelación")
	}
}

func TestLaVisibilidadSeExtiendeMientrasSeProcesa(t *testing.T) {
	store := newWorkerStore(t)
	candidates := newFakeCandidates()
	client := queuetest.New()

	// El scorer bloquea hasta que el test lo libera, que es la forma de tener un
	// procesamiento largo sin esperas reales.
	release := make(chan struct{})
	extended := make(chan struct{}, 1)
	scorer := &blockingScorer{release: release}

	worker := NewWorker(store, candidates, &extensionSpy{Fake: client, extended: extended}, scorer, testVisibilitySeconds)
	worker.heartbeatEvery = time.Millisecond

	batch := store.seed(employeeBatch(1, 10, BatchPending))
	candidates.employees[10] = []scoring.Pair{testPair(10, 100)}

	event, _ := NewJobRequestedEvent("event-1", batch, time.Unix(0, 0).UTC())
	body, _ := event.Body()
	if err := client.Send(context.Background(), body); err != nil {
		t.Fatalf("encolar: %v", err)
	}
	messages, _ := client.Receive(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.processMessage(context.Background(), messages[0])
	}()

	select {
	case <-extended:
	case <-time.After(5 * time.Second):
		close(release)
		<-done
		t.Fatal("un procesamiento largo debe renovar el plazo del mensaje")
	}

	close(release)
	<-done
}

// ── Interpretación del mensaje ───────────────────────────────────────────────

func TestCuerpoIlegibleSeReconoceSinTocarNingunBatch(t *testing.T) {
	fixture := newWorkerFixture(t, &unusableScorer{t: t})

	fixture.store.seed(employeeBatch(1, 10, BatchPending))
	if err := fixture.client.Send(context.Background(), "esto no es json"); err != nil {
		t.Fatalf("encolar: %v", err)
	}

	fixture.drain(t)

	if got, _ := fixture.store.batch(1); got.Status != BatchPending {
		t.Fatalf("un cuerpo ilegible no debe tocar ningún batch: quedó %q", got.Status)
	}
	if fixture.client.Pending() != 0 {
		t.Fatal("un cuerpo ilegible se reconoce: se lee igual de mal la décima vez")
	}
}

func TestVersionDesconocidaSeReconoceSinTocarNingunBatch(t *testing.T) {
	fixture := newWorkerFixture(t, &unusableScorer{t: t})

	fixture.store.seed(employeeBatch(1, 10, BatchPending))
	body := fmt.Sprintf(
		`{"event_id":"e1","version":%d,"subject_type":"employee","subject_id":10,"batch_id":1,"emitted_at":"1970-01-01T00:00:00Z"}`,
		JobRequestedVersion+1,
	)
	if err := fixture.client.Send(context.Background(), body); err != nil {
		t.Fatalf("encolar: %v", err)
	}

	fixture.drain(t)

	if got, _ := fixture.store.batch(1); got.Status != BatchPending {
		t.Fatalf("una versión desconocida no debe tocar ningún batch: quedó %q", got.Status)
	}
	if fixture.client.Pending() != 0 {
		t.Fatal("una versión desconocida se reconoce: interpretarla sería adivinar")
	}
}

func TestBatchInexistenteOTerminalSeReconoceSinTrabajo(t *testing.T) {
	cases := []struct {
		name   string
		seed   *BatchStatus
		expect BatchStatus
	}{
		{name: "inexistente"},
		{name: "completed", seed: statusPtr(BatchCompleted), expect: BatchCompleted},
		{name: "failed", seed: statusPtr(BatchFailed), expect: BatchFailed},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newWorkerFixture(t, &unusableScorer{t: t})

			batch := employeeBatch(1, 10, BatchPending)
			if testCase.seed != nil {
				batch.Status = *testCase.seed
				fixture.store.seed(batch)
				fixture.store.seedSet(1, []Candidate{{EmployeeID: 10, JobPositionID: 100}})
			}
			fixture.publish(t, employeeBatch(1, 10, BatchPending))

			fixture.drain(t)

			if fixture.client.Pending() != 0 {
				t.Fatal("un batch inexistente o terminal se reconoce sin trabajo")
			}
			if fixture.candidates.employeeCalls != 0 {
				t.Fatal("no debe resolverse ningún universo para un batch sin trabajo")
			}
			if testCase.seed == nil {
				return
			}
			if got, _ := fixture.store.batch(1); got.Status != testCase.expect {
				t.Fatalf("el estado cambió: quedó %q, era %q", got.Status, testCase.expect)
			}
			if len(fixture.store.set(1)) != 1 {
				t.Fatal("el conjunto del batch terminal debía quedar intacto")
			}
		})
	}
}

// Dos entregas del mismo mensaje dejan un único conjunto coherente: la segunda encuentra el
// batch ya completed y lo reconoce sin rehacer nada.
func TestIdempotenciaAnteRedelivery(t *testing.T) {
	scorer := &scoringStub{t: t, results: []scoring.Result{
		scoring.Scored(testPair(10, 100), 0.9),
	}}
	fixture := newWorkerFixture(t, scorer)

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.candidates.employees[10] = []scoring.Pair{testPair(10, 100)}
	fixture.publish(t, batch)

	messages, err := fixture.client.Receive(context.Background())
	if err != nil {
		t.Fatalf("recibir: %v", err)
	}

	// Primera entrega, reconocida. La segunda simula el redelivery de un mensaje cuyo
	// reconocimiento se perdió.
	fixture.worker.processMessage(context.Background(), messages[0])
	fixture.worker.processMessage(context.Background(), messages[0])

	if got := fixture.store.set(1); len(got) != 1 {
		t.Fatalf("el sujeto debe quedar con un único conjunto sin duplicados, tiene %d recomendaciones", len(got))
	}
	if scorer.callCount() != 1 {
		t.Fatalf("el segundo procesamiento no debe rehacer el trabajo: ScoreAll corrió %d veces", scorer.callCount())
	}
}

// Un batch que quedó en processing por un procesamiento interrumpido se reclama de nuevo: si
// no, el índice único parcial dejaría al sujeto bloqueado para siempre.
func TestBatchInterrumpidoEnProcessingSeRetoma(t *testing.T) {
	fixture := newWorkerFixture(t, &scoringStub{t: t})

	batch := fixture.store.seed(employeeBatch(1, 10, BatchProcessing))
	fixture.candidates.employees[10] = []scoring.Pair{testPair(10, 100)}
	fixture.publish(t, batch)

	fixture.drain(t)

	got, _ := fixture.store.batch(1)
	if got.Status != BatchCompleted {
		t.Fatalf("el batch interrumpido quedó %q, se esperaba completed", got.Status)
	}
	if fixture.client.Pending() != 0 {
		t.Fatal("el mensaje debía reconocerse")
	}
}

// ── Generación y reemplazo ───────────────────────────────────────────────────

func TestUniversoVacioCompletaElBatchSinConsultarElScoring(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		batch Batch
		seed  func(*fakeCandidates)
	}{
		{
			name:  "empleado",
			batch: employeeBatch(1, 10, BatchPending),
			seed:  func(c *fakeCandidates) { c.employees[10] = nil },
		},
		{
			name:  "puesto",
			batch: jobBatch(1, 100, BatchPending),
			seed:  func(c *fakeCandidates) { c.jobs[100] = nil },
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newWorkerFixture(t, &unusableScorer{t: t})

			batch := fixture.store.seed(testCase.batch)
			testCase.seed(fixture.candidates)
			fixture.publish(t, batch)

			fixture.drain(t)

			got, _ := fixture.store.batch(1)
			if got.Status != BatchCompleted {
				t.Fatalf("un universo vacío completa el batch, no lo falla: quedó %q", got.Status)
			}
			if len(fixture.store.set(1)) != 0 {
				t.Fatal("el conjunto debe quedar vacío")
			}
			if fixture.client.Pending() != 0 {
				t.Fatal("el mensaje debía reconocerse")
			}
		})
	}
}

// Es el comportamiento que la épica pide mientras el algoritmo no exista, y el test lo fija
// como tal: el día que llegue el algoritmo, este test tiene que cambiar.
func TestScoringNoDisponibleFallaElBatchSinAbrirProcessing(t *testing.T) {
	fixture := newWorkerFixture(t, scoring.Unavailable{})

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.candidates.employees[10] = []scoring.Pair{testPair(10, 100), testPair(10, 101)}
	fixture.publish(t, batch)

	fixture.drain(t)

	got, _ := fixture.store.batch(1)
	if got.Status != BatchFailed {
		t.Fatalf("sin scoring productivo el batch debe fallar explícitamente: quedó %q", got.Status)
	}
	for _, status := range fixture.store.statusesOf(1) {
		if status == BatchProcessing {
			t.Fatal("el batch no debe pasar por processing cuando el scoring no está disponible")
		}
	}
	if len(fixture.store.set(1)) != 0 {
		t.Fatal("no debe persistirse ninguna recomendación, ni siquiera sin puntaje")
	}
	if fixture.client.Pending() != 0 {
		t.Fatal("reintentar no puede prosperar mientras no exista el algoritmo: el mensaje se reconoce")
	}
}

func TestCaminoCompletoPersisteElConjuntoPuntuado(t *testing.T) {
	scorer := &scoringStub{t: t, results: []scoring.Result{
		scoring.Scored(testPair(10, 100), 0.9),
		scoring.Scored(testPair(10, 101), 0.1),
	}}
	fixture := newWorkerFixture(t, scorer)

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.candidates.employees[10] = []scoring.Pair{testPair(10, 100), testPair(10, 101)}
	fixture.publish(t, batch)

	fixture.drain(t)

	got, _ := fixture.store.batch(1)
	if got.Status != BatchCompleted {
		t.Fatalf("el batch quedó %q, se esperaba completed", got.Status)
	}

	set := fixture.store.set(1)
	if len(set) != 2 {
		t.Fatalf("se esperaban 2 recomendaciones, se obtuvieron %d", len(set))
	}
	for _, candidate := range set {
		if candidate.Score == nil {
			t.Fatalf("el puntaje del par (%d, %d) no viajó", candidate.EmployeeID, candidate.JobPositionID)
		}
	}
	// El batch pasó por processing: este es el único camino que lo abre.
	claimed := false
	for _, status := range fixture.store.statusesOf(1) {
		if status == BatchProcessing {
			claimed = true
		}
	}
	if !claimed {
		t.Fatal("el camino completo debe reclamar el batch antes de puntuar")
	}
}

func TestSoloLosParesElegiblesSePersisten(t *testing.T) {
	scorer := &scoringStub{t: t, results: []scoring.Result{
		scoring.Scored(testPair(10, 100), 0.9),
		scoring.Rejected(testPair(10, 101), "timezone incompatible"),
		scoring.Unscored(testPair(10, 102)),
	}}
	fixture := newWorkerFixture(t, scorer)

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.candidates.employees[10] = []scoring.Pair{
		testPair(10, 100), testPair(10, 101), testPair(10, 102),
	}
	fixture.publish(t, batch)

	fixture.drain(t)

	set := fixture.store.set(1)
	if len(set) != 2 {
		t.Fatalf("los inelegibles no se persisten: se esperaban 2 recomendaciones, hay %d", len(set))
	}

	byJob := map[int32]*float64{}
	for _, candidate := range set {
		byJob[candidate.JobPositionID] = candidate.Score
	}
	if _, rejected := byJob[101]; rejected {
		t.Fatal("un par descartado por filtro duro no es una recomendación")
	}
	if score, ok := byJob[102]; !ok || score != nil {
		t.Fatal("un par elegible sin puntaje sí se persiste, con puntaje ausente")
	}
	if score, ok := byJob[100]; !ok || score == nil || *score != 0.9 {
		t.Fatalf("el par puntuado debe conservar su total: %v", score)
	}
}

func TestSujetoInexistenteCompletaElBatchVacio(t *testing.T) {
	fixture := newWorkerFixture(t, &unusableScorer{t: t})

	// El sujeto no está programado en el doble: no existe.
	batch := fixture.store.seed(jobBatch(1, 100, BatchPending))
	fixture.publish(t, batch)

	fixture.drain(t)

	got, _ := fixture.store.batch(1)
	if got.Status != BatchCompleted {
		t.Fatalf("un sujeto que ya no existe cierra el batch, no lo deja abierto: quedó %q", got.Status)
	}
	if len(fixture.store.set(1)) != 0 {
		t.Fatal("no debe recomendarse nada para un sujeto que no está")
	}
	if fixture.client.Pending() != 0 {
		t.Fatal("el mensaje debía reconocerse")
	}
}

// ── Clasificación de errores y reconocimiento ────────────────────────────────

func TestClasificacionDeErrores(t *testing.T) {
	cases := []struct {
		name string
		err  error
		ack  bool
	}{
		{name: "sin error", err: nil, ack: true},
		{name: "cuerpo ilegible", err: fmt.Errorf("%w: x", errUnreadableMessage), ack: true},
		{name: "versión desconocida", err: fmt.Errorf("%w: 9", errUnknownVersion), ack: true},
		{name: "batch inexistente", err: ErrBatchNotFound, ack: true},
		{name: "batch ya terminal", err: ErrBatchNotClaimable, ack: true},
		{name: "sujeto inexistente", err: ErrSubjectNotFound, ack: true},
		{name: "sujeto inválido", err: ErrInvalidSubject, ack: true},
		{name: "scoring no disponible", err: scoring.ErrScoringUnavailable, ack: true},
		{name: "par inválido", err: fmt.Errorf("%w: %w", errScoringFailed, scoring.ErrInvalidPair), ack: true},
		{name: "fallo de scoring", err: fmt.Errorf("%w: raro", errScoringFailed), ack: true},
		{name: "fallo de base de datos", err: errors.New("connection refused"), ack: false},
		{name: "fallo envuelto de base de datos", err: fmt.Errorf("replace set: %w", errors.New("deadlock")), ack: false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := acknowledges(testCase.err); got != testCase.ack {
				t.Fatalf("acknowledges(%v) = %v, se esperaba %v", testCase.err, got, testCase.ack)
			}
		})
	}
}

func TestFalloDePersistenciaDevuelveElMensajeALaCola(t *testing.T) {
	fixture := newWorkerFixture(t, &scoringStub{t: t})

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.candidates.employees[10] = []scoring.Pair{testPair(10, 100)}
	fixture.store.completeErr = errors.New("connection refused")
	fixture.publish(t, batch)

	fixture.drain(t)

	if fixture.client.Pending() != 1 {
		t.Fatal("un fallo de infraestructura no se reconoce: el mensaje debe volver a la cola")
	}
	if len(fixture.store.set(1)) != 0 {
		t.Fatal("el reemplazo falló: no debe haber conjunto nuevo")
	}

	// El redelivery lo vuelve a entregar al vencer el plazo, con el conteo de recepciones
	// mayor; agotar el maxReceiveCount de la cola es lo que lo deriva a la DLQ.
	fixture.client.Advance(2 * queuetest.DefaultVisibilityTimeout)
	messages, err := fixture.client.Receive(context.Background())
	if err != nil {
		t.Fatalf("recibir: %v", err)
	}
	if len(messages) != 1 || messages[0].ReceiveCount != 2 {
		t.Fatalf("se esperaba un redelivery con ReceiveCount 2, se obtuvo %+v", messages)
	}
}

// El maxReceiveCount es un atributo de la cola en AWS y no de la aplicación; lo que la
// aplicación garantiza es no reconocer, que es lo que hace crecer el conteo hasta la DLQ.
func TestElConteoDeRecepcionesCreceMientrasNoSeReconoce(t *testing.T) {
	fixture := newWorkerFixture(t, &scoringStub{t: t})

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.candidates.employees[10] = []scoring.Pair{testPair(10, 100)}
	fixture.store.completeErr = errors.New("connection refused")
	fixture.publish(t, batch)

	for attempt := 1; attempt <= 3; attempt++ {
		messages, err := fixture.client.Receive(context.Background())
		if err != nil {
			t.Fatalf("recibir: %v", err)
		}
		if len(messages) != 1 {
			t.Fatalf("entrega %d: se esperaba 1 mensaje, llegaron %d", attempt, len(messages))
		}
		if messages[0].ReceiveCount != attempt {
			t.Fatalf("entrega %d: ReceiveCount = %d", attempt, messages[0].ReceiveCount)
		}

		fixture.worker.processMessage(context.Background(), messages[0])
		fixture.client.Advance(2 * queuetest.DefaultVisibilityTimeout)
	}
}

func TestFalloAlReconocerNoAlteraElEstadoPersistido(t *testing.T) {
	fixture := newWorkerFixture(t, &scoringStub{t: t})

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.candidates.employees[10] = []scoring.Pair{testPair(10, 100)}
	fixture.publish(t, batch)

	fixture.client.FailDelete(errors.New("red caída"))
	messages, err := fixture.client.Receive(context.Background())
	if err != nil {
		t.Fatalf("recibir: %v", err)
	}
	fixture.worker.processMessage(context.Background(), messages[0])

	got, _ := fixture.store.batch(1)
	if got.Status != BatchCompleted || len(fixture.store.set(1)) != 1 {
		t.Fatalf("el desenlace ya persistido no debe cambiar porque falle el reconocimiento: %+v", got)
	}

	// El redelivery se resuelve sin rehacer el trabajo.
	fixture.client.FailDelete(nil)
	fixture.client.Advance(2 * queuetest.DefaultVisibilityTimeout)
	fixture.drain(t)

	if len(fixture.store.set(1)) != 1 {
		t.Fatal("el redelivery no debe duplicar recomendaciones")
	}
	if fixture.client.Pending() != 0 {
		t.Fatal("el redelivery debía reconocer el mensaje")
	}
}

func TestUnMensajeEnvenenadoNoFrenaLosSiguientes(t *testing.T) {
	scorer := &scoringStub{t: t, results: []scoring.Result{scoring.Scored(testPair(10, 100), 0.5)}}
	fixture := newWorkerFixture(t, scorer)

	batch := fixture.store.seed(employeeBatch(1, 10, BatchPending))
	fixture.candidates.employees[10] = []scoring.Pair{testPair(10, 100)}

	if err := fixture.client.Send(context.Background(), "{{{ roto"); err != nil {
		t.Fatalf("encolar: %v", err)
	}
	fixture.publish(t, batch)

	if processed := fixture.drain(t); processed != 2 {
		t.Fatalf("se esperaban 2 mensajes en el lote, llegaron %d", processed)
	}

	got, _ := fixture.store.batch(1)
	if got.Status != BatchCompleted {
		t.Fatalf("el mensaje sano debía procesarse igual: el batch quedó %q", got.Status)
	}
	if fixture.client.Pending() != 0 {
		t.Fatal("ambos mensajes debían reconocerse")
	}
}

// ── Desacople ────────────────────────────────────────────────────────────────

// Sustituir la implementación de scoring cambia el conjunto persistido sin tocar el worker:
// es lo que permitirá conectar el algoritmo cuando exista.
func TestSustituirElScorerCambiaElConjuntoSinTocarElWorker(t *testing.T) {
	pairs := []scoring.Pair{testPair(10, 100)}

	withoutAlgorithm := newWorkerFixture(t, scoring.Unavailable{})
	withoutAlgorithm.store.seed(employeeBatch(1, 10, BatchPending))
	withoutAlgorithm.candidates.employees[10] = pairs
	withoutAlgorithm.publish(t, employeeBatch(1, 10, BatchPending))
	withoutAlgorithm.drain(t)

	withAlgorithm := newWorkerFixture(t, &scoringStub{
		t: t, results: []scoring.Result{scoring.Scored(testPair(10, 100), 0.75)},
	})
	withAlgorithm.store.seed(employeeBatch(1, 10, BatchPending))
	withAlgorithm.candidates.employees[10] = pairs
	withAlgorithm.publish(t, employeeBatch(1, 10, BatchPending))
	withAlgorithm.drain(t)

	failed, _ := withoutAlgorithm.store.batch(1)
	completed, _ := withAlgorithm.store.batch(1)
	if failed.Status != BatchFailed || completed.Status != BatchCompleted {
		t.Fatalf("estados = %q y %q, se esperaban failed y completed", failed.Status, completed.Status)
	}
	if len(withAlgorithm.store.set(1)) != 1 {
		t.Fatal("con algoritmo disponible el conjunto debe persistirse")
	}
}

func statusPtr(status BatchStatus) *BatchStatus { return &status }

// blockingScorer mantiene el procesamiento abierto hasta que el test lo libera.
type blockingScorer struct {
	release chan struct{}
}

var _ scoring.Scorer = (*blockingScorer)(nil)

func (s *blockingScorer) Score(context.Context, scoring.Pair) (scoring.Result, error) {
	return scoring.Result{}, nil
}

func (s *blockingScorer) ScoreAll(_ context.Context, pairs []scoring.Pair) ([]scoring.Result, error) {
	<-s.release

	results := make([]scoring.Result, 0, len(pairs))
	for _, pair := range pairs {
		results = append(results, scoring.Unscored(pair))
	}

	return results, nil
}

// extensionSpy avisa por un canal cada vez que se renueva el plazo de un mensaje.
type extensionSpy struct {
	*queuetest.Fake
	extended chan struct{}
}

var _ queue.Client = (*extensionSpy)(nil)

func (s *extensionSpy) ExtendVisibility(ctx context.Context, receiptHandle string, seconds int32) error {
	select {
	case s.extended <- struct{}{}:
	default:
	}

	return s.Fake.ExtendVisibility(ctx, receiptHandle, seconds)
}
