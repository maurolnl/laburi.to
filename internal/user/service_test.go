package user

import (
	"context"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
)

type fakeUserStore struct {
	savedUser         CreateUserReq
	loginUser         LoginRes
	savedRefreshToken SaveRefreshToken
}

func (f *fakeUserStore) Save(_ context.Context, req CreateUserReq) error {
	f.savedUser = req
	return nil
}

func (f *fakeUserStore) FindByEmail(_ context.Context, _ string) (LoginRes, error) {
	return f.loginUser, nil
}

func (f *fakeUserStore) SaveRefreshToken(_ context.Context, token SaveRefreshToken) error {
	f.savedRefreshToken = token
	return nil
}

func (f *fakeUserStore) GetCurrentUser(_ context.Context, _ int32) (User, error) {
	return User{}, nil
}

func TestSaveUserPreservesRole(t *testing.T) {
	for _, role := range []UserRole{UserRoleEmployee, UserRoleEmployer} {
		t.Run(string(role), func(t *testing.T) {
			store := &fakeUserStore{}
			service := NewService(store, "secret")
			request := newTestCreateUserRequest(withTestUserRole(role))

			if err := service.SaveUser(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if store.savedUser.Role != role {
				t.Fatalf("expected role %q, got %q", role, store.savedUser.Role)
			}
			if store.savedUser.Password == request.Password {
				t.Fatal("expected password to be hashed")
			}
		})
	}
}

func TestSaveUserRejectsInvalidRoleBeforePersistence(t *testing.T) {
	store := &fakeUserStore{}
	service := NewService(store, "secret")

	for _, role := range []UserRole{"", "admin"} {
		err := service.SaveUser(context.Background(), newTestCreateUserRequest(withTestUserRole(role)))
		if err != ErrInvalidUserRole {
			t.Fatalf("role %q: expected ErrInvalidUserRole, got %v", role, err)
		}
		if store.savedUser.Email != "" {
			t.Fatalf("role %q: expected invalid role not to reach persistence", role)
		}
	}
}

func TestLoginUsesPersistedBackfillRole(t *testing.T) {
	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	store := &fakeUserStore{loginUser: newTestLoginResult(
		withTestUserID(9),
		withTestUserEmail("historical@example.com"),
		withTestUserHash(hash),
		withTestUserRole(UserRoleEmployer),
	)}
	service := NewService(store, "secret")

	userID, role, token, _, err := service.Login(context.Background(), "historical@example.com", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	if userID != 9 || role != UserRoleEmployer {
		t.Fatalf("expected persisted employer role for user 9, got user=%d role=%q", userID, role)
	}
	principal, err := auth.ValidateJWT(token, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if principal.UserID != 9 || principal.Role != UserRoleEmployer {
		t.Fatalf("unexpected token principal: %#v", principal)
	}
	if store.savedRefreshToken.UserID != 9 {
		t.Fatalf("expected refresh token for user 9, got %d", store.savedRefreshToken.UserID)
	}
}
