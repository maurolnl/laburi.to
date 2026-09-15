package employer

import (
	"context"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type EmployerService interface {
	CreateEmployer(ctx context.Context, employerReq CreateEmployerRequest, principal auth.Principal) error
	GetEmployer(ctx context.Context, userID int32, principal auth.Principal) (Employer, error)
}

type employerService struct {
	store EmployerStore
}

func NewService(store EmployerStore) EmployerService {
	return &employerService{store: store}
}

func (s *employerService) CreateEmployer(ctx context.Context, employerReq CreateEmployerRequest, principal auth.Principal) error {
	if err := user.AuthorizeProfileRole(principal.Role, user.UserRoleEmployer); err != nil {
		return err
	}

	_, err := s.store.CreateEmployer(ctx, employerReq, principal.UserID)
	return err
}

func (s *employerService) GetEmployer(ctx context.Context, userID int32, principal auth.Principal) (Employer, error) {
	if err := user.AuthorizeProfileRole(principal.Role, user.UserRoleEmployer); err != nil {
		return Employer{}, err
	}

	employer, err := s.store.GetEmployerByUserID(ctx, userID)
	if err != nil {
		return Employer{}, err
	}
	employer.Normalize()
	return employer, nil
}
