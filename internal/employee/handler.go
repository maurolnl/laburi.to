package employee

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

type EmployeeHandler struct {
	service  EmployeeService
	validate *validator.Validate
}

func NewHandler(service EmployeeService, validate *validator.Validate) *EmployeeHandler {
	return &EmployeeHandler{
		service:  service,
		validate: validate,
	}
}

func (h *EmployeeHandler) CreateEmployee(w http.ResponseWriter, r *http.Request, userID int32) {
	defer r.Body.Close()

	employeeRequest := CreateEmployeeRequest{}
	err := json.NewDecoder(r.Body).Decode(&employeeRequest)

	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf(ErrInvalidEmployeeRequest.Error(), err.Error()))
		return
	}

	if err := h.validate.Struct(employeeRequest); err != nil {
		internal.PrintValidatorError(w, err)
		return
	}

	err = h.service.CreateEmployee(r.Context(), employeeRequest)
	if err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, ErrInternalErrorCreatingEmployee.Error())
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

	employeeResponse := GetEmployeeResponse{
		ID:                 employee.ID,
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
