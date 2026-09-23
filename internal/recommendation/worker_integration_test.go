package recommendation

import (
	"context"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue/queuetest"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/scoring"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

// Estos tests recorren el camino completo contra la base real y el doble de la cola:
// productor, mensaje, worker y reemplazo. Los dobles en memoria fijan el comportamiento del
// worker; estos fijan que ese comportamiento sea el mismo con las transacciones de verdad.

// fixedScorer puntúa todo par elegible con el mismo total. No es un algoritmo: es lo mínimo
// para observar un conjunto persistido con puntaje.
type fixedScorer struct {
	total float64
}

var _ scoring.Scorer = fixedScorer{}

func (s fixedScorer) Score(_ context.Context, pair scoring.Pair) (scoring.Result, error) {
	return scoring.Scored(pair, s.total), nil
}

func (s fixedScorer) ScoreAll(_ context.Context, pairs []scoring.Pair) ([]scoring.Result, error) {
	results := make([]scoring.Result, 0, len(pairs))
	for _, pair := range pairs {
		results = append(results, scoring.Scored(pair, s.total))
	}

	return results, nil
}

type integrationFixture struct {
	repo      *RecommendationRepository
	client    *queuetest.Fake
	publisher *QueueJobPublisher
}

func newIntegrationFixture(t *testing.T) integrationFixture {
	t.Helper()

	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	client := queuetest.New()

	return integrationFixture{repo: repo, client: client, publisher: NewQueueJobPublisher(repo, client)}
}

// runWorker procesa lo que haya en la cola con el scorer indicado.
func (f integrationFixture) runWorker(t *testing.T, scorer scoring.Scorer) {
	t.Helper()

	worker := NewWorker(f.repo, f.repo, f.client, scorer, testVisibilitySeconds)
	worker.heartbeatEvery = time.Hour

	messages, err := f.client.Receive(context.Background())
	if err != nil {
		t.Fatalf("recibir: %v", err)
	}
	for _, message := range messages {
		worker.processMessage(context.Background(), message)
	}
}

func TestElConjuntoVigenteSobreviveAUnBatchPosteriorFallido(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()

	db := testsupport.PostgresDB(t)
	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	jobID := newTestJobPosition(t, db, employerID)

	if err := fixture.publisher.PublishJob(ctx, NewEmployeeSubject(employeeID)); err != nil {
		t.Fatalf("PublishJob() error = %v", err)
	}
	fixture.runWorker(t, fixedScorer{total: 0.8})

	before, err := fixture.repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
	if err != nil {
		t.Fatalf("JobRecommendationsForEmployee() error = %v", err)
	}
	if before.Status != BatchCompleted || len(before.Items) != 1 || before.Items[0].JobPositionID != jobID {
		t.Fatalf("el primer conjunto no quedó como se esperaba: %+v", before)
	}

	// Segunda solicitud del mismo sujeto, esta vez sin algoritmo disponible.
	if err := fixture.publisher.PublishJob(ctx, NewEmployeeSubject(employeeID)); err != nil {
		t.Fatalf("PublishJob() error = %v", err)
	}
	fixture.runWorker(t, scoring.Unavailable{})

	after, err := fixture.repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
	if err != nil {
		t.Fatalf("JobRecommendationsForEmployee() error = %v", err)
	}
	if after.Status != BatchFailed {
		t.Fatalf("el estado vigente debe reflejar el batch fallido, es %q", after.Status)
	}
	if len(after.Items) != 1 || after.Items[0].JobPositionID != jobID {
		t.Fatalf("un fallo debe conservar el conjunto vigente anterior, quedó %+v", after.Items)
	}
	if after.Items[0].Score == nil || *after.Items[0].Score != 0.8 {
		t.Fatalf("el puntaje del conjunto anterior también debe conservarse: %v", after.Items[0].Score)
	}
}

func TestUnBatchCompletadoReemplazaElConjuntoEntero(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()

	db := testsupport.PostgresDB(t)
	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	firstJob := newTestJobPosition(t, db, employerID)

	if err := fixture.publisher.PublishJob(ctx, NewEmployeeSubject(employeeID)); err != nil {
		t.Fatalf("PublishJob() error = %v", err)
	}
	fixture.runWorker(t, fixedScorer{total: 0.8})

	// El puesto anterior se elimina y aparece otro: el conjunto nuevo no debe conservar nada
	// del anterior.
	if _, err := db.ExecContext(ctx,
		`UPDATE job_positions SET deleted_at = now() WHERE id = $1`, firstJob); err != nil {
		t.Fatalf("eliminar el puesto: %v", err)
	}
	secondJob := newTestJobPosition(t, db, employerID)

	if err := fixture.publisher.PublishJob(ctx, NewEmployeeSubject(employeeID)); err != nil {
		t.Fatalf("PublishJob() error = %v", err)
	}
	fixture.runWorker(t, fixedScorer{total: 0.4})

	result, err := fixture.repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
	if err != nil {
		t.Fatalf("JobRecommendationsForEmployee() error = %v", err)
	}
	if result.Status != BatchCompleted {
		t.Fatalf("estado vigente = %q, se esperaba completed", result.Status)
	}
	if len(result.Items) != 1 || result.Items[0].JobPositionID != secondJob {
		t.Fatalf("el reemplazo debe dejar solo el conjunto nuevo, quedó %+v", result.Items)
	}
}

// El puesto se elimina entre la emisión y el consumo: el worker no puede recomendarlo aunque
// el mensaje se haya emitido cuando todavía estaba vigente.
func TestUnPuestoEliminadoTrasLaEmisionNoSeRecomienda(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()

	db := testsupport.PostgresDB(t)
	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	survivingJob := newTestJobPosition(t, db, employerID)
	doomedJob := newTestJobPosition(t, db, employerID)

	if err := fixture.publisher.PublishJob(ctx, NewEmployeeSubject(employeeID)); err != nil {
		t.Fatalf("PublishJob() error = %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`UPDATE job_positions SET deleted_at = now() WHERE id = $1`, doomedJob); err != nil {
		t.Fatalf("eliminar el puesto: %v", err)
	}

	fixture.runWorker(t, fixedScorer{total: 0.5})

	result, err := fixture.repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
	if err != nil {
		t.Fatalf("JobRecommendationsForEmployee() error = %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].JobPositionID != survivingJob {
		t.Fatalf("un puesto eliminado nunca debe recomendarse, quedó %+v", result.Items)
	}
}

// El sujeto completo de punta a punta en el otro sentido, para que ninguna de las dos
// direcciones quede solo cubierta por dobles.
func TestGeneracionEnSentidoPuestoContraLaBaseReal(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()

	db := testsupport.PostgresDB(t)
	firstEmployee := newTestEmployee(t, db)
	secondEmployee := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	jobID := newTestJobPosition(t, db, employerID)

	if err := fixture.publisher.PublishJob(ctx, NewJobPositionSubject(jobID)); err != nil {
		t.Fatalf("PublishJob() error = %v", err)
	}
	fixture.runWorker(t, fixedScorer{total: 0.6})

	result, err := fixture.repo.EmployeeRecommendationsForJobPosition(ctx, jobID, fullPage())
	if err != nil {
		t.Fatalf("EmployeeRecommendationsForJobPosition() error = %v", err)
	}
	if result.Status != BatchCompleted || len(result.Items) != 2 {
		t.Fatalf("se esperaban 2 empleados recomendados, quedó %+v", result)
	}

	recommended := map[int32]bool{}
	for _, item := range result.Items {
		recommended[item.EmployeeID] = true
	}
	if !recommended[firstEmployee] || !recommended[secondEmployee] {
		t.Fatalf("faltan empleados en el conjunto: %+v", result.Items)
	}
}

// Sin candidatos, el batch completa vacío contra la base real igual que contra los dobles: el
// estado vigente queda completed y el conjunto vacío, no un fallo.
func TestUniversoVacioContraLaBaseReal(t *testing.T) {
	fixture := newIntegrationFixture(t)
	ctx := context.Background()

	db := testsupport.PostgresDB(t)
	employeeID := newTestEmployee(t, db)

	if err := fixture.publisher.PublishJob(ctx, NewEmployeeSubject(employeeID)); err != nil {
		t.Fatalf("PublishJob() error = %v", err)
	}
	fixture.runWorker(t, scoring.Unavailable{})

	result, err := fixture.repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
	if err != nil {
		t.Fatalf("JobRecommendationsForEmployee() error = %v", err)
	}
	if result.Status != BatchCompleted {
		t.Fatalf("un sujeto sin candidatos completa el batch: quedó %q", result.Status)
	}
	if len(result.Items) != 0 {
		t.Fatalf("el conjunto debe quedar vacío, quedó %+v", result.Items)
	}
}
