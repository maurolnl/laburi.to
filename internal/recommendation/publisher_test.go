package recommendation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue/queuetest"
)

// fakeStore reproduce de RecommendationStore solo lo que el productor usa, y con la misma
// garantía que da el índice único parcial de la base: un sujeto con un batch pending o
// processing no admite otro. Las operaciones que el productor no debe invocar fallan la
// prueba, que es como se comprueba que emitir no calcula ni persiste recomendaciones.
type fakeStore struct {
	t *testing.T

	nextBatchID int32
	batches     map[int32]Batch

	createdSubjects []Subject
	transitions     []batchTransition

	createErr     error
	transitionErr error
}

type batchTransition struct {
	batchID int32
	status  BatchStatus
}

func newFakeStore(t *testing.T) *fakeStore {
	t.Helper()

	return &fakeStore{t: t, batches: map[int32]Batch{}}
}

func (s *fakeStore) CreateBatch(_ context.Context, subject Subject) (Batch, error) {
	if s.createErr != nil {
		return Batch{}, s.createErr
	}
	if !subject.Valid() {
		return Batch{}, ErrInvalidSubject
	}

	for _, batch := range s.batches {
		if batch.SubjectType != subject.Type || batch.subjectID() != subject.id() {
			continue
		}
		if batch.Status == BatchPending || batch.Status == BatchProcessing {
			return Batch{}, ErrBatchAlreadyInFlight
		}
	}

	s.nextBatchID++
	batch := Batch{
		ID:            s.nextBatchID,
		SubjectType:   subject.Type,
		EmployeeID:    subject.EmployeeID,
		JobPositionID: subject.JobPositionID,
		Status:        BatchPending,
	}
	s.batches[batch.ID] = batch
	s.createdSubjects = append(s.createdSubjects, subject)

	return batch, nil
}

func (s *fakeStore) TransitionBatch(_ context.Context, batchID int32, status BatchStatus) (Batch, error) {
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

func (s *fakeStore) GetBatch(context.Context, int32) (Batch, error) {
	s.t.Error("el productor no debe leer batches: abre el suyo y no consulta ninguno más")
	return Batch{}, nil
}

func (s *fakeStore) ClaimBatch(context.Context, int32) (Batch, error) {
	s.t.Error("el productor no debe reclamar batches: reclamar trabajo es del consumidor")
	return Batch{}, nil
}

func (s *fakeStore) CompleteBatch(context.Context, int32, []Candidate) (Batch, error) {
	s.t.Error("el productor no debe completar batches: emitir no calcula recomendaciones")
	return Batch{}, nil
}

func (s *fakeStore) JobRecommendationsForEmployee(context.Context, int32, Page) (JobRecommendations, error) {
	s.t.Error("el productor no debe leer el conjunto vigente")
	return JobRecommendations{}, nil
}

func (s *fakeStore) EmployeeRecommendationsForJobPosition(context.Context, int32, Page) (EmployeeRecommendations, error) {
	s.t.Error("el productor no debe leer el conjunto vigente")
	return EmployeeRecommendations{}, nil
}

// subjectID es el equivalente de Subject.id para un batch ya persistido.
func (b Batch) subjectID() int32 {
	switch {
	case b.EmployeeID != nil:
		return *b.EmployeeID
	case b.JobPositionID != nil:
		return *b.JobPositionID
	default:
		return 0
	}
}

// unusableStore falla la prueba ante cualquier llamada. Sirve para comprobar que una
// emisión rechazada no toca la persistencia en absoluto.
type unusableStore struct {
	t *testing.T
}

var _ RecommendationStore = (*unusableStore)(nil)

func newUnusableStore(t *testing.T) *unusableStore {
	t.Helper()

	return &unusableStore{t: t}
}

func (s *unusableStore) CreateBatch(context.Context, Subject) (Batch, error) {
	s.t.Error("no se esperaba abrir ningún batch")
	return Batch{}, nil
}

func (s *unusableStore) TransitionBatch(context.Context, int32, BatchStatus) (Batch, error) {
	s.t.Error("no se esperaba transicionar ningún batch")
	return Batch{}, nil
}

func (s *unusableStore) GetBatch(context.Context, int32) (Batch, error) {
	s.t.Error("no se esperaba leer ningún batch")
	return Batch{}, nil
}

func (s *unusableStore) ClaimBatch(context.Context, int32) (Batch, error) {
	s.t.Error("no se esperaba reclamar ningún batch")
	return Batch{}, nil
}

func (s *unusableStore) CompleteBatch(context.Context, int32, []Candidate) (Batch, error) {
	s.t.Error("no se esperaba completar ningún batch")
	return Batch{}, nil
}

func (s *unusableStore) JobRecommendationsForEmployee(context.Context, int32, Page) (JobRecommendations, error) {
	s.t.Error("no se esperaba leer el conjunto vigente")
	return JobRecommendations{}, nil
}

func (s *unusableStore) EmployeeRecommendationsForJobPosition(context.Context, int32, Page) (EmployeeRecommendations, error) {
	s.t.Error("no se esperaba leer el conjunto vigente")
	return EmployeeRecommendations{}, nil
}

// newTestPublisher fija el reloj y los identificadores de evento para que el cuerpo del
// mensaje sea comparable contra un literal.
func newTestPublisher(store RecommendationStore, client queue.Client, eventIDs ...string) *QueueJobPublisher {
	publisher := NewQueueJobPublisher(store, client)
	publisher.now = func() time.Time { return fixedEmittedAt }

	emitted := 0
	publisher.newEventID = func() string {
		if emitted >= len(eventIDs) {
			return fmt.Sprintf("evento-no-previsto-%d", emitted)
		}

		id := eventIDs[emitted]
		emitted++

		return id
	}

	return publisher
}

func TestPublicaUnMensajePorSujeto(t *testing.T) {
	tests := []struct {
		name    string
		subject Subject
		want    string
	}{
		{
			name:    "empleado",
			subject: NewEmployeeSubject(12),
			want:    `{"event_id":"evento-1","version":1,"subject_type":"employee","subject_id":12,"batch_id":1,"emitted_at":"2026-09-22T15:34:56Z"}`,
		},
		{
			name:    "puesto de trabajo",
			subject: NewJobPositionSubject(34),
			want:    `{"event_id":"evento-1","version":1,"subject_type":"job_position","subject_id":34,"batch_id":1,"emitted_at":"2026-09-22T15:34:56Z"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeStore(t)
			client := queuetest.New()
			publisher := newTestPublisher(store, client, "evento-1")

			if err := publisher.PublishJob(context.Background(), test.subject); err != nil {
				t.Fatalf("PublishJob devolvió error: %v", err)
			}

			if len(store.createdSubjects) != 1 {
				t.Fatalf("batches abiertos = %d, want 1", len(store.createdSubjects))
			}
			if created := store.createdSubjects[0]; created != test.subject {
				t.Errorf("sujeto del batch = %+v, want %+v", created, test.subject)
			}

			sent := client.Sent()
			if len(sent) != 1 {
				t.Fatalf("mensajes publicados = %d, want 1", len(sent))
			}
			if sent[0] != test.want {
				t.Errorf("cuerpo publicado\n got: %s\nwant: %s", sent[0], test.want)
			}
		})
	}
}

func TestNoPublicaSiFallaLaAperturaDelBatch(t *testing.T) {
	store := newFakeStore(t)
	store.createErr = errors.New("la base no responde")
	client := queuetest.New()
	publisher := newTestPublisher(store, client, "evento-1")

	err := publisher.PublishJob(context.Background(), NewEmployeeSubject(12))
	if !errors.Is(err, store.createErr) {
		t.Fatalf("error = %v, want el de la persistencia", err)
	}

	if sent := client.Sent(); len(sent) != 0 {
		t.Errorf("mensajes publicados = %d, want 0: el batch es lo primero", len(sent))
	}
}

func TestRechazaSujetoInvalidoSinTocarNadaMas(t *testing.T) {
	store := newUnusableStore(t)
	client := queuetest.New()
	publisher := newTestPublisher(store, client, "evento-1")

	if err := publisher.PublishJob(context.Background(), Subject{}); !errors.Is(err, ErrInvalidSubject) {
		t.Fatalf("error = %v, want ErrInvalidSubject", err)
	}

	if sent := client.Sent(); len(sent) != 0 {
		t.Errorf("mensajes publicados = %d, want 0", len(sent))
	}
}

func TestNoPublicaSiElSujetoYaTieneTrabajoEnCurso(t *testing.T) {
	store := newFakeStore(t)
	client := queuetest.New()
	publisher := newTestPublisher(store, client, "evento-1", "evento-2")
	subject := NewEmployeeSubject(12)

	if err := publisher.PublishJob(context.Background(), subject); err != nil {
		t.Fatalf("primera emisión devolvió error: %v", err)
	}

	if err := publisher.PublishJob(context.Background(), subject); err != nil {
		t.Fatalf("segunda emisión devolvió error: %v, want nil: el trabajo en curso ya genera el conjunto", err)
	}

	if len(store.createdSubjects) != 1 {
		t.Errorf("batches abiertos = %d, want 1", len(store.createdSubjects))
	}
	if sent := client.Sent(); len(sent) != 1 {
		t.Errorf("mensajes publicados = %d, want 1", len(sent))
	}
}

func TestElTrabajoEnCursoDeUnSujetoNoFrenaAOtro(t *testing.T) {
	store := newFakeStore(t)
	client := queuetest.New()
	publisher := newTestPublisher(store, client, "evento-1", "evento-2")

	if err := publisher.PublishJob(context.Background(), NewEmployeeSubject(12)); err != nil {
		t.Fatalf("emisión del empleado devolvió error: %v", err)
	}
	if err := publisher.PublishJob(context.Background(), NewJobPositionSubject(12)); err != nil {
		t.Fatalf("emisión del puesto devolvió error: %v", err)
	}

	if sent := client.Sent(); len(sent) != 2 {
		t.Errorf("mensajes publicados = %d, want 2: son sujetos distintos", len(sent))
	}
}

func TestPublicaTrasElCierreDelTrabajoAnterior(t *testing.T) {
	store := newFakeStore(t)
	client := queuetest.New()
	publisher := newTestPublisher(store, client, "evento-1", "evento-2")
	subject := NewEmployeeSubject(12)

	if err := publisher.PublishJob(context.Background(), subject); err != nil {
		t.Fatalf("primera emisión devolvió error: %v", err)
	}
	if _, err := store.TransitionBatch(context.Background(), 1, BatchCompleted); err != nil {
		t.Fatalf("no se pudo completar el batch: %v", err)
	}

	if err := publisher.PublishJob(context.Background(), subject); err != nil {
		t.Fatalf("segunda emisión devolvió error: %v", err)
	}

	if len(store.createdSubjects) != 2 {
		t.Errorf("batches abiertos = %d, want 2", len(store.createdSubjects))
	}
	if sent := client.Sent(); len(sent) != 2 {
		t.Errorf("mensajes publicados = %d, want 2", len(sent))
	}
}

func TestFalloDePublicacionDejaElBatchFailed(t *testing.T) {
	store := newFakeStore(t)
	client := queuetest.New()
	sendErr := errors.New("SQS no responde")
	client.FailSend(sendErr)
	publisher := newTestPublisher(store, client, "evento-1")

	err := publisher.PublishJob(context.Background(), NewEmployeeSubject(12))
	if err == nil {
		t.Fatal("PublishJob devolvió nil: publicar falló y no puede reportarse como éxito")
	}
	if !errors.Is(err, sendErr) {
		t.Errorf("error = %v, want el del transporte", err)
	}

	if batch := store.batches[1]; batch.Status != BatchFailed {
		t.Errorf("estado del batch = %q, want %q", batch.Status, BatchFailed)
	}
}

func TestTransporteDeshabilitadoEsUnFalloDePublicacion(t *testing.T) {
	store := newFakeStore(t)
	publisher := newTestPublisher(store, queue.Disabled{}, "evento-1")

	err := publisher.PublishJob(context.Background(), NewEmployeeSubject(12))
	if !errors.Is(err, queue.ErrQueueDisabled) {
		t.Fatalf("error = %v, want ErrQueueDisabled", err)
	}

	if batch := store.batches[1]; batch.Status != BatchFailed {
		t.Errorf("estado del batch = %q, want %q", batch.Status, BatchFailed)
	}
}

func TestFalloAlMarcarElBatchConservaLaCausaOriginal(t *testing.T) {
	store := newFakeStore(t)
	store.transitionErr = errors.New("la base no responde")
	client := queuetest.New()
	sendErr := errors.New("SQS no responde")
	client.FailSend(sendErr)
	publisher := newTestPublisher(store, client, "evento-1")

	err := publisher.PublishJob(context.Background(), NewEmployeeSubject(12))
	if !errors.Is(err, sendErr) {
		t.Fatalf("error = %v, want la causa original de la publicación", err)
	}
	if errors.Is(err, store.transitionErr) {
		t.Error("el error de la transición tapó al de la publicación")
	}

	if len(store.transitions) != 1 || store.transitions[0].status != BatchFailed {
		t.Errorf("transiciones = %+v, want un intento a failed", store.transitions)
	}
}

// El fallo de publicación no puede dejar al sujeto bloqueado: es lo que hace seguro el
// reintento posterior.
func TestReintentoSeguroTrasUnFalloDePublicacion(t *testing.T) {
	store := newFakeStore(t)
	client := queuetest.New()
	client.FailSend(errors.New("SQS no responde"))
	publisher := newTestPublisher(store, client, "evento-1", "evento-2")
	subject := NewEmployeeSubject(12)

	if err := publisher.PublishJob(context.Background(), subject); err == nil {
		t.Fatal("la primera emisión debía fallar")
	}

	client.FailSend(nil)

	if err := publisher.PublishJob(context.Background(), subject); err != nil {
		t.Fatalf("el reintento devolvió error: %v", err)
	}

	if len(store.createdSubjects) != 2 {
		t.Errorf("batches abiertos = %d, want 2: el sujeto no quedó bloqueado", len(store.createdSubjects))
	}
	sent := client.Sent()
	if len(sent) != 1 {
		t.Fatalf("mensajes publicados = %d, want 1", len(sent))
	}
	want := `{"event_id":"evento-2","version":1,"subject_type":"employee","subject_id":12,"batch_id":2,"emitted_at":"2026-09-22T15:34:56Z"}`
	if sent[0] != want {
		t.Errorf("cuerpo del reintento\n got: %s\nwant: %s", sent[0], want)
	}
}

// Emitir no espera a que el trabajo se procese: al volver, el batch sigue donde lo dejó la
// apertura y nadie transicionó nada.
func TestLaEmisionNoEsperaElProcesamiento(t *testing.T) {
	store := newFakeStore(t)
	client := queuetest.New()
	publisher := newTestPublisher(store, client, "evento-1")

	if err := publisher.PublishJob(context.Background(), NewEmployeeSubject(12)); err != nil {
		t.Fatalf("PublishJob devolvió error: %v", err)
	}

	if batch := store.batches[1]; batch.Status != BatchPending {
		t.Errorf("estado del batch = %q, want %q", batch.Status, BatchPending)
	}
	if len(store.transitions) != 0 {
		t.Errorf("transiciones = %+v, want ninguna", store.transitions)
	}
}

// El noop no puede abrir un batch ni publicar porque no recibe ninguna dependencia: el
// compilador lo garantiza mejor que una aserción. Lo que queda por comprobar es que cumple
// el puerto y se resuelve sin error para cualquier sujeto, que es lo que permite inyectarlo
// donde no se quiere emitir.
func TestNoopJobPublisherSeResuelveSinEmitir(t *testing.T) {
	var publisher JobPublisher = NoopJobPublisher{}

	subjects := []Subject{NewEmployeeSubject(12), NewJobPositionSubject(34), {}}
	for _, subject := range subjects {
		if err := publisher.PublishJob(context.Background(), subject); err != nil {
			t.Errorf("PublishJob(%+v) devolvió error: %v", subject, err)
		}
	}
}

// El identificador de evento por defecto lo genera el constructor de producción; los demás
// tests lo fijan para poder comparar cuerpos, así que la unicidad hay que comprobarla acá.
func TestCadaEmisionLlevaUnIdentificadorDeEventoDistinto(t *testing.T) {
	store := newFakeStore(t)
	client := queuetest.New()
	publisher := NewQueueJobPublisher(store, client)
	subject := NewEmployeeSubject(12)

	for i := range 2 {
		if err := publisher.PublishJob(context.Background(), subject); err != nil {
			t.Fatalf("emisión %d devolvió error: %v", i, err)
		}
		if _, err := store.TransitionBatch(context.Background(), int32(i+1), BatchCompleted); err != nil {
			t.Fatalf("no se pudo completar el batch %d: %v", i+1, err)
		}
	}

	sent := client.Sent()
	if len(sent) != 2 {
		t.Fatalf("mensajes publicados = %d, want 2", len(sent))
	}

	first, second := decodeEvent(t, sent[0]), decodeEvent(t, sent[1])
	if first.EventID == second.EventID {
		t.Errorf("dos emisiones comparten el identificador de evento %q", first.EventID)
	}
	if first.EventID == "" {
		t.Error("el identificador de evento está vacío")
	}
	if first.BatchID == second.BatchID {
		t.Errorf("dos emisiones comparten el identificador de batch %d", first.BatchID)
	}
}

// Una reentrega es el mismo cuerpo: los dos identificadores que el consumidor usa para
// deduplicar se conservan, que es lo que hace posible un worker idempotente.
func TestLaReentregaConservaLosIdentificadores(t *testing.T) {
	store := newFakeStore(t)
	client := queuetest.New()
	publisher := newTestPublisher(store, client, "evento-1")

	if err := publisher.PublishJob(context.Background(), NewEmployeeSubject(12)); err != nil {
		t.Fatalf("PublishJob devolvió error: %v", err)
	}

	first, err := client.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive devolvió error: %v", err)
	}
	client.Advance(queuetest.DefaultVisibilityTimeout + time.Second)
	second, err := client.Receive(context.Background())
	if err != nil {
		t.Fatalf("Receive devolvió error: %v", err)
	}

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("entregas = %d y %d, want 1 y 1", len(first), len(second))
	}
	if second[0].ReceiveCount <= first[0].ReceiveCount {
		t.Fatalf("la segunda entrega no es una reentrega: ReceiveCount = %d", second[0].ReceiveCount)
	}

	original, redelivered := decodeEvent(t, first[0].Body), decodeEvent(t, second[0].Body)
	if original.EventID != redelivered.EventID || original.BatchID != redelivered.BatchID {
		t.Errorf("la reentrega cambió los identificadores: %+v vs %+v", original, redelivered)
	}
}

func decodeEvent(t *testing.T, body string) JobRequestedEvent {
	t.Helper()

	var event JobRequestedEvent
	if err := json.Unmarshal([]byte(body), &event); err != nil {
		t.Fatalf("el cuerpo no se deserializa: %v", err)
	}

	return event
}
