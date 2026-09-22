package scoring

import (
	"context"
	"errors"
	"testing"
)

// El worker de LAB-33 reacciona distinto según el error: una dependencia no disponible
// lleva el batch a failed, una entrada inválida es un bug del propio worker. El contrato
// debe permitir separarlos sin inspeccionar el texto del error.
func TestInvalidInputIsDistinguishableFromUnavailableDependency(t *testing.T) {
	invalid := Pair{
		Employee: EmployeeProfile{EmployeeID: 0, Experience: Experience1Y},
		Job:      samplePair().Job,
	}

	err := invalid.Validate()
	if err == nil {
		t.Fatal("a pair without an employee ID must be invalid")
	}
	if !errors.Is(err, ErrInvalidPair) {
		t.Error("an input error must be classifiable as ErrInvalidPair")
	}
	if errors.Is(err, ErrScoringUnavailable) {
		t.Error("an input error must not be confused with an unavailable dependency")
	}

	_, unavailableErr := Unavailable{}.Score(context.Background(), samplePair())
	if !errors.Is(unavailableErr, ErrScoringUnavailable) {
		t.Error("the production implementation must fail as unavailable")
	}
	if errors.Is(unavailableErr, ErrInvalidPair) {
		t.Error("an unavailable dependency must not be confused with an input error")
	}
}

func TestPairValidate(t *testing.T) {
	valid := samplePair()

	tests := []struct {
		name    string
		mutate  func(*Pair)
		wantErr error
	}{
		{name: "valid pair", mutate: func(*Pair) {}},
		{
			name:    "missing employee ID",
			mutate:  func(p *Pair) { p.Employee.EmployeeID = 0 },
			wantErr: ErrInvalidPair,
		},
		{
			name:    "missing job position ID",
			mutate:  func(p *Pair) { p.Job.JobPositionID = -1 },
			wantErr: ErrInvalidPair,
		},
		{
			name:    "unknown employee experience",
			mutate:  func(p *Pair) { p.Employee.Experience = "senior" },
			wantErr: ErrInvalidExperienceLevel,
		},
		{
			name:    "unknown required experience",
			mutate:  func(p *Pair) { p.Job.RequiredExperience = "" },
			wantErr: ErrInvalidExperienceLevel,
		},
		{
			name:    "unknown required education level",
			mutate:  func(p *Pair) { p.Job.RequiredEducationLevel = "phd" },
			wantErr: ErrInvalidEducationLevel,
		},
		{
			name:    "unknown employee education level",
			mutate:  func(p *Pair) { p.Employee.HighestEducation = education("phd") },
			wantErr: ErrInvalidEducationLevel,
		},
		{
			name:   "absent employee education is allowed",
			mutate: func(p *Pair) { p.Employee.HighestEducation = nil },
		},
		{
			name:   "absent availability is allowed",
			mutate: func(p *Pair) { p.Employee.AvailableHoursPerDay = nil },
		},
		{
			name:   "absent timezone is allowed",
			mutate: func(p *Pair) { p.Employee.Timezone = nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair := valid
			tt.mutate(&pair)

			err := pair.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tt.wantErr)
			}
			// Todo error de entrada envuelve ErrInvalidPair, aunque además identifique el
			// nivel concreto que falló.
			if !errors.Is(err, ErrInvalidPair) {
				t.Errorf("Validate() = %v, want it to also wrap ErrInvalidPair", err)
			}
		})
	}
}
