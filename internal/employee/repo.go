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

func (r *EmployeeRepository) CreateEmployee(ctx context.Context, employee CreateEmployeeRequest, userID int32) error {
	_, err := r.db.CreateEmployee(ctx, database.CreateEmployeeParams{
		Position:          employee.Position,
		Role:              employee.Role,
		YearsOfExperience: string(employee.YearsOfExperience),
		Certifications:    employee.Certifications,
		PortfolioUrl:      sql.NullString{String: employee.PortfolioURL},
		UserID:            userID,
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
		Email:             employee.Email,
		Position:          employee.Position,
		Role:              employee.Role,
		YearsOfExperience: employee.YearsOfExperience,
		Certifications:    employee.Certifications,
		PortfolioURL:      employee.PortfolioUrl.String,
		CreatedAt:         employee.CreatedAt,
		UpdatedAt:         employee.UpdatedAt,
	}, err
}
