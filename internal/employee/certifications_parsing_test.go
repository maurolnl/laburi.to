package employee

import (
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetCertificationsFromForm(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected []string
	}{
		{
			name:     "null certifications",
			value:    "null",
			expected: []string{},
		},
		{
			name:     "array with one certification",
			value:    `["aws"]`,
			expected: []string{"aws"},
		},
		{
			name:     "empty array",
			value:    `[]`,
			expected: []string{},
		},
		{
			name:     "array with two certifications",
			value:    `["aws", "gcp"]`,
			expected: []string{"aws", "gcp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/employees", strings.NewReader(""))
			req.MultipartForm = &multipart.Form{
				Value: map[string][]string{
					"certifications": {tt.value},
				},
			}

			got := getCertificationsFromForm(req)

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d certifications, got %d: %#v", len(tt.expected), len(got), got)
			}

			for i := range tt.expected {
				if got[i] != tt.expected[i] {
					t.Fatalf("expected certification %q at index %d, got %q", tt.expected[i], i, got[i])
				}
			}
		})
	}
}
