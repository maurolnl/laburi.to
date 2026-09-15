package employee

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

func TestAuthenticatedEmployeeMiddleWare(t *testing.T) {
	secret := "test-secret"
	validToken := func(userID int32) string {
		t.Helper()
		tok, err := auth.MakeJWT(userID, user.UserRoleEmployee, secret, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}

	tests := []struct {
		name           string
		authorization  string
		employeeID     string
		getEmployee    func(ctx context.Context, employeeID int32) (Employee, error)
		expectedStatus int
		expectNext     bool
		expectedEmpID  int32
	}{
		{
			name:           "missing token",
			employeeID:     "1",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid token format",
			authorization:  "Basic abc",
			employeeID:     "1",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid token signature",
			authorization:  "Bearer " + func() string { tok, _ := auth.MakeJWT(1, user.UserRoleEmployee, "other-secret", time.Hour); return tok }(),
			employeeID:     "1",
			getEmployee:    func(ctx context.Context, employeeID int32) (Employee, error) { return Employee{}, nil },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "non-numeric employee id",
			authorization:  "Bearer " + validToken(1),
			employeeID:     "abc",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:          "employee not found",
			authorization: "Bearer " + validToken(1),
			employeeID:    "99",
			getEmployee: func(ctx context.Context, employeeID int32) (Employee, error) {
				return Employee{}, errors.New("not found")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:          "forbidden other user",
			authorization: "Bearer " + validToken(2),
			employeeID:    "1",
			getEmployee: func(ctx context.Context, employeeID int32) (Employee, error) {
				return Employee{ID: 1, UserID: 1}, nil
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:          "owner allowed",
			authorization: "Bearer " + validToken(1),
			employeeID:    "1",
			getEmployee: func(ctx context.Context, employeeID int32) (Employee, error) {
				return Employee{ID: 1, UserID: 1}, nil
			},
			expectedStatus: http.StatusOK,
			expectNext:     true,
			expectedEmpID:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getEmployee := tt.getEmployee
			if getEmployee == nil {
				getEmployee = func(ctx context.Context, employeeID int32) (Employee, error) {
					return Employee{}, nil
				}
			}

			middleware := AuthenticatedEmployeeMiddleWare(AuthMiddlewareCfg{
				SecretKey:   secret,
				GetEmployee: getEmployee,
			})

			var nextCalled bool
			var gotEmployeeID int64
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				gotEmployeeID, _ = r.Context().Value(employeeIDKey).(int64)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("PUT", "/employees/"+tt.employeeID, nil)
			req.SetPathValue("employeeID", tt.employeeID)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			rec := httptest.NewRecorder()

			middleware(next).ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Fatalf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
			if nextCalled != tt.expectNext {
				t.Fatalf("expected next called=%v, got %v", tt.expectNext, nextCalled)
			}
			if tt.expectNext && gotEmployeeID != int64(tt.expectedEmpID) {
				t.Fatalf("expected employeeID %d in context, got %d", tt.expectedEmpID, gotEmployeeID)
			}
		})
	}
}
