package employee

import "errors"

var (
	ErrEmployeeNotFound              = errors.New("employee not found")
	ErrEmployeeAlreadyExists         = errors.New("employee already exists")
	ErrInvalidEmployeeRequest        = errors.New("invalid employee request")
	ErrInternalErrorCreatingEmployee = errors.New("internal error creating employee")
)
