package employee

import (
	"context"
	"fmt"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

func (h *EmployeeHandler) GetEmployee(w http.ResponseWriter, r *http.Request, employeeID int32) {
	defer r.Body.Close()

	employee, err := h.service.GetEmployee(r.Context(), employeeID)
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

func (s *employeeService) GetEmployee(ctx context.Context, ID int32) (Employee, error) {
	return s.repo.GetEmployee(ctx, ID)
}
