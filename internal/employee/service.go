package employee

import (
	"context"
	"log"
	"mime/multipart"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
)

const orphanCleanupTimeout = 30 * time.Second

type EmployeeService interface {
	CreateEmployee(ctx context.Context, employeeReq CreateEmployeeRequest, userID int32, file multipart.File, filename, contentType string, size int64) error
	GetEmployee(ctx context.Context, ID int32) (Employee, error)
	CreateLocation(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error
	CreateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error
	CreateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error
	CreateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest, documents []EducationDocumentUpload) error
}

type employeeService struct {
	repo     EmployeeStore
	uploader uploader.Service
}

func NewService(repo EmployeeStore, uploader uploader.Service) EmployeeService {
	return &employeeService{
		repo:     repo,
		uploader: uploader,
	}
}

func (s *employeeService) cleanupOrphanFile(bucket, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), orphanCleanupTimeout)
	defer cancel()

	if err := s.uploader.Delete(ctx, bucket, key); err != nil {
		log.Printf("employee: failed to delete orphan file bucket=%s key=%s: %v", bucket, key, err)
	}
}
