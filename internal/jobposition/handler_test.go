package jobposition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type fakeJobPositionService struct {
	createResult  JobPosition
	createErr     error
	listResult    []JobPosition
	listErr       error
	getResult     JobPosition
	getErr        error
	updateResult  JobPosition
	updateErr     error
	deleteErr     error
	createCalls   int
	lastRequest   CreateJobPositionRequest
	lastEmployer  int32
	lastPositionn int32
	lastPrincipal auth.Principal
}

func (f *fakeJobPositionService) CreateJobPosition(_ context.Context, employerID int32, request CreateJobPositionRequest, principal auth.Principal) (JobPosition, error) {
	f.createCalls++
	f.lastEmployer = employerID
	f.lastRequest = request
	f.lastPrincipal = principal
	return f.createResult, f.createErr
}

func (f *fakeJobPositionService) ListJobPositions(_ context.Context, employerID int32, principal auth.Principal) ([]JobPosition, error) {
	f.lastEmployer = employerID
	f.lastPrincipal = principal
	return f.listResult, f.listErr
}

func (f *fakeJobPositionService) GetJobPosition(_ context.Context, jobPositionID int32, principal auth.Principal) (JobPosition, error) {
	f.lastPositionn = jobPositionID
	f.lastPrincipal = principal
	return f.getResult, f.getErr
}

func (f *fakeJobPositionService) UpdateJobPosition(_ context.Context, jobPositionID int32, request UpdateJobPositionRequest, principal auth.Principal) (JobPosition, error) {
	f.lastPositionn = jobPositionID
	f.lastRequest = request
	f.lastPrincipal = principal
	return f.updateResult, f.updateErr
}

func (f *fakeJobPositionService) DeleteJobPosition(_ context.Context, jobPositionID int32, principal auth.Principal) error {
	f.lastPositionn = jobPositionID
	f.lastPrincipal = principal
	return f.deleteErr
}

func newTestMux(fake *fakeJobPositionService) *http.ServeMux {
	return newTestMuxWithService(fake)
}

// newTestMuxWithService monta las rutas sobre cualquier implementación del servicio, para
// poder ejercer el borde HTTP contra el servicio real y un store doble cuando el escenario
// depende de la conducta del dominio y no solo de la adaptación HTTP.
func newTestMuxWithService(service JobPositionService) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewHandler(service, validator.New(validator.WithRequiredStructEnabled())), testSecret)
	return mux
}

func doRequest(t *testing.T, mux *http.ServeMux, method, target, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func validBody(t *testing.T) string {
	t.Helper()
	return newTestJobPositionBody(t)
}

// employerJobsPath y jobPath derivan las rutas de los mismos identificadores que usan las
// factories, para que un cambio de identidad de prueba no deje targets desincronizados.
func employerJobsPath() string {
	return fmt.Sprintf("/employers/%d/jobs", testEmployerID)
}

func jobPath() string {
	return fmt.Sprintf("/jobs/%d", testJobPositionID)
}

type route struct {
	name   string
	method string
	target string
	body   string
}

func allRoutes(t *testing.T) []route {
	t.Helper()
	return []route{
		{"create", http.MethodPost, employerJobsPath(), validBody(t)},
		{"list", http.MethodGet, employerJobsPath(), ""},
		{"get", http.MethodGet, jobPath(), ""},
		{"update", http.MethodPut, jobPath(), validBody(t)},
		{"delete", http.MethodDelete, jobPath(), ""},
	}
}

// writeRoutes son las dos operaciones que aceptan cuerpo y, por lo tanto, atraviesan la
// validación del dominio.
func writeRoutes() []route {
	return []route{
		{"create", http.MethodPost, employerJobsPath(), ""},
		{"update", http.MethodPut, jobPath(), ""},
	}
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not the JSON error contract: %v (%s)", err, recorder.Body.String())
	}
	if body["error"] == "" {
		t.Fatalf("response body = %s, want a non-empty \"error\" key", recorder.Body.String())
	}
	return body["error"]
}

func TestJobPositionRoutesRequireAuthentication(t *testing.T) {
	for _, r := range allRoutes(t) {
		t.Run(r.name, func(t *testing.T) {
			fake := &fakeJobPositionService{}
			recorder := doRequest(t, newTestMux(fake), r.method, r.target, r.body, "")

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
			decodeError(t, recorder)
			if fake.createCalls != 0 {
				t.Fatal("service was reached without authentication")
			}
		})
	}
}

func TestJobPositionRoutesRejectInvalidPathIDs(t *testing.T) {
	tests := []route{
		{"non numeric employer", http.MethodPost, "/employers/abc/jobs", validBody(t)},
		{"zero employer", http.MethodGet, "/employers/0/jobs", ""},
		{"non numeric position", http.MethodGet, "/jobs/abc", ""},
		{"negative position", http.MethodDelete, "/jobs/-1", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := doRequest(t, newTestMux(&fakeJobPositionService{}), tt.method, tt.target, tt.body, employerToken(t))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			decodeError(t, recorder)
		})
	}
}

func TestJobPositionHandlerRejectsInvalidBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"malformed JSON", `{"position":`},
		{"trailing content", validBody(t) + `{"extra":true}`},
		{"missing required field", newTestJobPositionBodyWithout(t, "position")},
		{"missing timezone", newTestJobPositionBodyWithout(t, "timezone")},
		{"blank position after trim", newTestJobPositionBody(t, withTestJobPositionPosition("   "))},
		{"blank role after trim", newTestJobPositionBody(t, withTestJobPositionRole(" \t "))},
		{"experience out of domain", newTestJobPositionBody(t, withTestJobPositionExperience("20y"))},
		{"education out of domain", newTestJobPositionBody(t, withTestJobPositionEducationLevel("kindergarten"))},
		{"hours above range", newTestJobPositionBody(t, withTestJobPositionHours(testMaxHoursPerDay+1))},
		{"hours below range", newTestJobPositionBody(t, withTestJobPositionHours(testMinHoursPerDay-1))},
		{"blank technical resource", newTestJobPositionBody(t, withTestJobPositionResources([]string{"Laptop", "  "}))},
	}

	for _, method := range writeRoutes() {
		for _, tt := range tests {
			t.Run(method.name+"/"+tt.name, func(t *testing.T) {
				fake := &fakeJobPositionService{}
				recorder := doRequest(t, newTestMux(fake), method.method, method.target, tt.body, employerToken(t))

				if recorder.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusBadRequest, recorder.Body.String())
				}
				decodeError(t, recorder)
				if fake.createCalls != 0 {
					t.Fatal("service was reached with an invalid body")
				}
			})
		}
	}
}

func TestJobPositionHandlerMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		serviceErr error
		wantCode   int
	}{
		{"role forbidden", user.ErrProfileRoleForbidden, http.StatusForbidden},
		{"employer profile required", ErrEmployerProfileRequired, http.StatusForbidden},
		{"foreign job position", ErrJobPositionForbidden, http.StatusForbidden},
		{"not found", ErrJobPositionNotFound, http.StatusNotFound},
		{"invalid timezone", ErrInvalidTimezone, http.StatusBadRequest},
		{"internal", errors.New("database unavailable"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		for _, r := range allRoutes(t) {
			t.Run(tt.name+"/"+r.name, func(t *testing.T) {
				fake := &fakeJobPositionService{
					createErr: tt.serviceErr,
					listErr:   tt.serviceErr,
					getErr:    tt.serviceErr,
					updateErr: tt.serviceErr,
					deleteErr: tt.serviceErr,
				}
				recorder := doRequest(t, newTestMux(fake), r.method, r.target, r.body, employerToken(t))

				if recorder.Code != tt.wantCode {
					t.Fatalf("status = %d, want %d (%s)", recorder.Code, tt.wantCode, recorder.Body.String())
				}
				message := decodeError(t, recorder)
				if tt.wantCode == http.StatusInternalServerError && strings.Contains(message, "database unavailable") {
					t.Fatalf("error message leaked the internal failure: %q", message)
				}
				if tt.wantCode == http.StatusForbidden && message != "forbidden" {
					t.Fatalf("error message = %q, want an opaque \"forbidden\"", message)
				}
			})
		}
	}
}

func TestJobPositionHandlerSuccessResponses(t *testing.T) {
	position := newTestJobPosition()

	t.Run("create returns 201 with the position", func(t *testing.T) {
		fake := &fakeJobPositionService{createResult: position}
		recorder := doRequest(t, newTestMux(fake), http.MethodPost, employerJobsPath(), validBody(t), employerToken(t))

		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusCreated, recorder.Body.String())
		}
		if got := recorder.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("response is not JSON: %v", err)
		}
		for _, key := range []string{"id", "employer_id", "position", "role", "required_experience", "required_education_level", "available_hours_per_day", "timezone", "technical_resources", "created_at", "updated_at"} {
			if _, ok := body[key]; !ok {
				t.Fatalf("response is missing %q: %s", key, recorder.Body.String())
			}
		}
		if fake.lastEmployer != testEmployerID {
			t.Fatalf("employerID = %d, want %d", fake.lastEmployer, testEmployerID)
		}
		if fake.lastPrincipal.UserID != testUserID || fake.lastPrincipal.Role != user.UserRoleEmployer {
			t.Fatalf("principal = %#v, want the authenticated employer", fake.lastPrincipal)
		}
	})

	t.Run("create normalizes the request before the service", func(t *testing.T) {
		fake := &fakeJobPositionService{createResult: position}
		body := `{"position":"  Backend Engineer  ","role":"Go developer","required_experience":"2_to_5y",` +
			`"required_education_level":"university","available_hours_per_day":6,` +
			`"timezone":"America/Argentina/Buenos_Aires"}`
		doRequest(t, newTestMux(fake), http.MethodPost, "/employers/7/jobs", body, employerToken(t))

		if fake.lastRequest.Position != "Backend Engineer" {
			t.Fatalf("position = %q, want the trimmed value", fake.lastRequest.Position)
		}
		if fake.lastRequest.TechnicalResources == nil || len(fake.lastRequest.TechnicalResources) != 0 {
			t.Fatalf("technical resources = %#v, want an empty non-nil slice", fake.lastRequest.TechnicalResources)
		}
	})

	t.Run("empty list returns an array", func(t *testing.T) {
		fake := &fakeJobPositionService{listResult: []JobPosition{}}
		recorder := doRequest(t, newTestMux(fake), http.MethodGet, employerJobsPath(), "", employerToken(t))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if strings.TrimSpace(recorder.Body.String()) != "[]" {
			t.Fatalf("body = %s, want []", recorder.Body.String())
		}
	})

	t.Run("get and update return 200", func(t *testing.T) {
		fake := &fakeJobPositionService{getResult: position, updateResult: position}
		mux := newTestMux(fake)

		if recorder := doRequest(t, mux, http.MethodGet, jobPath(), "", employerToken(t)); recorder.Code != http.StatusOK {
			t.Fatalf("get status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if recorder := doRequest(t, mux, http.MethodPut, jobPath(), validBody(t), employerToken(t)); recorder.Code != http.StatusOK {
			t.Fatalf("update status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if fake.lastPositionn != testJobPositionID {
			t.Fatalf("jobPositionID = %d, want %d", fake.lastPositionn, testJobPositionID)
		}
	})

	t.Run("delete returns 204 without a body", func(t *testing.T) {
		fake := &fakeJobPositionService{}
		recorder := doRequest(t, newTestMux(fake), http.MethodDelete, jobPath(), "", employerToken(t))

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
		}
		if recorder.Body.Len() != 0 {
			t.Fatalf("body = %s, want an empty body", recorder.Body.String())
		}
	})
}

func TestJobPositionHandlerAcceptsEveryDomainValue(t *testing.T) {
	overrides := make([]struct {
		name   string
		option testJobPositionOption
	}, 0, len(testRequiredExperiences)+len(testRequiredEducationLevels)+2)

	for _, experience := range testRequiredExperiences {
		overrides = append(overrides, struct {
			name   string
			option testJobPositionOption
		}{"experience/" + experience, withTestJobPositionExperience(experience)})
	}
	for _, level := range testRequiredEducationLevels {
		overrides = append(overrides, struct {
			name   string
			option testJobPositionOption
		}{"education/" + level, withTestJobPositionEducationLevel(level)})
	}
	overrides = append(overrides,
		struct {
			name   string
			option testJobPositionOption
		}{"hours/minimum", withTestJobPositionHours(testMinHoursPerDay)},
		struct {
			name   string
			option testJobPositionOption
		}{"hours/maximum", withTestJobPositionHours(testMaxHoursPerDay)},
	)

	for _, override := range overrides {
		for _, r := range writeRoutes() {
			t.Run(r.name+"/"+override.name, func(t *testing.T) {
				position := newTestJobPosition(override.option)
				fake := &fakeJobPositionService{createResult: position, updateResult: position}
				body := newTestJobPositionBody(t, override.option)

				recorder := doRequest(t, newTestMux(fake), r.method, r.target, body, employerToken(t))

				wantCode := http.StatusCreated
				if r.method == http.MethodPut {
					wantCode = http.StatusOK
				}
				if recorder.Code != wantCode {
					t.Fatalf("status = %d, want %d (%s)", recorder.Code, wantCode, recorder.Body.String())
				}
				if !reflect.DeepEqual(fake.lastRequest, newTestCreateJobPositionRequest(override.option)) {
					t.Fatalf("service request = %#v, want %#v", fake.lastRequest, newTestCreateJobPositionRequest(override.option))
				}
			})
		}
	}
}

func TestJobPositionHandlerForwardsTechnicalResourceVariants(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"omitted", newTestJobPositionBodyWithout(t, "technical_resources"), []string{}},
		{"explicit null", newTestJobPositionBodyWith(t, map[string]any{"technical_resources": nil}), []string{}},
		{"empty", newTestJobPositionBody(t, withTestJobPositionResources([]string{})), []string{}},
		{"multiple", newTestJobPositionBody(t, withTestJobPositionResources([]string{"Laptop", "VPN", "Monitor"})), []string{"Laptop", "VPN", "Monitor"}},
	}

	for _, tt := range tests {
		for _, r := range writeRoutes() {
			t.Run(r.name+"/"+tt.name, func(t *testing.T) {
				fake := &fakeJobPositionService{createResult: newTestJobPosition(), updateResult: newTestJobPosition()}
				doRequest(t, newTestMux(fake), r.method, r.target, tt.body, employerToken(t))

				if fake.lastRequest.TechnicalResources == nil {
					t.Fatal("service received nil technical resources, want an empty non-nil slice")
				}
				if !reflect.DeepEqual(fake.lastRequest.TechnicalResources, tt.want) {
					t.Fatalf("technical resources = %#v, want %#v", fake.lastRequest.TechnicalResources, tt.want)
				}
			})
		}
	}
}

// TestJobPositionHandlerIgnoresClientAttribution comprueba que el cuerpo no puede reasignar
// el puesto: los campos de atribución no forman parte de CreateJobPositionRequest y el
// empleador siempre se deriva del path validado contra el JWT.
func TestJobPositionHandlerIgnoresClientAttribution(t *testing.T) {
	foreignEmployerID := testEmployerID + 1
	body := newTestJobPositionBodyWith(t, map[string]any{
		"id":          999,
		"employer_id": foreignEmployerID,
		"created_at":  "2000-01-01T00:00:00Z",
	})

	fake := &fakeJobPositionService{createResult: newTestJobPosition()}
	recorder := doRequest(t, newTestMux(fake), http.MethodPost, employerJobsPath(), body, employerToken(t))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if fake.lastEmployer != testEmployerID {
		t.Fatalf("employerID = %d, want %d derived from the path validated against the JWT", fake.lastEmployer, testEmployerID)
	}
	if !reflect.DeepEqual(fake.lastRequest, newTestCreateJobPositionRequest()) {
		t.Fatalf("service request = %#v, want the contract fields only", fake.lastRequest)
	}
	if fake.lastPrincipal.UserID != testUserID || fake.lastPrincipal.Role != user.UserRoleEmployer {
		t.Fatalf("principal = %#v, want the authenticated employer", fake.lastPrincipal)
	}
}

// TestJobPositionHandlerDoesNotReopenDeletedPositions ejerce el borde HTTP contra el
// servicio real: un puesto eliminado no puede editarse ni volver a eliminarse, y ninguna de
// las dos operaciones escribe ni notifica al proceso de recomendaciones.
func TestJobPositionHandlerDoesNotReopenDeletedPositions(t *testing.T) {
	tests := []route{
		{"update", http.MethodPut, jobPath(), validBody(t)},
		{"delete", http.MethodDelete, jobPath(), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := ownedStore()
			store.getErr = ErrJobPositionNotFound
			publisher := &fakePublisher{}

			recorder := doRequest(t, newTestMuxWithService(NewService(store, publisher)), tt.method, tt.target, tt.body, employerToken(t))

			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d (%s)", recorder.Code, http.StatusNotFound, recorder.Body.String())
			}
			if message := decodeError(t, recorder); message != ErrJobPositionNotFound.Error() {
				t.Fatalf("error = %q, want %q", message, ErrJobPositionNotFound.Error())
			}
			if store.writeCalls() != 0 {
				t.Fatalf("%s wrote a deleted job position: %#v", tt.name, store)
			}
			if len(publisher.published) != 0 {
				t.Fatalf("%s notified the recommendation process: %#v", tt.name, publisher.published)
			}
		})
	}
}

// TestJobPositionHandlerSurvivesPublisherFailure ejerce el borde HTTP contra el servicio real
// con un publicador que falla: el alta y la edición ya están persistidas, así que el cliente
// recibe su código de éxito y no un error por algo que ocurre después de su cambio.
func TestJobPositionHandlerSurvivesPublisherFailure(t *testing.T) {
	tests := []struct {
		route
		wantStatus int
	}{
		{route{"create", http.MethodPost, employerJobsPath(), validBody(t)}, http.StatusCreated},
		{route{"update", http.MethodPut, jobPath(), validBody(t)}, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := ownedStore()
			publisher := &fakePublisher{err: errors.New("queue unavailable")}

			recorder := doRequest(t, newTestMuxWithService(NewService(store, publisher)), tt.method, tt.target, tt.body, employerToken(t))

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
			if store.writeCalls() != 1 {
				t.Fatalf("%s did not persist its change: %#v", tt.name, store)
			}
			if len(publisher.published) != 1 || publisher.published[0] != newTestJobPosition().ID {
				t.Fatalf("published = %#v, want exactly the job position id", publisher.published)
			}
		})
	}
}
