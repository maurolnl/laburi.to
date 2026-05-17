package employee

import (
	"context"
	"database/sql"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

type EmployeeRepository struct {
	db *database.Queries
}

func NewRepository(db *database.Queries) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

func (r *EmployeeRepository) CreateEmployee(ctx context.Context, employee CreateEmployeeRequest) error {
	_, err := r.db.CreateEmployee(ctx, database.CreateEmployeeParams{
		Position:          employee.Position,
		Role:              employee.Role,
		YearsOfExperience: employee.YearsOfExperience,
		Certifications:    employee.Certifications,
		PortfolioUrl:      sql.NullString{String: employee.PortfolioURL},
	})

	return err
}

func (r *EmployeeRepository) GetEmployee(ctx context.Context, ID int32) (Employee, error) {
	employee, err := r.db.GetEmployee(ctx, ID)
	if err != nil {
		return Employee{}, err
	}

	return Employee{
		ID:                employee.ID,
		Position:          employee.Position,
		Role:              employee.Role,
		YearsOfExperience: employee.YearsOfExperience,
		Certifications:    employee.Certifications,
		PortfolioURL:      employee.PortfolioUrl.String,
		CreatedAt:         employee.CreatedAt,
		UpdatedAt:         employee.UpdatedAt,
	}, err
}

// func (r *EmployeeRepository) GetEmployeeById(id int) (*Employee, error) {
// 	var employee Employee
// 	err := r.db.QueryRow("SELECT id, name, email FROM employees WHERE id = ?", id).Scan(&employee.ID, &employee.Name, &employee.Email, &employee.Password)
// 	return &employee, err
// }

// func (r *EmployeeRepository) GetAllEmployees() ([]Employee, error) {
// 	var employees []Employee
// 	rows, err := r.db.Query("SELECT * FROM employees")
// 	rows.Scan(employees)

// 	return employees, err
// }
