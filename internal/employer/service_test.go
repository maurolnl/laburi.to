package employer

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type fakeEmployerStore struct {
	createResult  Employer
	createErr     error
	getResult     Employer
	getErr        error
	createCalls   int
	getCalls      int
	createRequest CreateEmployerRequest
	createUserID  int32
	getUserID     int32
	createContext context.Context
	getContext    context.Context
}

func (f *fakeEmployerStore) CreateEmployer(ctx context.Context, request CreateEmployerRequest, userID int32) (Employer, error) {
	f.createCalls++
	f.createContext = ctx
	f.createRequest = request
	f.createUserID = userID
	return f.createResult, f.createErr
}

func (f *fakeEmployerStore) GetEmployerByUserID(ctx context.Context, userID int32) (Employer, error) {
	f.getCalls++
	f.getContext = ctx
	f.getUserID = userID
	return f.getResult, f.getErr
}

func TestEmployerServiceCreateEmployer(t *testing.T) {
	request := newTestCreateEmployerRequest(withTestEmployerModalities(nil))
	ctx := context.WithValue(context.Background(), struct{}{}, "request-context")
	tests := []struct {
		name          string
		principal     auth.Principal
		storeErr      error
		wantErr       error
		wantStoreCall bool
	}{
		{name: "success", principal: newTestEmployerPrincipal(), wantStoreCall: true},
		{name: "employee role", principal: newTestEmployerPrincipal(withTestEmployerRole(user.UserRoleEmployee)), wantErr: user.ErrProfileRoleForbidden},
		{name: "duplicate", principal: newTestEmployerPrincipal(), storeErr: ErrEmployerAlreadyExists, wantErr: ErrEmployerAlreadyExists, wantStoreCall: true},
		{name: "incompatible profile", principal: newTestEmployerPrincipal(), storeErr: ErrEmployerProfileConflict, wantErr: ErrEmployerProfileConflict, wantStoreCall: true},
		{name: "internal", principal: newTestEmployerPrincipal(), storeErr: errors.New("database unavailable"), wantStoreCall: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeEmployerStore{createErr: tt.storeErr}
			service := NewService(store)
			err := service.CreateEmployer(ctx, request, tt.principal)

			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreateEmployer() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && tt.storeErr == nil && err != nil {
				t.Fatalf("CreateEmployer() error = %v", err)
			}
			if tt.storeErr != nil && !errors.Is(err, tt.storeErr) {
				t.Fatalf("CreateEmployer() error = %v, want propagated %v", err, tt.storeErr)
			}
			if (store.createCalls == 1) != tt.wantStoreCall {
				t.Fatalf("CreateEmployer store calls = %d, wantStoreCall %v", store.createCalls, tt.wantStoreCall)
			}
			if tt.wantStoreCall {
				if store.createUserID != tt.principal.UserID {
					t.Fatalf("CreateEmployer userID = %d, want %d", store.createUserID, tt.principal.UserID)
				}
				if !reflect.DeepEqual(store.createRequest, request) {
					t.Fatalf("CreateEmployer request = %#v, want %#v", store.createRequest, request)
				}
				if store.createContext != ctx {
					t.Fatal("CreateEmployer did not propagate context")
				}
			}
		})
	}
}

func TestEmployerServiceGetEmployer(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "request-context")
	internalErr := errors.New("database unavailable")
	tests := []struct {
		name          string
		principal     auth.Principal
		storeResult   Employer
		storeErr      error
		wantErr       error
		wantStoreCall bool
	}{
		{name: "success", principal: newTestEmployerPrincipal(), storeResult: newTestEmployer(), wantStoreCall: true},
		{name: "employee role", principal: newTestEmployerPrincipal(withTestEmployerRole(user.UserRoleEmployee)), wantErr: user.ErrProfileRoleForbidden},
		{name: "not found", principal: newTestEmployerPrincipal(), storeErr: ErrEmployerNotFound, wantErr: ErrEmployerNotFound, wantStoreCall: true},
		{name: "internal", principal: newTestEmployerPrincipal(), storeErr: internalErr, wantErr: internalErr, wantStoreCall: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeEmployerStore{getResult: tt.storeResult, getErr: tt.storeErr}
			service := NewService(store)
			got, err := service.GetEmployer(ctx, 42, tt.principal)

			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("GetEmployer() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("GetEmployer() error = %v", err)
				}
				if got.ID != tt.storeResult.ID || got.HiringModalities == nil {
					t.Fatalf("GetEmployer() = %#v, want normalized employer", got)
				}
			}
			if (store.getCalls == 1) != tt.wantStoreCall {
				t.Fatalf("GetEmployer store calls = %d, wantStoreCall %v", store.getCalls, tt.wantStoreCall)
			}
			if tt.wantStoreCall {
				if store.getUserID != 42 {
					t.Fatalf("GetEmployer userID = %d, want 42", store.getUserID)
				}
				if store.getContext != ctx {
					t.Fatal("GetEmployer did not propagate context")
				}
			}
		})
	}
}
