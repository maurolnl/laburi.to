package employee

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

const (
	ErrBadAvailabilityBody = "invalid availability request body"
)

func (h *EmployeeHandler) CreateAvailability(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	employeeID, err := internal.GetPathValueAsInt(r, "employeeID")
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrEmployeeNotFound.Error())
		return
	}

	createAvailabilityRequest := CreateEmployeeProfileAvailabilityRequest{}
	if err := json.NewDecoder(r.Body).Decode(&createAvailabilityRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrBadAvailabilityBody, err.Error()))
		return
	}

	if err := h.validate.Struct(createAvailabilityRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrBadAvailabilityBody, err.Error()))
		return
	}

	if err := h.service.CreateAvailability(r.Context(), employeeID, createAvailabilityRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	internal.RespondWithNoBody(w, http.StatusCreated)
}

func (s *employeeService) CreateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error {
	return s.repo.CreateAvailability(ctx, employeeID, availabilityRequest)
}

func (r *EmployeeRepository) CreateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error {
	q := database.New(r.db)
	_, err := q.CreateEmployeeProfileAvailability(ctx, database.CreateEmployeeProfileAvailabilityParams{
		EmployeeID:           employeeID,
		AvailableHoursPerDay: intToNullInt16(availabilityRequest.AvailableHoursPerDay, true),
		CompatibleProjects:   intToNullInt16(availabilityRequest.CompatibleProjects, availabilityRequest.CompatibleProjects != 0),
		IncompatibleProjects: intToNullInt16(availabilityRequest.IncompatibleProjects, availabilityRequest.IncompatibleProjects != 0),
	})

	return err
}

func intToNullInt16(value int, valid bool) sql.NullInt16 {
	if !valid {
		return sql.NullInt16{}
	}

	if value < math.MinInt16 || value > math.MaxInt16 {
		return sql.NullInt16{}
	}

	return sql.NullInt16{
		Int16: int16(value),
		Valid: true,
	}
}
