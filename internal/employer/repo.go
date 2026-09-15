package employer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

type employerQueries interface {
	CreateEmployer(ctx context.Context, arg database.CreateEmployerParams) (database.Employer, error)
	GetEmployerByUserID(ctx context.Context, userID int32) (database.Employer, error)
}

type EmployerRepository struct {
	queries employerQueries
}

func NewRepository(db *sql.DB) *EmployerRepository {
	return &EmployerRepository{queries: database.New(db)}
}

func (r *EmployerRepository) CreateEmployer(ctx context.Context, employerReq CreateEmployerRequest, userID int32) (Employer, error) {
	row, err := r.queries.CreateEmployer(ctx, database.CreateEmployerParams{
		UserID:           userID,
		Name:             employerReq.Name,
		Industry:         employerReq.Industry,
		Location:         employerReq.Location,
		HiringModalities: employerReq.HiringModalities,
	})
	if err != nil {
		return Employer{}, classifyCreateEmployerError(err)
	}

	return employerFromDatabase(row), nil
}

func (r *EmployerRepository) GetEmployerByUserID(ctx context.Context, userID int32) (Employer, error) {
	row, err := r.queries.GetEmployerByUserID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return Employer{}, ErrEmployerNotFound
	}
	if err != nil {
		return Employer{}, fmt.Errorf("get employer by user ID: %w", err)
	}

	return employerFromDatabase(row), nil
}

func classifyCreateEmployerError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch string(pqErr.Code) {
		case "23505":
			return ErrEmployerAlreadyExists
		case "23514":
			return ErrEmployerProfileConflict
		}
	}

	return fmt.Errorf("create employer: %w", err)
}

func employerFromDatabase(row database.Employer) Employer {
	employer := Employer{
		ID:               row.ID,
		UserID:           row.UserID,
		Name:             row.Name,
		Industry:         row.Industry,
		Location:         row.Location,
		HiringModalities: row.HiringModalities,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
	employer.Normalize()
	return employer
}
