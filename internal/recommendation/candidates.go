package recommendation

import (
	"context"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/scoring"
)

// CandidateSource resuelve el universo de pares a evaluar para un sujeto.
//
// Devuelve scoring.Pair ya armados y no modelos de dominio a propósito. Traducir las filas a
// la entrada normalizada es, según scoring/doc.go, responsabilidad de quien lee de la base;
// concentrarla acá deja al worker sin ninguna decisión de mapeo y hace que un cambio en los
// atributos comparables toque un solo lugar.
//
// El puerto vive en este paquete y no en internal/jobposition ni en internal/employee porque
// atiende los dos sentidos a la vez, y porque este paquete no importa ninguno de los dos:
// invertir esa dirección crearía un ciclo. Importar internal/scoring sí es válido, porque
// scoring no importa ningún paquete de dominio.
//
// Un puesto eliminado lógicamente nunca aparece en ningún universo, ni como candidato ni
// como sujeto. Un sujeto inexistente —o un puesto eliminado— produce ErrSubjectNotFound, que
// es distinto de un universo vacío: lo primero significa que no hay para quién recomendar y
// lo segundo que no hay qué recomendar.
type CandidateSource interface {
	// PairsForEmployee empareja al empleado contra todos los puestos vigentes.
	PairsForEmployee(ctx context.Context, employeeID int32) ([]scoring.Pair, error)

	// PairsForJobPosition empareja al puesto contra todos los empleados con perfil.
	PairsForJobPosition(ctx context.Context, jobPositionID int32) ([]scoring.Pair, error)
}

// highestEducation devuelve el nivel educativo de mayor rango entre los registrados, o nil
// si el empleado no completó el paso de educación.
//
// El orden sale de scoring.EducationLevel.Rank() y no de un CASE en SQL: ya es parte del
// contrato de scoring, y una segunda copia en una consulta se desincronizaría con la primera
// sin que nada lo señale. Un valor que el contrato no conoce se ignora en vez de romper la
// traducción: la base admite exactamente los cuatro niveles conocidos, así que un desconocido
// solo puede venir de un contrato que cambió, y descartarlo degrada la comparación en lugar
// de tumbar el batch entero.
func highestEducation(types []string) *scoring.EducationLevel {
	var highest *scoring.EducationLevel
	highestRank := -1

	for _, raw := range types {
		level := scoring.EducationLevel(raw)
		rank, ok := level.Rank()
		if !ok || rank <= highestRank {
			continue
		}

		highestRank = rank
		found := level
		highest = &found
	}

	return highest
}

// technicalResources distingue los dos estados que el contrato de scoring separa: nil cuando
// el empleado no informó sus recursos, y un slice vacío cuando informó explícitamente que no
// tiene ninguno.
func technicalResources(paidSoftware []string, hasTechProfile bool) []string {
	if !hasTechProfile {
		return nil
	}
	if paidSoftware == nil {
		return []string{}
	}

	return paidSoftware
}
