package employee

import "context"

type EmployeeStore interface {
	CreateEmployee(ctx context.Context, employee CreateEmployeeRequest, userID int32, file *EmployeeFileMetadata) (int32, error)
	UpdateEmployee(ctx context.Context, employeeID int32, employee CreateEmployeeRequest, file *EmployeeFileMetadata) error
	CreateLocationWithConnections(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error
	UpdateLocationWithConnections(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error
	CreateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error
	UpdateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error
	CreateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error
	UpdateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error
	CreateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest) error
	UpdateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest) error
	GetEmployee(ctx context.Context, ID int32) (Employee, error)

	// IsProfileComplete responde si el empleado tiene sus cinco pasos presentes: registro base,
	// locación, recursos técnicos, disponibilidad y al menos un título de educación. Es la
	// condición que habilita a solicitar la regeneración de sus recomendaciones.
	//
	// Un empleado inexistente devuelve false sin error: quien pregunta quiere saber si
	// corresponde disparar, no si el empleado existe.
	IsProfileComplete(ctx context.Context, employeeID int32) (bool, error)

	// GetEmployeeProfileByID lee el perfil completo por el identificador de empleado, que es la
	// clave con la que lo direcciona el borde que se lo expone a un tercero. Devuelve
	// ErrEmployeeProfileNotFound cuando el empleado no existe.
	//
	// Es una lectura distinta de GetEmployee, que toma el userID: acá los archivos viajan
	// identificados y nunca con su ubicación.
	GetEmployeeProfileByID(ctx context.Context, employeeID int32) (EmployeeProfile, error)

	// GetEmployeeByID es la lectura mínima con la que se resuelve la propiedad: identificador y
	// usuario dueño, sin el perfil entero. Devuelve ErrEmployeeProfileNotFound cuando el
	// empleado no existe.
	GetEmployeeByID(ctx context.Context, employeeID int32) (Employee, error)

	// GetEmployeeFile y GetEmployeeEducationDocument resuelven el archivo a entregar por el par
	// empleado/identificador. Ambas devuelven ErrFileNotFound tanto para el archivo inexistente
	// como para el que pertenece a otro empleado: quien pregunta ya está autorizado sobre este
	// perfil, y distinguirlos solo le diría que el identificador existe en otra parte.
	//
	// Un certificado cuyo contenido no terminó de subirse y un título sin documento son, a
	// estos efectos, inexistentes: no hay nada que entregar.
	GetEmployeeFile(ctx context.Context, employeeID, fileID int32) (StoredFile, error)
	GetEmployeeEducationDocument(ctx context.Context, employeeID, educationID int32) (StoredFile, error)
}

// StoredFile es la ubicación real de un archivo en S3 más lo necesario para entregarlo con su
// nombre. Nunca sale del backend: solo alimenta la firma de la URL.
type StoredFile struct {
	Bucket      string
	ObjectKey   string
	Filename    string
	ContentType string
}

// RecommendationAccess responde si un empleador puede mirar el perfil de un empleado ajeno. Es
// la condición de acceso de un tercero, y entra por un puerto para que este paquete no importe
// internal/recommendation: el mismo criterio con el que ya entra el disparador de regeneración.
type RecommendationAccess interface {
	EmployerHasCurrentRecommendation(ctx context.Context, employeeID, employerUserID int32) (bool, error)
}

type EmployeeFileMetadata struct {
	Type             string
	Bucket           string
	ObjectKey        string
	OriginalFilename string
	ContentType      string
	SizeBytes        int64
	ChecksumSHA256   string
	Status           string
}
