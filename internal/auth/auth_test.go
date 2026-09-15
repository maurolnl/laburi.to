package auth

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var red = "\033[31m"

func TestHashingPassword(t *testing.T) {
	password1 := "supersecurepassword123"
	password2 := "anotherPass1234!"
	hash1, _ := HashPassword(password1)
	hash2, _ := HashPassword(password2)

	tests := []struct {
		name          string
		password      string
		hash          string
		wantErr       bool
		matchPassword bool
	}{
		{
			name:          "it should hash password correctly",
			password:      password1,
			hash:          hash1,
			wantErr:       false,
			matchPassword: true,
		},
		{
			name:          "Incorrect password",
			password:      "wrongPassword",
			hash:          hash2,
			wantErr:       false,
			matchPassword: false,
		},
		{
			name:          "Password doesn't match different hash",
			password:      password1,
			hash:          hash2,
			wantErr:       false,
			matchPassword: false,
		},
		{
			name:          "empty password",
			password:      "",
			hash:          hash1,
			wantErr:       false,
			matchPassword: false,
		},
		{
			name:          "Invalid hash",
			password:      password1,
			hash:          "invalidHash",
			wantErr:       true,
			matchPassword: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			match, err := CheckPasswordHash(tc.password, tc.hash)
			if (err != nil) != tc.wantErr {
				t.Errorf("%sFAIL error running match hash passoword, err: %#v", red, err)
			}
			if match != tc.matchPassword {
				t.Errorf("%sFAIL expected match to be %v. Got: %v", red, tc.matchPassword, match)
			}
		})
	}

}

func TestUserRoleValid(t *testing.T) {
	tests := []struct {
		name  string
		role  UserRole
		valid bool
	}{
		{"employee is valid", UserRoleEmployee, true},
		{"employer is valid", UserRoleEmployer, true},
		{"empty role is invalid", "", false},
		{"unknown role is invalid", "admin", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.role.Valid(); got != tc.valid {
				t.Errorf("%sFAIL expected validity %v for role %q, got %v", red, tc.valid, tc.role, got)
			}
		})
	}
}

func TestMakeJWTRejectsInvalidRole(t *testing.T) {
	_, err := MakeJWT(1, "admin", "secret", time.Hour)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("%sFAIL expected ErrInvalidToken for invalid role, got %v", red, err)
	}
}

func TestValidateJWTRejectsMissingOrInvalidRole(t *testing.T) {
	for _, role := range []UserRole{"", "admin"} {
		t.Run(string(role), func(t *testing.T) {
			claims := accessTokenClaims{
				Role: role,
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    TokenTypeAccess,
					Subject:   "1",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ValidateJWT(token, "secret"); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("expected ErrInvalidToken for role %q, got %v", role, err)
			}
		})
	}
}

func TestJWTValidation(t *testing.T) {
	secret := "secret"
	expiresIn := time.Hour

	tests := []struct {
		name           string
		userID         int32
		role           UserRole
		secret         string
		validateSecret string
		expiresIn      time.Duration
		wantErr        bool
	}{
		{
			name:           "valid employee token",
			userID:         0,
			role:           UserRoleEmployee,
			secret:         secret,
			validateSecret: secret,
			expiresIn:      expiresIn,
			wantErr:        false,
		},
		{
			name:           "valid employer token",
			userID:         42,
			role:           UserRoleEmployer,
			secret:         secret,
			validateSecret: secret,
			expiresIn:      expiresIn,
			wantErr:        false,
		},
		{
			name:           "invalid token signature",
			userID:         1,
			role:           UserRoleEmployee,
			secret:         "wrongSecret",
			validateSecret: secret,
			expiresIn:      expiresIn,
			wantErr:        true,
		},
		{
			name:           "invalid token role rejected",
			userID:         1,
			role:           "admin",
			secret:         secret,
			validateSecret: secret,
			expiresIn:      expiresIn,
			wantErr:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := MakeJWT(tc.userID, tc.role, tc.secret, tc.expiresIn)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("%sFAIL error generating token, err: %#v", red, err)
				}
				return
			}
			principal, err := ValidateJWT(token, tc.validateSecret)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("%sFAIL error validating token, err: %#v", red, err)
				}
				return
			}
			if tc.wantErr {
				t.Errorf("%sFAIL expected validation error", red)
				return
			}
			if principal.UserID != tc.userID {
				t.Errorf("%sFAIL expected userID %d, got %d", red, tc.userID, principal.UserID)
			}
			if principal.Role != tc.role {
				t.Errorf("%sFAIL expected role %q, got %q", red, tc.role, principal.Role)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	validHeaders := http.Header{}
	validHeaders.Add("Authorization", "Bearer 1223748923498")
	invalidHeaders := http.Header{}
	invalidHeaders.Add("Authorization", "1223748923498")

	tests := []struct {
		name        string
		header      http.Header
		expectedErr error
	}{
		{

			name:        "Valid bearer",
			header:      validHeaders,
			expectedErr: nil,
		},
		{
			name:        "Should throw error bearer",
			header:      invalidHeaders,
			expectedErr: ErrBadTokenFormat,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GetBearerToken(tc.header)
			if err != nil && tc.expectedErr == nil {
				t.Errorf("%sFAIL running get bearer token with header:%s. Error: %#v", red, tc.header, err)
			} else if err != nil && tc.expectedErr != nil && !errors.Is(err, tc.expectedErr) {
				t.Errorf("%sFAIL expected error: %v, is not equal to actual error: %v", red, tc.expectedErr, err)
			}
		})
	}
}
