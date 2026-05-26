package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/employee"
)

type employeeGetter interface {
	GetEmployee(ctx context.Context, ID int32) (employee.Employee, error)
}

func (app *application) authenticatedUserMiddleWare(next func(w http.ResponseWriter, r *http.Request, userID int32)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		accessToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			internal.RespondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Error reading access token: %v", err))
			return
		}

		userID, err := auth.ValidateJWT(accessToken, app.config.secretKey)
		if err != nil {
			internal.RespondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Error validating access token: %v", err))
			return
		}

		next(w, r, userID)
	}
}

func (app *application) authenticatedEmployeeMiddleWare(store employeeGetter) internal.AuthMiddleware {
	return func(next func(w http.ResponseWriter, r *http.Request, employeeID int32)) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			accessToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				internal.RespondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Error reading access token: %v", err))
				return
			}

			userID, err := auth.ValidateJWT(accessToken, app.config.secretKey)
			if err != nil {
				internal.RespondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Error validating access token: %v", err))
				return
			}

			employeeIDParam := r.PathValue("employeeID")
			employeeID, err := strconv.ParseInt(employeeIDParam, 10, 32)
			if err != nil {
				internal.RespondWithError(w, http.StatusBadRequest, employee.ErrEmployeeNotFound.Error())
				return
			}

			emp, err := store.GetEmployee(r.Context(), int32(employeeID))
			if err != nil {
				internal.RespondWithError(w, http.StatusBadRequest, employee.ErrEmployeeNotFound.Error())
				return
			}

			if emp.UserID != userID {
				internal.RespondWithError(w, http.StatusForbidden, employee.ErrEmployeeNotFound.Error())
				return
			}

			next(w, r, int32(employeeID))
		}
	}
}
