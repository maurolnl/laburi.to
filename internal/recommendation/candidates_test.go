package recommendation

import (
	"context"
	"errors"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/scoring"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

func jobIDsOf(pairs []scoring.Pair) map[int32]scoring.Pair {
	byID := make(map[int32]scoring.Pair, len(pairs))
	for _, pair := range pairs {
		byID[pair.Job.JobPositionID] = pair
	}

	return byID
}

func employeeIDsOf(pairs []scoring.Pair) map[int32]scoring.Pair {
	byID := make(map[int32]scoring.Pair, len(pairs))
	for _, pair := range pairs {
		byID[pair.Employee.EmployeeID] = pair
	}

	return byID
}

func TestPairsForEmployeeCubreLosPuestosVigentes(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employeeID := newTestEmployee(t, db, withTestEmployeeExperience("5_to_10y"))
	newTestEmployeeAvailability(t, db, employeeID, 6)
	newTestEmployeeLocation(t, db, employeeID, "America/Argentina/Buenos_Aires")
	newTestEmployeeTech(t, db, employeeID, []string{"figma"})
	newTestEmployeeEducation(t, db, employeeID, "tertiary")
	newTestEmployeeEducation(t, db, employeeID, "postgraduate")
	newTestEmployeeEducation(t, db, employeeID, "university")

	employerID := newTestEmployer(t, db)
	firstJob := newTestJobPosition(t, db, employerID)
	secondJob := newTestJobPosition(t, db, employerID,
		withTestJobPositionRequirements("1y", "tertiary", 4, "Europe/Madrid", []string{"jira"}))

	pairs, err := repo.PairsForEmployee(ctx, employeeID)
	if err != nil {
		t.Fatalf("PairsForEmployee() error = %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("se esperaban 2 pares, uno por puesto vigente, se obtuvieron %d", len(pairs))
	}

	byJob := jobIDsOf(pairs)
	if _, ok := byJob[firstJob]; !ok {
		t.Fatalf("falta el par del puesto %d", firstJob)
	}

	second, ok := byJob[secondJob]
	if !ok {
		t.Fatalf("falta el par del puesto %d", secondJob)
	}
	if second.Job.RequiredExperience != scoring.Experience1Y ||
		second.Job.RequiredEducationLevel != scoring.EducationTertiary ||
		second.Job.AvailableHoursPerDay != 4 ||
		second.Job.Timezone != "Europe/Madrid" ||
		len(second.Job.TechnicalResources) != 1 || second.Job.TechnicalResources[0] != "jira" {
		t.Fatalf("los atributos comparables del puesto no viajaron completos: %+v", second.Job)
	}

	employee := second.Employee
	if employee.EmployeeID != employeeID {
		t.Fatalf("EmployeeID = %d, se esperaba %d", employee.EmployeeID, employeeID)
	}
	if employee.Experience != scoring.Experience5To10Y {
		t.Fatalf("Experience = %q, se esperaba %q", employee.Experience, scoring.Experience5To10Y)
	}
	if employee.AvailableHoursPerDay == nil || *employee.AvailableHoursPerDay != 6 {
		t.Fatalf("AvailableHoursPerDay = %v, se esperaba 6", employee.AvailableHoursPerDay)
	}
	if employee.Timezone == nil || *employee.Timezone != "America/Argentina/Buenos_Aires" {
		t.Fatalf("Timezone = %v", employee.Timezone)
	}
	// Postgrado es el de mayor rango entre los tres registrados: el orden sale del contrato
	// de scoring y no del orden de inserción.
	if employee.HighestEducation == nil || *employee.HighestEducation != scoring.EducationPostgraduate {
		t.Fatalf("HighestEducation = %v, se esperaba postgraduate", employee.HighestEducation)
	}
}

func TestPairsForEmployeeExcluyePuestosEliminados(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	activeJob := newTestJobPosition(t, db, employerID)
	deletedJob := newTestJobPosition(t, db, employerID, withTestJobPositionDeleted())

	pairs, err := repo.PairsForEmployee(ctx, employeeID)
	if err != nil {
		t.Fatalf("PairsForEmployee() error = %v", err)
	}

	byJob := jobIDsOf(pairs)
	if _, ok := byJob[deletedJob]; ok {
		t.Fatal("un puesto eliminado lógicamente nunca debe entrar al universo de candidatos")
	}
	if _, ok := byJob[activeJob]; !ok {
		t.Fatalf("falta el puesto vigente %d", activeJob)
	}
}

func TestPairsForEmployeeSinPuestosVigentes(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)

	employeeID := newTestEmployee(t, db)

	pairs, err := repo.PairsForEmployee(context.Background(), employeeID)
	if err != nil {
		t.Fatalf("PairsForEmployee() error = %v, un universo vacío no es un fallo", err)
	}
	if len(pairs) != 0 {
		t.Fatalf("se esperaba un universo vacío, se obtuvieron %d pares", len(pairs))
	}
}

func TestPairsForEmployeeSujetoInexistente(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)

	_, err := repo.PairsForEmployee(context.Background(), 999_999)
	if !errors.Is(err, ErrSubjectNotFound) {
		t.Fatalf("PairsForEmployee() error = %v, se esperaba ErrSubjectNotFound", err)
	}
}

// El perfil se completa en cinco pasos, así que un empleado a medias es candidato igual:
// decidir si alcanza es un filtro duro, y los filtros duros no están definidos.
func TestPairsConservanLaAusenciaDeDatosDelPerfil(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)

	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	newTestJobPosition(t, db, employerID)

	pairs, err := repo.PairsForEmployee(context.Background(), employeeID)
	if err != nil {
		t.Fatalf("PairsForEmployee() error = %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("un empleado con perfil incompleto también es candidato, se obtuvieron %d pares", len(pairs))
	}

	employee := pairs[0].Employee
	if employee.AvailableHoursPerDay != nil {
		t.Fatalf("sin paso de disponibilidad las horas deben ser ausentes, no cero: %v", *employee.AvailableHoursPerDay)
	}
	if employee.Timezone != nil {
		t.Fatalf("sin paso de ubicación el timezone debe ser ausente, no vacío: %q", *employee.Timezone)
	}
	if employee.HighestEducation != nil {
		t.Fatalf("sin paso de educación el nivel debe ser ausente: %q", *employee.HighestEducation)
	}
	if employee.TechnicalResources != nil {
		t.Fatalf("sin paso de recursos la lista debe ser nil y no vacía: %v", employee.TechnicalResources)
	}
}

// nil y slice vacío son estados distintos: no informó recursos frente a informó que no tiene.
func TestRecursosInformadosComoNingunoSeDistinguenDeNoInformados(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employerID := newTestEmployer(t, db)
	newTestJobPosition(t, db, employerID)

	declaredNone := newTestEmployee(t, db)
	newTestEmployeeTech(t, db, declaredNone, []string{})

	notReported := newTestEmployee(t, db)

	declaredPairs, err := repo.PairsForEmployee(ctx, declaredNone)
	if err != nil {
		t.Fatalf("PairsForEmployee() error = %v", err)
	}
	if got := declaredPairs[0].Employee.TechnicalResources; got == nil || len(got) != 0 {
		t.Fatalf("informar que no tiene recursos debe dar un slice vacío, se obtuvo %v", got)
	}

	silentPairs, err := repo.PairsForEmployee(ctx, notReported)
	if err != nil {
		t.Fatalf("PairsForEmployee() error = %v", err)
	}
	if got := silentPairs[0].Employee.TechnicalResources; got != nil {
		t.Fatalf("no informar recursos debe dar nil, se obtuvo %v", got)
	}
}

func TestPairsForJobPositionCubreLosEmpleados(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	firstEmployee := newTestEmployee(t, db)
	secondEmployee := newTestEmployee(t, db, withTestEmployeeExperience("more_10y"))
	newTestEmployeeEducation(t, db, secondEmployee, "university")

	employerID := newTestEmployer(t, db)
	jobID := newTestJobPosition(t, db, employerID,
		withTestJobPositionRequirements("1y", "tertiary", 4, "Europe/Madrid", []string{"jira"}))

	pairs, err := repo.PairsForJobPosition(ctx, jobID)
	if err != nil {
		t.Fatalf("PairsForJobPosition() error = %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("se esperaban 2 pares, uno por empleado, se obtuvieron %d", len(pairs))
	}

	byEmployee := employeeIDsOf(pairs)
	if _, ok := byEmployee[firstEmployee]; !ok {
		t.Fatalf("falta el par del empleado %d", firstEmployee)
	}

	second, ok := byEmployee[secondEmployee]
	if !ok {
		t.Fatalf("falta el par del empleado %d", secondEmployee)
	}
	if second.Employee.Experience != scoring.ExperienceMore10Y {
		t.Fatalf("Experience = %q", second.Employee.Experience)
	}
	if second.Employee.HighestEducation == nil || *second.Employee.HighestEducation != scoring.EducationUniversity {
		t.Fatalf("HighestEducation = %v", second.Employee.HighestEducation)
	}
	if second.Job.JobPositionID != jobID || second.Job.Timezone != "Europe/Madrid" {
		t.Fatalf("el puesto no viajó completo: %+v", second.Job)
	}
}

func TestPairsForJobPositionSinEmpleados(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)

	employerID := newTestEmployer(t, db)
	jobID := newTestJobPosition(t, db, employerID)

	pairs, err := repo.PairsForJobPosition(context.Background(), jobID)
	if err != nil {
		t.Fatalf("PairsForJobPosition() error = %v, un universo vacío no es un fallo", err)
	}
	if len(pairs) != 0 {
		t.Fatalf("se esperaba un universo vacío, se obtuvieron %d pares", len(pairs))
	}
}

// El puesto eliminado como sujeto se distingue del universo vacío: no hay para quién
// recomendar, no es que no haya qué recomendar.
func TestPairsForJobPositionSujetoEliminado(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employerID := newTestEmployer(t, db)
	newTestEmployee(t, db)
	deletedJob := newTestJobPosition(t, db, employerID, withTestJobPositionDeleted())

	_, err := repo.PairsForJobPosition(ctx, deletedJob)
	if !errors.Is(err, ErrSubjectNotFound) {
		t.Fatalf("PairsForJobPosition() error = %v, se esperaba ErrSubjectNotFound", err)
	}

	_, err = repo.PairsForJobPosition(ctx, 999_999)
	if !errors.Is(err, ErrSubjectNotFound) {
		t.Fatalf("PairsForJobPosition() sobre un puesto inexistente error = %v, se esperaba ErrSubjectNotFound", err)
	}
}

// Si la traducción produjera pares que el contrato rechaza, el worker fallaría batches por
// una entrada que él mismo armó mal.
func TestLosParesResueltosSuperanLaValidacionDelContrato(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	complete := newTestEmployee(t, db, withTestEmployeeExperience("less_1y"))
	newTestEmployeeAvailability(t, db, complete, 8)
	newTestEmployeeLocation(t, db, complete, "Europe/Madrid")
	newTestEmployeeTech(t, db, complete, []string{"figma"})
	newTestEmployeeEducation(t, db, complete, "high-school-orientation")

	incomplete := newTestEmployee(t, db)

	employerID := newTestEmployer(t, db)
	jobID := newTestJobPosition(t, db, employerID)

	forEmployee, err := repo.PairsForEmployee(ctx, incomplete)
	if err != nil {
		t.Fatalf("PairsForEmployee() error = %v", err)
	}
	forJob, err := repo.PairsForJobPosition(ctx, jobID)
	if err != nil {
		t.Fatalf("PairsForJobPosition() error = %v", err)
	}

	for _, pair := range append(forEmployee, forJob...) {
		if err := pair.Validate(); err != nil {
			t.Fatalf("par inválido %+v: %v", pair, err)
		}
	}
}
