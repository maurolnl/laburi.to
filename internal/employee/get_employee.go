package employee

import (
	"context"
	"fmt"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

func (h *EmployeeHandler) GetEmployee(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	authUserID, ok := user.UserIDFromContext(r.Context())
	if !ok {
		internal.RespondWithError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	pathUserID, err := internal.GetPathValueAsInt(r, "userID")
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrEmployeeNotFound.Error())
		return
	}

	if authUserID != pathUserID {
		internal.RespondWithError(w, http.StatusForbidden, "forbidden")
		return
	}

	employee, err := h.service.GetEmployee(r.Context(), pathUserID)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrEmployeeNotFound, err.Error()))
		return
	}

	employeeResponse := GetEmployeeResponse{
		ID:                   employee.ID,
		UserID:               employee.UserID,
		Email:                employee.Email,
		Position:             employee.Position,
		Role:                 employee.Role,
		YearsOfExperience:    employee.YearsOfExperience,
		Certifications:       employee.Certifications,
		PortfolioURL:         employee.PortfolioURL,
		Timezone:             employee.Timezone,
		Os:                   employee.Os,
		PaidSoftware:         employee.PaidSoftware,
		AvailableHoursPerDay: employee.AvailableHoursPerDay,
		CompatibleProjects:   employee.CompatibleProjects,
		IncompatibleProjects: employee.IncompatibleProjects,
		InternetConnections:  employee.InternetConnections,
		Education:            employee.Education,
		Files:                employee.Files,
		CreatedAt:            employee.CreatedAt,
		UpdatedAt:            employee.UpdatedAt,
	}

	internal.RespondWithJSON(w, http.StatusOK, employeeResponse)
}

func (s *employeeService) GetEmployee(ctx context.Context, ID int32) (Employee, error) {
	return s.repo.GetEmployee(ctx, ID)
}
