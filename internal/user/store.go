package user

import "context"

type UserStore interface {
	Save(ctx context.Context, user CreateUserReq) error
	FindByEmail(ctx context.Context, email string) (LoginRes, error)
	SaveRefreshToken(ctx context.Context, token SaveRefreshToken) error
	GetCurrentUser(ctx context.Context, userID int32) (User, error)
}
