package employer

import "context"

type EmployerStore interface {
	CreateEmployer(ctx context.Context, employer CreateEmployerRequest, userID int32) (Employer, error)
	GetEmployerByUserID(ctx context.Context, userID int32) (Employer, error)
}
