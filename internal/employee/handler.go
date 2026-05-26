// Package employee provides HTTP handlers and business logic
// managing employee profiles
package employee

import (
	"github.com/go-playground/validator/v10"
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
