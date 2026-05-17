package user

import "context"

type UserStore interface {
	Save(ctx context.Context, user CreateUserReq) error
	FindByEmail(ctx context.Context, email string) (User, error)
	SaveRefreshToken(ctx context.Context, token SaveRefreshToken) error
}
