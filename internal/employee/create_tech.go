package employee

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

const (
	ErrBadTechBody = "invalid tech request body"
)

func (h *EmployeeHandler) CreateTech(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	employeeID, err := internal.GetPathValueAsInt(r, "employeeID")
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrEmployeeNotFound.Error())
		return
	}

	createEmployeeTechRequest := CreateEmployeeTechRequest{}
	if err := json.NewDecoder(r.Body).Decode(&createEmployeeTechRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrBadTechBody, err.Error()))
		return
	}

	if err := h.validate.Struct(createEmployeeTechRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrBadTechBody, err.Error()))
		return
	}

	if err := h.service.CreateTech(r.Context(), employeeID, createEmployeeTechRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	internal.RespondWithNoBody(w, http.StatusCreated)
}

func (s *employeeService) CreateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error {
	if techRequest.PaidSoftware == nil {
		techRequest.PaidSoftware = []string{}
	}
	return s.repo.CreateTech(ctx, employeeID, techRequest)
}

func (r *EmployeeRepository) CreateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error {
	q := database.New(r.db)
	_, err := q.CreateEmployeeProfileTech(ctx, database.CreateEmployeeProfileTechParams{
		EmployeeID: employeeID,
		Os: sql.NullString{
			String: techRequest.Os,
			Valid:  techRequest.Os != "",
		},
		PaidSoftware: techRequest.PaidSoftware,
	})

	return err
}
