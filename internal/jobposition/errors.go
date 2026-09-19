package jobposition

import "errors"

var (
	ErrJobPositionNotFound     = errors.New("job position not found")
	ErrJobPositionForbidden    = errors.New("job position does not belong to the authenticated employer")
	ErrEmployerProfileRequired = errors.New("an employer profile is required to manage job positions")
	ErrInvalidJSON             = errors.New("invalid JSON body")
	ErrInvalidEmployerID       = errors.New("invalid employer ID")
	ErrInvalidJobPositionID    = errors.New("invalid job position ID")
	ErrInvalidTimezone         = errors.New("invalid timezone")

	ErrInternalErrorCreatingJobPosition = errors.New("internal error creating job position")
	ErrInternalErrorListingJobPositions = errors.New("internal error listing job positions")
	ErrInternalErrorGettingJobPosition  = errors.New("internal error getting job position")
	ErrInternalErrorUpdatingJobPosition = errors.New("internal error updating job position")
	ErrInternalErrorDeletingJobPosition = errors.New("internal error deleting job position")
)
