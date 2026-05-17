package employee

import "context"

type EmployeeStore interface {
	CreateEmployee(ctx context.Context, employee CreateEmployeeRequest) error
	GetEmployee(ctx context.Context, ID int32) (Employee, error)
}
