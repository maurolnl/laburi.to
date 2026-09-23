package scoring

import (
	"context"
	"errors"
	"testing"
)

func TestUnavailableDeclaresItselfUnavailable(t *testing.T) {
	if err := (Unavailable{}).Available(context.Background()); !errors.Is(err, ErrScoringUnavailable) {
		t.Fatalf("Available() error = %v, want ErrScoringUnavailable", err)
	}
}

// El valor de la comprobación previa es que el llamador decida sin evaluar. Un scorer espía
// que cuenta invocaciones lo hace observable: tras preguntar por la disponibilidad, ni Score
// ni ScoreAll deben haber corrido.
func TestAvailabilityCheckEvaluatesNoPair(t *testing.T) {
	spy := &spyScorer{err: ErrScoringUnavailable}

	if err := spy.Available(context.Background()); !errors.Is(err, ErrScoringUnavailable) {
		t.Fatalf("Available() error = %v, want ErrScoringUnavailable", err)
	}

	if spy.scoreCalls != 0 || spy.scoreAllCalls != 0 {
		t.Fatalf("the availability check must not evaluate: Score called %d times, ScoreAll %d times",
			spy.scoreCalls, spy.scoreAllCalls)
	}
}

// El fallo de la comprobación y el de la evaluación son el mismo error, así que un llamador
// los clasifica con un único errors.Is en vez de distinguir dos formas de decir lo mismo.
func TestAvailabilityFailureMatchesEvaluationFailure(t *testing.T) {
	ctx := context.Background()

	checkErr := (Unavailable{}).Available(ctx)
	_, scoreErr := Unavailable{}.Score(ctx, samplePair())

	if !errors.Is(checkErr, ErrScoringUnavailable) || !errors.Is(scoreErr, ErrScoringUnavailable) {
		t.Fatalf("both failures must classify the same: check = %v, score = %v", checkErr, scoreErr)
	}
}

// La comprobación es opcional: una implementación que siempre puede evaluar no está obligada
// a escribir el método, y el llamador la detecta con una aserción de tipo.
func TestAvailabilityIsOptional(t *testing.T) {
	var scorer Scorer = fakeScorer{fallback: fakeOutcome{kind: fakeScored, total: 1}}

	if _, ok := scorer.(Availability); ok {
		t.Fatal("a scorer that does not implement the check must not satisfy Availability")
	}

	// Sin comprobación disponible, el llamador asume disponibilidad y evalúa.
	if _, err := scorer.ScoreAll(context.Background(), []Pair{samplePair()}); err != nil {
		t.Fatalf("ScoreAll() error = %v, want nil", err)
	}
}

func TestUnavailableSatisfiesAvailability(t *testing.T) {
	var scorer Scorer = Unavailable{}

	if _, ok := scorer.(Availability); !ok {
		t.Fatal("the production implementation must expose the availability check")
	}
}

type spyScorer struct {
	err           error
	scoreCalls    int
	scoreAllCalls int
}

var (
	_ Scorer       = (*spyScorer)(nil)
	_ Availability = (*spyScorer)(nil)
)

func (s *spyScorer) Available(context.Context) error { return s.err }

func (s *spyScorer) Score(_ context.Context, _ Pair) (Result, error) {
	s.scoreCalls++
	return Result{}, s.err
}

func (s *spyScorer) ScoreAll(_ context.Context, _ []Pair) ([]Result, error) {
	s.scoreAllCalls++
	return nil, s.err
}
