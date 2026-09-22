package queuetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue"
)

func TestSendReceiveDelete(t *testing.T) {
	fake := New()
	ctx := context.Background()

	if err := fake.Send(ctx, "body-1"); err != nil {
		t.Fatalf("Send() = %v, want nil", err)
	}

	messages, err := fake.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Receive() returned %d messages, want 1", len(messages))
	}
	if messages[0].Body != "body-1" {
		t.Errorf("Body = %q, want body-1", messages[0].Body)
	}
	if messages[0].ReceiveCount != 1 {
		t.Errorf("ReceiveCount = %d, want 1", messages[0].ReceiveCount)
	}

	if err := fake.Delete(ctx, messages[0].ReceiptHandle); err != nil {
		t.Fatalf("Delete() = %v, want nil", err)
	}
	if fake.Pending() != 0 {
		t.Errorf("Pending() = %d, want 0", fake.Pending())
	}
}

// Un mensaje recibido no debe volver a entregarse mientras dure su visibility timeout: es lo
// que impide que dos workers procesen el mismo batch en paralelo.
func TestReceivedMessageIsInvisibleUntilItsTimeoutExpires(t *testing.T) {
	fake := New().WithVisibilityTimeout(30 * time.Second)
	ctx := context.Background()

	if err := fake.Send(ctx, "body-1"); err != nil {
		t.Fatalf("Send() = %v, want nil", err)
	}
	if _, err := fake.Receive(ctx); err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}

	fake.Advance(29 * time.Second)

	messages, err := fake.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
	if len(messages) != 0 {
		t.Fatalf("Receive() returned %d messages, want none while the message is invisible", len(messages))
	}
}

// Entrega al menos una vez: un mensaje no confirmado vuelve, con su conteo incrementado y un
// receipt handle nuevo.
func TestUnacknowledgedMessageIsRedelivered(t *testing.T) {
	fake := New().WithVisibilityTimeout(30 * time.Second)
	ctx := context.Background()

	if err := fake.Send(ctx, "body-1"); err != nil {
		t.Fatalf("Send() = %v, want nil", err)
	}

	first, err := fake.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}

	fake.Advance(31 * time.Second)

	second, err := fake.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
	if len(second) != 1 {
		t.Fatalf("Receive() returned %d messages, want the redelivered one", len(second))
	}
	if second[0].ReceiveCount != 2 {
		t.Errorf("ReceiveCount = %d, want 2", second[0].ReceiveCount)
	}
	if second[0].ID != first[0].ID {
		t.Error("a redelivery must keep the same message id")
	}
	if second[0].ReceiptHandle == first[0].ReceiptHandle {
		t.Error("a redelivery must produce a new receipt handle")
	}
}

func TestDeletedMessageIsNotRedelivered(t *testing.T) {
	fake := New().WithVisibilityTimeout(30 * time.Second)
	ctx := context.Background()

	if err := fake.Send(ctx, "body-1"); err != nil {
		t.Fatalf("Send() = %v, want nil", err)
	}

	messages, err := fake.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
	if err := fake.Delete(ctx, messages[0].ReceiptHandle); err != nil {
		t.Fatalf("Delete() = %v, want nil", err)
	}

	fake.Advance(time.Hour)

	redelivered, err := fake.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
	if len(redelivered) != 0 {
		t.Fatalf("Receive() returned %d messages, want none after a delete", len(redelivered))
	}
}

func TestExtendVisibilityPostponesTheRedelivery(t *testing.T) {
	fake := New().WithVisibilityTimeout(30 * time.Second)
	ctx := context.Background()

	if err := fake.Send(ctx, "body-1"); err != nil {
		t.Fatalf("Send() = %v, want nil", err)
	}

	messages, err := fake.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
	if err := fake.ExtendVisibility(ctx, messages[0].ReceiptHandle, 120); err != nil {
		t.Fatalf("ExtendVisibility() = %v, want nil", err)
	}

	fake.Advance(60 * time.Second)

	early, err := fake.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
	if len(early) != 0 {
		t.Fatal("the message must stay invisible within the extended deadline")
	}

	fake.Advance(61 * time.Second)

	late, err := fake.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
	if len(late) != 1 {
		t.Fatal("the message must come back once the extended deadline expires")
	}
}

// Un receipt handle vencido no es un error en SQS; el doble no debe inventar uno.
func TestDeleteWithAStaleReceiptHandleSucceeds(t *testing.T) {
	fake := New()

	if err := fake.Delete(context.Background(), "unknown-receipt"); err != nil {
		t.Fatalf("Delete() = %v, want nil", err)
	}
}

func TestSentExposesEveryPublishedBody(t *testing.T) {
	fake := New()
	ctx := context.Background()

	for _, body := range []string{"body-1", "body-2"} {
		if err := fake.Send(ctx, body); err != nil {
			t.Fatalf("Send() = %v, want nil", err)
		}
	}

	messages, err := fake.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
	for _, message := range messages {
		if err := fake.Delete(ctx, message.ReceiptHandle); err != nil {
			t.Fatalf("Delete() = %v, want nil", err)
		}
	}

	sent := fake.Sent()
	if len(sent) != 2 || sent[0] != "body-1" || sent[1] != "body-2" {
		t.Fatalf("Sent() = %v, want every published body in order, including the processed ones", sent)
	}

	sent[0] = "mutated"
	if fake.Sent()[0] != "body-1" {
		t.Error("Sent() must return a copy; a caller must not be able to mutate the fake")
	}
}

func TestForcedErrors(t *testing.T) {
	sentinel := errors.New("queue is unreachable")
	ctx := context.Background()

	t.Run("send", func(t *testing.T) {
		fake := New()
		fake.FailSend(sentinel)

		if err := fake.Send(ctx, "body"); !errors.Is(err, sentinel) {
			t.Fatalf("Send() = %v, want the forced error", err)
		}
		if fake.Pending() != 0 {
			t.Error("a failed send must not enqueue anything")
		}
	})

	t.Run("receive", func(t *testing.T) {
		fake := New()
		fake.FailReceive(sentinel)

		if _, err := fake.Receive(ctx); !errors.Is(err, sentinel) {
			t.Fatalf("Receive() = %v, want the forced error", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		fake := New()
		fake.FailDelete(sentinel)

		if err := fake.Delete(ctx, "receipt"); !errors.Is(err, sentinel) {
			t.Fatalf("Delete() = %v, want the forced error", err)
		}
	})

	t.Run("extend visibility", func(t *testing.T) {
		fake := New()
		fake.FailExtend(sentinel)

		if err := fake.ExtendVisibility(ctx, "receipt", 30); !errors.Is(err, sentinel) {
			t.Fatalf("ExtendVisibility() = %v, want the forced error", err)
		}
	})

	t.Run("cleared", func(t *testing.T) {
		fake := New()
		fake.FailSend(sentinel)
		fake.FailSend(nil)

		if err := fake.Send(ctx, "body"); err != nil {
			t.Fatalf("Send() = %v, want nil after clearing the forced error", err)
		}
	})
}

// El doble es un queue.Client: quien lo use en LAB-32 o LAB-33 no necesita adaptarlo.
func TestFakeSatisfiesTheQueuePort(t *testing.T) {
	var client queue.Client = New()

	if _, err := client.Receive(context.Background()); err != nil {
		t.Fatalf("Receive() = %v, want nil", err)
	}
}
