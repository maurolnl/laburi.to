package employer

import "errors"

var (
	ErrEmployerNotFound              = errors.New("employer profile not found")
	ErrEmployerAlreadyExists         = errors.New("employer profile already exists")
	ErrEmployerProfileConflict       = errors.New("user already has an incompatible profile")
	ErrInvalidJSON                   = errors.New("invalid JSON body")
	ErrInvalidUserID                 = errors.New("invalid user ID")
	ErrInternalErrorCreatingEmployer = errors.New("internal error creating employer profile")
	ErrInternalErrorGettingEmployer  = errors.New("internal error getting employer profile")
)
