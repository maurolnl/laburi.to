package files

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
)

func multipartRequest(t *testing.T, files []struct {
	filename    string
	contentType string
	content     string
}) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, input := range files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="certifications_file"; filename="`+input.filename+`"`)
		header.Set("Content-Type", input.contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(input.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/employees", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestGetPDF(t *testing.T) {
	tests := []struct {
		name        string
		files       []struct{ filename, contentType, content string }
		maxSize     int64
		expectedErr error
		expectedNil bool
	}{
		{name: "missing file", expectedNil: true},
		{name: "valid PDF", files: []struct{ filename, contentType, content string }{{"certificate.pdf", "application/pdf", "%PDF-1.4"}}},
		{name: "spoofed PDF content", files: []struct{ filename, contentType, content string }{{"certificate.pdf", "application/pdf", "plain text"}}, expectedErr: ErrUnsupportedFileType},
		{name: "unsupported type", files: []struct{ filename, contentType, content string }{{"certificate.txt", "text/plain", "text"}}, expectedErr: ErrUnsupportedFileType},
		{name: "too large", files: []struct{ filename, contentType, content string }{{"certificate.pdf", "application/pdf", strings.Repeat("x", 11)}}, maxSize: 10, expectedErr: ErrFileTooLarge},
		{name: "multiple files", files: []struct{ filename, contentType, content string }{{"one.pdf", "application/pdf", "%PDF"}, {"two.pdf", "application/pdf", "%PDF"}}, expectedErr: ErrInvalidFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := multipartRequest(t, tt.files)
			if err := req.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}

			var (
				pdf *PDFFile
				err error
			)
			if tt.maxSize > 0 {
				pdf, err = GetPDF(req, "certifications_file", tt.maxSize)
			} else {
				pdf, err = GetPDF(req, "certifications_file")
			}
			if !errors.Is(err, tt.expectedErr) {
				t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
			}
			if tt.expectedNil && pdf != nil {
				t.Fatal("expected no file")
			}
			if tt.expectedErr == nil && !tt.expectedNil {
				if pdf == nil {
					t.Fatal("expected a PDF")
				}
				defer pdf.File.Close()
				if pdf.Filename != tt.files[0].filename || pdf.ContentType != "application/pdf" {
					t.Fatalf("unexpected PDF metadata: %#v", pdf)
				}
			}
		})
	}
}
