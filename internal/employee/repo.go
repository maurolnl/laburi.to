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

func (r *EmployeeRepository) CreateEmployee(ctx context.Context, employee CreateEmployeeRequest, userID int32, file EmployeeFileMetadata) (int32, error) {
	employeeID, err := r.db.CreateEmployee(ctx, database.CreateEmployeeParams{
		Position:          employee.Position,
		Role:              employee.Role,
		YearsOfExperience: string(employee.YearsOfExperience),
		Certifications:    employee.Certifications,
		PortfolioUrl: sql.NullString{
			String: employee.PortfolioURL,
			Valid:  employee.PortfolioURL != "",
		},
		UserID:           userID,
		Type:             file.Type,
		Bucket:           file.Bucket,
		ObjectKey:        file.ObjectKey,
		OriginalFilename: file.OriginalFilename,
		ContentType:      file.ContentType,
		SizeBytes:        file.SizeBytes,
		ChecksumSha256: sql.NullString{
			String: file.ChecksumSHA256,
			Valid:  file.ChecksumSHA256 != "",
		},
		Status: file.Status,
	})
	if err != nil {
		return 0, err
	}

	return employeeID, nil
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
