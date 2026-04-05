package employee

import "fmt"

type EmployeeService struct {
	repo *EmployeeRepository
}

func NewEmployeeService(repo *EmployeeRepository) *EmployeeService {
	return &EmployeeService{repo: repo}
}

func (s *EmployeeService) GetEmployees() {
	fmt.Println("Getting employees...")
}
