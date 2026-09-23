// Estos tests fijan la frontera entre el contrato de scoring y lo que se persiste, así que
// necesitan los dos paquetes a la vez. Viven en el paquete externo scoring_test y no en
// scoring porque internal/recommendation importa internal/scoring desde que resuelve el
// universo de candidatos: un archivo `package scoring` que importara recommendation cerraría
// un ciclo. El paquete externo se compila aparte y no lo cierra.
package scoring_test

import (
	"reflect"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/recommendation"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/scoring"
)

func samplePair() scoring.Pair {
	return scoring.Pair{
		Employee: scoring.EmployeeProfile{
			EmployeeID: 7,
			Experience: scoring.Experience2To5Y,
		},
		Job: scoring.JobRequirements{
			JobPositionID:          42,
			RequiredExperience:     scoring.Experience1Y,
			RequiredEducationLevel: scoring.EducationUniversity,
		},
	}
}

// El par conserva lo único que la persistencia necesita para guardar el resultado: los
// dos identificadores. Sin ellos, el llamador tendría que correlacionar resultados con
// entradas por posición.
func TestPairKeepsIdentifiersNeededByPersistence(t *testing.T) {
	pair := samplePair()

	employeeID, jobPositionID := pair.IDs()
	if employeeID != 7 || jobPositionID != 42 {
		t.Fatalf("IDs() = (%d, %d), want (7, 42)", employeeID, jobPositionID)
	}

	candidate := recommendation.Candidate{
		EmployeeID:    employeeID,
		JobPositionID: jobPositionID,
	}
	if candidate.EmployeeID != 7 || candidate.JobPositionID != 42 {
		t.Fatal("a candidate must be buildable from the pair alone")
	}
}

// El criterio de aceptación pide que incorporar indicadores no cambie el transporte ni el
// esquema principal. Este test lo fija: al construir el Candidate que se persiste, solo
// viaja el total.
func TestResultToCandidateCarriesOnlyTheTotal(t *testing.T) {
	result := scoring.Scored(
		samplePair(),
		0.82,
		scoring.Indicator{Name: "experience", Weight: 0.6, Value: 0.9},
		scoring.Indicator{Name: "timezone", Weight: 0.4, Value: 0.7},
	)

	candidate := recommendation.Candidate{
		EmployeeID:    result.EmployeeID,
		JobPositionID: result.JobPositionID,
		Score:         result.Total,
	}

	if candidate.Score == nil || *candidate.Score != 0.82 {
		t.Fatalf("Score = %v, want 0.82", candidate.Score)
	}

	fields := reflect.VisibleFields(reflect.TypeOf(candidate))
	if len(fields) != 3 {
		t.Fatalf("recommendation.Candidate has %d fields; indicators must not have leaked into the schema", len(fields))
	}
}

func TestUnscoredResultMapsToCandidateWithoutScore(t *testing.T) {
	result := scoring.Unscored(samplePair())

	candidate := recommendation.Candidate{
		EmployeeID:    result.EmployeeID,
		JobPositionID: result.JobPositionID,
		Score:         result.Total,
	}

	if candidate.Score != nil {
		t.Fatal("an unscored result must persist without a score")
	}
}
