package jobposition

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCreateJobPositionRequestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   CreateJobPositionRequest
		want CreateJobPositionRequest
	}{
		{
			name: "trims outer spaces",
			in: CreateJobPositionRequest{
				Position:               "  Backend Engineer  ",
				Role:                   "\tGo developer\n",
				RequiredExperience:     " 2_to_5y ",
				RequiredEducationLevel: " university ",
				AvailableHoursPerDay:   6,
				Timezone:               " America/Argentina/Buenos_Aires ",
				TechnicalResources:     []string{"  Laptop  ", "Monitor"},
			},
			want: CreateJobPositionRequest{
				Position:               "Backend Engineer",
				Role:                   "Go developer",
				RequiredExperience:     "2_to_5y",
				RequiredEducationLevel: "university",
				AvailableHoursPerDay:   6,
				Timezone:               "America/Argentina/Buenos_Aires",
				TechnicalResources:     []string{"Laptop", "Monitor"},
			},
		},
		{
			name: "nil technical resources becomes empty list",
			in:   CreateJobPositionRequest{TechnicalResources: nil},
			want: CreateJobPositionRequest{TechnicalResources: []string{}},
		},
		{
			name: "empty technical resources stays empty",
			in:   CreateJobPositionRequest{TechnicalResources: []string{}},
			want: CreateJobPositionRequest{TechnicalResources: []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in
			got.Normalize()
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Normalize() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCreateJobPositionRequestUnmarshalTechnicalResources(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "explicit null", body: `{"technical_resources":null}`, want: []string{}},
		{name: "omitted", body: `{}`, want: []string{}},
		{name: "empty array", body: `{"technical_resources":[]}`, want: []string{}},
		{name: "values", body: `{"technical_resources":["Laptop","VPN"]}`, want: []string{"Laptop", "VPN"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request CreateJobPositionRequest
			if err := json.Unmarshal([]byte(tt.body), &request); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			request.Normalize()
			if !reflect.DeepEqual(request.TechnicalResources, tt.want) {
				t.Fatalf("TechnicalResources = %#v, want %#v", request.TechnicalResources, tt.want)
			}
		})
	}
}

func TestJobPositionNormalizeNeverExposesNull(t *testing.T) {
	position := JobPosition{}
	position.Normalize()

	encoded, err := json.Marshal(position)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	resources, ok := decoded["technical_resources"].([]any)
	if !ok || len(resources) != 0 {
		t.Fatalf("technical_resources = %#v, want empty JSON array", decoded["technical_resources"])
	}
}
