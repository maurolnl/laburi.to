package employee

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

const (
	ErrBadLocationBody = "invalid location request body"
)

func (s *EmployeeHandler) CreateLocation(w http.ResponseWriter, r *http.Request, employeeID int32) {
	defer r.Body.Close()

	createEmployeeLocationRequest := CreateEmployeeLocationRequest{}
	err := json.NewDecoder(r.Body).Decode(&createEmployeeLocationRequest)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrBadLocationBody, err.Error()))
		return
	}

	if err := s.validate.Struct(createEmployeeLocationRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("%s: %s", ErrBadLocationBody, err.Error()))
		return
	}

	if err := s.service.CreateLocation(r.Context(), employeeID, createEmployeeLocationRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	internal.RespondWithNoBody(w, http.StatusCreated)
}

func (s *employeeService) CreateLocation(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error {
	return s.repo.CreateLocationWithConnections(ctx, employeeID, locationRequest)
}
