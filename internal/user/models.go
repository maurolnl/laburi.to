package user

import "time"

// <Verb>Entity<Action>
// Action = Req or Res
// <> = Optional
type (
	CreateUserReq struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=8"`
	}
	LoginRes struct {
		ID             int32
		Email          string
		HashedPassword string
	}
	User struct {
		ID    int32
		Email string
	}
	UserRes struct {
		ID           int32  `json:"id"`
		Email        string `json:"email"`
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
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
