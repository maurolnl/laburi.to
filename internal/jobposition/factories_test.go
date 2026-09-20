package jobposition

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

const (
	testUserID        int32 = 42
	testEmployerID    int32 = 7
	testJobPositionID int32 = 11

	testSecret = "test-secret"
)

var testJobPositionTime = time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)

// testRequiredExperiences y testRequiredEducationLevels declaran el dominio completo que
// `validate:"oneof=..."` acepta en CreateJobPositionRequest. Las tablas de prueba se
// generan desde acá: agregar o quitar un valor del modelo obliga a tocar este único lugar,
// y los tests de cardinalidad fallan si ambos dominios dejan de coincidir.
var (
	testRequiredExperiences     = []string{"less_1y", "1y", "2_to_5y", "5_to_10y", "more_10y"}
	testRequiredEducationLevels = []string{"university", "postgraduate", "high-school-orientation", "tertiary"}
)

// Límites de available_hours_per_day impuestos por el modelo.
const (
	testMinHoursPerDay int16 = 1
	testMaxHoursPerDay int16 = 8
)

type testJobPositionOption func(*testJobPositionData)

type testJobPositionData struct {
	id                 int32
	employerID         int32
	userID             int32
	userRole           user.UserRole
	position           string
	jobRole            string
	experience         string
	educationLevel     string
	hoursPerDay        int16
	timezone           string
	technicalResources []string
	createdAt          time.Time
	updatedAt          time.Time
}

func defaultTestJobPositionData() testJobPositionData {
	return testJobPositionData{
		id:                 testJobPositionID,
		employerID:         testEmployerID,
		userID:             testUserID,
		userRole:           user.UserRoleEmployer,
		position:           "Backend Engineer",
		jobRole:            "Go developer",
		experience:         "2_to_5y",
		educationLevel:     "university",
		hoursPerDay:        6,
		timezone:           "America/Argentina/Buenos_Aires",
		technicalResources: []string{"Laptop"},
		createdAt:          testJobPositionTime,
		updatedAt:          testJobPositionTime,
	}
}

func withTestJobPositionID(id int32) testJobPositionOption {
	return func(data *testJobPositionData) { data.id = id }
}

func withTestJobPositionEmployerID(employerID int32) testJobPositionOption {
	return func(data *testJobPositionData) { data.employerID = employerID }
}

func withTestJobPositionUserID(userID int32) testJobPositionOption {
	return func(data *testJobPositionData) { data.userID = userID }
}

func withTestJobPositionUserRole(role user.UserRole) testJobPositionOption {
	return func(data *testJobPositionData) { data.userRole = role }
}

func withTestJobPositionPosition(position string) testJobPositionOption {
	return func(data *testJobPositionData) { data.position = position }
}

func withTestJobPositionRole(jobRole string) testJobPositionOption {
	return func(data *testJobPositionData) { data.jobRole = jobRole }
}

func withTestJobPositionExperience(experience string) testJobPositionOption {
	return func(data *testJobPositionData) { data.experience = experience }
}

func withTestJobPositionEducationLevel(level string) testJobPositionOption {
	return func(data *testJobPositionData) { data.educationLevel = level }
}

func withTestJobPositionHours(hours int16) testJobPositionOption {
	return func(data *testJobPositionData) { data.hoursPerDay = hours }
}

func withTestJobPositionTimezone(timezone string) testJobPositionOption {
	return func(data *testJobPositionData) { data.timezone = timezone }
}

// withTestJobPositionResources clona la slice recibida: el llamador puede seguir mutando la
// suya sin alterar el objeto construido, y `nil` sigue significando "sin recursos".
func withTestJobPositionResources(resources []string) testJobPositionOption {
	return func(data *testJobPositionData) {
		data.technicalResources = slices.Clone(resources)
	}
}

func withTestJobPositionTimes(createdAt, updatedAt time.Time) testJobPositionOption {
	return func(data *testJobPositionData) {
		data.createdAt = createdAt
		data.updatedAt = updatedAt
	}
}

func buildTestJobPositionData(options ...testJobPositionOption) testJobPositionData {
	data := defaultTestJobPositionData()
	for _, option := range options {
		option(&data)
	}
	data.technicalResources = slices.Clone(data.technicalResources)
	return data
}

func newTestCreateJobPositionRequest(options ...testJobPositionOption) CreateJobPositionRequest {
	data := buildTestJobPositionData(options...)
	return CreateJobPositionRequest{
		Position:               data.position,
		Role:                   data.jobRole,
		RequiredExperience:     data.experience,
		RequiredEducationLevel: data.educationLevel,
		AvailableHoursPerDay:   data.hoursPerDay,
		Timezone:               data.timezone,
		TechnicalResources:     slices.Clone(data.technicalResources),
	}
}

func newTestJobPosition(options ...testJobPositionOption) JobPosition {
	data := buildTestJobPositionData(options...)
	return JobPosition{
		ID:                     data.id,
		EmployerID:             data.employerID,
		Position:               data.position,
		Role:                   data.jobRole,
		RequiredExperience:     data.experience,
		RequiredEducationLevel: data.educationLevel,
		AvailableHoursPerDay:   data.hoursPerDay,
		Timezone:               data.timezone,
		TechnicalResources:     slices.Clone(data.technicalResources),
		CreatedAt:              data.createdAt,
		UpdatedAt:              data.updatedAt,
	}
}

func newTestDatabaseJobPosition(options ...testJobPositionOption) database.JobPosition {
	data := buildTestJobPositionData(options...)
	return database.JobPosition{
		ID:                     data.id,
		EmployerID:             data.employerID,
		Position:               data.position,
		Role:                   data.jobRole,
		RequiredExperience:     data.experience,
		RequiredEducationLevel: data.educationLevel,
		AvailableHoursPerDay:   data.hoursPerDay,
		Timezone:               data.timezone,
		TechnicalResources:     slices.Clone(data.technicalResources),
		CreatedAt:              data.createdAt,
		UpdatedAt:              data.updatedAt,
	}
}

func newTestCreateJobPositionParams(options ...testJobPositionOption) database.CreateJobPositionParams {
	data := buildTestJobPositionData(options...)
	return database.CreateJobPositionParams{
		EmployerID:             data.employerID,
		Position:               data.position,
		Role:                   data.jobRole,
		RequiredExperience:     data.experience,
		RequiredEducationLevel: data.educationLevel,
		AvailableHoursPerDay:   data.hoursPerDay,
		Timezone:               data.timezone,
		TechnicalResources:     slices.Clone(data.technicalResources),
	}
}

func newTestUpdateJobPositionParams(options ...testJobPositionOption) database.UpdateActiveJobPositionParams {
	data := buildTestJobPositionData(options...)
	return database.UpdateActiveJobPositionParams{
		ID:                     data.id,
		Position:               data.position,
		Role:                   data.jobRole,
		RequiredExperience:     data.experience,
		RequiredEducationLevel: data.educationLevel,
		AvailableHoursPerDay:   data.hoursPerDay,
		Timezone:               data.timezone,
		TechnicalResources:     slices.Clone(data.technicalResources),
	}
}

// newTestDatabaseEmployerRow es la fila que la query de sqlc devuelve al resolver el
// empleador del principal; solo su ID participa del contrato del repositorio.
func newTestDatabaseEmployerRow(options ...testJobPositionOption) database.Employer {
	data := buildTestJobPositionData(options...)
	return database.Employer{ID: data.employerID, UserID: data.userID}
}

func newTestJobPositionPrincipal(options ...testJobPositionOption) auth.Principal {
	data := buildTestJobPositionData(options...)
	return auth.Principal{UserID: data.userID, Role: data.userRole}
}

func employerPrincipal() auth.Principal {
	return newTestJobPositionPrincipal()
}

func employeePrincipal() auth.Principal {
	return newTestJobPositionPrincipal(withTestJobPositionUserRole(user.UserRoleEmployee))
}

func employerToken(t *testing.T) string {
	t.Helper()

	principal := employerPrincipal()
	token, err := auth.MakeJWT(principal.UserID, principal.Role, testSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// newTestJobPositionBody serializa la request de factory: el cuerpo HTTP y el modelo del
// dominio no pueden divergir, y un escenario inválido se expresa como un override visible.
func newTestJobPositionBody(t *testing.T, options ...testJobPositionOption) string {
	t.Helper()

	encoded, err := json.Marshal(newTestCreateJobPositionRequest(options...))
	if err != nil {
		t.Fatalf("marshal job position body: %v", err)
	}
	return string(encoded)
}

// newTestJobPositionBodyWithout construye un cuerpo al que le faltan campos obligatorios sin
// escribir JSON a mano, para que quitar un campo del modelo no deje el escenario obsoleto.
func newTestJobPositionBodyWithout(t *testing.T, fields ...string) string {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal([]byte(newTestJobPositionBody(t)), &body); err != nil {
		t.Fatalf("decode job position body: %v", err)
	}
	for _, field := range fields {
		if _, ok := body[field]; !ok {
			t.Fatalf("field %q is not part of the job position contract", field)
		}
		delete(body, field)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal job position body: %v", err)
	}
	return string(encoded)
}

// newTestJobPositionBodyWith agrega claves ajenas al contrato sobre un cuerpo válido, para
// comprobar que la atribución del puesto nunca proviene del cliente.
func newTestJobPositionBodyWith(t *testing.T, extra map[string]any) string {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal([]byte(newTestJobPositionBody(t)), &body); err != nil {
		t.Fatalf("decode job position body: %v", err)
	}
	for key, value := range extra {
		body[key] = value
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal job position body: %v", err)
	}
	return string(encoded)
}

// fakeJobPositionStore sustituye la persistencia y registra cuántas veces se alcanzó cada
// operación, de modo que los tests puedan afirmar que una petición rechazada no escribió.
type fakeJobPositionStore struct {
	employerID    int32
	employerErr   error
	createResult  JobPosition
	createErr     error
	getResult     JobPosition
	getErr        error
	listResult    []JobPosition
	listErr       error
	updateResult  JobPosition
	updateErr     error
	deleteErr     error
	employerCalls int
	createCalls   int
	getCalls      int
	listCalls     int
	updateCalls   int
	deleteCalls   int

	createEmployerID int32
	createRequest    CreateJobPositionRequest
	updateRequest    UpdateJobPositionRequest
}

func (f *fakeJobPositionStore) GetEmployerIDByUserID(context.Context, int32) (int32, error) {
	f.employerCalls++
	return f.employerID, f.employerErr
}

func (f *fakeJobPositionStore) CreateJobPosition(_ context.Context, employerID int32, request CreateJobPositionRequest) (JobPosition, error) {
	f.createCalls++
	f.createEmployerID = employerID
	f.createRequest = request
	return f.createResult, f.createErr
}

func (f *fakeJobPositionStore) GetActiveJobPositionByID(context.Context, int32) (JobPosition, error) {
	f.getCalls++
	return f.getResult, f.getErr
}

func (f *fakeJobPositionStore) ListActiveJobPositionsByEmployer(context.Context, int32) ([]JobPosition, error) {
	f.listCalls++
	return f.listResult, f.listErr
}

func (f *fakeJobPositionStore) UpdateActiveJobPosition(_ context.Context, _ int32, request UpdateJobPositionRequest) (JobPosition, error) {
	f.updateCalls++
	f.updateRequest = request
	return f.updateResult, f.updateErr
}

func (f *fakeJobPositionStore) SoftDeleteJobPosition(context.Context, int32) error {
	f.deleteCalls++
	return f.deleteErr
}

func (f *fakeJobPositionStore) writeCalls() int {
	return f.createCalls + f.updateCalls + f.deleteCalls
}

func ownedStore(options ...testJobPositionOption) *fakeJobPositionStore {
	return &fakeJobPositionStore{
		employerID:   testEmployerID,
		createResult: newTestJobPosition(options...),
		getResult:    newTestJobPosition(options...),
		updateResult: newTestJobPosition(options...),
	}
}

// fakePublisher es el doble del puerto de recomendaciones. Toda la suite lo usa en lugar de
// SQS: ninguna prueba del paquete abre red, lee entorno ni necesita credenciales.
type fakePublisher struct {
	err       error
	published []JobPosition
}

func (f *fakePublisher) JobPositionPublished(_ context.Context, position JobPosition) error {
	f.published = append(f.published, position)
	return f.err
}

func TestJobPositionFactoriesDefaultsAndOverrides(t *testing.T) {
	request := newTestCreateJobPositionRequest()
	if request.Position != "Backend Engineer" || request.Role != "Go developer" ||
		request.RequiredExperience != "2_to_5y" || request.RequiredEducationLevel != "university" ||
		request.AvailableHoursPerDay != 6 || request.Timezone != "America/Argentina/Buenos_Aires" {
		t.Fatalf("unexpected request defaults: %#v", request)
	}

	position := newTestJobPosition()
	if position.ID != testJobPositionID || position.EmployerID != testEmployerID ||
		position.CreatedAt != testJobPositionTime || position.UpdatedAt != testJobPositionTime {
		t.Fatalf("unexpected job position defaults: %#v", position)
	}

	createdAt := testJobPositionTime.Add(-time.Hour)
	row := newTestDatabaseJobPosition(
		withTestJobPositionID(99),
		withTestJobPositionEmployerID(100),
		withTestJobPositionPosition("Data Engineer"),
		withTestJobPositionRole("Python developer"),
		withTestJobPositionExperience("more_10y"),
		withTestJobPositionEducationLevel("tertiary"),
		withTestJobPositionHours(testMaxHoursPerDay),
		withTestJobPositionTimezone("Europe/Madrid"),
		withTestJobPositionTimes(createdAt, testJobPositionTime),
	)
	if row.ID != 99 || row.EmployerID != 100 || row.Position != "Data Engineer" ||
		row.Role != "Python developer" || row.RequiredExperience != "more_10y" ||
		row.RequiredEducationLevel != "tertiary" || row.AvailableHoursPerDay != testMaxHoursPerDay ||
		row.Timezone != "Europe/Madrid" || row.CreatedAt != createdAt {
		t.Fatalf("unexpected database job position overrides: %#v", row)
	}

	principal := newTestJobPositionPrincipal(
		withTestJobPositionUserID(5),
		withTestJobPositionUserRole(user.UserRoleEmployee),
	)
	if principal.UserID != 5 || principal.Role != user.UserRoleEmployee {
		t.Fatalf("unexpected principal overrides: %#v", principal)
	}
}

func TestJobPositionFactoriesCoverEveryEnumValue(t *testing.T) {
	if len(testRequiredExperiences) != 5 || len(testRequiredEducationLevels) != 4 {
		t.Fatalf("domain tables drifted: %d experiences, %d education levels",
			len(testRequiredExperiences), len(testRequiredEducationLevels))
	}

	for _, experience := range testRequiredExperiences {
		if got := newTestCreateJobPositionRequest(withTestJobPositionExperience(experience)); got.RequiredExperience != experience {
			t.Fatalf("RequiredExperience = %q, want %q", got.RequiredExperience, experience)
		}
	}
	for _, level := range testRequiredEducationLevels {
		if got := newTestCreateJobPositionRequest(withTestJobPositionEducationLevel(level)); got.RequiredEducationLevel != level {
			t.Fatalf("RequiredEducationLevel = %q, want %q", got.RequiredEducationLevel, level)
		}
	}
}

func TestJobPositionFactoriesBuildTechnicalResourceVariants(t *testing.T) {
	tests := []struct {
		name      string
		resources []string
		want      []string
	}{
		{name: "nil", resources: nil, want: nil},
		{name: "empty", resources: []string{}, want: []string{}},
		{name: "multiple", resources: []string{"Laptop", "VPN", "Monitor"}, want: []string{"Laptop", "VPN", "Monitor"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := newTestCreateJobPositionRequest(withTestJobPositionResources(tt.resources))
			if !slices.Equal(request.TechnicalResources, tt.want) {
				t.Fatalf("TechnicalResources = %#v, want %#v", request.TechnicalResources, tt.want)
			}
			if tt.resources == nil && request.TechnicalResources != nil {
				t.Fatalf("TechnicalResources = %#v, want nil so the override can exercise Normalize", request.TechnicalResources)
			}
		})
	}
}

func TestJobPositionFactoriesProduceIndependentData(t *testing.T) {
	first := newTestJobPosition()
	second := newTestJobPosition()
	first.TechnicalResources[0] = "Mutated"
	if second.TechnicalResources[0] != "Laptop" {
		t.Fatalf("factory instances share technical resources: %#v", second.TechnicalResources)
	}

	source := []string{"Laptop", "VPN"}
	request := newTestCreateJobPositionRequest(withTestJobPositionResources(source))
	source[0] = "Changed"
	if request.TechnicalResources[0] != "Laptop" {
		t.Fatalf("override kept a reference to the caller slice: %#v", request.TechnicalResources)
	}

	row := newTestDatabaseJobPosition(withTestJobPositionResources(source))
	row.TechnicalResources[0] = "Mutated"
	if newTestDatabaseJobPosition(withTestJobPositionResources(source)).TechnicalResources[0] != "Changed" {
		t.Fatal("database factory instances share technical resources")
	}
}

func TestJobPositionFactoriesBuildConsistentBodies(t *testing.T) {
	var decoded CreateJobPositionRequest
	body := newTestJobPositionBody(t, withTestJobPositionExperience("less_1y"))
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("factory body is not valid JSON: %v (%s)", err, body)
	}
	if !reflect.DeepEqual(decoded, newTestCreateJobPositionRequest(withTestJobPositionExperience("less_1y"))) {
		t.Fatalf("body = %s, want the factory request", body)
	}

	without := newTestJobPositionBodyWithout(t, "position")
	var partial map[string]any
	if err := json.Unmarshal([]byte(without), &partial); err != nil {
		t.Fatalf("body without a field is not valid JSON: %v", err)
	}
	if _, ok := partial["position"]; ok {
		t.Fatalf("body = %s, want the position field removed", without)
	}

	with := newTestJobPositionBodyWith(t, map[string]any{"employer_id": 999})
	var extended map[string]any
	if err := json.Unmarshal([]byte(with), &extended); err != nil {
		t.Fatalf("body with extra keys is not valid JSON: %v", err)
	}
	if extended["employer_id"] != float64(999) {
		t.Fatalf("body = %s, want the injected employer_id", with)
	}
}
