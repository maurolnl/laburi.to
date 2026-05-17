package auth

import (
	"errors"
	"net/http"
	"testing"
	"time"
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

func TestJWTValidation(t *testing.T) {
	secret := "secret"
	expiresIn := time.Hour

	tests := []struct {
		name      string
		userID    int32
		secret    string
		expiresIn time.Duration
		wantErr   bool
	}{
		{
			name:      "valid token",
			userID:    0,
			secret:    secret,
			expiresIn: expiresIn,
			wantErr:   false,
		},
		{
			name:      "invalid token",
			userID:    1,
			secret:    "wrongSecret",
			expiresIn: expiresIn,
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			token, err := MakeJWT(tc.userID, tc.secret, tc.expiresIn)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("%sFAIL error generating token, err: %#v", red, err)
				}
				return
			}
			_, err = ValidateJWT(token, tc.secret)
			if err != nil {
				t.Errorf("%sFAIL error validating token, err: %#v", red, err)
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
