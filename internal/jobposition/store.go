package jobposition

import "context"

type JobPositionStore interface {
	// GetEmployerIDByUserID resuelve el empleador del principal autenticado. Devuelve
	// ErrEmployerProfileRequired cuando la cuenta todavía no creó su perfil de empleador.
	GetEmployerIDByUserID(ctx context.Context, userID int32) (int32, error)
	CreateJobPosition(ctx context.Context, employerID int32, request CreateJobPositionRequest) (JobPosition, error)
	GetActiveJobPositionByID(ctx context.Context, jobPositionID int32) (JobPosition, error)
	ListActiveJobPositionsByEmployer(ctx context.Context, employerID int32) ([]JobPosition, error)
	UpdateActiveJobPosition(ctx context.Context, jobPositionID int32, request UpdateJobPositionRequest) (JobPosition, error)
	SoftDeleteJobPosition(ctx context.Context, jobPositionID int32) error
}
