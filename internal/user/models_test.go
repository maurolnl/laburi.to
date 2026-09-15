package user

import (
	"testing"
)

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
				t.Errorf("expected validity %v for role %q, got %v", tc.valid, tc.role, got)
			}
		})
	}
}

func TestAuthorizeProfileRole(t *testing.T) {
	tests := []struct {
		name        string
		role        UserRole
		profileType UserRole
		wantErr     bool
	}{
		{"employee creates employee profile", UserRoleEmployee, UserRoleEmployee, false},
		{"employer creates employer profile", UserRoleEmployer, UserRoleEmployer, false},
		{"employee cannot create employer profile", UserRoleEmployee, UserRoleEmployer, true},
		{"employer cannot create employee profile", UserRoleEmployer, UserRoleEmployee, true},
		{"invalid account role rejected", "admin", UserRoleEmployee, true},
		{"invalid profile role rejected", UserRoleEmployee, "admin", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := AuthorizeProfileRole(tc.role, tc.profileType)
			if (err != nil) != tc.wantErr {
				t.Errorf("AuthorizeProfileRole(%q, %q) error = %v, wantErr %v", tc.role, tc.profileType, err, tc.wantErr)
			}
		})
	}
}
