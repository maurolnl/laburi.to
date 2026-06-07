// Package employee provides HTTP handlers and business logic
// managing employee profiles
package employee

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type EmployeeHandler struct {
	service  EmployeeService
	validate *validator.Validate
}

func NewHandler(service EmployeeService, validate *validator.Validate) *EmployeeHandler {
	return &EmployeeHandler{
		service:  service,
		validate: validate,
	}
}

func BuildHandlers(store EmployeeStore, validate *validator.Validate, uploader uploader.Service) *EmployeeHandler {
	employeeService := NewService(store, uploader)
	employeeHandler := NewHandler(employeeService, validate)

	return employeeHandler
}

func RegisterRoutes(mux *http.ServeMux, h *EmployeeHandler, repo *EmployeeRepository, secretKey string) {
	authMiddleware := user.AuthenticatedUser(secretKey)
	employeeMiddlware := AuthenticatedEmployeeMiddleWare(AuthMiddlewareCfg{
		SecretKey:   secretKey,
		GetEmployee: repo.GetEmployeeByID,
	})

	mux.Handle("POST /employees", authMiddleware(http.HandlerFunc(h.CreateEmployee)))
	mux.Handle("PUT /employees/{employeeID}", employeeMiddlware(http.HandlerFunc(h.UpdateEmployee)))

	mux.Handle("POST /employees/{employeeID}/location", employeeMiddlware(http.HandlerFunc(h.CreateLocation)))
	mux.Handle("PUT /employees/{employeeID}/location", employeeMiddlware(http.HandlerFunc(h.UpdateLocation)))
	mux.Handle("POST /employees/{employeeID}/tech", employeeMiddlware(http.HandlerFunc(h.CreateTech)))
	mux.Handle("PUT /employees/{employeeID}/tech", employeeMiddlware(http.HandlerFunc(h.UpdateTech)))
	mux.Handle("POST /employees/{employeeID}/availability", employeeMiddlware(http.HandlerFunc(h.CreateAvailability)))
	mux.Handle("PUT /employees/{employeeID}/availability", employeeMiddlware(http.HandlerFunc(h.UpdateAvailability)))
	mux.Handle("POST /employees/{employeeID}/education", employeeMiddlware(http.HandlerFunc(h.CreateEducation)))
	mux.Handle("PUT /employees/{employeeID}/education", employeeMiddlware(http.HandlerFunc(h.UpdateEducation)))
	mux.Handle("GET /users/{userID}/employee", authMiddleware(http.HandlerFunc(h.GetEmployee)))
}
