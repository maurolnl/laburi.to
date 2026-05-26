package employee

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
)

func (h *EmployeeHandler) CreateEmployee(w http.ResponseWriter, r *http.Request, userID int32) {
	defer r.Body.Close()

	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		internal.RespondWithError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := getFileFromBody(r)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	if file != nil {
		defer file.Close()
	}

	var filename, fileContentType string
	var fileSize int64
	if header != nil {
		filename = header.Filename
		fileContentType = header.Header.Get("Content-Type")
		fileSize = header.Size
	}

	employeeRequest := CreateEmployeeRequest{
		BaseEmployeeRequest: BaseEmployeeRequest{
			Position:          r.FormValue("position"),
			Role:              r.FormValue("role"),
			YearsOfExperience: YearsOfExperience(r.FormValue("years_of_experience")),
			Certifications:    getCertificationsFromForm(r),
			PortfolioURL:      r.FormValue("portfolio_url"),
		},
	}

	if err := h.validate.Struct(employeeRequest); err != nil {
		internal.PrintValidatorError(w, err)
		return
	}

	err = h.service.CreateEmployee(
		r.Context(),
		employeeRequest,
		userID,
		file,
		filename,
		fileContentType,
		fileSize,
	)
	if err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("%s: %s", ErrInternalErrorCreatingEmployee.Error(), err.Error()))
		return
	}

	internal.RespondWithNoBody(w, http.StatusCreated)
}

var (
	ErrInvalidFile         = fmt.Errorf("invalid file")
	ErrFileTooLarge        = fmt.Errorf("file too large")
	ErrUnsupportedFileType = fmt.Errorf("unsupported file type")
)

func getFileFromBody(r *http.Request) (multipart.File, *multipart.FileHeader, error) {
	file, header, err := r.FormFile("certifications_file")
	if err != nil && err != http.ErrMissingFile {
		return nil, nil, ErrInvalidFile
	}

	if file != nil {
		if header.Size > maxUploadSize {
			file.Close()
			return nil, nil, ErrFileTooLarge
		}

		contentType := header.Header.Get("Content-Type")

		switch contentType {
		case "application/pdf":
			return file, header, nil
		default:
			file.Close()
			return nil, nil, ErrUnsupportedFileType
		}
	}

	return nil, nil, nil
}

func getCertificationsFromForm(r *http.Request) []string {
	certifications := r.MultipartForm.Value["certifications"]
	if len(certifications) == 0 {
		certifications = r.MultipartForm.Value["certifications[]"]
	}

	if len(certifications) == 1 && strings.TrimSpace(certifications[0]) == "null" {
		return nil
	}

	if len(certifications) == 1 && strings.HasPrefix(strings.TrimSpace(certifications[0]), "[") {
		var parsed []string
		if err := json.Unmarshal([]byte(certifications[0]), &parsed); err == nil {
			return parsed
		}
	}

	return certifications
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
