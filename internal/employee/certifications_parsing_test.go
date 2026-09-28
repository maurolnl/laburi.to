package employee

import (
	"errors"
	"strings"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/files"
)

func pdfPart(name, filename string) multipartFilePart {
	return multipartFilePart{name: name, filename: filename, contentType: "application/pdf", content: "%PDF-1.4 test"}
}

func parseCertificationsRequest(t *testing.T, fields map[string]string, parts []multipartFilePart) ([]CertificationEntry, error) {
	t.Helper()

	req := newEmployeeMultipartRequest(t, fields, parts)
	if err := req.ParseMultipartForm(maxUploadSize); err != nil {
		t.Fatalf("parse multipart: %v", err)
	}
	t.Cleanup(func() { _ = req.MultipartForm.RemoveAll() })

	entries, err := parseCertificationsForm(req)
	t.Cleanup(func() { closeCertificationUploads(entries) })
	return entries, err
}

func TestParseCertificationsFormAcceptsValidItems(t *testing.T) {
	entries, err := parseCertificationsRequest(t, map[string]string{
		"certifications": `[
			{"name": " Scrum Master ", "document": "certification_document_0"},
			{"name": "AWS Cloud Practitioner", "document": "certification_document_1"},
			{"name": "Kubernetes", "document_id": 31},
			{"name": "ITIL", "document": ""}
		]`,
	}, []multipartFilePart{
		pdfPart("certification_document_0", "scrum.pdf"),
		pdfPart("certification_document_1", "aws.pdf"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %+v", entries)
	}
	// El nombre se recorta: es lo que se persiste y contra lo que se asocia el PDF.
	if entries[0].Name != "Scrum Master" || entries[0].Upload == nil || entries[0].Upload.Filename != "scrum.pdf" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}
	if entries[1].Upload == nil || entries[1].Upload.Filename != "aws.pdf" || entries[1].Upload.ContentType != "application/pdf" {
		t.Fatalf("unexpected second entry: %+v", entries[1])
	}
	if entries[2].Upload != nil || entries[2].KeepFileID == nil || *entries[2].KeepFileID != 31 {
		t.Fatalf("unexpected kept entry: %+v", entries[2])
	}
	if entries[3].Upload != nil || entries[3].KeepFileID != nil {
		t.Fatalf("certification without PDF should have no document: %+v", entries[3])
	}
	if names := certificationNames(entries); strings.Join(names, "|") != "Scrum Master|AWS Cloud Practitioner|Kubernetes|ITIL" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestParseCertificationsFormAcceptsEmptyValues(t *testing.T) {
	for name, fields := range map[string]map[string]string{
		"absent":      {},
		"empty":       {"certifications": ""},
		"null":        {"certifications": "null"},
		"empty array": {"certifications": "[]"},
	} {
		t.Run(name, func(t *testing.T) {
			entries, err := parseCertificationsRequest(t, fields, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("expected no entries, got %+v", entries)
			}
		})
	}
}

func TestParseCertificationsFormRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name    string
		fields  map[string]string
		parts   []multipartFilePart
		wantErr error
	}{
		{
			name:    "legacy repeated field",
			fields:  map[string]string{"certifications[]": "aws"},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "legacy array of names",
			fields:  map[string]string{"certifications": `["aws"]`},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "legacy single file without certification",
			fields:  map[string]string{"certifications": `[{"name": "AWS"}]`},
			parts:   []multipartFilePart{pdfPart("certifications_file", "cert.pdf")},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "invalid JSON",
			fields:  map[string]string{"certifications": `[{"name": "AWS"`},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "unknown field",
			fields:  map[string]string{"certifications": `[{"name": "AWS", "file": "x"}]`},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "empty name",
			fields:  map[string]string{"certifications": `[{"name": "  "}]`},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "duplicated name ignoring case",
			fields:  map[string]string{"certifications": `[{"name": "AWS"}, {"name": " aws "}]`},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "document and document_id",
			fields:  map[string]string{"certifications": `[{"name": "AWS", "document": "certification_document_0", "document_id": 3}]`},
			parts:   []multipartFilePart{pdfPart("certification_document_0", "aws.pdf")},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "same file for two certifications",
			fields:  map[string]string{"certifications": `[{"name": "AWS", "document": "cert0"}, {"name": "GCP", "document": "cert0"}]`},
			parts:   []multipartFilePart{pdfPart("cert0", "cloud.pdf")},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "same document_id twice",
			fields:  map[string]string{"certifications": `[{"name": "AWS", "document_id": 3}, {"name": "GCP", "document_id": 3}]`},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "non positive document_id",
			fields:  map[string]string{"certifications": `[{"name": "AWS", "document_id": 0}]`},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "referenced file missing",
			fields:  map[string]string{"certifications": `[{"name": "AWS", "document": "cert0"}]`},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:    "file without certification",
			fields:  map[string]string{"certifications": `[{"name": "AWS", "document": "cert0"}]`},
			parts:   []multipartFilePart{pdfPart("cert0", "aws.pdf"), pdfPart("cert1", "extra.pdf")},
			wantErr: ErrInvalidCertifications,
		},
		{
			name:   "non PDF file",
			fields: map[string]string{"certifications": `[{"name": "AWS", "document": "cert0"}]`},
			parts: []multipartFilePart{
				{name: "cert0", filename: "cert.txt", contentType: "text/plain", content: "text"},
			},
			wantErr: files.ErrUnsupportedFileType,
		},
		{
			name:   "PDF larger than five megabytes",
			fields: map[string]string{"certifications": `[{"name": "AWS", "document": "cert0"}]`},
			parts: []multipartFilePart{
				{name: "cert0", filename: "large.pdf", contentType: "application/pdf", content: "%PDF-1.4" + strings.Repeat("x", maxUploadSize)},
			},
			wantErr: files.ErrFileTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := parseCertificationsRequest(t, tt.fields, tt.parts)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
			if entries != nil {
				t.Fatalf("expected no entries on error, got %+v", entries)
			}
		})
	}
}
