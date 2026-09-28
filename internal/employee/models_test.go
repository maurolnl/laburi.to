package employee

import (
	"database/sql"
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
		{name: "availability accepts omitted counts", request: CreateEmployeeProfileAvailabilityRequest{BaseEmployeeProfileAvailability: BaseEmployeeProfileAvailability{AvailableHoursPerDay: 4}}},
		{name: "availability accepts explicit zero counts", request: CreateEmployeeProfileAvailabilityRequest{BaseEmployeeProfileAvailability: BaseEmployeeProfileAvailability{AvailableHoursPerDay: 4, CompatibleProjects: intPtr(0), IncompatibleProjects: intPtr(0)}}},
		{name: "availability rejects negative compatible projects", request: CreateEmployeeProfileAvailabilityRequest{BaseEmployeeProfileAvailability: BaseEmployeeProfileAvailability{AvailableHoursPerDay: 4, CompatibleProjects: intPtr(-1)}}, wantErr: true},
		{name: "availability rejects counts above smallint", request: CreateEmployeeProfileAvailabilityRequest{BaseEmployeeProfileAvailability: BaseEmployeeProfileAvailability{AvailableHoursPerDay: 4, CompatibleProjects: intPtr(32768)}}, wantErr: true},
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

func TestIntToNullInt16(t *testing.T) {
	tests := []struct {
		name  string
		value *int
		want  sql.NullInt16
	}{
		{name: "omitted is null", value: nil, want: sql.NullInt16{}},
		{name: "explicit zero is kept", value: intPtr(0), want: sql.NullInt16{Int16: 0, Valid: true}},
		{name: "positive is kept", value: intPtr(3), want: sql.NullInt16{Int16: 3, Valid: true}},
		{name: "out of range is null", value: intPtr(32768), want: sql.NullInt16{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := intToNullInt16(tt.value); got != tt.want {
				t.Fatalf("intToNullInt16() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func intPtr(value int) *int {
	return &value
}
