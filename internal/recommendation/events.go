package recommendation

import (
	"encoding/json"
	"fmt"
	"time"
)

// JobRequestedVersion es la versión del contrato del mensaje que este paquete emite. Viaja
// en todo mensaje para que un consumidor pueda rechazar un contrato que no entiende en vez
// de interpretarlo mal.
//
// Es un entero monótono y no una cadena tipo semver: el consumidor compara contra el único
// valor que implementa, y comparar cadenas invita a errores de orden. Un cambio
// incompatible del cuerpo sube este número.
const JobRequestedVersion = 1

// JobRequestedEvent es el cuerpo del mensaje que solicita regenerar las recomendaciones de
// un sujeto. Es un sobre de ruteo, no una copia del dominio: lleva identificadores y nada
// más, así que el consumidor resuelve desde la base todo dato que necesite.
//
// No contiene datos personales, credenciales ni el perfil del empleado o la descripción del
// puesto. Agregar un campo acá es agregarlo a un mensaje que viaja fuera del proceso y queda
// en la cola: pensarlo dos veces.
//
// A diferencia de Batch, el sujeto se identifica con un único SubjectID y no con un par
// excluyente EmployeeID/JobPositionID. Batch necesita dos campos porque son dos claves
// foráneas distintas; el mensaje no, SubjectType ya desambigua, y repetir la exclusividad
// donde ninguna restricción la puede hacer cumplir solo agregaría un estado inválido
// representable.
type JobRequestedEvent struct {
	// EventID identifica esta emisión y es distinto en cada una, incluso para el mismo
	// sujeto. Dos entregas del mismo mensaje lo conservan.
	EventID string `json:"event_id"`

	Version     int         `json:"version"`
	SubjectType SubjectType `json:"subject_type"`
	SubjectID   int32       `json:"subject_id"`

	// BatchID identifica el trabajo a ejecutar. Es lo que le alcanza a un consumidor
	// idempotente para decidir si ya lo hizo, sin comparar el resto del cuerpo.
	BatchID int32 `json:"batch_id"`

	EmittedAt time.Time `json:"emitted_at"`
}

// NewJobRequestedEvent arma el evento que respalda un batch ya persistido. Recibe el batch
// y no el sujeto porque el identificador del batch es obligatorio en el mensaje: no hay
// evento posible antes de abrirlo.
//
// emittedAt se normaliza a UTC. time.Time se serializa con su offset, y sin normalizar el
// cuerpo dependería de la zona horaria del proceso que lo emitió.
func NewJobRequestedEvent(eventID string, batch Batch, emittedAt time.Time) (JobRequestedEvent, error) {
	subjectID, err := batchSubjectID(batch)
	if err != nil {
		return JobRequestedEvent{}, err
	}

	return JobRequestedEvent{
		EventID:     eventID,
		Version:     JobRequestedVersion,
		SubjectType: batch.SubjectType,
		SubjectID:   subjectID,
		BatchID:     batch.ID,
		EmittedAt:   emittedAt.UTC(),
	}, nil
}

// batchSubjectID colapsa el par excluyente del batch en el identificador único del mensaje,
// y rechaza las combinaciones que la base no debería haber aceptado.
func batchSubjectID(batch Batch) (int32, error) {
	switch batch.SubjectType {
	case SubjectEmployee:
		if batch.EmployeeID == nil || batch.JobPositionID != nil {
			return 0, ErrInvalidSubject
		}
		return *batch.EmployeeID, nil
	case SubjectJobPosition:
		if batch.JobPositionID == nil || batch.EmployeeID != nil {
			return 0, ErrInvalidSubject
		}
		return *batch.JobPositionID, nil
	default:
		return 0, ErrInvalidSubject
	}
}

// Body serializa el evento al cuerpo que viaja por la cola. El transporte recibe un string
// ya serializado y no interpreta su contenido, así que el formato lo define este paquete.
func (e JobRequestedEvent) Body() (string, error) {
	body, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("serialize recommendation job event: %w", err)
	}

	return string(body), nil
}
