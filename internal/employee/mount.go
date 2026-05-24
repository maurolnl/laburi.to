package employee

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
)

type MountEmployee struct {
	Middleware internal.AuthMiddleware
}

func BuildHandlers(psqlDB *database.Queries, validate *validator.Validate, uploader uploader.Service) *EmployeeHandler {
	employeeRepo := NewRepository(psqlDB)
	employeeService := NewService(employeeRepo, uploader)
	employeeHandler := NewHandler(employeeService, validate)

	return employeeHandler
}

func RegisterRoutes(mux *http.ServeMux, h *EmployeeHandler, middleware MountEmployee) {
	mux.HandleFunc("POST /employees", middleware.Middleware(h.CreateEmployee))
	mux.HandleFunc("PUT /employees/{employeeID}", middleware.Middleware(h.UpdateEmployee))
	mux.HandleFunc("GET /employees/{employeeID}", middleware.Middleware(h.GetEmployee))
}
