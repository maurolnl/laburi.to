package jobposition

import (
	"context"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type JobPositionService interface {
	CreateJobPosition(ctx context.Context, employerID int32, request CreateJobPositionRequest, principal auth.Principal) (JobPosition, error)
	ListJobPositions(ctx context.Context, employerID int32, principal auth.Principal) ([]JobPosition, error)
	GetJobPosition(ctx context.Context, jobPositionID int32, principal auth.Principal) (JobPosition, error)
	UpdateJobPosition(ctx context.Context, jobPositionID int32, request UpdateJobPositionRequest, principal auth.Principal) (JobPosition, error)
	DeleteJobPosition(ctx context.Context, jobPositionID int32, principal auth.Principal) error
}

type jobPositionService struct {
	store     JobPositionStore
	publisher JobPositionEventPublisher
}

func NewService(store JobPositionStore, publisher JobPositionEventPublisher) JobPositionService {
	return &jobPositionService{store: store, publisher: publisher}
}

// resolveEmployerID deriva la identidad del empleador exclusivamente del principal
// autenticado. Ningún identificador enviado por el cliente participa de esta resolución.
func (s *jobPositionService) resolveEmployerID(ctx context.Context, principal auth.Principal) (int32, error) {
	if err := user.AuthorizeProfileRole(principal.Role, user.UserRoleEmployer); err != nil {
		return 0, err
	}

	return s.store.GetEmployerIDByUserID(ctx, principal.UserID)
}

// resolveOwnedCollection contrasta el employerID del path contra el empleador derivado
// del JWT. El path solo sirve para detectar un acceso ajeno.
func (s *jobPositionService) resolveOwnedCollection(ctx context.Context, employerID int32, principal auth.Principal) (int32, error) {
	ownEmployerID, err := s.resolveEmployerID(ctx, principal)
	if err != nil {
		return 0, err
	}
	if ownEmployerID != employerID {
		return 0, ErrJobPositionForbidden
	}

	return ownEmployerID, nil
}

// resolveOwnedPosition carga el puesto activo y verifica que pertenezca al empleador del
// principal. Distinguir "no existe" de "es ajeno" exige las dos consultas separadas.
func (s *jobPositionService) resolveOwnedPosition(ctx context.Context, jobPositionID int32, principal auth.Principal) (JobPosition, error) {
	ownEmployerID, err := s.resolveEmployerID(ctx, principal)
	if err != nil {
		return JobPosition{}, err
	}

	position, err := s.store.GetActiveJobPositionByID(ctx, jobPositionID)
	if err != nil {
		return JobPosition{}, err
	}
	if position.EmployerID != ownEmployerID {
		return JobPosition{}, ErrJobPositionForbidden
	}

	return position, nil
}

func (s *jobPositionService) CreateJobPosition(ctx context.Context, employerID int32, request CreateJobPositionRequest, principal auth.Principal) (JobPosition, error) {
	ownEmployerID, err := s.resolveOwnedCollection(ctx, employerID, principal)
	if err != nil {
		return JobPosition{}, err
	}

	position, err := s.store.CreateJobPosition(ctx, ownEmployerID, request)
	if err != nil {
		return JobPosition{}, err
	}

	position.Normalize()
	publish(ctx, s.publisher, position.ID)

	return position, nil
}

func (s *jobPositionService) ListJobPositions(ctx context.Context, employerID int32, principal auth.Principal) ([]JobPosition, error) {
	ownEmployerID, err := s.resolveOwnedCollection(ctx, employerID, principal)
	if err != nil {
		return nil, err
	}

	positions, err := s.store.ListActiveJobPositionsByEmployer(ctx, ownEmployerID)
	if err != nil {
		return nil, err
	}
	if positions == nil {
		positions = []JobPosition{}
	}
	for i := range positions {
		positions[i].Normalize()
	}

	return positions, nil
}

func (s *jobPositionService) GetJobPosition(ctx context.Context, jobPositionID int32, principal auth.Principal) (JobPosition, error) {
	position, err := s.resolveOwnedPosition(ctx, jobPositionID, principal)
	if err != nil {
		return JobPosition{}, err
	}

	position.Normalize()
	return position, nil
}

func (s *jobPositionService) UpdateJobPosition(ctx context.Context, jobPositionID int32, request UpdateJobPositionRequest, principal auth.Principal) (JobPosition, error) {
	if _, err := s.resolveOwnedPosition(ctx, jobPositionID, principal); err != nil {
		return JobPosition{}, err
	}

	position, err := s.store.UpdateActiveJobPosition(ctx, jobPositionID, request)
	if err != nil {
		return JobPosition{}, err
	}

	position.Normalize()
	publish(ctx, s.publisher, position.ID)

	return position, nil
}

func (s *jobPositionService) DeleteJobPosition(ctx context.Context, jobPositionID int32, principal auth.Principal) error {
	if _, err := s.resolveOwnedPosition(ctx, jobPositionID, principal); err != nil {
		return err
	}

	return s.store.SoftDeleteJobPosition(ctx, jobPositionID)
}
