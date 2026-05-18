package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/alexedwards/argon2id"
)

var (
	accessTokenExpirationTime = 30 * time.Minute
)

func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", err
	}

	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}

	return match, nil
}

func MakeRefreshToken() string {
	refreshToken := make([]byte, 32)

	rand.Read(refreshToken)

	return hex.EncodeToString(refreshToken)
}

func GenerateGrants(userID int32, secretKey string, ctx context.Context) (token, refreshToken string, refreshTokenExpiration time.Time, err error) {
	tokenExpiration := accessTokenExpirationTime
	token, err = MakeJWT(userID, secretKey, tokenExpiration)
	if err != nil {
		return "", "", time.Time{}, err
	}

	refreshToken = MakeRefreshToken()
	refreshTokenExpiration = time.Now().UTC().Add(60 * 24 * time.Hour)

	return token, refreshToken, refreshTokenExpiration, nil
}
