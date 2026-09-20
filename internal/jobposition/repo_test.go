package jobposition

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

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
		{name: "success", result: newTestDatabaseEmployerRow(), wantID: testEmployerID},
		{name: "missing employer profile", queryErr: sql.ErrNoRows, wantErr: ErrEmployerProfileRequired},
		{name: "internal", queryErr: internalErr, wantErr: internalErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := &fakeJobPositionQueries{employerResult: tt.result, employerErr: tt.queryErr}
			ctx := context.WithValue(context.Background(), struct{}{}, "request-context")

			got, err := newTestRepository(queries, nil).GetEmployerIDByUserID(ctx, testUserID)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("GetEmployerIDByUserID() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.wantID {
				t.Fatalf("GetEmployerIDByUserID() = %d, %v; want %d, nil", got, err, tt.wantID)
			}
			if queries.employerUserID != testUserID {
				t.Fatalf("GetEmployerIDByUserID userID = %d, want %d", queries.employerUserID, testUserID)
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

	got, err := newTestRepository(queries, nil).CreateJobPosition(ctx, testEmployerID, newTestCreateJobPositionRequest())
	if err != nil {
		t.Fatalf("CreateJobPosition() error = %v", err)
	}
	if !reflect.DeepEqual(got, newTestJobPosition()) {
		t.Fatalf("CreateJobPosition() = %#v, want %#v", got, newTestJobPosition())
	}

	wantParams := newTestCreateJobPositionParams()
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
		_, err := newTestRepository(queries, ErrInvalidTimezone).CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest())
		if !errors.Is(err, ErrInvalidTimezone) {
			t.Fatalf("CreateJobPosition() error = %v, want %v", err, ErrInvalidTimezone)
		}
		if queries.createParams.EmployerID != 0 {
			t.Fatalf("CreateJobPosition reached the database with %#v", queries.createParams)
		}
	})

	t.Run("update", func(t *testing.T) {
		queries := &fakeJobPositionQueries{updateResult: newTestDatabaseJobPosition()}
		_, err := newTestRepository(queries, ErrInvalidTimezone).UpdateActiveJobPosition(context.Background(), testJobPositionID, newTestCreateJobPositionRequest())
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
		if _, err := repo.GetActiveJobPositionByID(ctx, testJobPositionID); !errors.Is(err, ErrJobPositionNotFound) {
			t.Fatalf("GetActiveJobPositionByID() error = %v, want %v", err, ErrJobPositionNotFound)
		}
	})

	t.Run("update", func(t *testing.T) {
		repo := newTestRepository(&fakeJobPositionQueries{updateErr: sql.ErrNoRows}, nil)
		if _, err := repo.UpdateActiveJobPosition(ctx, testJobPositionID, newTestCreateJobPositionRequest()); !errors.Is(err, ErrJobPositionNotFound) {
			t.Fatalf("UpdateActiveJobPosition() error = %v, want %v", err, ErrJobPositionNotFound)
		}
	})

	t.Run("delete", func(t *testing.T) {
		repo := newTestRepository(&fakeJobPositionQueries{deleteErr: sql.ErrNoRows}, nil)
		if err := repo.SoftDeleteJobPosition(ctx, testJobPositionID); !errors.Is(err, ErrJobPositionNotFound) {
			t.Fatalf("SoftDeleteJobPosition() error = %v, want %v", err, ErrJobPositionNotFound)
		}
	})
}

func TestJobPositionRepositoryListReturnsEmptySliceAndMapsRows(t *testing.T) {
	t.Run("no rows", func(t *testing.T) {
		repo := newTestRepository(&fakeJobPositionQueries{listResult: nil}, nil)
		got, err := repo.ListActiveJobPositionsByEmployer(context.Background(), testEmployerID)
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

		got, err := repo.ListActiveJobPositionsByEmployer(context.Background(), testEmployerID)
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

	_, err := repo.CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest())
	if !errors.Is(err, internalErr) {
		t.Fatalf("CreateJobPosition() error = %v, want the wrapped original error", err)
	}
	if errors.Is(err, ErrJobPositionNotFound) || errors.Is(err, ErrJobPositionForbidden) {
		t.Fatalf("CreateJobPosition() incorrectly classified internal error: %v", err)
	}
}

func TestJobPositionRepositoryForwardsTechnicalResourceVariants(t *testing.T) {
	tests := []struct {
		name      string
		resources []string
	}{
		{name: "none", resources: []string{}},
		{name: "single", resources: []string{"Laptop"}},
		{name: "multiple", resources: []string{"Laptop", "VPN", "Monitor"}},
	}

	for _, tt := range tests {
		t.Run("create/"+tt.name, func(t *testing.T) {
			option := withTestJobPositionResources(tt.resources)
			queries := &fakeJobPositionQueries{createResult: newTestDatabaseJobPosition(option)}

			got, err := newTestRepository(queries, nil).CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest(option))
			if err != nil {
				t.Fatalf("CreateJobPosition() error = %v", err)
			}
			if !reflect.DeepEqual(queries.createParams, newTestCreateJobPositionParams(option)) {
				t.Fatalf("create params = %#v, want %#v", queries.createParams, newTestCreateJobPositionParams(option))
			}
			if !reflect.DeepEqual(got.TechnicalResources, tt.resources) {
				t.Fatalf("TechnicalResources = %#v, want %#v in the same order", got.TechnicalResources, tt.resources)
			}
		})

		t.Run("update/"+tt.name, func(t *testing.T) {
			option := withTestJobPositionResources(tt.resources)
			queries := &fakeJobPositionQueries{updateResult: newTestDatabaseJobPosition(option)}

			got, err := newTestRepository(queries, nil).UpdateActiveJobPosition(context.Background(), testJobPositionID, newTestCreateJobPositionRequest(option))
			if err != nil {
				t.Fatalf("UpdateActiveJobPosition() error = %v", err)
			}
			if !reflect.DeepEqual(queries.updateParams, newTestUpdateJobPositionParams(option)) {
				t.Fatalf("update params = %#v, want %#v", queries.updateParams, newTestUpdateJobPositionParams(option))
			}
			if !reflect.DeepEqual(got.TechnicalResources, tt.resources) {
				t.Fatalf("TechnicalResources = %#v, want %#v in the same order", got.TechnicalResources, tt.resources)
			}
		})
	}
}

// TestJobPositionRepositoryNormalizesNullTechnicalResources cubre la fila persistida con
// NULL: el contrato nunca expone null, ni siquiera cuando la columna lo permite.
func TestJobPositionRepositoryNormalizesNullTechnicalResources(t *testing.T) {
	option := withTestJobPositionResources(nil)
	queries := &fakeJobPositionQueries{getResult: newTestDatabaseJobPosition(option)}

	got, err := newTestRepository(queries, nil).GetActiveJobPositionByID(context.Background(), testJobPositionID)
	if err != nil {
		t.Fatalf("GetActiveJobPositionByID() error = %v", err)
	}
	if got.TechnicalResources == nil || len(got.TechnicalResources) != 0 {
		t.Fatalf("TechnicalResources = %#v, want an empty non-nil slice", got.TechnicalResources)
	}
}
