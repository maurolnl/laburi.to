package employer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type createEmployerCall struct {
	ctx       context.Context
	request   CreateEmployerRequest
	principal auth.Principal
}

type getEmployerCall struct {
	ctx       context.Context
	userID    int32
	principal auth.Principal
}

type fakeEmployerService struct {
	createErr   error
	createCalls []createEmployerCall
	getResult   Employer
	getErr      error
	getCalls    []getEmployerCall
}

func (f *fakeEmployerService) CreateEmployer(ctx context.Context, request CreateEmployerRequest, principal auth.Principal) error {
	f.createCalls = append(f.createCalls, createEmployerCall{ctx: ctx, request: request, principal: principal})
	return f.createErr
}

func (f *fakeEmployerService) GetEmployer(ctx context.Context, userID int32, principal auth.Principal) (Employer, error) {
	f.getCalls = append(f.getCalls, getEmployerCall{ctx: ctx, userID: userID, principal: principal})
	return f.getResult, f.getErr
}

func newTestHandler(t *testing.T) (*EmployerHandler, *fakeEmployerService) {
	t.Helper()
	fake := &fakeEmployerService{}
	return NewHandler(fake, validator.New()), fake
}

func makeToken(t *testing.T, secret string, userID int32, role user.UserRole) string {
	t.Helper()
	tok, err := auth.MakeJWT(userID, role, secret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func makeEmployerToken(t *testing.T, secret string, userID int32) string {
	t.Helper()
	return makeToken(t, secret, userID, user.UserRoleEmployer)
}

func makeEmployeeToken(t *testing.T, secret string, userID int32) string {
	t.Helper()
	return makeToken(t, secret, userID, user.UserRoleEmployee)
}

func TestCreateEmployer(t *testing.T) {
	const secret = "test-secret"
	normalizedReq := newTestCreateEmployerRequest()

	tests := []struct {
		name          string
		authorization string
		body          string
		serviceErr    error
		expectedCode  int
		expectCall    bool
		expectedReq   CreateEmployerRequest
		expectedUser  int32
		expectedRole  user.UserRole
		bodyExcludes  string
	}{
		{
			name:          "success with normalization and ignored user_id",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			body:          `{"name":"  Acme  ","industry":" Software ","location":"Remote","hiring_modalities":[" Full time "],"user_id":999}`,
			expectedCode:  http.StatusCreated,
			expectCall:    true,
			expectedReq:   normalizedReq,
			expectedUser:  42,
			expectedRole:  user.UserRoleEmployer,
		},
		{
			name:         "missing authentication",
			body:         `{"name":"Acme","industry":"Software","location":"Remote","hiring_modalities":["Full time"]}`,
			expectedCode: http.StatusUnauthorized,
			expectCall:   false,
		},
		{
			name:          "employee role forbidden",
			authorization: "Bearer " + makeEmployeeToken(t, secret, 7),
			body:          `{"name":"Acme","industry":"Software","location":"Remote","hiring_modalities":["Full time"]}`,
			serviceErr:    user.ErrProfileRoleForbidden,
			expectedCode:  http.StatusForbidden,
			expectCall:    true,
			expectedReq:   normalizedReq,
			expectedUser:  7,
			expectedRole:  user.UserRoleEmployee,
		},
		{
			name:          "malformed JSON",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			body:          `{"name":`,
			expectedCode:  http.StatusBadRequest,
			expectCall:    false,
		},
		{
			name:          "trailing JSON",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			body:          `{"name":"Acme"}{"extra":"x"}`,
			expectedCode:  http.StatusBadRequest,
			expectCall:    false,
		},
		{
			name:          "null modalities",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			body:          `{"name":"Acme","industry":"Software","location":"Remote","hiring_modalities":null}`,
			expectedCode:  http.StatusBadRequest,
			expectCall:    false,
		},
		{
			name:          "validator rejects whitespace-only required field",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			body:          `{"name":"   ","industry":"Software","location":"Remote","hiring_modalities":[]}`,
			expectedCode:  http.StatusBadRequest,
			expectCall:    false,
		},
		{
			name:          "validator rejects whitespace-only modality",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			body:          `{"name":"Acme","industry":"Software","location":"Remote","hiring_modalities":["  "]}`,
			expectedCode:  http.StatusBadRequest,
			expectCall:    false,
		},
		{
			name:          "conflict employer already exists",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			body:          `{"name":"Acme","industry":"Software","location":"Remote","hiring_modalities":["Full time"]}`,
			serviceErr:    ErrEmployerAlreadyExists,
			expectedCode:  http.StatusConflict,
			expectCall:    true,
			expectedReq:   normalizedReq,
			expectedUser:  42,
			expectedRole:  user.UserRoleEmployer,
		},
		{
			name:          "conflict incompatible profile",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			body:          `{"name":"Acme","industry":"Software","location":"Remote","hiring_modalities":["Full time"]}`,
			serviceErr:    ErrEmployerProfileConflict,
			expectedCode:  http.StatusConflict,
			expectCall:    true,
			expectedReq:   normalizedReq,
			expectedUser:  42,
			expectedRole:  user.UserRoleEmployer,
		},
		{
			name:          "internal error does not leak details",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			body:          `{"name":"Acme","industry":"Software","location":"Remote","hiring_modalities":["Full time"]}`,
			serviceErr:    errors.New("internal database password=secret"),
			expectedCode:  http.StatusInternalServerError,
			expectCall:    true,
			expectedReq:   normalizedReq,
			expectedUser:  42,
			expectedRole:  user.UserRoleEmployer,
			bodyExcludes:  "password=secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, fake := newTestHandler(t)
			fake.createErr = tt.serviceErr

			req := httptest.NewRequest(http.MethodPost, "/employers", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			rec := httptest.NewRecorder()

			user.AuthenticatedUser(secret)(http.HandlerFunc(h.CreateEmployer)).ServeHTTP(rec, req)

			if rec.Code != tt.expectedCode {
				t.Fatalf("expected status %d, got %d: %s", tt.expectedCode, rec.Code, rec.Body.String())
			}
			if (len(fake.createCalls) > 0) != tt.expectCall {
				t.Fatalf("expected service call=%v, got %d calls", tt.expectCall, len(fake.createCalls))
			}
			if tt.expectCall {
				call := fake.createCalls[0]
				if call.principal.UserID != tt.expectedUser {
					t.Fatalf("expected principal userID %d, got %d", tt.expectedUser, call.principal.UserID)
				}
				if call.principal.Role != tt.expectedRole {
					t.Fatalf("expected principal role %q, got %q", tt.expectedRole, call.principal.Role)
				}
				if !reflect.DeepEqual(call.request, tt.expectedReq) {
					t.Fatalf("expected request %#v, got %#v", tt.expectedReq, call.request)
				}
			}
			if tt.bodyExcludes != "" && strings.Contains(rec.Body.String(), tt.bodyExcludes) {
				t.Fatalf("response leaked internal detail: %s", rec.Body.String())
			}
		})
	}
}

func TestCreateEmployerErrorFormats(t *testing.T) {
	const secret = "test-secret"
	token := "Bearer " + makeEmployerToken(t, secret, 42)

	t.Run("invalid JSON uses JSON error contract", func(t *testing.T) {
		h, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPost, "/employers", strings.NewReader(`{"name":`))
		req.Header.Set("Authorization", token)
		rec := httptest.NewRecorder()

		user.AuthenticatedUser(secret)(http.HandlerFunc(h.CreateEmployer)).ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if got, want := rec.Body.String(), `{"error":"invalid JSON body"}`; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})

	t.Run("field validation keeps shared plain-text contract", func(t *testing.T) {
		h, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPost, "/employers", strings.NewReader(`{"name":" ","industry":"Software","location":"Remote"}`))
		req.Header.Set("Authorization", token)
		rec := httptest.NewRecorder()

		user.AuthenticatedUser(secret)(http.HandlerFunc(h.CreateEmployer)).ServeHTTP(rec, req)

		if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
			t.Fatalf("Content-Type = %q, want text/plain; charset=utf-8", got)
		}
		if got, want := rec.Body.String(), "Name is required\n"; got != want {
			t.Fatalf("body = %q, want %q", got, want)
		}
	})
}

func TestGetEmployer(t *testing.T) {
	const secret = "test-secret"
	tests := []struct {
		name           string
		authorization  string
		pathUserID     string
		serviceResult  Employer
		serviceErr     error
		expectedCode   int
		expectCall     bool
		expectedUserID int32
		expectedRole   user.UserRole
		bodyExcludes   string
		checkBody      func(t *testing.T, body string)
	}{
		{
			name:           "success returns snake_case and empty modalities",
			authorization:  "Bearer " + makeEmployerToken(t, secret, 42),
			pathUserID:     "42",
			serviceResult:  newTestEmployer(withTestEmployerModalities(nil)),
			expectedCode:   http.StatusOK,
			expectCall:     true,
			expectedUserID: 42,
			expectedRole:   user.UserRoleEmployer,
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, `"user_id"`) {
					t.Fatalf("expected snake_case user_id in body: %s", body)
				}
				if !strings.Contains(body, `"hiring_modalities"`) {
					t.Fatalf("expected snake_case hiring_modalities in body: %s", body)
				}
				if !strings.Contains(body, `"created_at"`) {
					t.Fatalf("expected snake_case created_at in body: %s", body)
				}
				if strings.Contains(body, `"userID"`) || strings.Contains(body, `"hiringModalities"`) {
					t.Fatalf("unexpected camelCase keys in body: %s", body)
				}
				if !strings.Contains(body, `"hiring_modalities":[]`) {
					t.Fatalf("expected empty modalities array in body: %s", body)
				}
			},
		},
		{
			name:         "missing authentication",
			pathUserID:   "42",
			expectedCode: http.StatusUnauthorized,
			expectCall:   false,
		},
		{
			name:          "invalid user id",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			pathUserID:    "abc",
			expectedCode:  http.StatusBadRequest,
			expectCall:    false,
		},
		{
			name:          "non-positive user id zero",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			pathUserID:    "0",
			expectedCode:  http.StatusBadRequest,
			expectCall:    false,
		},
		{
			name:          "non-positive user id negative",
			authorization: "Bearer " + makeEmployerToken(t, secret, 42),
			pathUserID:    "-1",
			expectedCode:  http.StatusBadRequest,
			expectCall:    false,
		},
		{
			name:           "forbidden other user without service call",
			authorization:  "Bearer " + makeEmployerToken(t, secret, 42),
			pathUserID:     "43",
			serviceResult:  newTestEmployer(withTestEmployerName("Private employer")),
			expectedCode:   http.StatusForbidden,
			expectCall:     false,
			expectedUserID: 42,
			expectedRole:   user.UserRoleEmployer,
			bodyExcludes:   "Private employer",
		},
		{
			name:           "employee cannot access another user without service call",
			authorization:  "Bearer " + makeEmployeeToken(t, secret, 5),
			pathUserID:     "42",
			serviceResult:  newTestEmployer(withTestEmployerName("Private employer")),
			expectedCode:   http.StatusForbidden,
			expectCall:     false,
			expectedUserID: 5,
			expectedRole:   user.UserRoleEmployee,
			bodyExcludes:   "Private employer",
		},
		{
			name:           "employee role forbidden",
			authorization:  "Bearer " + makeEmployeeToken(t, secret, 5),
			pathUserID:     "5",
			serviceErr:     user.ErrProfileRoleForbidden,
			expectedCode:   http.StatusForbidden,
			expectCall:     true,
			expectedUserID: 5,
			expectedRole:   user.UserRoleEmployee,
		},
		{
			name:           "not found",
			authorization:  "Bearer " + makeEmployerToken(t, secret, 42),
			pathUserID:     "42",
			serviceErr:     ErrEmployerNotFound,
			expectedCode:   http.StatusNotFound,
			expectCall:     true,
			expectedUserID: 42,
			expectedRole:   user.UserRoleEmployer,
		},
		{
			name:           "internal error does not leak details",
			authorization:  "Bearer " + makeEmployerToken(t, secret, 42),
			pathUserID:     "42",
			serviceErr:     errors.New("internal database password=secret"),
			expectedCode:   http.StatusInternalServerError,
			expectCall:     true,
			expectedUserID: 42,
			expectedRole:   user.UserRoleEmployer,
			bodyExcludes:   "password=secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, fake := newTestHandler(t)
			fake.getResult = tt.serviceResult
			fake.getErr = tt.serviceErr

			req := httptest.NewRequest(http.MethodGet, "/users/"+tt.pathUserID+"/employer", nil)
			req.SetPathValue("userID", tt.pathUserID)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			rec := httptest.NewRecorder()

			user.AuthenticatedUser(secret)(http.HandlerFunc(h.GetEmployer)).ServeHTTP(rec, req)

			if rec.Code != tt.expectedCode {
				t.Fatalf("expected status %d, got %d: %s", tt.expectedCode, rec.Code, rec.Body.String())
			}
			if (len(fake.getCalls) > 0) != tt.expectCall {
				t.Fatalf("expected service call=%v, got %d calls", tt.expectCall, len(fake.getCalls))
			}
			if tt.expectCall {
				call := fake.getCalls[0]
				if call.principal.UserID != tt.expectedUserID {
					t.Fatalf("expected principal userID %d, got %d", tt.expectedUserID, call.principal.UserID)
				}
				if call.principal.Role != tt.expectedRole {
					t.Fatalf("expected principal role %q, got %q", tt.expectedRole, call.principal.Role)
				}
				wantUserID, err := strconv.ParseInt(tt.pathUserID, 10, 32)
				if err != nil {
					t.Fatalf("invalid pathUserID %q in test table", tt.pathUserID)
				}
				if call.userID != int32(wantUserID) {
					t.Fatalf("expected userID %d, got %d", wantUserID, call.userID)
				}
			}
			if tt.bodyExcludes != "" && strings.Contains(rec.Body.String(), tt.bodyExcludes) {
				t.Fatalf("response leaked internal detail: %s", rec.Body.String())
			}
			if tt.checkBody != nil {
				tt.checkBody(t, rec.Body.String())
			}
		})
	}
}

func TestEmployerRoutesRequireAuthentication(t *testing.T) {
	h, fake := newTestHandler(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, h, "test-secret")

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create", method: http.MethodPost, path: "/employers", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/users/42/employer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", got)
			}
		})
	}
	if len(fake.createCalls) != 0 || len(fake.getCalls) != 0 {
		t.Fatalf("unauthenticated routes called service: create=%d get=%d", len(fake.createCalls), len(fake.getCalls))
	}
}
