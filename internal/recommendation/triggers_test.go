package recommendation

import (
	"context"
	"errors"
	"testing"
)

// fakeJobPublisher registra las solicitudes recibidas sin abrir batches ni publicar nada. Es el
// doble que los tests de este paquete usan para afirmar qué sujeto llega al puerto.
type fakeJobPublisher struct {
	subjects []Subject
	err      error
}

var _ JobPublisher = (*fakeJobPublisher)(nil)

func (p *fakeJobPublisher) PublishJob(_ context.Context, subject Subject) error {
	p.subjects = append(p.subjects, subject)
	return p.err
}

func TestTriggerPublishesEmployeeSubject(t *testing.T) {
	publisher := &fakeJobPublisher{}
	trigger := NewTrigger(publisher)

	if err := trigger.EmployeeProfileCompleted(context.Background(), 42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.subjects) != 1 {
		t.Fatalf("expected exactly one request, got %d", len(publisher.subjects))
	}

	subject := publisher.subjects[0]
	if subject.Type != SubjectEmployee {
		t.Fatalf("expected subject type %q, got %q", SubjectEmployee, subject.Type)
	}
	if subject.EmployeeID == nil || *subject.EmployeeID != 42 {
		t.Fatalf("expected employee 42, got %v", subject.EmployeeID)
	}
	if subject.JobPositionID != nil {
		t.Fatalf("expected no job position on an employee subject, got %d", *subject.JobPositionID)
	}
	if !subject.Valid() {
		t.Fatal("expected a valid subject")
	}
}

func TestTriggerPublishesJobPositionSubject(t *testing.T) {
	publisher := &fakeJobPublisher{}
	trigger := NewTrigger(publisher)

	if err := trigger.JobPositionPublished(context.Background(), 7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publisher.subjects) != 1 {
		t.Fatalf("expected exactly one request, got %d", len(publisher.subjects))
	}

	subject := publisher.subjects[0]
	if subject.Type != SubjectJobPosition {
		t.Fatalf("expected subject type %q, got %q", SubjectJobPosition, subject.Type)
	}
	if subject.JobPositionID == nil || *subject.JobPositionID != 7 {
		t.Fatalf("expected job position 7, got %v", subject.JobPositionID)
	}
	if subject.EmployeeID != nil {
		t.Fatalf("expected no employee on a job position subject, got %d", *subject.EmployeeID)
	}
	if !subject.Valid() {
		t.Fatal("expected a valid subject")
	}
}

// TestTriggerPropagatesPublisherError comprueba que el error llega al borde de escritura tal
// cual: es ese borde el que decide registrarlo sin revertir su propio cambio, y envolverlo acá
// solo le agregaría ruido a la línea de diagnóstico.
func TestTriggerPropagatesPublisherError(t *testing.T) {
	cause := errors.New("transport unavailable")
	trigger := NewTrigger(&fakeJobPublisher{err: cause})

	t.Run("employee", func(t *testing.T) {
		err := trigger.EmployeeProfileCompleted(context.Background(), 1)
		if !errors.Is(err, cause) {
			t.Fatalf("expected the publisher cause, got %v", err)
		}
	})

	t.Run("job position", func(t *testing.T) {
		err := trigger.JobPositionPublished(context.Background(), 1)
		if !errors.Is(err, cause) {
			t.Fatalf("expected the publisher cause, got %v", err)
		}
	})
}

// TestTriggerOverNoopPublisherDoesNothing documenta el entorno sin emisión: el borde de
// escritura recibe igualmente un Trigger, no una referencia nula, y ninguna solicitud abre
// batch ni falla.
func TestTriggerOverNoopPublisherDoesNothing(t *testing.T) {
	trigger := NewTrigger(NoopJobPublisher{})

	if err := trigger.EmployeeProfileCompleted(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := trigger.JobPositionPublished(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
