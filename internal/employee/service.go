package employee

import (
	"context"
	"log"
	"mime/multipart"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
)

const orphanCleanupTimeout = 30 * time.Second

type EmployeeService interface {
	CreateEmployee(ctx context.Context, employeeReq CreateEmployeeRequest, userID int32, file multipart.File, filename, contentType string, size int64) error
	GetEmployee(ctx context.Context, ID int32) (Employee, error)
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

func (s *employeeService) CreateEmployee(ctx context.Context, employeeReq CreateEmployeeRequest, userID int32, file multipart.File, filename, contentType string, size int64) error {
	out, err := s.uploader.Upload(ctx, uploader.UploadInput{
		File:        file,
		Filename:    filename,
		ContentType: contentType,
	})
	if err != nil {
		return err
	}

	_, err = s.repo.CreateEmployee(ctx, employeeReq, userID, EmployeeFileMetadata{
		Type:             certificationFileType,
		Bucket:           aws.ToString(out.Bucket),
		ObjectKey:        aws.ToString(out.Key),
		OriginalFilename: filename,
		ContentType:      contentType,
		SizeBytes:        size,
		ChecksumSHA256:   aws.ToString(out.ChecksumSHA256),
		Status:           employeeFileStatusUploaded,
	})
	if err != nil {
		go s.cleanupOrphanFile(aws.ToString(out.Bucket), aws.ToString(out.Key))
		return err
	}
	return nil
}

func (s *employeeService) cleanupOrphanFile(bucket, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), orphanCleanupTimeout)
	defer cancel()

	if err := s.uploader.Delete(ctx, bucket, key); err != nil {
		log.Printf("employee: failed to delete orphan file bucket=%s key=%s: %v", bucket, key, err)
	}
}

func (s *employeeService) GetEmployee(ctx context.Context, ID int32) (Employee, error) {
	return s.repo.GetEmployee(ctx, ID)
}
