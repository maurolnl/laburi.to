package queue

import (
	"context"
	"errors"
	"testing"
)

// Un consumidor debe recibir un Client utilizable y no nil, y toda operación sobre el
// transporte apagado debe fallar de forma identificable en vez de simular éxito.
func TestDisabledFailsEveryOperation(t *testing.T) {
	var client Client = Disabled{}
	ctx := context.Background()

	t.Run("send", func(t *testing.T) {
		if err := client.Send(ctx, "body"); !errors.Is(err, ErrQueueDisabled) {
			t.Fatalf("Send() = %v, want ErrQueueDisabled", err)
		}
	})

	t.Run("receive", func(t *testing.T) {
		messages, err := client.Receive(ctx)
		if !errors.Is(err, ErrQueueDisabled) {
			t.Fatalf("Receive() = %v, want ErrQueueDisabled", err)
		}
		if len(messages) != 0 {
			t.Errorf("Receive() returned %d messages, want none", len(messages))
		}
	})

	t.Run("delete", func(t *testing.T) {
		if err := client.Delete(ctx, "receipt"); !errors.Is(err, ErrQueueDisabled) {
			t.Fatalf("Delete() = %v, want ErrQueueDisabled", err)
		}
	})

	t.Run("extend visibility", func(t *testing.T) {
		if err := client.ExtendVisibility(ctx, "receipt", 30); !errors.Is(err, ErrQueueDisabled) {
			t.Fatalf("ExtendVisibility() = %v, want ErrQueueDisabled", err)
		}
	})
}
