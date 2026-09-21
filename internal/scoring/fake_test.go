package scoring

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// fakeScorer es el doble determinista del contrato. Vive en un archivo _test.go a
// propósito: el criterio de aceptación pide un fake solo para tests y nunca un algoritmo
// temporal en producción, y el compilador lo garantiza mejor que la disciplina. Un
// constructor exportado en un archivo normal sería seleccionable desde cmd/api.go por
// error.
//
// Los consumidores de otros paquetes (LAB-33) escriben su propio doble contra la
// interfaz, como ya hace employee/fakes_test.go.
type fakeScorer struct {
	// outcomes fija el desenlace por par. Los pares ausentes usan fallback.
	outcomes map[pairKey]fakeOutcome
	fallback fakeOutcome
	// err, si está presente, hace fallar toda evaluación.
	err error
}

type pairKey struct {
	employeeID    int32
	jobPositionID int32
}

type fakeKind int

const (
	fakeScored fakeKind = iota
	fakeUnscored
	fakeRejected
)

type fakeOutcome struct {
	kind       fakeKind
	total      float64
	reason     string
	indicators []Indicator
}

func keyOf(pair Pair) pairKey {
	employeeID, jobPositionID := pair.IDs()
	return pairKey{employeeID: employeeID, jobPositionID: jobPositionID}
}

func (f fakeScorer) outcomeFor(pair Pair) fakeOutcome {
	if outcome, ok := f.outcomes[keyOf(pair)]; ok {
		return outcome
	}
	return f.fallback
}

func (f fakeScorer) Score(_ context.Context, pair Pair) (Result, error) {
	if f.err != nil {
		return Result{}, f.err
	}

	outcome := f.outcomeFor(pair)
	switch outcome.kind {
	case fakeRejected:
		return Rejected(pair, outcome.reason), nil
	case fakeUnscored:
		return Unscored(pair), nil
	default:
		return Scored(pair, outcome.total, outcome.indicators...), nil
	}
}

// ScoreAll es un loop sobre Score, que es la implementación trivial que el contrato
// permite. Falla entera para no devolver resultados parciales.
func (f fakeScorer) ScoreAll(ctx context.Context, pairs []Pair) ([]Result, error) {
	results := make([]Result, 0, len(pairs))
	for _, pair := range pairs {
		result, err := f.Score(ctx, pair)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (f fakeScorer) Eligible(_ context.Context, pair Pair) (Eligibility, error) {
	if f.err != nil {
		return Eligibility{}, f.err
	}
	if outcome := f.outcomeFor(pair); outcome.kind == fakeRejected {
		return Discarded(outcome.reason), nil
	}
	return Accepted(), nil
}

var (
	_ Scorer     = fakeScorer{}
	_ HardFilter = fakeScorer{}
)

func pairFor(employeeID, jobPositionID int32) Pair {
	pair := samplePair()
	pair.Employee.EmployeeID = employeeID
	pair.Job.JobPositionID = jobPositionID
	return pair
}

func TestFakeScorerIsReproducible(t *testing.T) {
	ctx := context.Background()
	pair := samplePair()
	fake := fakeScorer{fallback: fakeOutcome{kind: fakeScored, total: 0.64}}

	first, err := fake.Score(ctx, pair)
	if err != nil {
		t.Fatalf("Score() error = %v", err)
	}
	second, err := fake.Score(ctx, pair)
	if err != nil {
		t.Fatalf("Score() error = %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("the fake must be deterministic: %+v != %+v", first, second)
	}
	if *first.Total != *second.Total {
		t.Fatal("repeated evaluations must return the same score")
	}
}

func TestFakeScorerCoversEveryOutcome(t *testing.T) {
	ctx := context.Background()

	scoredPair := pairFor(1, 10)
	unscoredPair := pairFor(2, 20)
	rejectedPair := pairFor(3, 30)

	fake := fakeScorer{outcomes: map[pairKey]fakeOutcome{
		keyOf(scoredPair):   {kind: fakeScored, total: 0.9},
		keyOf(unscoredPair): {kind: fakeUnscored},
		keyOf(rejectedPair): {kind: fakeRejected, reason: "timezone mismatch"},
	}}

	scored, _ := fake.Score(ctx, scoredPair)
	if !scored.Eligible || !scored.HasScore() {
		t.Error("the fake must be able to produce a scored pair")
	}

	unscored, _ := fake.Score(ctx, unscoredPair)
	if !unscored.Eligible || unscored.HasScore() {
		t.Error("the fake must be able to produce an eligible pair without a score")
	}

	rejected, _ := fake.Score(ctx, rejectedPair)
	if rejected.Eligible || rejected.Reason == "" {
		t.Error("the fake must be able to produce a rejected pair")
	}

	failing := fakeScorer{err: ErrScoringUnavailable}
	if _, err := failing.Score(ctx, scoredPair); !errors.Is(err, ErrScoringUnavailable) {
		t.Error("the fake must be able to produce a failure")
	}
}

// assertScorerContract es el test que cualquier implementación futura de Scorer debe
// pasar. Verifica las tres obligaciones que el contrato le impone a ScoreAll:
// correspondencia uno a uno con orden preservado, equivalencia con Score par por par, y
// lote vacío sin error. LAB-33 debería correrlo contra la implementación que inyecte.
func assertScorerContract(t *testing.T, scorer Scorer, pairs []Pair) {
	t.Helper()

	ctx := context.Background()

	t.Run("empty batch", func(t *testing.T) {
		results, err := scorer.ScoreAll(ctx, nil)
		if err != nil {
			t.Fatalf("ScoreAll() on an empty batch error = %v, want nil", err)
		}
		if len(results) != 0 {
			t.Fatalf("ScoreAll() on an empty batch returned %d results", len(results))
		}
	})

	t.Run("one result per pair in order", func(t *testing.T) {
		results, err := scorer.ScoreAll(ctx, pairs)
		if err != nil {
			t.Fatalf("ScoreAll() error = %v", err)
		}
		if len(results) != len(pairs) {
			t.Fatalf("ScoreAll() returned %d results for %d pairs", len(results), len(pairs))
		}
		for i, result := range results {
			employeeID, jobPositionID := pairs[i].IDs()
			if result.EmployeeID != employeeID || result.JobPositionID != jobPositionID {
				t.Fatalf("result %d belongs to pair (%d, %d), want (%d, %d)", i, result.EmployeeID, result.JobPositionID, employeeID, jobPositionID)
			}
		}
	})

	t.Run("batch matches individual evaluation", func(t *testing.T) {
		results, err := scorer.ScoreAll(ctx, pairs)
		if err != nil {
			t.Fatalf("ScoreAll() error = %v", err)
		}
		for i, pair := range pairs {
			individual, err := scorer.Score(ctx, pair)
			if err != nil {
				t.Fatalf("Score() error = %v", err)
			}
			if !reflect.DeepEqual(individual, results[i]) {
				t.Fatalf("pair %d: batch result %+v differs from individual result %+v", i, results[i], individual)
			}
		}
	})
}

func TestFakeScorerHonoursTheBatchContract(t *testing.T) {
	scoredPair := pairFor(1, 10)
	unscoredPair := pairFor(2, 20)
	rejectedPair := pairFor(3, 30)

	fake := fakeScorer{outcomes: map[pairKey]fakeOutcome{
		keyOf(scoredPair):   {kind: fakeScored, total: 0.9, indicators: []Indicator{{Name: "experience", Weight: 1, Value: 0.9}}},
		keyOf(unscoredPair): {kind: fakeUnscored},
		keyOf(rejectedPair): {kind: fakeRejected, reason: "timezone mismatch"},
	}}

	assertScorerContract(t, fake, []Pair{scoredPair, unscoredPair, rejectedPair})
}

// Un par inelegible no invalida al resto del lote: cada par conserva su desenlace.
func TestMixedBatchKeepsEveryOutcome(t *testing.T) {
	scoredPair := pairFor(1, 10)
	rejectedPair := pairFor(3, 30)

	fake := fakeScorer{outcomes: map[pairKey]fakeOutcome{
		keyOf(scoredPair):   {kind: fakeScored, total: 0.5},
		keyOf(rejectedPair): {kind: fakeRejected, reason: "missing education"},
	}}

	results, err := fake.ScoreAll(context.Background(), []Pair{scoredPair, rejectedPair})
	if err != nil {
		t.Fatalf("ScoreAll() error = %v: an ineligible pair must not fail the batch", err)
	}
	if !results[0].HasScore() {
		t.Error("the eligible pair must keep its score")
	}
	if results[1].Eligible || results[1].HasScore() {
		t.Error("the ineligible pair must keep its own outcome")
	}
}

// Un fallo sí invalida el lote entero: devolver resultados parciales tentaría al worker a
// completar el batch con lo que alcanzó a llegar.
func TestFailureReturnsNoPartialResults(t *testing.T) {
	fake := fakeScorer{err: ErrScoringUnavailable}

	results, err := fake.ScoreAll(context.Background(), []Pair{pairFor(1, 10), pairFor(2, 20)})
	if !errors.Is(err, ErrScoringUnavailable) {
		t.Fatalf("ScoreAll() error = %v, want ErrScoringUnavailable", err)
	}
	if results != nil {
		t.Fatal("a failed batch must not return partial results")
	}
}
