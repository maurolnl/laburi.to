package user

import (
	"errors"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
)

type UserRole = auth.UserRole

const (
	UserRoleEmployee = auth.UserRoleEmployee
	UserRoleEmployer = auth.UserRoleEmployer
)

var (
	ErrInvalidUserRole      = errors.New("invalid user role")
	ErrProfileRoleForbidden = errors.New("account role cannot create this profile type")
)

func AuthorizeProfileRole(role, profileType UserRole) error {
	if !role.Valid() || !profileType.Valid() || role != profileType {
		return ErrProfileRoleForbidden
	}
	return nil
}

// <Verb>Entity<Action>
// Action = Req or Res
// <> = Optional
type (
	CreateUserReq struct {
		Email    string   `json:"email" validate:"required,email"`
		Password string   `json:"password" validate:"required,min=8"`
		Role     UserRole `json:"role" validate:"required,oneof=employee employer"`
	}
	LoginRes struct {
		ID             int32
		Email          string
		HashedPassword string
		Role           UserRole
	}
	User struct {
		ID    int32
		Email string
		Role  UserRole
	}
	UserRes struct {
		ID           int32    `json:"id"`
		Email        string   `json:"email"`
		Role         UserRole `json:"role"`
		Token        string   `json:"token"`
		RefreshToken string   `json:"refreshToken"`
	}
	LoginUserRequest struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}
	SaveRefreshToken struct {
		Token     string
		UserID    int32
		ExpiresAt time.Time
	}
)
