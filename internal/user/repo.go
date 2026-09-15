package user

import (
	"context"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

type UserRepository struct {
	db *database.Queries
}

func NewRepository(db *database.Queries) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Save(ctx context.Context, user CreateUserReq) error {
	_, err := r.db.CreateUser(ctx, database.CreateUserParams{
		Email:          user.Email,
		HashedPassword: user.Password,
		Role:           string(user.Role),
	})
	return err
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (LoginRes, error) {
	user, err := r.db.GetUserByEmail(ctx, email)
	if err != nil {
		return LoginRes{}, err
	}
	role := UserRole(user.Role)
	if !role.Valid() {
		return LoginRes{}, ErrInvalidUserRole
	}
	return LoginRes{
		ID:             user.ID,
		Email:          user.Email,
		HashedPassword: user.HashedPassword,
		Role:           role,
	}, nil
}

func (r *UserRepository) SaveRefreshToken(ctx context.Context, token SaveRefreshToken) error {
	_, err := r.db.CreateRefreshToken(ctx, database.CreateRefreshTokenParams{
		Token:     token.Token,
		UserID:    token.UserID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: token.ExpiresAt,
	})
	return err
}

func (r *UserRepository) GetCurrentUser(ctx context.Context, userID int32) (User, error) {
	user, err := r.db.GetUserByID(ctx, userID)
	if err != nil {
		return User{}, err
	}
	role := UserRole(user.Role)
	if !role.Valid() {
		return User{}, ErrInvalidUserRole
	}
	return User{
		ID:    user.ID,
		Email: user.Email,
		Role:  role,
	}, nil
}
