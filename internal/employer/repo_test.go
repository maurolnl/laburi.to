package employer

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

type fakeEmployerQueries struct {
	createResult database.Employer
	createErr    error
	getResult    database.Employer
	getErr       error
	createParams database.CreateEmployerParams
	getUserID    int32
}

func (f *fakeEmployerQueries) CreateEmployer(_ context.Context, arg database.CreateEmployerParams) (database.Employer, error) {
	f.createParams = arg
	return f.createResult, f.createErr
}

func (f *fakeEmployerQueries) GetEmployerByUserID(_ context.Context, userID int32) (database.Employer, error) {
	f.getUserID = userID
	return f.getResult, f.getErr
}

func TestEmployerRepositoryCreateEmployer(t *testing.T) {
	now := time.Now()
	queries := &fakeEmployerQueries{createResult: database.Employer{
		ID: 7, UserID: 42, Name: "Acme", Industry: "Software", Location: "Remote",
		HiringModalities: nil, CreatedAt: now, UpdatedAt: now,
	}}
	repo := &EmployerRepository{queries: queries}
	request := CreateEmployerRequest{
		Name: "Acme", Industry: "Software", Location: "Remote", HiringModalities: []string{},
	}

	got, err := repo.CreateEmployer(context.Background(), request, 42)
	if err != nil {
		t.Fatalf("CreateEmployer() error = %v", err)
	}
	if got.ID != 7 || got.UserID != 42 || got.HiringModalities == nil {
		t.Fatalf("CreateEmployer() = %#v, want mapped employer with empty modalities", got)
	}
	wantParams := database.CreateEmployerParams{
		UserID: 42, Name: "Acme", Industry: "Software", Location: "Remote", HiringModalities: []string{},
	}
	if !reflect.DeepEqual(queries.createParams, wantParams) {
		t.Fatalf("CreateEmployer params = %#v, want %#v", queries.createParams, wantParams)
	}
}

func TestEmployerRepositoryClassifiesCreateErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{name: "duplicate", err: &pq.Error{Code: "23505"}, wantErr: ErrEmployerAlreadyExists},
		{name: "incompatible profile", err: &pq.Error{Code: "23514"}, wantErr: ErrEmployerProfileConflict},
		{name: "internal", err: errors.New("database unavailable"), wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &EmployerRepository{queries: &fakeEmployerQueries{createErr: tt.err}}
			_, err := repo.CreateEmployer(context.Background(), CreateEmployerRequest{}, 42)
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreateEmployer() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("CreateEmployer() error = %v, want wrapped original error", err)
				}
				if errors.Is(err, ErrEmployerAlreadyExists) || errors.Is(err, ErrEmployerProfileConflict) {
					t.Fatalf("CreateEmployer() incorrectly classified internal error: %v", err)
				}
			}
		})
	}
}

func TestEmployerRepositoryGetEmployerByUserID(t *testing.T) {
	internalErr := errors.New("database unavailable")
	tests := []struct {
		name     string
		queryErr error
		wantErr  error
	}{
		{name: "not found", queryErr: sql.ErrNoRows, wantErr: ErrEmployerNotFound},
		{name: "internal", queryErr: internalErr, wantErr: internalErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := &fakeEmployerQueries{getErr: tt.queryErr}
			repo := &EmployerRepository{queries: queries}
			_, err := repo.GetEmployerByUserID(context.Background(), 42)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("GetEmployerByUserID() error = %v, want %v", err, tt.wantErr)
			}
			if queries.getUserID != 42 {
				t.Fatalf("GetEmployerByUserID userID = %d, want 42", queries.getUserID)
			}
		})
	}
}
