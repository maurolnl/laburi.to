package scoring

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/employee"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/jobposition"
)

func TestExperienceLevelValid(t *testing.T) {
	tests := []struct {
		name  string
		level ExperienceLevel
		want  bool
	}{
		{name: "less than a year", level: ExperienceLess1Y, want: true},
		{name: "one year", level: Experience1Y, want: true},
		{name: "two to five", level: Experience2To5Y, want: true},
		{name: "five to ten", level: Experience5To10Y, want: true},
		{name: "more than ten", level: ExperienceMore10Y, want: true},
		{name: "empty", level: "", want: false},
		{name: "unknown", level: "senior", want: false},
		{name: "wrong casing", level: "LESS_1Y", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.Valid(); got != tt.want {
				t.Fatalf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEducationLevelValid(t *testing.T) {
	tests := []struct {
		name  string
		level EducationLevel
		want  bool
	}{
		{name: "high school orientation", level: EducationHighSchoolOrientation, want: true},
		{name: "tertiary", level: EducationTertiary, want: true},
		{name: "university", level: EducationUniversity, want: true},
		{name: "postgraduate", level: EducationPostgraduate, want: true},
		{name: "empty", level: "", want: false},
		{name: "unknown", level: "phd", want: false},
		{name: "underscore instead of dash", level: "high_school_orientation", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.Valid(); got != tt.want {
				t.Fatalf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExperienceLevelOrder(t *testing.T) {
	ascending := []ExperienceLevel{
		ExperienceLess1Y,
		Experience1Y,
		Experience2To5Y,
		Experience5To10Y,
		ExperienceMore10Y,
	}

	for i := 1; i < len(ascending); i++ {
		lower, higher := ascending[i-1], ascending[i]
		if !higher.AtLeast(lower) {
			t.Errorf("%q should be at least %q", higher, lower)
		}
		if lower.AtLeast(higher) {
			t.Errorf("%q should not reach %q", lower, higher)
		}
	}

	if !Experience2To5Y.AtLeast(Experience2To5Y) {
		t.Error("a level must reach itself")
	}
}

func TestEducationLevelOrder(t *testing.T) {
	ascending := []EducationLevel{
		EducationHighSchoolOrientation,
		EducationTertiary,
		EducationUniversity,
		EducationPostgraduate,
	}

	for i := 1; i < len(ascending); i++ {
		lower, higher := ascending[i-1], ascending[i]
		if !higher.AtLeast(lower) {
			t.Errorf("%q should be at least %q", higher, lower)
		}
		if lower.AtLeast(higher) {
			t.Errorf("%q should not reach %q", lower, higher)
		}
	}
}

func TestUnknownLevelHasNoRankAndNeverSatisfies(t *testing.T) {
	if _, ok := ExperienceLevel("senior").Rank(); ok {
		t.Error("an unknown experience level must not have a rank")
	}
	if _, ok := EducationLevel("phd").Rank(); ok {
		t.Error("an unknown education level must not have a rank")
	}
	if ExperienceLevel("senior").AtLeast(ExperienceLess1Y) {
		t.Error("an unknown level must never satisfy a requirement")
	}
	if ExperienceMore10Y.AtLeast("senior") {
		t.Error("a requirement expressed with an unknown level must never be satisfied")
	}
}

// TestExperienceLevelsMatchDomainEnums y su equivalente de educación son el test de
// contrato que el design prometió: leen el oneof real de los structs de employee y
// jobposition en vez de repetir una lista a mano, de modo que agregar o renombrar un
// nivel en cualquiera de los dos paquetes rompa esta suite en lugar de producir pares
// que nunca matchean.
func TestExperienceLevelsMatchDomainEnums(t *testing.T) {
	want := []string{
		string(ExperienceLess1Y),
		string(Experience1Y),
		string(Experience2To5Y),
		string(Experience5To10Y),
		string(ExperienceMore10Y),
	}

	sources := map[string][]string{
		"jobposition.CreateJobPositionRequest.RequiredExperience": oneOfValues(t, jobposition.CreateJobPositionRequest{}, "RequiredExperience"),
		"employee.BaseEmployeeRequest.YearsOfExperience":          oneOfValues(t, employee.BaseEmployeeRequest{}, "YearsOfExperience"),
	}

	for source, got := range sources {
		t.Run(source, func(t *testing.T) {
			assertSameSet(t, got, want)
		})
	}
}

func TestEducationLevelsMatchDomainEnums(t *testing.T) {
	want := []string{
		string(EducationHighSchoolOrientation),
		string(EducationTertiary),
		string(EducationUniversity),
		string(EducationPostgraduate),
	}

	sources := map[string][]string{
		"jobposition.CreateJobPositionRequest.RequiredEducationLevel": oneOfValues(t, jobposition.CreateJobPositionRequest{}, "RequiredEducationLevel"),
		"employee.EmployeeEducationTitles.EducationType":              oneOfValues(t, employee.EmployeeEducationTitles{}, "EducationType"),
	}

	for source, got := range sources {
		t.Run(source, func(t *testing.T) {
			assertSameSet(t, got, want)
		})
	}
}

// oneOfValues extrae la lista de un `validate:"...,oneof=a b c"` del campo indicado.
func oneOfValues(t *testing.T, target any, field string) []string {
	t.Helper()

	structField, ok := reflect.TypeOf(target).FieldByName(field)
	if !ok {
		t.Fatalf("field %q not found in %T", field, target)
	}

	for _, rule := range strings.Split(structField.Tag.Get("validate"), ",") {
		if values, found := strings.CutPrefix(rule, "oneof="); found {
			return strings.Fields(values)
		}
	}

	t.Fatalf("field %q in %T has no oneof rule", field, target)
	return nil
}

func assertSameSet(t *testing.T, got, want []string) {
	t.Helper()

	gotSorted := append([]string(nil), got...)
	wantSorted := append([]string(nil), want...)
	sort.Strings(gotSorted)
	sort.Strings(wantSorted)

	if !reflect.DeepEqual(gotSorted, wantSorted) {
		t.Fatalf("scoring levels diverged from the domain enum:\n  domain  = %v\n  scoring = %v", gotSorted, wantSorted)
	}
}
