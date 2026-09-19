package jobposition

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

var testJobPositionTime = time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)

type fakeJobPositionQueries struct {
	employerResult database.Employer
	employerErr    error
	createResult   database.JobPosition
	createErr      error
	getResult      database.JobPosition
	getErr         error
	listResult     []database.JobPosition
	listErr        error
	updateResult   database.JobPosition
	updateErr      error
	deleteResult   database.SoftDeleteJobPositionRow
	deleteErr      error

	employerUserID int32
	createParams   database.CreateJobPositionParams
	getID          int32
	listEmployerID int32
	updateParams   database.UpdateActiveJobPositionParams
	deleteID       int32
	lastContext    context.Context
}

func (f *fakeJobPositionQueries) GetEmployerByUserID(ctx context.Context, userID int32) (database.Employer, error) {
	f.lastContext = ctx
	f.employerUserID = userID
	return f.employerResult, f.employerErr
}

func (f *fakeJobPositionQueries) CreateJobPosition(ctx context.Context, arg database.CreateJobPositionParams) (database.JobPosition, error) {
	f.lastContext = ctx
	f.createParams = arg
	return f.createResult, f.createErr
}

func (f *fakeJobPositionQueries) GetActiveJobPositionByID(ctx context.Context, id int32) (database.JobPosition, error) {
	f.lastContext = ctx
	f.getID = id
	return f.getResult, f.getErr
}

func (f *fakeJobPositionQueries) ListActiveJobPositionsByEmployer(ctx context.Context, employerID int32) ([]database.JobPosition, error) {
	f.lastContext = ctx
	f.listEmployerID = employerID
	return f.listResult, f.listErr
}

func (f *fakeJobPositionQueries) UpdateActiveJobPosition(ctx context.Context, arg database.UpdateActiveJobPositionParams) (database.JobPosition, error) {
	f.lastContext = ctx
	f.updateParams = arg
	return f.updateResult, f.updateErr
}

func (f *fakeJobPositionQueries) SoftDeleteJobPosition(ctx context.Context, id int32) (database.SoftDeleteJobPositionRow, error) {
	f.lastContext = ctx
	f.deleteID = id
	return f.deleteResult, f.deleteErr
}

func newTestDatabaseJobPosition() database.JobPosition {
	return database.JobPosition{
		ID:                     11,
		EmployerID:             7,
		Position:               "Backend Engineer",
		Role:                   "Go developer",
		RequiredExperience:     "2_to_5y",
		RequiredEducationLevel: "university",
		AvailableHoursPerDay:   6,
		Timezone:               "America/Argentina/Buenos_Aires",
		TechnicalResources:     []string{"Laptop"},
		CreatedAt:              testJobPositionTime,
		UpdatedAt:              testJobPositionTime,
	}
}

func newTestJobPosition() JobPosition {
	return JobPosition{
		ID:                     11,
		EmployerID:             7,
		Position:               "Backend Engineer",
		Role:                   "Go developer",
		RequiredExperience:     "2_to_5y",
		RequiredEducationLevel: "university",
		AvailableHoursPerDay:   6,
		Timezone:               "America/Argentina/Buenos_Aires",
		TechnicalResources:     []string{"Laptop"},
		CreatedAt:              testJobPositionTime,
		UpdatedAt:              testJobPositionTime,
	}
}

func newTestCreateJobPositionRequest() CreateJobPositionRequest {
	return CreateJobPositionRequest{
		Position:               "Backend Engineer",
		Role:                   "Go developer",
		RequiredExperience:     "2_to_5y",
		RequiredEducationLevel: "university",
		AvailableHoursPerDay:   6,
		Timezone:               "America/Argentina/Buenos_Aires",
		TechnicalResources:     []string{"Laptop"},
	}
}

func newTestRepository(queries *fakeJobPositionQueries, timezoneErr error) *JobPositionRepository {
	return &JobPositionRepository{
		queries: queries,
		validateTimezone: func(context.Context, string) error {
			return timezoneErr
		},
	}
}

func TestJobPositionRepositoryGetEmployerIDByUserID(t *testing.T) {
	internalErr := errors.New("database unavailable")
	tests := []struct {
		name     string
		result   database.Employer
		queryErr error
		wantID   int32
		wantErr  error
	}{
		{name: "success", result: database.Employer{ID: 7, UserID: 42}, wantID: 7},
		{name: "missing employer profile", queryErr: sql.ErrNoRows, wantErr: ErrEmployerProfileRequired},
		{name: "internal", queryErr: internalErr, wantErr: internalErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := &fakeJobPositionQueries{employerResult: tt.result, employerErr: tt.queryErr}
			ctx := context.WithValue(context.Background(), struct{}{}, "request-context")

			got, err := newTestRepository(queries, nil).GetEmployerIDByUserID(ctx, 42)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("GetEmployerIDByUserID() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.wantID {
				t.Fatalf("GetEmployerIDByUserID() = %d, %v; want %d, nil", got, err, tt.wantID)
			}
			if queries.employerUserID != 42 {
				t.Fatalf("GetEmployerIDByUserID userID = %d, want 42", queries.employerUserID)
			}
			if queries.lastContext != ctx {
				t.Fatal("GetEmployerIDByUserID did not propagate context")
			}
		})
	}
}

func TestJobPositionRepositoryCreateJobPosition(t *testing.T) {
	queries := &fakeJobPositionQueries{createResult: newTestDatabaseJobPosition()}
	ctx := context.WithValue(context.Background(), struct{}{}, "request-context")

	got, err := newTestRepository(queries, nil).CreateJobPosition(ctx, 7, newTestCreateJobPositionRequest())
	if err != nil {
		t.Fatalf("CreateJobPosition() error = %v", err)
	}
	if !reflect.DeepEqual(got, newTestJobPosition()) {
		t.Fatalf("CreateJobPosition() = %#v, want %#v", got, newTestJobPosition())
	}

	wantParams := database.CreateJobPositionParams{
		EmployerID:             7,
		Position:               "Backend Engineer",
		Role:                   "Go developer",
		RequiredExperience:     "2_to_5y",
		RequiredEducationLevel: "university",
		AvailableHoursPerDay:   6,
		Timezone:               "America/Argentina/Buenos_Aires",
		TechnicalResources:     []string{"Laptop"},
	}
	if !reflect.DeepEqual(queries.createParams, wantParams) {
		t.Fatalf("CreateJobPosition params = %#v, want %#v", queries.createParams, wantParams)
	}
	if queries.lastContext != ctx {
		t.Fatal("CreateJobPosition did not propagate context")
	}
}

func TestJobPositionRepositoryRejectsUnknownTimezoneBeforeWriting(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		queries := &fakeJobPositionQueries{createResult: newTestDatabaseJobPosition()}
		_, err := newTestRepository(queries, ErrInvalidTimezone).CreateJobPosition(context.Background(), 7, newTestCreateJobPositionRequest())
		if !errors.Is(err, ErrInvalidTimezone) {
			t.Fatalf("CreateJobPosition() error = %v, want %v", err, ErrInvalidTimezone)
		}
		if queries.createParams.EmployerID != 0 {
			t.Fatalf("CreateJobPosition reached the database with %#v", queries.createParams)
		}
	})

	t.Run("update", func(t *testing.T) {
		queries := &fakeJobPositionQueries{updateResult: newTestDatabaseJobPosition()}
		_, err := newTestRepository(queries, ErrInvalidTimezone).UpdateActiveJobPosition(context.Background(), 11, newTestCreateJobPositionRequest())
		if !errors.Is(err, ErrInvalidTimezone) {
			t.Fatalf("UpdateActiveJobPosition() error = %v, want %v", err, ErrInvalidTimezone)
		}
		if queries.updateParams.ID != 0 {
			t.Fatalf("UpdateActiveJobPosition reached the database with %#v", queries.updateParams)
		}
	})
}

func TestJobPositionRepositoryTranslatesNoRows(t *testing.T) {
	ctx := context.Background()

	t.Run("get", func(t *testing.T) {
		repo := newTestRepository(&fakeJobPositionQueries{getErr: sql.ErrNoRows}, nil)
		if _, err := repo.GetActiveJobPositionByID(ctx, 11); !errors.Is(err, ErrJobPositionNotFound) {
			t.Fatalf("GetActiveJobPositionByID() error = %v, want %v", err, ErrJobPositionNotFound)
		}
	})

	t.Run("update", func(t *testing.T) {
		repo := newTestRepository(&fakeJobPositionQueries{updateErr: sql.ErrNoRows}, nil)
		if _, err := repo.UpdateActiveJobPosition(ctx, 11, newTestCreateJobPositionRequest()); !errors.Is(err, ErrJobPositionNotFound) {
			t.Fatalf("UpdateActiveJobPosition() error = %v, want %v", err, ErrJobPositionNotFound)
		}
	})

	t.Run("delete", func(t *testing.T) {
		repo := newTestRepository(&fakeJobPositionQueries{deleteErr: sql.ErrNoRows}, nil)
		if err := repo.SoftDeleteJobPosition(ctx, 11); !errors.Is(err, ErrJobPositionNotFound) {
			t.Fatalf("SoftDeleteJobPosition() error = %v, want %v", err, ErrJobPositionNotFound)
		}
	})
}

func TestJobPositionRepositoryListReturnsEmptySliceAndMapsRows(t *testing.T) {
	t.Run("no rows", func(t *testing.T) {
		repo := newTestRepository(&fakeJobPositionQueries{listResult: nil}, nil)
		got, err := repo.ListActiveJobPositionsByEmployer(context.Background(), 7)
		if err != nil {
			t.Fatalf("ListActiveJobPositionsByEmployer() error = %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Fatalf("ListActiveJobPositionsByEmployer() = %#v, want an empty non-nil slice", got)
		}
	})

	t.Run("maps rows", func(t *testing.T) {
		row := newTestDatabaseJobPosition()
		row.TechnicalResources = nil
		repo := newTestRepository(&fakeJobPositionQueries{listResult: []database.JobPosition{row}}, nil)

		got, err := repo.ListActiveJobPositionsByEmployer(context.Background(), 7)
		if err != nil || len(got) != 1 {
			t.Fatalf("ListActiveJobPositionsByEmployer() = %#v, %v", got, err)
		}
		if got[0].TechnicalResources == nil || len(got[0].TechnicalResources) != 0 {
			t.Fatalf("TechnicalResources = %#v, want an empty non-nil slice", got[0].TechnicalResources)
		}
	})
}

func TestJobPositionRepositoryDoesNotLeakDatabaseErrors(t *testing.T) {
	internalErr := errors.New("pq: relation does not exist")
	repo := newTestRepository(&fakeJobPositionQueries{createErr: internalErr}, nil)

	_, err := repo.CreateJobPosition(context.Background(), 7, newTestCreateJobPositionRequest())
	if !errors.Is(err, internalErr) {
		t.Fatalf("CreateJobPosition() error = %v, want the wrapped original error", err)
	}
	if errors.Is(err, ErrJobPositionNotFound) || errors.Is(err, ErrJobPositionForbidden) {
		t.Fatalf("CreateJobPosition() incorrectly classified internal error: %v", err)
	}
}
