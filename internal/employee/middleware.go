package employee

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/maurolnl/bolsa-de-trabajo-back/cmd/middleware"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
)

type AuthMiddlewareCfg struct {
	SecretKey   string
	GetEmployee func(ctx context.Context, employeeID int32) (Employee, error)
}

type employeeIDContextKey int

const employeeIDKey employeeIDContextKey = iota

func AuthenticatedEmployeeMiddleWare(cfg AuthMiddlewareCfg) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accessToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				internal.RespondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Error reading access token: %v", err))
				return
			}

			userID, err := auth.ValidateJWT(accessToken, cfg.SecretKey)
			if err != nil {
				internal.RespondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Error validating access token: %v", err))
				return
			}

			employeeIDParam := r.PathValue("employeeID")
			employeeID, err := strconv.ParseInt(employeeIDParam, 10, 32)
			if err != nil {
				internal.RespondWithError(w, http.StatusBadRequest, ErrEmployeeNotFound.Error())
				return
			}

			emp, err := cfg.GetEmployee(r.Context(), int32(employeeID))
			if err != nil {
				internal.RespondWithError(w, http.StatusBadRequest, ErrEmployeeNotFound.Error())
				return
			}

			if emp.UserID != userID {
				internal.RespondWithError(w, http.StatusForbidden, ErrEmployeeNotFound.Error())
				return
			}

			ctx := context.WithValue(r.Context(), employeeIDKey, employeeID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
