package employee

import (
	"encoding/json"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

type EmployeeHandler struct {
	service  EmployeeService
	validate *validator.Validate
}

const maxUploadSize = 5 << 20 // 5 MB

func NewHandler(service EmployeeService, validate *validator.Validate) *EmployeeHandler {
	return &EmployeeHandler{
		service:  service,
		validate: validate,
	}
}

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

	employeeRequest := CreateEmployeeRequest{
		Position:          r.FormValue("position"),
		Role:              r.FormValue("role"),
		YearsOfExperience: YearsOfExperience(r.FormValue("years_of_experience")),
		Certifications:    getCertificationsFromForm(r),
		PortfolioURL:      r.FormValue("portfolio_url"),
	}

	if err := h.validate.Struct(employeeRequest); err != nil {
		internal.PrintValidatorError(w, err)
		return
	}

	file, err := getFileFromBody(r)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	if file != nil {
		//here we should upload it to s3 and get the URL
		fmt.Printf("Got file: %v", file)
		file.Close()
	}

	err = h.service.CreateEmployee(r.Context(), employeeRequest, userID)
	if err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("%s: %s", ErrInternalErrorCreatingEmployee.Error(), err.Error()))
		return
	}

	internal.RespondWithNoBody(w, http.StatusCreated)
}

func (h *EmployeeHandler) GetEmployee(w http.ResponseWriter, r *http.Request, userID int32) {
	defer r.Body.Close()

	employeeIDPV := r.PathValue("employeeID")
	if employeeIDPV == "" {
		internal.RespondWithError(w, http.StatusBadRequest, ErrInvalidEmployeeRequest.Error())
		return
	}

	employeeID, err := strconv.ParseInt(employeeIDPV, 10, 32)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrInvalidEmployeeRequest.Error())
		return
	}

	employee, err := h.service.GetEmployee(r.Context(), int32(employeeID))
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrEmployeeNotFound, err.Error()))
		return
	}

	employeeResponse := GetEmployeeResponse{
		ID:                 employee.ID,
		Email:              employee.Email,
		Position:           employee.Position,
		Role:               employee.Role,
		YearsOfExperience:  employee.YearsOfExperience,
		Certifications:     employee.Certifications,
		CertificationsFile: employee.CertificationsFile,
		PortfolioURL:       employee.PortfolioURL,
		CreatedAt:          employee.CreatedAt,
		UpdatedAt:          employee.UpdatedAt,
	}

	internal.RespondWithJson(w, http.StatusOK, employeeResponse)
}

var (
	ErrInvalidFile         = fmt.Errorf("invalid file")
	ErrFileTooLarge        = fmt.Errorf("file too large")
	ErrUnsupportedFileType = fmt.Errorf("unsupported file type")
)

func getFileFromBody(r *http.Request) (multipart.File, error) {
	file, header, err := r.FormFile("certifications_file")
	if err != nil && err != http.ErrMissingFile {
		return nil, ErrInvalidFile
	}

	if file != nil {
		if header.Size > maxUploadSize {
			file.Close()
			return nil, ErrFileTooLarge
		}

		switch header.Header.Get("Content-Type") {
		case "application/pdf":
			return file, nil
		default:
			file.Close()
			return nil, ErrUnsupportedFileType
		}
	}

	return nil, nil
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

func getPortfolioURLFromForm(r *http.Request) string {
	if r.MultipartForm == nil {
		return ""
	}

	if values := r.MultipartForm.Value["portfolio_url"]; len(values) > 0 {
		return values[0]
	}
	return ""
}
