package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
)

type fakeUserService struct {
	savedUsers       []CreateUserReq
	loginRole        UserRole
	currentUser      User
	currentUserID    int32
	getCurrentCalled bool
}

func (f *fakeUserService) SaveUser(_ context.Context, req CreateUserReq) error {
	f.savedUsers = append(f.savedUsers, req)
	return nil
}

func (f *fakeUserService) Login(_ context.Context, _, _ string) (int32, UserRole, string, string, error) {
	return 7, f.loginRole, "access-token", "refresh-token", nil
}

func (f *fakeUserService) GetCurrentUser(_ context.Context, userID int32) (User, error) {
	f.currentUserID = userID
	f.getCurrentCalled = true
	return f.currentUser, nil
}

func TestRegisterUserRoles(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantRole   UserRole
		wantSave   bool
	}{
		{"employee", `{"email":"employee@example.com","password":"secret123","role":"employee"}`, http.StatusOK, UserRoleEmployee, true},
		{"employer", `{"email":"employer@example.com","password":"secret123","role":"employer"}`, http.StatusOK, UserRoleEmployer, true},
		{"missing role", `{"email":"user@example.com","password":"secret123"}`, http.StatusBadRequest, "", false},
		{"invalid role", `{"email":"user@example.com","password":"secret123","role":"admin"}`, http.StatusBadRequest, "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeUserService{}
			handler := NewHandler(service, validator.New())
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(tc.body))

			handler.RegisterUser(recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tc.wantStatus, recorder.Code, recorder.Body.String())
			}
			if got := len(service.savedUsers); (got == 1) != tc.wantSave {
				t.Fatalf("expected save=%v, got %d calls", tc.wantSave, got)
			}
			if tc.wantSave && service.savedUsers[0].Role != tc.wantRole {
				t.Fatalf("expected role %q, got %q", tc.wantRole, service.savedUsers[0].Role)
			}
		})
	}
}

func TestLoginIncludesRole(t *testing.T) {
	service := &fakeUserService{loginRole: UserRoleEmployer}
	handler := NewHandler(service, validator.New())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"employer@example.com","password":"secret123"}`))

	handler.Login(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["role"] != string(UserRoleEmployer) {
		t.Fatalf("expected employer role, got %#v", response["role"])
	}
}

func TestGetCurrentUserUsesTokenPrincipalAndExposesRole(t *testing.T) {
	const secret = "test-secret"
	service := &fakeUserService{currentUser: User{ID: 7, Email: "employee@example.com", Role: UserRoleEmployee}}
	handler := NewHandler(service, validator.New())
	token, err := auth.MakeJWT(7, UserRoleEmployee, secret, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	AuthenticatedUser(secret)(http.HandlerFunc(handler.GetCurrentUser)).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !service.getCurrentCalled || service.currentUserID != 7 {
		t.Fatalf("expected current user lookup for token user 7, got %d", service.currentUserID)
	}
	var response map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["ID"] != float64(7) || response["Role"] != string(UserRoleEmployee) {
		t.Fatalf("unexpected response: %#v", response)
	}
}
