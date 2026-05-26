package employee

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/cmd/middleware"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
)

func BuildHandlers(store EmployeeStore, validate *validator.Validate, uploader uploader.Service) *EmployeeHandler {
	employeeService := NewService(store, uploader)
	employeeHandler := NewHandler(employeeService, validate)

	return employeeHandler
}

func RegisterRoutes(mux *http.ServeMux, h *EmployeeHandler) {
	mux.HandleFunc("POST /employees", middleware.AuthenticatedUser(h.CreateEmployee))
	mux.HandleFunc("POST /employees/{employeeID}/location", middleware.EmployeeMiddleware(h.CreateLocation))
	mux.HandleFunc("POST /employees/{employeeID}/tech", middleware.EmployeeMiddleware(h.CreateTech))
	mux.HandleFunc("POST /employees/{employeeID}/availability", middleware.EmployeeMiddleware(h.CreateAvailability))
	mux.HandleFunc("POST /employees/{employeeID}/education", middleware.EmployeeMiddleware(h.CreateEducation))
	mux.HandleFunc("GET /employees/{employeeID}/education", middleware.EmployeeMiddleware(h.GetEmployee))
	mux.HandleFunc("GET /timezones", middleware.Middleware(h.GetTimezones))
}
