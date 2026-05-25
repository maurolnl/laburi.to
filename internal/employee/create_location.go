package employee

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

const (
	ErrBadLocationBody = "invalid location request body"
)

func getPathValue(r *http.Request, key string) (int32, error) {
	employeeIDPV := r.PathValue(key)
	employeeID, err := strconv.ParseInt(employeeIDPV, 10, 32)

	return int32(employeeID), err
}

func (s *EmployeeHandler) CreateLocation(w http.ResponseWriter, r *http.Request, _ int32) {
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

	employeeID, err := getPathValue(r, "employeeID")
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrEmployeeNotFound.Error())
		return
	}

	_, err = s.service.GetEmployee(r.Context(), employeeID)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrEmployeeNotFound.Error())
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
