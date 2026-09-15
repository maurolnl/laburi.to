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

type UserRole string

const (
	UserRoleEmployee UserRole = "employee"
	UserRoleEmployer UserRole = "employer"
)

type Principal struct {
	UserID int32
	Role   UserRole
}

func (r UserRole) Valid() bool {
	return r == UserRoleEmployee || r == UserRoleEmployer
}

type accessTokenClaims struct {
	Role UserRole `json:"role"`
	jwt.RegisteredClaims
}

func MakeJWT(userID int32, role UserRole, tokenSecret string, expiresIn time.Duration) (string, error) {
	if !role.Valid() {
		return "", ErrInvalidToken
	}

	now := time.Now().UTC()
	expiration := now.Add(expiresIn)

	claims := accessTokenClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    TokenTypeAccess,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiration),
			Subject:   strconv.Itoa(int(userID)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString, tokenSecret string) (Principal, error) {
	// We declare an empty RegisteredClaims struct
	// and pass it to ParseWithClaims to populate it with the token's claims
	// once is decoded inside the ParseWithClaims function.
	// This is a struct that follow the behaviour of jwt.Claims
	// and is used to store the claims of a JWT token.
	// We need to pass a pointer so the function know
	// which struct type has to fill.
	claims := &accessTokenClaims{}

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
		return Principal{}, err
	}
	if !claims.Role.Valid() {
		return Principal{}, ErrInvalidToken
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 32)
	if err != nil {
		return Principal{}, err
	}

	return Principal{UserID: int32(userID), Role: claims.Role}, nil
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
