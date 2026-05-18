package employee

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

type MountEmployee struct {
	Middleware internal.AuthMiddleware
}

func BuildHandlers(psqlDB *database.Queries, validate *validator.Validate) *EmployeeHandler {
	employeeRepo := NewRepository(psqlDB)
	employeeService := NewService(employeeRepo)
	employeeHandler := NewHandler(employeeService, validate)

	return employeeHandler
}

func RegisterRoutes(mux *http.ServeMux, h *EmployeeHandler, middleware MountEmployee) {
	mux.HandleFunc("POST /employees", middleware.Middleware(h.CreateEmployee))
	mux.HandleFunc("GET /employees/{employeeID}", middleware.Middleware(h.GetEmployee))
}
