package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeAccess = "chirpy-access"
)

var (
	ErrInvalidToken   = errors.New("invalid token")
	ErrBadTokenFormat = errors.New("bad authorization prefix")
	ErrCannotValidate = errors.New("cannot validate token")
)

func MakeJWT(userID int32, tokenSecret string, expiresIn time.Duration) (string, error) {
	now := time.Now().UTC()
	expiration := now.Add(expiresIn)

	claims := jwt.RegisteredClaims{
		Issuer:    TokenTypeAccess,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiration),
		Subject:   strconv.Itoa(int(userID)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString, tokenSecret string) (int32, error) {
	// We declare an empty RegisteredClaims struct
	// and pass it to ParseWithClaims to populate it with the token's claims
	// once is decoded inside the ParseWithClaims function.
	// This is a struct that follow the behaviour of jwt.Claims
	// and is used to store the claims of a JWT token.
	// We need to pass a pointer so the function know
	// which struct type has to fill.
	claims := &jwt.RegisteredClaims{}

	_, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		// This is a callback since it may handle multiple token secrets
		// that need to be chosen dynamically.
		// Here it seems overkill but it has been design for flexibility.
		func(token *jwt.Token) (interface{}, error) {
			return []byte(tokenSecret), nil
		},
		jwt.WithIssuer(TokenTypeAccess),
	)
	if err != nil {
		return 0, err
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 32)
	if err != nil {
		return 0, err
	}

	return int32(userID), nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authorizationHeader := headers.Get("Authorization")
	prefix := "Bearer "

	if authorizationHeader == "" {
		return "", ErrInvalidToken
	}

	if !strings.HasPrefix(authorizationHeader, prefix) {
		return "", ErrBadTokenFormat
	}

	return strings.TrimPrefix(authorizationHeader, prefix), nil
}
