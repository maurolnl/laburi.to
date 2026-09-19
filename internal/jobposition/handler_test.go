package jobposition

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

const testSecret = "test-secret"

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
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewHandler(fake, validator.New(validator.WithRequiredStructEnabled())), testSecret)
	return mux
}

func employerToken(t *testing.T) string {
	t.Helper()
	token, err := auth.MakeJWT(testUserID, user.UserRoleEmployer, testSecret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
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

func validBody() string {
	return `{"position":"Backend Engineer","role":"Go developer","required_experience":"2_to_5y",` +
		`"required_education_level":"university","available_hours_per_day":6,` +
		`"timezone":"America/Argentina/Buenos_Aires","technical_resources":["Laptop"]}`
}

type route struct {
	name   string
	method string
	target string
	body   string
}

func allRoutes() []route {
	return []route{
		{"create", http.MethodPost, "/employers/7/jobs", validBody()},
		{"list", http.MethodGet, "/employers/7/jobs", ""},
		{"get", http.MethodGet, "/jobs/11", ""},
		{"update", http.MethodPut, "/jobs/11", validBody()},
		{"delete", http.MethodDelete, "/jobs/11", ""},
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
	for _, r := range allRoutes() {
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
		{"non numeric employer", http.MethodPost, "/employers/abc/jobs", validBody()},
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
		{"trailing content", validBody() + `{"extra":true}`},
		{"missing required field", `{"role":"Go developer","required_experience":"2_to_5y","required_education_level":"university","available_hours_per_day":6,"timezone":"America/Argentina/Buenos_Aires"}`},
		{"blank position after trim", strings.Replace(validBody(), `"Backend Engineer"`, `"   "`, 1)},
		{"experience out of domain", strings.Replace(validBody(), `"2_to_5y"`, `"20y"`, 1)},
		{"education out of domain", strings.Replace(validBody(), `"university"`, `"kindergarten"`, 1)},
		{"hours above range", strings.Replace(validBody(), `"available_hours_per_day":6`, `"available_hours_per_day":9`, 1)},
		{"hours below range", strings.Replace(validBody(), `"available_hours_per_day":6`, `"available_hours_per_day":0`, 1)},
	}

	for _, method := range []struct {
		name   string
		method string
		target string
	}{
		{"create", http.MethodPost, "/employers/7/jobs"},
		{"update", http.MethodPut, "/jobs/11"},
	} {
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
		for _, r := range allRoutes() {
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
		recorder := doRequest(t, newTestMux(fake), http.MethodPost, "/employers/7/jobs", validBody(), employerToken(t))

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
		recorder := doRequest(t, newTestMux(fake), http.MethodGet, "/employers/7/jobs", "", employerToken(t))

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

		if recorder := doRequest(t, mux, http.MethodGet, "/jobs/11", "", employerToken(t)); recorder.Code != http.StatusOK {
			t.Fatalf("get status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if recorder := doRequest(t, mux, http.MethodPut, "/jobs/11", validBody(), employerToken(t)); recorder.Code != http.StatusOK {
			t.Fatalf("update status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if fake.lastPositionn != testJobPositionID {
			t.Fatalf("jobPositionID = %d, want %d", fake.lastPositionn, testJobPositionID)
		}
	})

	t.Run("delete returns 204 without a body", func(t *testing.T) {
		fake := &fakeJobPositionService{}
		recorder := doRequest(t, newTestMux(fake), http.MethodDelete, "/jobs/11", "", employerToken(t))

		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
		}
		if recorder.Body.Len() != 0 {
			t.Fatalf("body = %s, want an empty body", recorder.Body.String())
		}
	})
}
