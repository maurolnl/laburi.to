package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
)

type MiddlewareConfig struct {
	SecretKey string
	DB        employee.EmployeeRepository
}

func CreateStack(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			middleware := middlewares[i]
			next = middleware(next)
		}

		return next
	}
}

func (w *wrapperResponseWriter) writeHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.statusCode = statusCode
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		writerWithStatusCode := &wrapperResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(writerWithStatusCode, r)

		log.Println(writerWithStatusCode, r.Method, r.URL.Path, time.Since(start))
	})
}

func AuthenticatedUser(cfg MiddlewareConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accessToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				internal.RespondWithError(w, http.StatusUnauthorized, err.Error())
				return
			}

			userID, err := auth.ValidateJWT(accessToken, cfg.SecretKey)
			if err != nil {
				internal.RespondWithError(w, http.StatusUnauthorized, err.Error())
				return
			}

			ctx := context.WithValue(r.Context(), "userID", userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AuthenticatedEmployeeMiddleWare(cfg MiddlewareConfig) Middleware {
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
				internal.RespondWithError(w, http.StatusBadRequest, employee.ErrEmployeeNotFound.Error())
				return
			}

			emp, err := employee.eetEmploye(r.Context(), int32(employeeID))
			if err != nil {
				internal.RespondWithError(w, http.StatusBadRequest, employee.ErrEmployeeNotFound.Error())
				return
			}

			if emp.UserID != userID {
				internal.RespondWithError(w, http.StatusForbidden, employee.ErrEmployeeNotFound.Error())
				return
			}

			next.ServeHTTP(w, r, employeeID)
		})
	}
}

// return func(next func(w http.ResponseWriter, r *http.Request, employeeID int32)) func(http.ResponseWriter, *http.Request) {
// 	return
// }
// func(w http.ResponseWriter, r *http.Request) {
//
// 			next(w, r, int32(employeeID))
// 		}
