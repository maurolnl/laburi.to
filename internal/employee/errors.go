package employee

import "errors"

var (
	ErrEmployeeNotFound              = errors.New("employee not found")
	ErrEmployeeAlreadyExists         = errors.New("employee already exists")
	ErrInvalidEmployeeRequest        = errors.New("invalid employee request")
	ErrInvalidTimezone               = errors.New("invalid timezone")
	ErrInternalErrorCreatingEmployee = errors.New("internal error creating employee")
	ErrBadLocationBody               = errors.New("invalid location request body")
	ErrInvalidMultiPartForm          = errors.New("invalid multipart form")
)
