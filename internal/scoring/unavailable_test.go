package scoring

import (
	"context"
	"errors"
	"testing"
)

func TestUnavailableNeverProducesScores(t *testing.T) {
	ctx := context.Background()
	pair := samplePair()

	t.Run("Score", func(t *testing.T) {
		result, err := Unavailable{}.Score(ctx, pair)
		if !errors.Is(err, ErrScoringUnavailable) {
			t.Fatalf("Score() error = %v, want ErrScoringUnavailable", err)
		}
		if result.HasScore() {
			t.Fatal("the production implementation must never return a score")
		}
		if result.Eligible {
			t.Fatal("a failed evaluation must not look like an eligible pair")
		}
	})

	t.Run("ScoreAll", func(t *testing.T) {
		results, err := Unavailable{}.ScoreAll(ctx, []Pair{pair, pair})
		if !errors.Is(err, ErrScoringUnavailable) {
			t.Fatalf("ScoreAll() error = %v, want ErrScoringUnavailable", err)
		}
		if results != nil {
			t.Fatal("ScoreAll must not return partial results: a half batch would tempt the caller to complete it")
		}
	})

	t.Run("Eligible", func(t *testing.T) {
		eligibility, err := Unavailable{}.Eligible(ctx, pair)
		if !errors.Is(err, ErrScoringUnavailable) {
			t.Fatalf("Eligible() error = %v, want ErrScoringUnavailable", err)
		}
		if eligibility.Eligible {
			t.Fatal("a failed hard filter must not declare the pair eligible")
		}
	})
}

// Sin pares no hay nada que puntuar y ninguna dependencia hace falta. Es el sujeto sin
// candidatos, que la épica define como estado vacío y no como fallo.
func TestUnavailableAcceptsEmptyBatch(t *testing.T) {
	results, err := Unavailable{}.ScoreAll(context.Background(), nil)
	if err != nil {
		t.Fatalf("ScoreAll() on an empty batch error = %v, want nil", err)
	}
	if len(results) != 0 {
		t.Fatalf("ScoreAll() on an empty batch returned %d results", len(results))
	}
}

func TestUnavailableSatisfiesBothPorts(t *testing.T) {
	var (
		scorer Scorer     = Unavailable{}
		filter HardFilter = Unavailable{}
	)
	if scorer == nil || filter == nil {
		t.Fatal("Unavailable must satisfy both ports")
	}
}
