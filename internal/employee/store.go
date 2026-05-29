package employee

import "context"

type EmployeeStore interface {
	CreateEmployee(ctx context.Context, employee CreateEmployeeRequest, userID int32, file EmployeeFileMetadata) (int32, error)
	CreateLocationWithConnections(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error
	CreateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error
	CreateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error
	CreateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest) error
	GetEmployee(ctx context.Context, ID int32) (Employee, error)
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
