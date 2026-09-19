package employer

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/lib/pq"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

type fakeEmployerQueries struct {
	createResult  database.Employer
	createErr     error
	getResult     database.Employer
	getErr        error
	createParams  database.CreateEmployerParams
	getUserID     int32
	createContext context.Context
	getContext    context.Context
}

func (f *fakeEmployerQueries) CreateEmployer(ctx context.Context, arg database.CreateEmployerParams) (database.Employer, error) {
	f.createContext = ctx
	f.createParams = arg
	return f.createResult, f.createErr
}

func (f *fakeEmployerQueries) GetEmployerByUserID(ctx context.Context, userID int32) (database.Employer, error) {
	f.getContext = ctx
	f.getUserID = userID
	return f.getResult, f.getErr
}

func TestEmployerRepositoryCreateEmployer(t *testing.T) {
	queries := &fakeEmployerQueries{createResult: newTestDatabaseEmployer(withTestEmployerModalities(nil))}
	repo := &EmployerRepository{queries: queries}
	request := newTestCreateEmployerRequest(withTestEmployerModalities([]string{}))
	ctx := context.WithValue(context.Background(), struct{}{}, "request-context")

	got, err := repo.CreateEmployer(ctx, request, 42)
	if err != nil {
		t.Fatalf("CreateEmployer() error = %v", err)
	}
	wantEmployer := newTestEmployer(withTestEmployerModalities(nil))
	wantEmployer.Normalize()
	if !reflect.DeepEqual(got, wantEmployer) {
		t.Fatalf("CreateEmployer() = %#v, want %#v", got, wantEmployer)
	}
	wantParams := database.CreateEmployerParams{
		UserID: 42, Name: "Acme", Industry: "Software", Location: "Remote", HiringModalities: []string{},
	}
	if !reflect.DeepEqual(queries.createParams, wantParams) {
		t.Fatalf("CreateEmployer params = %#v, want %#v", queries.createParams, wantParams)
	}
	if queries.createContext != ctx {
		t.Fatal("CreateEmployer did not propagate context")
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
		name        string
		queryResult database.Employer
		queryErr    error
		wantErr     error
	}{
		{name: "success", queryResult: newTestDatabaseEmployer(withTestEmployerModalities(nil))},
		{name: "not found", queryErr: sql.ErrNoRows, wantErr: ErrEmployerNotFound},
		{name: "internal", queryErr: internalErr, wantErr: internalErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := &fakeEmployerQueries{getResult: tt.queryResult, getErr: tt.queryErr}
			repo := &EmployerRepository{queries: queries}
			ctx := context.WithValue(context.Background(), struct{}{}, "request-context")
			got, err := repo.GetEmployerByUserID(ctx, 42)
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("GetEmployerByUserID() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				want := newTestEmployer(withTestEmployerModalities(nil))
				want.Normalize()
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("GetEmployerByUserID() = %#v, %v; want %#v, nil", got, err, want)
				}
			}
			if queries.getUserID != 42 {
				t.Fatalf("GetEmployerByUserID userID = %d, want 42", queries.getUserID)
			}
			if queries.getContext != ctx {
				t.Fatal("GetEmployerByUserID did not propagate context")
			}
		})
	}
}
