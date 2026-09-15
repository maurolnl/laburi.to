package employer

import (
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestCreateEmployerRequestNormalizeAndValidate(t *testing.T) {
	validate := validator.New(validator.WithRequiredStructEnabled())
	tests := []struct {
		name    string
		request CreateEmployerRequest
		want    CreateEmployerRequest
		wantErr bool
	}{
		{
			name: "trims valid free-form values",
			request: CreateEmployerRequest{
				Name:             "  Acme  ",
				Industry:         "  Software  ",
				Location:         "  Remote  ",
				HiringModalities: []string{" Full time ", " Contractor "},
			},
			want: CreateEmployerRequest{
				Name:             "Acme",
				Industry:         "Software",
				Location:         "Remote",
				HiringModalities: []string{"Full time", "Contractor"},
			},
		},
		{
			name: "converts omitted modalities to empty slice",
			request: CreateEmployerRequest{
				Name:     "Acme",
				Industry: "Software",
				Location: "Remote",
			},
			want: CreateEmployerRequest{
				Name:             "Acme",
				Industry:         "Software",
				Location:         "Remote",
				HiringModalities: []string{},
			},
		},
		{
			name: "rejects whitespace-only required field",
			request: CreateEmployerRequest{
				Name:             "  ",
				Industry:         "Software",
				Location:         "Remote",
				HiringModalities: []string{},
			},
			want: CreateEmployerRequest{
				Name:             "",
				Industry:         "Software",
				Location:         "Remote",
				HiringModalities: []string{},
			},
			wantErr: true,
		},
		{
			name: "rejects whitespace-only modality",
			request: CreateEmployerRequest{
				Name:             "Acme",
				Industry:         "Software",
				Location:         "Remote",
				HiringModalities: []string{"  "},
			},
			want: CreateEmployerRequest{
				Name:             "Acme",
				Industry:         "Software",
				Location:         "Remote",
				HiringModalities: []string{""},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.request.Normalize()
			if !reflect.DeepEqual(tt.request, tt.want) {
				t.Fatalf("Normalize() = %#v, want %#v", tt.request, tt.want)
			}

			err := validate.Struct(tt.request)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEmployerNormalizeConvertsNilModalitiesToEmptySlice(t *testing.T) {
	employer := Employer{}
	employer.Normalize()

	if employer.HiringModalities == nil || len(employer.HiringModalities) != 0 {
		t.Fatalf("HiringModalities = %#v, want empty non-nil slice", employer.HiringModalities)
	}
}
