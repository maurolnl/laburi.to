package employee

import (
	"context"
	"database/sql"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

type EmployeeRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

func (r *EmployeeRepository) CreateEmployee(ctx context.Context, employee CreateEmployeeRequest, userID int32, file EmployeeFileMetadata) (int32, error) {
	q := database.New(r.db)
	employeeID, err := q.CreateEmployee(ctx, database.CreateEmployeeParams{
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
	q := database.New(r.db)
	employee, err := q.GetEmployee(ctx, ID)
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

func (r *EmployeeRepository) CreateLocationWithConnections(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := database.New(r.db).WithTx(tx)
	if err := validateTimezone(ctx, tx, locationRequest.Timezone); err != nil {
		return err
	}

	for _, conn := range locationRequest.InternetConnections {
		if _, err := qtx.CreateEmployeeConnection(ctx, database.CreateEmployeeConnectionParams{
			EmployeeID: employeeID,
			Type:       conn.Type,
			Speed:      conn.Speed,
		}); err != nil {
			return err
		}
	}

	if _, err := qtx.CreateEmployeeLocation(ctx, database.CreateEmployeeLocationParams{
		EmployeeID: employeeID,
		Timezone:   locationRequest.Timezone,
	}); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *EmployeeRepository) GetTimezones(ctx context.Context) ([]Timezone, error) {
	return getTimezones(ctx, r.db)
}

func validateTimezone(ctx context.Context, db database.DBTX, timezone string) error {
	const query = `
		SELECT EXISTS(
			SELECT 1
			FROM pg_timezone_names
			WHERE name = $1
		)
	`

	var exists bool
	if err := db.QueryRowContext(ctx, query, timezone).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrInvalidTimezone
	}

	return nil
}

func getTimezones(ctx context.Context, db database.DBTX) ([]Timezone, error) {
	const query = `
		SELECT name, abbrev, utc_offset::text, is_dst
		FROM pg_timezone_names
		ORDER BY name
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	timezones := []Timezone{}
	for rows.Next() {
		var timezone Timezone
		if err := rows.Scan(&timezone.Name, &timezone.Abbrev, &timezone.UTCOffset, &timezone.IsDST); err != nil {
			return nil, err
		}
		timezones = append(timezones, timezone)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return timezones, nil
}
