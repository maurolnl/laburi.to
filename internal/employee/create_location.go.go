package employee

import (
	"encoding/json"
	"io"
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

func getParsedBody(body io.ReadCloser) error {
	return nil
}

func (s *EmployeeHandler) CreateLocation(w http.ResponseWriter, r *http.Request, userID int32) {
	defer r.Body.Close()

	createEmployeeLocationRequest := CreateEmployeeLocationRequest{}
	err := json.NewDecoder(r.Body).Decode(&CreateEmployeeLocationRequest)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrBadLocationBody+err.Error())
		return
	}

	if err := s.validate.Struct(CreateLocationRequest); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrBadLocationBody+err.Error())
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

	// Past validation
	for index, conn := range createEmployeeLocationRequest.InternetConnections {
		// create internet connnections
	}

	s.service.CreateLocation(r.Context(), employeeID)
}
