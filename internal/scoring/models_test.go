package scoring

import (
	"reflect"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/recommendation"
)

func hours(value int16) *int16 { return &value }

func tz(value string) *string { return &value }

func education(value EducationLevel) *EducationLevel { return &value }

func samplePair() Pair {
	return Pair{
		Employee: EmployeeProfile{
			EmployeeID:           7,
			Experience:           Experience2To5Y,
			HighestEducation:     education(EducationUniversity),
			AvailableHoursPerDay: hours(6),
			Timezone:             tz("America/Argentina/Buenos_Aires"),
			TechnicalResources:   []string{"figma"},
		},
		Job: JobRequirements{
			JobPositionID:          42,
			RequiredExperience:     Experience1Y,
			RequiredEducationLevel: EducationTertiary,
			AvailableHoursPerDay:   8,
			Timezone:               "America/Argentina/Buenos_Aires",
			TechnicalResources:     []string{"figma"},
		},
	}
}

// El perfil de empleado se completa en cinco pasos, así que un atributo puede no estar
// informado todavía. Cero horas disponibles y horas desconocidas son estados distintos y
// el contrato debe poder expresar ambos.
func TestEmployeeProfileAbsenceDiffersFromZeroValue(t *testing.T) {
	absent := EmployeeProfile{EmployeeID: 1, Experience: ExperienceLess1Y}
	zero := EmployeeProfile{
		EmployeeID:           1,
		Experience:           ExperienceLess1Y,
		AvailableHoursPerDay: hours(0),
		Timezone:             tz(""),
		TechnicalResources:   []string{},
	}

	if absent.AvailableHoursPerDay != nil {
		t.Error("an employee that did not complete the availability step must have no hours")
	}
	if zero.AvailableHoursPerDay == nil || *zero.AvailableHoursPerDay != 0 {
		t.Error("zero declared hours must be representable")
	}
	if absent.Timezone != nil {
		t.Error("an employee that did not complete the location step must have no timezone")
	}
	if zero.Timezone == nil {
		t.Error("an explicitly empty timezone must be distinguishable from an absent one")
	}
	if absent.TechnicalResources != nil {
		t.Error("undeclared resources must stay nil")
	}
	if zero.TechnicalResources == nil {
		t.Error("declaring no resources must be distinguishable from not declaring them")
	}
	if absent.HighestEducation != nil {
		t.Error("an employee that did not complete the education step must have no level")
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

func TestIndicatorContribution(t *testing.T) {
	indicator := Indicator{Name: "experience", Weight: 0.5, Value: 0.8}
	if got := indicator.Contribution(); got != 0.4 {
		t.Fatalf("Contribution() = %v, want 0.4", got)
	}
}

func TestEligibilityConstructors(t *testing.T) {
	accepted := Accepted()
	if !accepted.Eligible {
		t.Error("Accepted() must be eligible")
	}
	if accepted.Reason != "" {
		t.Error("an accepted pair carries no reason")
	}

	discarded := Discarded("timezone mismatch")
	if discarded.Eligible {
		t.Error("Discarded() must not be eligible")
	}
	if discarded.Reason != "timezone mismatch" {
		t.Errorf("Reason = %q, want the discard reason", discarded.Reason)
	}
}

func TestResultOutcomesAreMutuallyDistinguishable(t *testing.T) {
	pair := samplePair()

	tests := []struct {
		name         string
		result       Result
		wantEligible bool
		wantScore    bool
		wantReason   string
	}{
		{name: "scored", result: Scored(pair, 0.75), wantEligible: true, wantScore: true},
		{name: "scored zero", result: Scored(pair, 0), wantEligible: true, wantScore: true},
		{name: "eligible without score", result: Unscored(pair), wantEligible: true, wantScore: false},
		{name: "rejected", result: Rejected(pair, "timezone mismatch"), wantEligible: false, wantScore: false, wantReason: "timezone mismatch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Eligible != tt.wantEligible {
				t.Errorf("Eligible = %v, want %v", tt.result.Eligible, tt.wantEligible)
			}
			if tt.result.HasScore() != tt.wantScore {
				t.Errorf("HasScore() = %v, want %v", tt.result.HasScore(), tt.wantScore)
			}
			if tt.result.Reason != tt.wantReason {
				t.Errorf("Reason = %q, want %q", tt.result.Reason, tt.wantReason)
			}
			if tt.result.EmployeeID != 7 || tt.result.JobPositionID != 42 {
				t.Error("every result must carry the identifiers of its pair")
			}
		})
	}
}

// Es la distinción que LAB-29 ya hizo en la base al declarar score NUMERIC NULL: sin
// ella, un par puntuado con cero sería indistinguible de uno que no se pudo puntuar.
func TestZeroScoreDiffersFromAbsentScore(t *testing.T) {
	pair := samplePair()

	zero := Scored(pair, 0)
	absent := Unscored(pair)

	if !zero.HasScore() {
		t.Fatal("a zero score is still a score")
	}
	if absent.HasScore() {
		t.Fatal("an unscored result has no score")
	}
	if *zero.Total != 0 {
		t.Fatalf("Total = %v, want 0", *zero.Total)
	}
}

func TestRejectedResultNeverCarriesScoreOrIndicators(t *testing.T) {
	rejected := Rejected(samplePair(), "missing required education level")

	if rejected.Total != nil {
		t.Error("a rejected pair must not carry a score")
	}
	if len(rejected.Indicators) != 0 {
		t.Error("a rejected pair must not carry indicators")
	}
}

func TestScoredAcceptsPartialIndicatorSets(t *testing.T) {
	pair := samplePair()

	none := Scored(pair, 0.5)
	if len(none.Indicators) != 0 {
		t.Error("an implementation may report a total with no indicator detail")
	}

	partial := Scored(pair, 0.5, Indicator{Name: "experience", Weight: 1, Value: 0.5})
	if len(partial.Indicators) != 1 {
		t.Error("an implementation may report only the indicators it computed")
	}
}

// El criterio de aceptación pide que incorporar indicadores no cambie el transporte ni el
// esquema principal. Este test lo fija: al construir el Candidate que se persiste, solo
// viaja el total.
func TestResultToCandidateCarriesOnlyTheTotal(t *testing.T) {
	result := Scored(
		samplePair(),
		0.82,
		Indicator{Name: "experience", Weight: 0.6, Value: 0.9},
		Indicator{Name: "timezone", Weight: 0.4, Value: 0.7},
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
	result := Unscored(samplePair())

	candidate := recommendation.Candidate{
		EmployeeID:    result.EmployeeID,
		JobPositionID: result.JobPositionID,
		Score:         result.Total,
	}

	if candidate.Score != nil {
		t.Fatal("an unscored result must persist without a score")
	}
}
