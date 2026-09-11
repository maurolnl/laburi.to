package employee

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestEmployeeRequestValidation(t *testing.T) {
	validate := validator.New()
	validBase := BaseEmployeeRequest{
		Position:          "Backend developer",
		Role:              "Developer",
		YearsOfExperience: Years2To5Y,
	}

	tests := []struct {
		name    string
		request any
		wantErr bool
	}{
		{name: "valid base without optional fields", request: CreateEmployeeRequest{BaseEmployeeRequest: validBase}},
		{name: "missing required position", request: CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Role: "Developer", YearsOfExperience: Years2To5Y}}, wantErr: true},
		{name: "invalid years of experience", request: CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Backend developer", Role: "Developer", YearsOfExperience: "invalid"}}, wantErr: true},
		{name: "optional valid portfolio", request: CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Backend developer", Role: "Developer", YearsOfExperience: Years2To5Y, PortfolioURL: "https://example.com"}}},
		{name: "optional invalid portfolio", request: CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Backend developer", Role: "Developer", YearsOfExperience: Years2To5Y, PortfolioURL: "not-a-url"}}, wantErr: true},
		{name: "availability accepts optional zero counts", request: CreateEmployeeProfileAvailabilityRequest{BaseEmployeeProfileAvailability: BaseEmployeeProfileAvailability{AvailableHoursPerDay: 4}}},
		{name: "availability rejects negative compatible projects", request: CreateEmployeeProfileAvailabilityRequest{BaseEmployeeProfileAvailability: BaseEmployeeProfileAvailability{AvailableHoursPerDay: 4, CompatibleProjects: -1}}, wantErr: true},
		{name: "availability rejects counts above smallint", request: CreateEmployeeProfileAvailabilityRequest{BaseEmployeeProfileAvailability: BaseEmployeeProfileAvailability{AvailableHoursPerDay: 4, CompatibleProjects: 32768}}, wantErr: true},
		{name: "availability rejects missing required hours", request: CreateEmployeeProfileAvailabilityRequest{}, wantErr: true},
		{name: "tech accepts absent optional values", request: CreateEmployeeTechRequest{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.request)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
