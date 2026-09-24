package employee

import (
	"context"
	"log"
	"mime/multipart"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
)

const orphanCleanupTimeout = 30 * time.Second

type EmployeeService interface {
	CreateEmployee(ctx context.Context, employeeReq CreateEmployeeRequest, principal auth.Principal, file multipart.File, filename, contentType string, size int64) error
	UpdateEmployee(ctx context.Context, employeeID int32, employeeReq CreateEmployeeRequest, file multipart.File, filename, contentType string, size int64) error
	GetEmployee(ctx context.Context, ID int32) (Employee, error)
	CreateLocation(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error
	UpdateLocation(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error
	CreateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error
	UpdateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error
	CreateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error
	UpdateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error
	CreateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest, documents []EducationDocumentUpload) error
	UpdateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest, documents []EducationDocumentUpload) error

	// GetEmployeeProfile, CertificateDownloadURL y EducationDocumentDownloadURL comparten la
	// misma autorización: quien no puede ver el perfil no puede obtener ninguna URL de sus
	// archivos. El principal llega entero y no como identificador suelto porque el rol decide
	// qué regla se aplica.
	GetEmployeeProfile(ctx context.Context, employeeID int32, principal auth.Principal) (EmployeeProfileResponse, error)
	CertificateDownloadURL(ctx context.Context, employeeID, fileID int32, principal auth.Principal) (DownloadURLResponse, error)
	EducationDocumentDownloadURL(ctx context.Context, employeeID, educationID int32, principal auth.Principal) (DownloadURLResponse, error)
}

type employeeService struct {
	repo      EmployeeStore
	uploader  uploader.Service
	access    RecommendationAccess
	publisher EmployeeEventPublisher
	// now existe para que los tests puedan fijar el instante de caducidad de una URL. En
	// producción es time.Now y nadie la pasa.
	now func() time.Time
}

// NewService recibe el acceso por recomendación como dependencia aparte del store: es la única
// regla de este paquete que depende de datos que este paquete no posee.
func NewService(repo EmployeeStore, uploader uploader.Service, access RecommendationAccess, publisher EmployeeEventPublisher) EmployeeService {
	return &employeeService{
		repo:      repo,
		uploader:  uploader,
		access:    access,
		publisher: publisher,
		now:       time.Now,
	}
}

// profileChanged cierra el ciclo de toda escritura de perfil ya persistida: si el perfil quedó
// completo, solicita regenerar sus recomendaciones. No devuelve nada porque ningún desenlace
// de la emisión puede invalidar el cambio que la precedió.
func (s *employeeService) profileChanged(ctx context.Context, employeeID int32) {
	publishIfComplete(ctx, s.repo, s.publisher, employeeID)
}

func (s *employeeService) cleanupOrphanFile(bucket, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), orphanCleanupTimeout)
	defer cancel()

	if err := s.uploader.Delete(ctx, bucket, key); err != nil {
		log.Printf("employee: failed to delete orphan file bucket=%s key=%s: %v", bucket, key, err)
	}
}
