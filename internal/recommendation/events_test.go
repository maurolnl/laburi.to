package recommendation

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"
)

func int32Ptr(value int32) *int32 { return &value }

// fixedEmittedAt es deliberadamente una hora con offset no nulo: normalizar a UTC es parte
// del contrato, y con un instante ya en UTC el test pasaría sin ejercitarlo.
var fixedEmittedAt = time.Date(2026, 9, 22, 12, 34, 56, 0, time.FixedZone("ART", -3*60*60))

func TestCuerpoDelMensajePorTipoDeSujeto(t *testing.T) {
	tests := []struct {
		name  string
		batch Batch
		want  string
	}{
		{
			name: "empleado",
			batch: Batch{
				ID:          77,
				SubjectType: SubjectEmployee,
				EmployeeID:  int32Ptr(12),
			},
			want: `{"event_id":"11111111-1111-1111-1111-111111111111","version":1,"subject_type":"employee","subject_id":12,"batch_id":77,"emitted_at":"2026-09-22T15:34:56Z"}`,
		},
		{
			name: "puesto de trabajo",
			batch: Batch{
				ID:            78,
				SubjectType:   SubjectJobPosition,
				JobPositionID: int32Ptr(34),
			},
			want: `{"event_id":"11111111-1111-1111-1111-111111111111","version":1,"subject_type":"job_position","subject_id":34,"batch_id":78,"emitted_at":"2026-09-22T15:34:56Z"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, err := NewJobRequestedEvent("11111111-1111-1111-1111-111111111111", test.batch, fixedEmittedAt)
			if err != nil {
				t.Fatalf("NewJobRequestedEvent devolvió error: %v", err)
			}

			body, err := event.Body()
			if err != nil {
				t.Fatalf("Body devolvió error: %v", err)
			}

			if body != test.want {
				t.Errorf("cuerpo del mensaje inesperado\n got: %s\nwant: %s", body, test.want)
			}
		})
	}
}

// El conjunto de claves se afirma entero: agregar un campo al evento rompe este test en vez
// de filtrar un dato nuevo hacia la cola en silencio.
func TestCuerpoDelMensajeSinDatosSensibles(t *testing.T) {
	batch := Batch{ID: 77, SubjectType: SubjectEmployee, EmployeeID: int32Ptr(12)}

	event, err := NewJobRequestedEvent("evento", batch, fixedEmittedAt)
	if err != nil {
		t.Fatalf("NewJobRequestedEvent devolvió error: %v", err)
	}

	body, err := event.Body()
	if err != nil {
		t.Fatalf("Body devolvió error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("el cuerpo no es JSON válido: %v", err)
	}

	keys := make([]string, 0, len(decoded))
	for key := range decoded {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	want := []string{"batch_id", "emitted_at", "event_id", "subject_id", "subject_type", "version"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("el mensaje no lleva exactamente los campos del contrato\n got: %v\nwant: %v", keys, want)
	}
}

func TestCuerpoDelMensajeSeDeserializaAlMismoEvento(t *testing.T) {
	batch := Batch{ID: 78, SubjectType: SubjectJobPosition, JobPositionID: int32Ptr(34)}

	event, err := NewJobRequestedEvent("evento", batch, fixedEmittedAt)
	if err != nil {
		t.Fatalf("NewJobRequestedEvent devolvió error: %v", err)
	}

	body, err := event.Body()
	if err != nil {
		t.Fatalf("Body devolvió error: %v", err)
	}

	var decoded JobRequestedEvent
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("el cuerpo no se deserializa: %v", err)
	}

	if !decoded.EmittedAt.Equal(event.EmittedAt) {
		t.Errorf("instante de emisión = %v, want %v", decoded.EmittedAt, event.EmittedAt)
	}
	decoded.EmittedAt = event.EmittedAt

	if decoded != event {
		t.Errorf("evento deserializado = %+v, want %+v", decoded, event)
	}
}

// Un consumidor debe poder decidir si entiende el contrato leyendo solo la versión, sin
// inferirla de la presencia o ausencia de otros campos.
func TestVersionLegibleSinElRestoDelCuerpo(t *testing.T) {
	batch := Batch{ID: 77, SubjectType: SubjectEmployee, EmployeeID: int32Ptr(12)}

	event, err := NewJobRequestedEvent("evento", batch, fixedEmittedAt)
	if err != nil {
		t.Fatalf("NewJobRequestedEvent devolvió error: %v", err)
	}

	body, err := event.Body()
	if err != nil {
		t.Fatalf("Body devolvió error: %v", err)
	}

	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("el cuerpo no se deserializa: %v", err)
	}

	if envelope.Version != JobRequestedVersion {
		t.Errorf("versión = %d, want %d", envelope.Version, JobRequestedVersion)
	}
}

func TestEventoRechazaBatchConSujetoInvalido(t *testing.T) {
	tests := []struct {
		name  string
		batch Batch
	}{
		{name: "sin sujeto", batch: Batch{ID: 1, SubjectType: SubjectEmployee}},
		{name: "dos sujetos", batch: Batch{ID: 1, SubjectType: SubjectEmployee, EmployeeID: int32Ptr(1), JobPositionID: int32Ptr(2)}},
		{name: "sujeto cruzado", batch: Batch{ID: 1, SubjectType: SubjectJobPosition, EmployeeID: int32Ptr(1)}},
		{name: "tipo desconocido", batch: Batch{ID: 1, SubjectType: SubjectType("employeer"), EmployeeID: int32Ptr(1)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewJobRequestedEvent("evento", test.batch, fixedEmittedAt); !errors.Is(err, ErrInvalidSubject) {
				t.Errorf("error = %v, want ErrInvalidSubject", err)
			}
		})
	}
}
