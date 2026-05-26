package employee

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
)

const (
	ErrBadEducationBody = "invalid education request body"
)

type EducationDocumentUpload struct {
	Index       int
	File        multipart.File
	Filename    string
	ContentType string
	Size        int64
}

type uploadedEducationDocument struct {
	Bucket string
	Key    string
}

func (h *EmployeeHandler) CreateEducation(w http.ResponseWriter, r *http.Request, employeeID int32) {
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

	createEducationRequest, err := getEducationRequestFromForm(r)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrBadEducationBody, err.Error()))
		return
	}

	if err := h.validate.Struct(createEducationRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrBadEducationBody, err.Error()))
		return
	}

	documents, err := getEducationDocumentsFromForm(r, createEducationRequest)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer closeEducationDocuments(documents)

	if err := h.service.CreateEducation(r.Context(), employeeID, createEducationRequest, documents); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	internal.RespondWithNoBody(w, http.StatusCreated)
}

func getEducationRequestFromForm(r *http.Request) (CreateEmployeeEducationRequest, error) {
	var educationRequest CreateEmployeeEducationRequest
	raw := r.FormValue("education")
	if strings.TrimSpace(raw) == "" {
		return educationRequest, ErrInvalidEmployeeRequest
	}

	if err := json.Unmarshal([]byte(raw), &educationRequest); err != nil {
		return educationRequest, err
	}

	return educationRequest, nil
}

func getEducationDocumentsFromForm(r *http.Request, educationRequest CreateEmployeeEducationRequest) ([]EducationDocumentUpload, error) {
	documents := []EducationDocumentUpload{}

	for i, education := range educationRequest.EducationTitles {
		fileKey := educationDocumentFileKey(education.Document)
		if fileKey == "" {
			continue
		}

		header := getMultipartFileHeader(r, fileKey)
		if header == nil {
			return nil, ErrInvalidFile
		}

		file, err := header.Open()
		if err != nil {
			return nil, ErrInvalidFile
		}

		if header.Size > maxUploadSize {
			file.Close()
			return nil, ErrFileTooLarge
		}

		contentType := header.Header.Get("Content-Type")
		if contentType != "application/pdf" {
			file.Close()
			return nil, ErrUnsupportedFileType
		}

		documents = append(documents, EducationDocumentUpload{
			Index:       i,
			File:        file,
			Filename:    header.Filename,
			ContentType: contentType,
			Size:        header.Size,
		})
	}

	return documents, nil
}

func educationDocumentFileKey(document *string) string {
	if document == nil {
		return ""
	}

	return strings.TrimSpace(*document)
}

func getMultipartFileHeader(r *http.Request, key string) *multipart.FileHeader {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil
	}

	headers := r.MultipartForm.File[key]

	if len(headers) > 0 {
		return headers[0]
	}

	return nil
}

func closeEducationDocuments(documents []EducationDocumentUpload) {
	for _, document := range documents {
		document.File.Close()
	}
}

func (s *employeeService) CreateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest, documents []EducationDocumentUpload) error {
	uploadedDocuments := []uploadedEducationDocument{}
	for _, document := range documents {
		out, err := s.uploader.Upload(ctx, uploader.UploadInput{
			File:        document.File,
			Filename:    document.Filename,
			ContentType: document.ContentType,
		})
		if err != nil {
			cleanupUploadedDocuments(s, uploadedDocuments)
			return err
		}

		key := aws.ToString(out.Key)
		educationRequest.EducationTitles[document.Index].Document = &key
		uploadedDocuments = append(uploadedDocuments, uploadedEducationDocument{
			Bucket: aws.ToString(out.Bucket),
			Key:    key,
		})
	}

	if err := s.repo.CreateEducation(ctx, employeeID, educationRequest); err != nil {
		cleanupUploadedDocuments(s, uploadedDocuments)
		return err
	}

	return nil
}

func cleanupUploadedDocuments(s *employeeService, documents []uploadedEducationDocument) {
	for _, document := range documents {
		go s.cleanupOrphanFile(document.Bucket, document.Key)
	}
}

func (r *EmployeeRepository) CreateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := database.New(r.db).WithTx(tx)
	for _, education := range educationRequest.EducationTitles {
		_, err := qtx.CreateEmployeeEducation(ctx, database.CreateEmployeeEducationParams{
			EmployeeID:    employeeID,
			EducationType: education.EducationType,
			Title:         education.Title,
			Status:        education.Status,
			Certification: sql.NullString{
				String: stringValue(education.Document),
				Valid:  education.Document != nil && strings.TrimSpace(*education.Document) != "",
			},
		})
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
