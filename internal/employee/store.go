package employee

import "context"

type EmployeeStore interface {
	CreateEmployee(ctx context.Context, employee CreateEmployeeRequest, userID int32) error
	GetEmployee(ctx context.Context, ID int32) (Employee, error)
}
