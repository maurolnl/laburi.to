package jobposition

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

type jobPositionQueries interface {
	GetEmployerByUserID(ctx context.Context, userID int32) (database.Employer, error)
	CreateJobPosition(ctx context.Context, arg database.CreateJobPositionParams) (database.JobPosition, error)
	GetActiveJobPositionByID(ctx context.Context, id int32) (database.JobPosition, error)
	ListActiveJobPositionsByEmployer(ctx context.Context, employerID int32) ([]database.JobPosition, error)
	UpdateActiveJobPosition(ctx context.Context, arg database.UpdateActiveJobPositionParams) (database.JobPosition, error)
	SoftDeleteJobPosition(ctx context.Context, id int32) (database.SoftDeleteJobPositionRow, error)
}

type JobPositionRepository struct {
	queries jobPositionQueries
	// validateTimezone se inyecta como función para que el mapeo del repositorio sea
	// testeable sin una base de datos real.
	validateTimezone func(ctx context.Context, timezone string) error
}

func NewRepository(db *sql.DB) *JobPositionRepository {
	return &JobPositionRepository{
		queries: database.New(db),
		validateTimezone: func(ctx context.Context, timezone string) error {
			return validateTimezone(ctx, db, timezone)
		},
	}
}

func (r *JobPositionRepository) GetEmployerIDByUserID(ctx context.Context, userID int32) (int32, error) {
	row, err := r.queries.GetEmployerByUserID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrEmployerProfileRequired
	}
	if err != nil {
		return 0, fmt.Errorf("get employer by user ID: %w", err)
	}

	return row.ID, nil
}

func (r *JobPositionRepository) CreateJobPosition(ctx context.Context, employerID int32, request CreateJobPositionRequest) (JobPosition, error) {
	if err := r.validateTimezone(ctx, request.Timezone); err != nil {
		return JobPosition{}, err
	}

	row, err := r.queries.CreateJobPosition(ctx, database.CreateJobPositionParams{
		EmployerID:             employerID,
		Position:               request.Position,
		Role:                   request.Role,
		RequiredExperience:     request.RequiredExperience,
		RequiredEducationLevel: request.RequiredEducationLevel,
		AvailableHoursPerDay:   request.AvailableHoursPerDay,
		Timezone:               request.Timezone,
		TechnicalResources:     request.TechnicalResources,
	})
	if err != nil {
		return JobPosition{}, fmt.Errorf("create job position: %w", err)
	}

	return jobPositionFromDatabase(row), nil
}

func (r *JobPositionRepository) GetActiveJobPositionByID(ctx context.Context, jobPositionID int32) (JobPosition, error) {
	row, err := r.queries.GetActiveJobPositionByID(ctx, jobPositionID)
	if errors.Is(err, sql.ErrNoRows) {
		return JobPosition{}, ErrJobPositionNotFound
	}
	if err != nil {
		return JobPosition{}, fmt.Errorf("get active job position: %w", err)
	}

	return jobPositionFromDatabase(row), nil
}

func (r *JobPositionRepository) ListActiveJobPositionsByEmployer(ctx context.Context, employerID int32) ([]JobPosition, error) {
	rows, err := r.queries.ListActiveJobPositionsByEmployer(ctx, employerID)
	if err != nil {
		return nil, fmt.Errorf("list active job positions: %w", err)
	}

	positions := make([]JobPosition, 0, len(rows))
	for _, row := range rows {
		positions = append(positions, jobPositionFromDatabase(row))
	}

	return positions, nil
}

func (r *JobPositionRepository) UpdateActiveJobPosition(ctx context.Context, jobPositionID int32, request UpdateJobPositionRequest) (JobPosition, error) {
	if err := r.validateTimezone(ctx, request.Timezone); err != nil {
		return JobPosition{}, err
	}

	row, err := r.queries.UpdateActiveJobPosition(ctx, database.UpdateActiveJobPositionParams{
		ID:                     jobPositionID,
		Position:               request.Position,
		Role:                   request.Role,
		RequiredExperience:     request.RequiredExperience,
		RequiredEducationLevel: request.RequiredEducationLevel,
		AvailableHoursPerDay:   request.AvailableHoursPerDay,
		Timezone:               request.Timezone,
		TechnicalResources:     request.TechnicalResources,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return JobPosition{}, ErrJobPositionNotFound
	}
	if err != nil {
		return JobPosition{}, fmt.Errorf("update active job position: %w", err)
	}

	return jobPositionFromDatabase(row), nil
}

func (r *JobPositionRepository) SoftDeleteJobPosition(ctx context.Context, jobPositionID int32) error {
	_, err := r.queries.SoftDeleteJobPosition(ctx, jobPositionID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrJobPositionNotFound
	}
	if err != nil {
		return fmt.Errorf("soft delete job position: %w", err)
	}

	return nil
}

// validateTimezone sigue el precedente de internal/employee: PostgreSQL es la única
// fuente de verdad sobre qué identificadores de zona horaria existen.
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

func jobPositionFromDatabase(row database.JobPosition) JobPosition {
	position := JobPosition{
		ID:                     row.ID,
		EmployerID:             row.EmployerID,
		Position:               row.Position,
		Role:                   row.Role,
		RequiredExperience:     row.RequiredExperience,
		RequiredEducationLevel: row.RequiredEducationLevel,
		AvailableHoursPerDay:   row.AvailableHoursPerDay,
		Timezone:               row.Timezone,
		TechnicalResources:     row.TechnicalResources,
		CreatedAt:              row.CreatedAt,
		UpdatedAt:              row.UpdatedAt,
	}
	position.Normalize()
	return position
}
