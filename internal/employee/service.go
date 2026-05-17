package employee

import (
	"context"
)

type EmployeeService interface {
	CreateEmployee(ctx context.Context, employeeReq CreateEmployeeRequest) error
	GetEmployee(ctx context.Context, ID int32) (Employee, error)
}

type employeeService struct {
	repo EmployeeStore
}

func NewService(repo EmployeeStore) EmployeeService {
	return &employeeService{
		repo: repo,
	}
}

func (s *employeeService) CreateEmployee(ctx context.Context, employeeReq CreateEmployeeRequest) error {
	err := s.repo.CreateEmployee(ctx, employeeReq)

	if err != nil {
		return err
	}

	return nil
}

func (s *employeeService) GetEmployee(ctx context.Context, ID int32) (Employee, error) {
	return s.repo.GetEmployee(ctx, ID)
}
