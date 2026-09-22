package recommendation

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue"
)

// JobPublisher es el puerto de emisión de solicitudes de recomendación. No menciona ningún
// proveedor de colas ni la estructura del mensaje: sustituir el transporte no obliga a
// tocar a sus llamadores.
//
// Recibe Subject y no un par de parámetros sueltos porque sus constructores —
// NewEmployeeSubject, NewJobPositionSubject— impiden armar una combinación inválida, así
// que una sola operación cubre los dos sentidos de la recomendación.
//
// Devuelve solo error y no el Batch a propósito. El llamador es un borde de escritura que
// ya persistió su propio cambio; devolverle el batch lo tentaría a esperar su resultado,
// que es justo lo que la emisión asíncrona existe para evitar.
type JobPublisher interface {
	PublishJob(ctx context.Context, subject Subject) error
}

// NoopJobPublisher es la implementación inerte del puerto: no abre batch, no publica y no
// falla. Existe para que ningún llamador reciba nil ni tenga que comprobarlo, igual que
// queue.Disabled y scoring.Unavailable.
//
// A diferencia de esas dos, devuelve nil en lugar de error. No emitir es un estado de
// despliegue legítimo —es lo que hace hoy jobposition.NoopEventPublisher—; la prohibición de
// reportar éxito falso aplica a la implementación que sí dice haber publicado.
type NoopJobPublisher struct{}

var _ JobPublisher = NoopJobPublisher{}

func (NoopJobPublisher) PublishJob(context.Context, Subject) error {
	return nil
}

// QueueJobPublisher abre el batch en la persistencia y publica el mensaje en la cola, en ese
// orden. Su cero no es utilizable: construirlo con NewQueueJobPublisher.
type QueueJobPublisher struct {
	store  RecommendationStore
	client queue.Client

	// now y newEventID son inyectables para que los tests puedan afirmar el cuerpo completo
	// del mensaje contra un literal, en vez de comprobar campo por campo que "algo hay". No
	// se exponen: nadie fuera del paquete necesita cambiarlos.
	now        func() time.Time
	newEventID func() string
}

var _ JobPublisher = (*QueueJobPublisher)(nil)

func NewQueueJobPublisher(store RecommendationStore, client queue.Client) *QueueJobPublisher {
	return &QueueJobPublisher{
		store:      store,
		client:     client,
		now:        time.Now,
		newEventID: uuid.NewString,
	}
}

// PublishJob abre un batch pending para el sujeto y publica el mensaje que lo referencia.
//
// El orden importa: el mensaje lleva el identificador del batch, así que ningún mensaje
// puede publicarse antes de que su batch exista. Lo que no hace es calcular: no evalúa
// indicadores, no lee el perfil ni el puesto y no espera a que el consumidor procese nada.
//
// Desenlaces posibles:
//
//   - El sujeto ya tiene un batch pending o processing: no publica nada y devuelve nil. El
//     trabajo en curso ya va a producir el conjunto del sujeto, así que un segundo mensaje
//     solo agregaría trabajo que el worker tendría que descartar.
//   - La publicación falla: el batch recién abierto pasa a failed y el error se propaga
//     envolviendo el del transporte. Un transporte deshabilitado entra por acá.
//   - Cualquier otro fallo de la persistencia: se propaga sin publicar nada.
func (p *QueueJobPublisher) PublishJob(ctx context.Context, subject Subject) error {
	if !subject.Valid() {
		return ErrInvalidSubject
	}

	batch, err := p.store.CreateBatch(ctx, subject)
	if err != nil {
		if errors.Is(err, ErrBatchAlreadyInFlight) {
			// Es el caso normal de dos ediciones seguidas, no una anomalía: se registra como
			// diagnóstico y no como error. La línea nombra al sujeto solo por sus
			// identificadores.
			log.Printf("recommendation job for %s %d skipped: a batch is already in flight", subject.Type, subject.id())
			return nil
		}

		return fmt.Errorf("open recommendation batch: %w", err)
	}

	event, err := NewJobRequestedEvent(p.newEventID(), batch, p.now())
	if err != nil {
		return p.failBatch(ctx, batch.ID, err)
	}

	body, err := event.Body()
	if err != nil {
		return p.failBatch(ctx, batch.ID, err)
	}

	if err := p.client.Send(ctx, body); err != nil {
		return p.failBatch(ctx, batch.ID, fmt.Errorf("publish recommendation job: %w", err))
	}

	return nil
}

// failBatch cierra el batch que quedó abierto sin mensaje y devuelve la causa original.
//
// Sin esta transición el sujeto quedaría con un batch pending que nadie va a procesar y que,
// por el índice único parcial, bloquearía toda solicitud futura. failed es el estado que la
// épica reserva para eso y deja al sujeto disponible para el próximo intento.
//
// El contexto se desprende de la cancelación del original: la causa más común de un fallo de
// envío es que el llamador se fue, y usar un contexto ya cancelado para cerrar el batch
// dejaría justamente el estado zombi que esta función existe para evitar.
//
// Si la transición también falla, se devuelve igual la causa original: cambiar el error
// ocultaría por qué no hay mensaje, que es lo que el llamador necesita saber.
func (p *QueueJobPublisher) failBatch(ctx context.Context, batchID int32, cause error) error {
	if _, err := p.store.TransitionBatch(context.WithoutCancel(ctx), batchID, BatchFailed); err != nil {
		log.Printf("recommendation batch %d could not be marked as failed: %v", batchID, err)
	}

	return cause
}
