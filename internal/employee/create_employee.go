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
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/files"
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

	pdf, err := files.GetPDF(r, "certifications_file")
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	if pdf != nil {
		defer pdf.File.Close()
	}

	var filename, fileContentType string
	var fileSize int64
	if pdf != nil {
		filename = pdf.Filename
		fileContentType = pdf.ContentType
		fileSize = pdf.Size
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
		pdf.File,
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
