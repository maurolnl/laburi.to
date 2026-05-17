package user

import (
	"errors"
)

var (
	ErrEmailOrPasswordRequired = errors.New("email and password are required")
	ErrInvalidCredentials      = errors.New("invalid credentials")
	ErrCouldNotValidateUser    = errors.New("could not validate user credentials")
	ErrCouldNotGenerateToken   = func(err error) error {
		return errors.New("could not generate token: " + err.Error())
	}
)
