package recommendation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/scoring"
)

// Fallos propios del consumo. Existen como sentinelas y no como errores anónimos porque la
// decisión de reconocer o devolver el mensaje se toma clasificándolos: un error que no se
// puede clasificar termina devuelto a la cola, que es el lado seguro.
var (
	// errUnreadableMessage es un cuerpo que no corresponde al contrato. Se lee igual de mal
	// la décima vez, así que reintentarlo solo consume entregas.
	errUnreadableMessage = errors.New("the recommendation job message could not be read")

	// errUnknownVersion es un contrato que este consumidor no implementa. Interpretarlo sería
	// adivinar.
	errUnknownVersion = errors.New("unsupported recommendation job message version")

	// errScoringFailed envuelve todo fallo del contrato de scoring, para distinguirlo de un
	// fallo de infraestructura ocurrido en el mismo tramo.
	errScoringFailed = errors.New("scoring the candidate universe failed")
)

// defaultReceiveBackoff es la espera tras un fallo de recepción. Corta a propósito: el fallo
// típico es de red y se resuelve solo, pero reintentar sin pausa convertiría una cola caída
// en un bucle de errores a máxima velocidad.
const defaultReceiveBackoff = time.Second

// Worker consume las solicitudes de regeneración de recomendaciones.
//
// Depende de cuatro puertos y de ninguna implementación concreta: sustituir el transporte o
// la implementación de scoring no lo toca. No conoce los indicadores, sus pesos ni cómo se
// combinan, y no menciona ningún tipo del SDK del proveedor de colas.
//
// Su cero no es utilizable: construirlo con NewWorker.
type Worker struct {
	store      RecommendationStore
	candidates CandidateSource
	client     queue.Client
	scorer     scoring.Scorer

	// visibilityTimeoutSeconds es el plazo que se renueva mientras un mensaje se procesa, y
	// heartbeatEvery cada cuánto se renueva. Renovar a la mitad del plazo deja margen para que
	// una renovación perdida no venza el mensaje.
	visibilityTimeoutSeconds int32
	heartbeatEvery           time.Duration

	// receiveBackoff es inyectable para que los tests no esperen un segundo real por cada
	// fallo de recepción que ejercitan. No se expone: nadie fuera del paquete lo necesita.
	receiveBackoff time.Duration
}

func NewWorker(
	store RecommendationStore,
	candidates CandidateSource,
	client queue.Client,
	scorer scoring.Scorer,
	visibilityTimeoutSeconds int32,
) *Worker {
	heartbeat := time.Duration(visibilityTimeoutSeconds) * time.Second / 2
	if heartbeat <= 0 {
		heartbeat = time.Second
	}

	return &Worker{
		store:                    store,
		candidates:               candidates,
		client:                   client,
		scorer:                   scorer,
		visibilityTimeoutSeconds: visibilityTimeoutSeconds,
		heartbeatEvery:           heartbeat,
		receiveBackoff:           defaultReceiveBackoff,
	}
}

// Run consume la cola hasta que el contexto se cancela.
//
// No devuelve error porque ninguno lo termina: un fallo de recepción se registra y se
// reintenta, y un fallo de procesamiento se resuelve mensaje a mensaje. Lo único que lo
// detiene es la cancelación, que es una decisión del llamador y no una anomalía.
//
// Cancelar corta la recepción en curso —Receive bloquea con long polling— y no inicia ningún
// procesamiento nuevo. El mensaje que ya estaba en curso sí se termina: abandonarlo a mitad
// de camino es justo lo que deja un batch en processing bloqueando al sujeto.
func (w *Worker) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		messages, err := w.client.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			log.Printf("receiving recommendation jobs failed: %v", err)
			if !sleepOrDone(ctx, w.receiveBackoff) {
				return
			}
			continue
		}

		for _, message := range messages {
			if ctx.Err() != nil {
				return
			}
			w.processMessage(ctx, message)
		}
	}
}

// processMessage lleva un mensaje a un desenlace persistido y decide si reconocerlo.
//
// El trabajo corre con un contexto desprendido de la cancelación del ciclo, por la misma
// razón que QueueJobPublisher.failBatch: la causa más común de que el ciclo termine es un
// apagado, y usar un contexto ya cancelado para cerrar el batch dejaría el estado zombi que
// esa decisión existe para evitar.
func (w *Worker) processMessage(ctx context.Context, message queue.Message) {
	workCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	defer cancel()

	stopHeartbeat := w.keepVisible(workCtx, message.ReceiptHandle)
	defer stopHeartbeat()

	err := w.handle(workCtx, message)
	if err != nil {
		log.Printf("recommendation job message %s failed after %d deliveries: %v",
			message.ID, message.ReceiveCount, err)
	}

	if !acknowledges(err) {
		// Sin reconocimiento el mensaje vuelve a entregarse al vencer su plazo, y al agotar
		// el maxReceiveCount que la cola tiene configurado en AWS termina en la DLQ.
		return
	}

	if deleteErr := w.client.Delete(workCtx, message.ReceiptHandle); deleteErr != nil {
		// El desenlace ya está persistido. El redelivery vuelve a encontrar el batch terminal
		// y lo reconoce sin rehacer trabajo, así que no hay nada que revertir.
		log.Printf("acknowledging recommendation job message %s failed: %v", message.ID, deleteErr)
	}
}

// handle es el procesamiento de un mensaje. Devuelve nil cuando el batch quedó resuelto, y el
// error que corresponda en cualquier otro caso; quién decide el reconocimiento es
// acknowledges, no esta función.
//
// El orden no es arbitrario. Los candidatos se resuelven con el batch todavía en pending,
// porque es una lectura y porque las dos decisiones que siguen —universo vacío y scoring no
// disponible— no deben abrir processing para un trabajo que no va a prosperar. Abrirlo
// dejaría al sujeto bloqueado por el índice único parcial ante cualquier fallo posterior.
func (w *Worker) handle(ctx context.Context, message queue.Message) error {
	event, err := decodeJobRequestedEvent(message.Body)
	if err != nil {
		return err
	}

	batch, err := w.store.GetBatch(ctx, event.BatchID)
	if err != nil {
		// Un batch inexistente es un mensaje que sobrevivió a su batch: no hay trabajo y no
		// hay nada que crear.
		return err
	}

	if batch.Status == BatchCompleted || batch.Status == BatchFailed {
		// Redelivery o mensaje duplicado de un trabajo ya cerrado. El conjunto vigente que
		// ese batch dejó queda intacto justamente porque no se lo vuelve a tocar.
		log.Printf("recommendation batch %d already %s: message %s acknowledged without work",
			batch.ID, batch.Status, message.ID)
		return nil
	}

	pairs, err := w.resolveCandidates(ctx, batch)
	if errors.Is(err, ErrSubjectNotFound) {
		// El sujeto ya no existe, o el puesto fue eliminado entre la emisión y el consumo. No
		// hay para quién recomendar, pero el batch tiene que cerrarse igual: dejarlo abierto
		// bloquearía al sujeto para siempre.
		log.Printf("recommendation batch %d has no subject anymore: completing it empty", batch.ID)
		return w.completeEmpty(ctx, batch)
	}
	if err != nil {
		return err
	}

	if len(pairs) == 0 {
		// Estado vacío, no fallo. No se consulta el scoring: no hay nada que puntuar, igual
		// que ScoreAll con un lote vacío no necesita ninguna dependencia.
		return w.completeEmpty(ctx, batch)
	}

	if err := w.scoringAvailable(ctx); err != nil {
		// pending → failed sin pasar por processing. Es el desenlace que la épica reserva para
		// la ausencia de algoritmo, y deja al sujeto libre para una solicitud nueva.
		if _, transitionErr := w.store.TransitionBatch(ctx, batch.ID, BatchFailed); transitionErr != nil {
			return fmt.Errorf("fail recommendation batch %d without scoring: %w", batch.ID, transitionErr)
		}
		return err
	}

	if _, err := w.store.ClaimBatch(ctx, batch.ID); err != nil {
		return err
	}

	results, err := w.scorer.ScoreAll(ctx, pairs)
	if err != nil {
		return w.failBatch(ctx, batch.ID, fmt.Errorf("%w: %w", errScoringFailed, err))
	}

	if _, err := w.store.CompleteBatch(ctx, batch.ID, candidatesFrom(results)); err != nil {
		// El reemplazo es atómico: el conjunto vigente anterior sigue intacto y el batch no
		// quedó completed, así que el redelivery puede reintentarlo.
		return fmt.Errorf("replace recommendation set of batch %d: %w", batch.ID, err)
	}

	return nil
}

// completeEmpty cierra el batch con un conjunto vacío. Reclama primero para no completar un
// batch que otra entrega ya cerró mientras tanto.
func (w *Worker) completeEmpty(ctx context.Context, batch Batch) error {
	if _, err := w.store.ClaimBatch(ctx, batch.ID); err != nil {
		return err
	}

	if _, err := w.store.CompleteBatch(ctx, batch.ID, nil); err != nil {
		return fmt.Errorf("complete recommendation batch %d empty: %w", batch.ID, err)
	}

	return nil
}

// failBatch cierra el batch y devuelve la causa original. Si la transición también falla, se
// devuelve igual la causa: cambiar el error ocultaría por qué no hay recomendaciones, que es
// lo que hay que diagnosticar.
func (w *Worker) failBatch(ctx context.Context, batchID int32, cause error) error {
	if _, err := w.store.TransitionBatch(ctx, batchID, BatchFailed); err != nil {
		log.Printf("recommendation batch %d could not be marked as failed: %v", batchID, err)
	}

	return cause
}

func (w *Worker) resolveCandidates(ctx context.Context, batch Batch) ([]scoring.Pair, error) {
	// El sujeto sale del batch persistido y no del mensaje: es la misma razón por la que
	// CompleteBatch lo toma de la fila que está reemplazando, y así no puede desalinearse.
	subjectID, err := batchSubjectID(batch)
	if err != nil {
		return nil, err
	}

	switch batch.SubjectType {
	case SubjectEmployee:
		return w.candidates.PairsForEmployee(ctx, subjectID)
	case SubjectJobPosition:
		return w.candidates.PairsForJobPosition(ctx, subjectID)
	default:
		return nil, ErrInvalidSubject
	}
}

// scoringAvailable consulta la comprobación previa del contrato si la implementación la
// ofrece. Una que no la ofrece se considera disponible: implementarla es la excepción.
func (w *Worker) scoringAvailable(ctx context.Context) error {
	probe, ok := w.scorer.(scoring.Availability)
	if !ok {
		return nil
	}

	return probe.Available(ctx)
}

// keepVisible renueva el plazo del mensaje mientras se lo procesa, y devuelve la función que
// detiene la renovación.
//
// Reduce la ventana en la que una entrega lenta se solapa con un redelivery del mismo
// mensaje. No la elimina, y no hace falta que lo haga: el reemplazo del conjunto es atómico y
// completo, así que dos procesamientos solapados dejan un conjunto coherente igual.
func (w *Worker) keepVisible(ctx context.Context, receiptHandle string) func() {
	// stop es propio de la renovación y no se apoya en la cancelación del contexto de
	// trabajo: quien la detiene es el fin del procesamiento, que ocurre antes de que ese
	// contexto se cancele. Esperar a la cancelación acá trabaría el retorno.
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(w.heartbeatEvery)
		defer ticker.Stop()
		defer close(done)

		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := w.client.ExtendVisibility(ctx, receiptHandle, w.visibilityTimeoutSeconds); err != nil {
					// Perder una renovación no invalida el trabajo en curso: en el peor caso el
					// mensaje vuelve a entregarse y el reclamo lo resuelve.
					log.Printf("extending recommendation job message visibility failed: %v", err)
				}
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}

// decodeJobRequestedEvent interpreta el cuerpo y rechaza lo que no puede interpretar. El
// cuerpo no se registra: viene de fuera del proceso y no hay garantía de qué trae.
func decodeJobRequestedEvent(body string) (JobRequestedEvent, error) {
	var event JobRequestedEvent
	if err := json.Unmarshal([]byte(body), &event); err != nil {
		return JobRequestedEvent{}, fmt.Errorf("%w: %v", errUnreadableMessage, err)
	}

	if event.Version != JobRequestedVersion {
		return JobRequestedEvent{}, fmt.Errorf("%w: %d", errUnknownVersion, event.Version)
	}

	if event.BatchID <= 0 || !event.SubjectType.Valid() {
		return JobRequestedEvent{}, fmt.Errorf("%w: incomplete routing envelope", errUnreadableMessage)
	}

	return event, nil
}

// candidatesFrom traduce los resultados al conjunto que se persiste.
//
// Los inelegibles no viajan: un filtro duro los descartó y no son recomendaciones. Los
// elegibles sí, con su total tal cual, de modo que la ausencia de puntaje llegue a la base
// como NULL y siga siendo distinguible de un puntaje cero.
func candidatesFrom(results []scoring.Result) []Candidate {
	candidates := make([]Candidate, 0, len(results))
	for _, result := range results {
		if !result.Eligible {
			continue
		}

		candidates = append(candidates, Candidate{
			EmployeeID:    result.EmployeeID,
			JobPositionID: result.JobPositionID,
			Score:         result.Total,
		})
	}

	return candidates
}

// terminalFailures son los fallos que reintentar no puede cambiar. Todo lo demás se considera
// recuperable, que es el lado seguro: un error desconocido vuelve a la cola en vez de darse
// por resuelto.
var terminalFailures = []error{
	errUnreadableMessage,
	errUnknownVersion,
	errScoringFailed,
	scoring.ErrScoringUnavailable,
	scoring.ErrInvalidPair,
	ErrBatchNotFound,
	ErrBatchNotClaimable,
	ErrInvalidSubject,
	ErrSubjectNotFound,
}

// acknowledges decide si el mensaje se reconoce. El desenlace del batch ya está persistido
// cuando se la consulta: reconocer es solo declarar que el mensaje no tiene que volver.
func acknowledges(err error) bool {
	if err == nil {
		return true
	}

	for _, terminal := range terminalFailures {
		if errors.Is(err, terminal) {
			return true
		}
	}

	return false
}

// sleepOrDone espera el plazo indicado y devuelve false si el contexto se canceló antes.
func sleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
