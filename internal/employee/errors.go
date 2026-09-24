package employee

import "errors"

var (
	ErrEmployeeNotFound              = errors.New("employee not found")
	ErrEmployeeAlreadyExists         = errors.New("employee already exists")
	ErrInvalidEmployeeRequest        = errors.New("invalid employee request")
	ErrInvalidTimezone               = errors.New("invalid timezone")
	ErrInternalErrorCreatingEmployee = errors.New("internal error creating employee")
	ErrInternalErrorUpdatingEmployee = errors.New("internal error updating employee")
	ErrBadLocationBody               = errors.New("invalid location request body")
	ErrInvalidMultiPartForm          = errors.New("invalid multipart form")

	// ErrEmployeeProfileNotFound es la ausencia del empleado en la lectura por identificador.
	// No llega nunca al cliente tal cual: la autorización lo convierte en la misma respuesta
	// que el perfil ajeno, para que nadie pueda descubrir qué identificadores existen.
	ErrEmployeeProfileNotFound = errors.New("employee profile not found")

	// ErrFileNotFound cubre el archivo inexistente y el que pertenece a otro empleado. Los dos
	// son lo mismo para quien ya está autorizado sobre este perfil.
	ErrFileNotFound = errors.New("file not found")

	// ErrProfileAccessForbidden es la única respuesta a toda falla de autorización del perfil:
	// empleado ajeno, empleador sin vínculo vigente y empleado inexistente. Distinguirlos le
	// permitiría a un empleador enumerar qué empleados existen recorriendo identificadores.
	ErrProfileAccessForbidden = errors.New("profile access forbidden")

	ErrInternalErrorReadingProfile  = errors.New("internal error reading employee profile")
	ErrInternalErrorSigningDownload = errors.New("internal error signing download url")
)
