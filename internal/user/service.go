package user

import (
	"context"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
)

type UserService interface {
	SaveUser(ctx context.Context, user CreateUserReq) error
	Login(ctx context.Context, email, password string) (int32, string, string, error)
}

type userService struct {
	repo      UserStore
	secretKey string
}

func NewService(repo UserStore, secretKey string) UserService {
	return &userService{
		repo:      repo,
		secretKey: secretKey,
	}
}

func (s *userService) SaveUser(ctx context.Context, user CreateUserReq) error {
	hashedPassword, err := auth.HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = hashedPassword

	err = s.repo.Save(ctx, user)
	return err
}

func (s *userService) Login(ctx context.Context, email, password string) (int32, string, string, error) {
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return 0, "", "", err
	}

	match, err := auth.CheckPasswordHash(password, user.HashedPassword)
	if err != nil {
		return 0, "", "", ErrCouldNotValidateUser
	} else if !match {
		return 0, "", "", ErrInvalidCredentials
	}

	token, refreshToken, refreshTokenExpiration, err := auth.GenerateGrants(user.ID, s.secretKey, ctx)
	if err != nil {
		return 0, "", "", ErrCouldNotGenerateToken(err)
	}

	err = s.repo.SaveRefreshToken(ctx, SaveRefreshToken{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: refreshTokenExpiration,
	})

	if err != nil {
		return 0, "", "", err
	}

	return user.ID, token, refreshToken, nil
}
