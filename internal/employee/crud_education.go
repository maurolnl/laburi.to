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
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/files"
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

func (h *EmployeeHandler) CreateEducation(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	employeeID, err := internal.GetPathValueAsInt(r, "employeeID")
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrEmployeeNotFound.Error())
		return
	}

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

	if err := h.service.CreateEducation(r.Context(), employeeID, createEducationRequest, documents); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	internal.RespondWithNoBody(w, http.StatusCreated)
}

func (h *EmployeeHandler) UpdateEducation(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	employeeID, err := internal.GetPathValueAsInt(r, "employeeID")
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrEmployeeNotFound.Error())
		return
	}

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

	updateEducationRequest, err := getEducationRequestFromForm(r)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrBadEducationBody, err.Error()))
		return
	}

	if err := h.validate.Struct(updateEducationRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrBadEducationBody, err.Error()))
		return
	}

	documents, err := getEducationDocumentsFromForm(r, updateEducationRequest)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.UpdateEducation(r.Context(), employeeID, updateEducationRequest, documents); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	internal.RespondWithNoBody(w, http.StatusOK)
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

		pdf, err := files.GetPDF(r, fileKey, maxUploadSize)
		if err != nil || pdf == nil {
			return nil, err
		}

		defer pdf.File.Close()

		documents = append(documents, EducationDocumentUpload{
			Index:       i,
			File:        pdf.File,
			Filename:    pdf.Filename,
			ContentType: pdf.ContentType,
			Size:        pdf.Size,
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

func (s *employeeService) UpdateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest, documents []EducationDocumentUpload) error {
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

	if err := s.repo.UpdateEducation(ctx, employeeID, educationRequest); err != nil {
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

func (r *EmployeeRepository) UpdateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := database.New(r.db).WithTx(tx)
	if err := qtx.DeleteEmployeeEducation(ctx, employeeID); err != nil {
		return err
	}

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
