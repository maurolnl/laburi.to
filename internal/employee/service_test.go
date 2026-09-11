package employee

import (
	"context"
	"errors"
	"mime/multipart"
	"strings"
	"testing"
	"time"
)

type fakeFile struct {
	*strings.Reader
}

func (f *fakeFile) Close() error { return nil }

func newFakeFile(content string) *fakeFile {
	return &fakeFile{Reader: strings.NewReader(content)}
}

func newTestService(t *testing.T) (*employeeService, *fakeEmployeeStore, *fakeUploader) {
	t.Helper()
	store := &fakeEmployeeStore{}
	upl := newFakeUploader()
	return NewService(store, upl).(*employeeService), store, upl
}

func TestCreateEmployeeUploadMetadata(t *testing.T) {
	tests := []struct {
		name         string
		file         *fakeFile
		filename     string
		contentType  string
		size         int64
		expectedNil  bool
		expectedType string
	}{
		{
			name:        "without file",
			expectedNil: true,
		},
		{
			name:         "with valid PDF file",
			file:         newFakeFile("%PDF-1.4"),
			filename:     "cert.pdf",
			contentType:  "application/pdf",
			size:         8,
			expectedNil:  false,
			expectedType: certificationFileType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, store, uploader := newTestService(t)
			req := CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Dev", Role: "Backend", YearsOfExperience: Years2To5Y}}

			var file multipart.File
			if tt.file != nil {
				file = tt.file
			}

			err := service.CreateEmployee(context.Background(), req, 1, file, tt.filename, tt.contentType, tt.size)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			store.mu.Lock()
			if len(store.createEmployeeCalls) != 1 {
				t.Fatalf("expected 1 store call, got %d", len(store.createEmployeeCalls))
			}
			call := store.createEmployeeCalls[0]
			store.mu.Unlock()

			if (call.File == nil) != tt.expectedNil {
				t.Fatalf("expected file nil=%v, got %v", tt.expectedNil, call.File == nil)
			}

			if !tt.expectedNil {
				if call.File.Type != tt.expectedType {
					t.Fatalf("expected type %q, got %q", tt.expectedType, call.File.Type)
				}
				if call.File.Bucket != "test-bucket" || call.File.ObjectKey != "employees/cert.pdf" {
					t.Fatalf("unexpected bucket/key: %s/%s", call.File.Bucket, call.File.ObjectKey)
				}
				if call.File.OriginalFilename != tt.filename || call.File.ContentType != tt.contentType || call.File.SizeBytes != tt.size {
					t.Fatalf("unexpected metadata: filename=%s contentType=%s size=%d", call.File.OriginalFilename, call.File.ContentType, call.File.SizeBytes)
				}
				if call.File.ChecksumSHA256 != "deadbeef" {
					t.Fatalf("unexpected checksum %q", call.File.ChecksumSHA256)
				}
				if call.File.Status != employeeFileStatusUploaded {
					t.Fatalf("unexpected status %q", call.File.Status)
				}

				uploader.mu.Lock()
				if len(uploader.uploadCalls) != 1 {
					t.Fatalf("expected 1 upload call, got %d", len(uploader.uploadCalls))
				}
				uploader.mu.Unlock()
			}
		})
	}
}

func TestCreateEmployeeCleanupOnStoreFailure(t *testing.T) {
	service, store, upl := newTestService(t)
	store.createEmployeeErr = errors.New("store failed")

	file := newFakeFile("%PDF-1.4")
	req := CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Dev", Role: "Backend", YearsOfExperience: Years2To5Y}}

	err := service.CreateEmployee(context.Background(), req, 1, file, "cert.pdf", "application/pdf", 8)
	if err == nil {
		t.Fatal("expected error")
	}

	select {
	case call := <-upl.deleteCalls:
		if call.Bucket != "test-bucket" || call.Key != "employees/cert.pdf" {
			t.Fatalf("unexpected cleanup bucket/key: %s/%s", call.Bucket, call.Key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected asynchronous cleanup call")
	}
}

func TestUpdateEmployeeUploadMetadata(t *testing.T) {
	service, store, upl := newTestService(t)

	file := newFakeFile("%PDF-1.5")
	req := CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Senior Dev", Role: "Backend", YearsOfExperience: Years5To10Y}}

	err := service.UpdateEmployee(context.Background(), 5, req, file, "updated.pdf", "application/pdf", 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store.mu.Lock()
	if len(store.updateEmployeeCalls) != 1 {
		t.Fatalf("expected 1 store call, got %d", len(store.updateEmployeeCalls))
	}
	call := store.updateEmployeeCalls[0]
	store.mu.Unlock()

	if call.EmployeeID != 5 {
		t.Fatalf("expected employeeID 5, got %d", call.EmployeeID)
	}
	if call.File == nil {
		t.Fatal("expected file metadata")
	}
	if call.File.OriginalFilename != "updated.pdf" || call.File.ObjectKey != "employees/cert.pdf" {
		t.Fatalf("unexpected metadata: filename=%s key=%s", call.File.OriginalFilename, call.File.ObjectKey)
	}

	upl.mu.Lock()
	if len(upl.uploadCalls) != 1 {
		t.Fatalf("expected 1 upload call, got %d", len(upl.uploadCalls))
	}
	upl.mu.Unlock()
}

func TestUpdateEmployeeCleanupOnStoreFailure(t *testing.T) {
	service, store, upl := newTestService(t)
	store.updateEmployeeErr = errors.New("store failed")

	file := newFakeFile("%PDF-1.5")
	req := CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Dev", Role: "Backend", YearsOfExperience: Years2To5Y}}

	err := service.UpdateEmployee(context.Background(), 5, req, file, "cert.pdf", "application/pdf", 8)
	if err == nil {
		t.Fatal("expected error")
	}

	select {
	case call := <-upl.deleteCalls:
		if call.Bucket != "test-bucket" || call.Key != "employees/cert.pdf" {
			t.Fatalf("unexpected cleanup bucket/key: %s/%s", call.Bucket, call.Key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected asynchronous cleanup call")
	}
}
