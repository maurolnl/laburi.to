package recommendation

import "testing"

func TestQueryStatusFrom(t *testing.T) {
	tests := []struct {
		batch BatchStatus
		want  QueryStatus
	}{
		{BatchPending, StatusPending},
		{BatchProcessing, StatusProcessing},
		{BatchCompleted, StatusCompleted},
		{BatchFailed, StatusFailed},
	}

	for _, test := range tests {
		t.Run(string(test.batch), func(t *testing.T) {
			if got := queryStatusFrom(test.batch); got != test.want {
				t.Fatalf("se esperaba %q, se obtuvo %q", test.want, got)
			}
		})
	}

	// Un estado que este código no conoce no debe llegar al cliente como un sexto valor.
	t.Run("estado desconocido", func(t *testing.T) {
		if got := queryStatusFrom(BatchStatus("archived")); got != StatusNone {
			t.Fatalf("se esperaba %q, se obtuvo %q", StatusNone, got)
		}
	})
}
