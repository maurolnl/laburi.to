package user

import "time"

// <Verb>Entity<Action>
// Action = Req or Res
// <> = Optional
type (
	CreateUserReq struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	User struct {
		ID             int32
		Email          string
		HashedPassword string
	}
	UserRes struct {
		ID           int32  `json:"id"`
		Email        string `json:"email"`
		Token        string `json:"token"`
		RefreshToken string `json:"refreshToken"`
	}
	LoginUserRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	SaveRefreshToken struct {
		Token     string
		UserID    int32
		ExpiresAt time.Time
	}
)
