package recommendation

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/lib/pq"
)

// Las factories insertan filas reales: los tests de este paquete verifican el
// comportamiento del esquema, así que no sirve un fake. Siguen el patrón de opciones de
// internal/employer/factories_test.go.

var testSequence int32

// nextTestID evita colisiones en las columnas únicas (email) sin depender del orden de
// ejecución de los tests.
func nextTestID() int32 {
	testSequence++
	return testSequence
}

type testUserData struct {
	email string
	role  string
}

type testUserOption func(*testUserData)

func withTestUserRole(role string) testUserOption {
	return func(data *testUserData) { data.role = role }
}

func newTestUser(t *testing.T, db execer, opts ...testUserOption) int32 {
	t.Helper()

	data := testUserData{
		email: fmt.Sprintf("user-%d-%d@laburi.to", nextTestID(), time.Now().UnixNano()),
		role:  "employee",
	}
	for _, opt := range opts {
		opt(&data)
	}

	var id int32
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO users (email, hashed_password, role, created_at, updated_at)
		VALUES ($1, $2, $3, now(), now())
		RETURNING id
	`, data.email, "hash", data.role).Scan(&id)
	if err != nil {
		t.Fatalf("factory: crear usuario: %v", err)
	}

	return id
}

type testEmployeeData struct {
	position          string
	role              string
	yearsOfExperience string
	updatedAt         *time.Time
}

type testEmployeeOption func(*testEmployeeData)

func withTestEmployeePosition(position string) testEmployeeOption {
	return func(data *testEmployeeData) { data.position = position }
}

// withTestEmployeeUpdatedAt fija la última actualización del perfil, que es el desempate
// del orden de candidatos.
func withTestEmployeeUpdatedAt(updatedAt time.Time) testEmployeeOption {
	return func(data *testEmployeeData) { data.updatedAt = &updatedAt }
}

func newTestEmployee(t *testing.T, db execer, opts ...testEmployeeOption) int32 {
	t.Helper()

	data := testEmployeeData{
		position:          "Backend Developer",
		role:              "Individual Contributor",
		yearsOfExperience: "2_to_5y",
	}
	for _, opt := range opts {
		opt(&data)
	}

	userID := newTestUser(t, db)

	updatedAt := time.Now().UTC()
	if data.updatedAt != nil {
		updatedAt = *data.updatedAt
	}

	var id int32
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO employees (position, role, years_of_experience, certifications, user_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, now(), $6)
		RETURNING id
	`, data.position, data.role, data.yearsOfExperience, pq.Array([]string{}), userID, updatedAt).Scan(&id)
	if err != nil {
		t.Fatalf("factory: crear empleado: %v", err)
	}

	return id
}

func newTestEmployer(t *testing.T, db execer) int32 {
	t.Helper()

	userID := newTestUser(t, db, withTestUserRole("employer"))

	var id int32
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO employers (user_id, name, industry, location, hiring_modalities)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, userID, "Acme", "Software", "Remote", pq.Array([]string{"Full time"})).Scan(&id)
	if err != nil {
		t.Fatalf("factory: crear empleador: %v", err)
	}

	return id
}

type testJobPositionData struct {
	position               string
	createdAt              *time.Time
	deleted                bool
	requiredExperience     string
	requiredEducationLevel string
	availableHoursPerDay   int16
	timezone               string
	technicalResources     []string
}

type testJobPositionOption func(*testJobPositionData)

func withTestJobPositionPosition(position string) testJobPositionOption {
	return func(data *testJobPositionData) { data.position = position }
}

// withTestJobPositionCreatedAt fija la fecha de publicación, que es el desempate del orden
// de puestos.
func withTestJobPositionCreatedAt(createdAt time.Time) testJobPositionOption {
	return func(data *testJobPositionData) { data.createdAt = &createdAt }
}

func withTestJobPositionDeleted() testJobPositionOption {
	return func(data *testJobPositionData) { data.deleted = true }
}

func newTestJobPosition(t *testing.T, db execer, employerID int32, opts ...testJobPositionOption) int32 {
	t.Helper()

	data := testJobPositionData{
		position:               "Backend Developer",
		requiredExperience:     "2_to_5y",
		requiredEducationLevel: "university",
		availableHoursPerDay:   8,
		timezone:               "America/Argentina/Buenos_Aires",
		technicalResources:     []string{},
	}
	for _, opt := range opts {
		opt(&data)
	}

	createdAt := time.Now().UTC()
	if data.createdAt != nil {
		createdAt = *data.createdAt
	}

	var deletedAt sql.NullTime
	if data.deleted {
		deletedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}

	var id int32
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO job_positions (
			employer_id, position, role, required_experience, required_education_level,
			available_hours_per_day, timezone, technical_resources, created_at, updated_at, deleted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), $10)
		RETURNING id
	`, employerID, data.position, "Individual Contributor", data.requiredExperience, data.requiredEducationLevel,
		data.availableHoursPerDay, data.timezone, pq.Array(data.technicalResources), createdAt, deletedAt).Scan(&id)
	if err != nil {
		t.Fatalf("factory: crear puesto: %v", err)
	}

	return id
}

type testBatchData struct {
	status BatchStatus
}

type testBatchOption func(*testBatchData)

func withTestBatchStatus(status BatchStatus) testBatchOption {
	return func(data *testBatchData) { data.status = status }
}

// newTestBatch inserta un batch directamente, sin pasar por el repositorio, para que los
// tests puedan construir estados que el repositorio no permite alcanzar en un solo paso.
func newTestBatch(t *testing.T, db execer, subject Subject, opts ...testBatchOption) int32 {
	t.Helper()

	data := testBatchData{status: BatchPending}
	for _, opt := range opts {
		opt(&data)
	}

	var id int32
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO recommendation_batches (subject_type, employee_id, job_position_id, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, string(subject.Type), nullInt32(subject.EmployeeID), nullInt32(subject.JobPositionID), string(data.status)).Scan(&id)
	if err != nil {
		t.Fatalf("factory: crear batch: %v", err)
	}

	return id
}

func newTestRecommendation(t *testing.T, db execer, batchID, employeeID, jobPositionID int32, score *float64) int32 {
	t.Helper()

	var id int32
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO recommendations (batch_id, employee_id, job_position_id, score)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, batchID, employeeID, jobPositionID, scoreToDatabase(score)).Scan(&id)
	if err != nil {
		t.Fatalf("factory: crear recomendación: %v", err)
	}

	return id
}

func scorePtr(value float64) *float64 { return &value }

// execer abstrae *sql.DB y *sql.Tx, de modo que las mismas factories sirvan para los tests
// que verifican constraints dentro de una transacción revertida y para los que necesitan
// observar un commit real.
type execer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Factories de los pasos opcionales del perfil del empleado. Existen sueltas y no como
// opciones de newTestEmployee porque el punto de los tests del universo de candidatos es
// justamente el empleado al que le faltan pasos: cada test agrega solo los que necesita.

func withTestEmployeeExperience(level string) testEmployeeOption {
	return func(data *testEmployeeData) { data.yearsOfExperience = level }
}

func newTestEmployeeAvailability(t *testing.T, db execer, employeeID int32, hours int16) {
	t.Helper()

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO employee_profile_availability (employee_id, available_hours_per_day)
		VALUES ($1, $2)
	`, employeeID, hours)
	if err != nil {
		t.Fatalf("factory: crear disponibilidad: %v", err)
	}
}

func newTestEmployeeLocation(t *testing.T, db execer, employeeID int32, timezone string) {
	t.Helper()

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO employee_location (employee_id, timezone) VALUES ($1, $2)
	`, employeeID, timezone)
	if err != nil {
		t.Fatalf("factory: crear locación: %v", err)
	}
}

// newTestEmployeeTech admite paidSoftware nil para cubrir la fila de recursos existente pero
// sin software declarado, que es distinta de no tener fila.
func newTestEmployeeTech(t *testing.T, db execer, employeeID int32, paidSoftware []string) {
	t.Helper()

	var software any
	if paidSoftware != nil {
		software = pq.Array(paidSoftware)
	}

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO employee_profile_tech (employee_id, os, paid_software) VALUES ($1, $2, $3)
	`, employeeID, "linux", software)
	if err != nil {
		t.Fatalf("factory: crear recursos técnicos: %v", err)
	}
}

func newTestEmployeeEducation(t *testing.T, db execer, employeeID int32, educationType string) {
	t.Helper()

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO employee_education (employee_id, education_type, title, status)
		VALUES ($1, $2, $3, 'completed')
	`, employeeID, educationType, "Título")
	if err != nil {
		t.Fatalf("factory: crear educación: %v", err)
	}
}

// withTestJobPositionRequirements fija los atributos comparables del puesto, que son los que
// viajan a la entrada normalizada del scoring.
func withTestJobPositionRequirements(experience, education string, hours int16, timezone string, resources []string) testJobPositionOption {
	return func(data *testJobPositionData) {
		data.requiredExperience = experience
		data.requiredEducationLevel = education
		data.availableHoursPerDay = hours
		data.timezone = timezone
		data.technicalResources = resources
	}
}
