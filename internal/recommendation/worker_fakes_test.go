package recommendation

import (
	"context"
	"sync"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/scoring"
)

// workerStore es el doble de persistencia del consumidor. Reproduce las garantías
// observables del repositorio —reclamo que no toca batches terminales y reemplazo que poda
// los batches anteriores del sujeto— y no su implementación.
//
// Es distinto de fakeStore, que es el doble del productor: cada uno declara como error lo que
// su sujeto de prueba no debe hacer. El consumidor nunca abre batches.
type workerStore struct {
	t *testing.T

	mu      sync.Mutex
	batches map[int32]Batch
	sets    map[int32][]Candidate

	transitions []batchTransition
	claims      []int32

	getErr        error
	claimErr      error
	completeErr   error
	transitionErr error
}

var _ RecommendationStore = (*workerStore)(nil)

func newWorkerStore(t *testing.T) *workerStore {
	t.Helper()

	return &workerStore{t: t, batches: map[int32]Batch{}, sets: map[int32][]Candidate{}}
}

// seed inserta un batch en el estado indicado, como haría una emisión previa.
func (s *workerStore) seed(batch Batch) Batch {
	s.mu.Lock()
	defer s.mu.Unlock()

	if batch.Status == "" {
		batch.Status = BatchPending
	}
	s.batches[batch.ID] = batch

	return batch
}

func (s *workerStore) seedSet(batchID int32, candidates []Candidate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sets[batchID] = candidates
}

func (s *workerStore) batch(batchID int32) (Batch, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	batch, ok := s.batches[batchID]
	return batch, ok
}

func (s *workerStore) set(batchID int32) []Candidate {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sets[batchID]
}

// statusesOf devuelve la secuencia de estados por la que pasó un batch, que es como se
// comprueba que un camino no abrió processing.
func (s *workerStore) statusesOf(batchID int32) []BatchStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	var statuses []BatchStatus
	for _, claimed := range s.claims {
		if claimed == batchID {
			statuses = append(statuses, BatchProcessing)
		}
	}
	for _, transition := range s.transitions {
		if transition.batchID == batchID {
			statuses = append(statuses, transition.status)
		}
	}

	return statuses
}

func (s *workerStore) CreateBatch(context.Context, Subject) (Batch, error) {
	s.t.Error("el consumidor no debe abrir batches: abrirlos es del productor")
	return Batch{}, nil
}

func (s *workerStore) GetBatch(_ context.Context, batchID int32) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.getErr != nil {
		return Batch{}, s.getErr
	}

	batch, ok := s.batches[batchID]
	if !ok {
		return Batch{}, ErrBatchNotFound
	}

	return batch, nil
}

func (s *workerStore) ClaimBatch(_ context.Context, batchID int32) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.claimErr != nil {
		return Batch{}, s.claimErr
	}

	batch, ok := s.batches[batchID]
	if !ok {
		return Batch{}, ErrBatchNotFound
	}
	if batch.Status == BatchCompleted || batch.Status == BatchFailed {
		return Batch{}, ErrBatchNotClaimable
	}

	batch.Status = BatchProcessing
	s.batches[batchID] = batch
	s.claims = append(s.claims, batchID)

	return batch, nil
}

func (s *workerStore) TransitionBatch(_ context.Context, batchID int32, status BatchStatus) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.transitions = append(s.transitions, batchTransition{batchID: batchID, status: status})

	if s.transitionErr != nil {
		return Batch{}, s.transitionErr
	}

	batch, ok := s.batches[batchID]
	if !ok {
		return Batch{}, ErrBatchNotFound
	}

	batch.Status = status
	s.batches[batchID] = batch

	return batch, nil
}

// CompleteBatch reproduce el reemplazo atómico: completa, guarda el conjunto y descarta los
// batches anteriores del mismo sujeto. Ante error no toca nada, que es lo que garantiza que
// el conjunto vigente anterior sobreviva a un fallo.
func (s *workerStore) CompleteBatch(_ context.Context, batchID int32, candidates []Candidate) (Batch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.completeErr != nil {
		return Batch{}, s.completeErr
	}

	batch, ok := s.batches[batchID]
	if !ok {
		return Batch{}, ErrBatchNotFound
	}

	batch.Status = BatchCompleted
	s.batches[batchID] = batch
	s.sets[batchID] = append([]Candidate(nil), candidates...)

	for id, other := range s.batches {
		if id == batchID || other.SubjectType != batch.SubjectType || other.subjectID() != batch.subjectID() {
			continue
		}
		delete(s.batches, id)
		delete(s.sets, id)
	}

	return batch, nil
}

func (s *workerStore) JobRecommendationsForEmployee(context.Context, int32, Page) (JobRecommendations, error) {
	s.t.Error("el consumidor no debe leer el conjunto vigente: leerlo es de los endpoints")
	return JobRecommendations{}, nil
}

func (s *workerStore) EmployeeRecommendationsForJobPosition(context.Context, int32, Page) (EmployeeRecommendations, error) {
	s.t.Error("el consumidor no debe leer el conjunto vigente: leerlo es de los endpoints")
	return EmployeeRecommendations{}, nil
}

// fakeCandidates resuelve universos preprogramados. Un sujeto que no fue programado no
// existe, que es la distinción entre "no hay para quién recomendar" y "no hay qué recomendar".
type fakeCandidates struct {
	mu sync.Mutex

	employees map[int32][]scoring.Pair
	jobs      map[int32][]scoring.Pair
	err       error

	employeeCalls int
	jobCalls      int
}

var _ CandidateSource = (*fakeCandidates)(nil)

func newFakeCandidates() *fakeCandidates {
	return &fakeCandidates{employees: map[int32][]scoring.Pair{}, jobs: map[int32][]scoring.Pair{}}
}

func (c *fakeCandidates) PairsForEmployee(_ context.Context, employeeID int32) ([]scoring.Pair, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.employeeCalls++
	if c.err != nil {
		return nil, c.err
	}

	pairs, ok := c.employees[employeeID]
	if !ok {
		return nil, ErrSubjectNotFound
	}

	return pairs, nil
}

func (c *fakeCandidates) PairsForJobPosition(_ context.Context, jobPositionID int32) ([]scoring.Pair, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.jobCalls++
	if c.err != nil {
		return nil, c.err
	}

	pairs, ok := c.jobs[jobPositionID]
	if !ok {
		return nil, ErrSubjectNotFound
	}

	return pairs, nil
}

// scoringStub NO implementa scoring.Availability a propósito: así los tests del camino feliz
// también comprueban que la comprobación previa es opcional y que un scorer sin ella se
// considera disponible.
type scoringStub struct {
	t *testing.T

	mu      sync.Mutex
	results []scoring.Result
	err     error
	calls   int
}

var _ scoring.Scorer = (*scoringStub)(nil)

func (s *scoringStub) Score(context.Context, scoring.Pair) (scoring.Result, error) {
	s.t.Error("el consumidor debe puntuar por lote y no par por par")
	return scoring.Result{}, nil
}

func (s *scoringStub) ScoreAll(_ context.Context, pairs []scoring.Pair) ([]scoring.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.results != nil {
		return s.results, nil
	}

	results := make([]scoring.Result, 0, len(pairs))
	for _, pair := range pairs {
		results = append(results, scoring.Unscored(pair))
	}

	return results, nil
}

func (s *scoringStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls
}

// unusableScorer falla la prueba ante cualquier evaluación. Sirve para comprobar que un
// universo vacío se completa sin consultar el scoring.
type unusableScorer struct {
	t *testing.T
}

var _ scoring.Scorer = (*unusableScorer)(nil)

func (s *unusableScorer) Score(context.Context, scoring.Pair) (scoring.Result, error) {
	s.t.Error("no se esperaba puntuar ningún par")
	return scoring.Result{}, nil
}

func (s *unusableScorer) ScoreAll(context.Context, []scoring.Pair) ([]scoring.Result, error) {
	s.t.Error("no se esperaba puntuar ningún lote")
	return nil, nil
}

func testPair(employeeID, jobPositionID int32) scoring.Pair {
	return scoring.Pair{
		Employee: scoring.EmployeeProfile{
			EmployeeID: employeeID,
			Experience: scoring.Experience2To5Y,
		},
		Job: scoring.JobRequirements{
			JobPositionID:          jobPositionID,
			RequiredExperience:     scoring.Experience1Y,
			RequiredEducationLevel: scoring.EducationUniversity,
		},
	}
}
