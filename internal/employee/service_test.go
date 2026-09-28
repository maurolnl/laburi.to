package employee

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
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
	service, store, upl, _ := newTestServiceWithPublisher(t)
	return service, store, upl
}

// newTestProfileService arma el servicio con los dobles que necesitan las lecturas por
// identificador de empleado: el store, el uploader que registra las firmas y el acceso por
// recomendación.
func newTestProfileService(t *testing.T) (*employeeService, *fakeEmployeeStore, *fakeUploader, *fakeRecommendationAccess) {
	t.Helper()
	store := &fakeEmployeeStore{}
	upl := newFakeUploader()
	access := &fakeRecommendationAccess{}
	return NewService(store, upl, access, &fakeEmployeePublisher{}).(*employeeService), store, upl, access
}

// newTestServiceWithPublisher arma el servicio con el doble del puerto de recomendaciones, para
// los tests que además afirman qué se notificó.
func newTestServiceWithPublisher(t *testing.T) (*employeeService, *fakeEmployeeStore, *fakeUploader, *fakeEmployeePublisher) {
	t.Helper()
	store := &fakeEmployeeStore{}
	upl := newFakeUploader()
	publisher := &fakeEmployeePublisher{}
	return NewService(store, upl, &fakeRecommendationAccess{}, publisher).(*employeeService), store, upl, publisher
}

func pdfUpload(filename string) *CertificationUpload {
	return &CertificationUpload{File: newFakeFile("%PDF-1.4"), Filename: filename, ContentType: "application/pdf", Size: 8}
}

func int32Ptr(value int32) *int32 {
	return &value
}

func expectDeletes(t *testing.T, upl *fakeUploader, keys ...string) {
	t.Helper()

	got := map[string]bool{}
	for range keys {
		select {
		case call := <-upl.deleteCalls:
			got[call.Key] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("expected asynchronous deletes for %v, got %v", keys, got)
		}
	}
	for _, key := range keys {
		if !got[key] {
			t.Fatalf("expected delete of %q, got %v", key, got)
		}
	}
}

func TestCreateEmployeeUploadsOnePDFPerCertification(t *testing.T) {
	service, store, upl := newTestService(t)
	employeePrincipal := auth.Principal{UserID: 1, Role: user.UserRoleEmployee}
	req := CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Dev", Role: "Backend", YearsOfExperience: Years2To5Y}}

	err := service.CreateEmployee(context.Background(), req, employeePrincipal, []CertificationEntry{
		{Name: "Scrum Master", Upload: pdfUpload("scrum.pdf")},
		{Name: "AWS Cloud Practitioner", Upload: pdfUpload("aws.pdf")},
		{Name: "ITIL"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.createEmployeeCalls) != 1 {
		t.Fatalf("expected 1 store call, got %d", len(store.createEmployeeCalls))
	}
	files := store.createEmployeeCalls[0].Files
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %+v", files)
	}

	want := map[string]string{"Scrum Master": "scrum.pdf", "AWS Cloud Practitioner": "aws.pdf"}
	for _, file := range files {
		if want[file.CertificationName] != file.OriginalFilename {
			t.Fatalf("file %q associated to %q", file.OriginalFilename, file.CertificationName)
		}
		if file.Type != certificationFileType || file.Status != employeeFileStatusUploaded || file.ChecksumSHA256 != "deadbeef" {
			t.Fatalf("unexpected metadata: %+v", file)
		}
		if file.Bucket != "test-bucket" || file.ObjectKey != "employees/"+file.OriginalFilename || file.SizeBytes != 8 {
			t.Fatalf("unexpected location: %+v", file)
		}
	}

	upl.mu.Lock()
	defer upl.mu.Unlock()
	if len(upl.uploadCalls) != 2 {
		t.Fatalf("expected 2 upload calls, got %d", len(upl.uploadCalls))
	}
}

func TestCreateEmployeeRejectsKeptDocument(t *testing.T) {
	service, store, upl := newTestService(t)
	employeePrincipal := auth.Principal{UserID: 1, Role: user.UserRoleEmployee}
	req := CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Dev", Role: "Backend", YearsOfExperience: Years2To5Y}}

	err := service.CreateEmployee(context.Background(), req, employeePrincipal, []CertificationEntry{
		{Name: "Scrum Master", Upload: pdfUpload("scrum.pdf")},
		{Name: "ITIL", KeepFileID: int32Ptr(9)},
	})
	if !errors.Is(err, ErrInvalidCertifications) {
		t.Fatalf("expected ErrInvalidCertifications, got %v", err)
	}
	if len(store.createEmployeeCalls) != 0 || len(upl.uploadCalls) != 0 {
		t.Fatalf("expected no store or upload calls, got %d/%d", len(store.createEmployeeCalls), len(upl.uploadCalls))
	}
}

func TestCreateEmployeeEmployerRejectedBeforeUpload(t *testing.T) {
	service, store, upl := newTestService(t)
	employerPrincipal := auth.Principal{UserID: 1, Role: user.UserRoleEmployer}
	req := CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Dev", Role: "Backend", YearsOfExperience: Years2To5Y}}

	err := service.CreateEmployee(context.Background(), req, employerPrincipal, []CertificationEntry{{Name: "AWS", Upload: pdfUpload("cert.pdf")}})
	if !errors.Is(err, user.ErrProfileRoleForbidden) {
		t.Fatalf("expected ErrProfileRoleForbidden, got %v", err)
	}

	store.mu.Lock()
	storeCalls := len(store.createEmployeeCalls)
	store.mu.Unlock()
	if storeCalls != 0 {
		t.Fatalf("expected no store call, got %d", storeCalls)
	}

	upl.mu.Lock()
	uploadCalls := len(upl.uploadCalls)
	upl.mu.Unlock()
	if uploadCalls != 0 {
		t.Fatalf("expected no upload call, got %d", uploadCalls)
	}
}

func TestCreateEmployeeCleanupOnStoreFailure(t *testing.T) {
	service, store, upl := newTestService(t)
	store.createEmployeeErr = errors.New("store failed")
	employeePrincipal := auth.Principal{UserID: 1, Role: user.UserRoleEmployee}
	req := CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Dev", Role: "Backend", YearsOfExperience: Years2To5Y}}

	err := service.CreateEmployee(context.Background(), req, employeePrincipal, []CertificationEntry{
		{Name: "Scrum Master", Upload: pdfUpload("scrum.pdf")},
		{Name: "AWS", Upload: pdfUpload("aws.pdf")},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	expectDeletes(t, upl, "employees/scrum.pdf", "employees/aws.pdf")
}

func TestUpdateEmployeeKeepsReplacesAndRemovesDocuments(t *testing.T) {
	service, store, upl := newTestService(t)
	store.updateEmployeeRemoved = []RemovedFile{
		{Bucket: "test-bucket", ObjectKey: "certifications/aws-viejo.pdf"},
		{Bucket: "test-bucket", ObjectKey: "certifications/k8s.pdf"},
	}
	req := CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Senior Dev", Role: "Backend", YearsOfExperience: Years5To10Y}}

	err := service.UpdateEmployee(context.Background(), 5, req, []CertificationEntry{
		{Name: "Scrum Master", KeepFileID: int32Ptr(31)},
		{Name: "AWS Cloud Practitioner", Upload: pdfUpload("aws-nuevo.pdf")},
		{Name: "Kubernetes"},
	})
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
	if len(call.Kept) != 1 || call.Kept[0] != (KeptCertificationFile{FileID: 31, Name: "Scrum Master"}) {
		t.Fatalf("unexpected kept files: %+v", call.Kept)
	}
	if len(call.Files) != 1 || call.Files[0].CertificationName != "AWS Cloud Practitioner" || call.Files[0].OriginalFilename != "aws-nuevo.pdf" {
		t.Fatalf("unexpected new files: %+v", call.Files)
	}

	// Lo que la base dio de baja se borra del almacenamiento después del commit.
	expectDeletes(t, upl, "certifications/aws-viejo.pdf", "certifications/k8s.pdf")
}

func TestUpdateEmployeeCleanupOnStoreFailure(t *testing.T) {
	service, store, upl := newTestService(t)
	store.updateEmployeeErr = invalidCertifications("certification document not found")
	store.updateEmployeeRemoved = []RemovedFile{{Bucket: "test-bucket", ObjectKey: "certifications/no-borrar.pdf"}}
	req := CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{Position: "Dev", Role: "Backend", YearsOfExperience: Years2To5Y}}

	err := service.UpdateEmployee(context.Background(), 5, req, []CertificationEntry{
		{Name: "AWS", Upload: pdfUpload("cert.pdf")},
		{Name: "ITIL", KeepFileID: int32Ptr(999)},
	})
	if !errors.Is(err, ErrInvalidCertifications) {
		t.Fatalf("expected ErrInvalidCertifications, got %v", err)
	}

	// Solo se limpia lo recién subido: una transacción revertida no dio de baja nada.
	expectDeletes(t, upl, "employees/cert.pdf")
	select {
	case call := <-upl.deleteCalls:
		t.Fatalf("unexpected delete %+v", call)
	case <-time.After(100 * time.Millisecond):
	}
}
