package employee

import (
	"context"
	"mime/multipart"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
)

type fakeEmployeeService struct {
	mu sync.Mutex

	createEmployeeCalls []createEmployeeCall
	createEmployeeErr   error

	updateEmployeeCalls []updateEmployeeCall
	updateEmployeeErr   error

	getEmployeeID    int32
	getEmployeeData  Employee
	getEmployeeErr   error
	getEmployeeCalls int

	createLocationCalls []locationCall
	createLocationErr   error
	updateLocationCalls []locationCall
	updateLocationErr   error

	createTechCalls []techCall
	createTechErr   error
	updateTechCalls []techCall
	updateTechErr   error

	createAvailabilityCalls []availabilityCall
	createAvailabilityErr   error
	updateAvailabilityCalls []availabilityCall
	updateAvailabilityErr   error

	createEducationCalls []educationCall
	createEducationErr   error
	updateEducationCalls []educationCall
	updateEducationErr   error
}

type createEmployeeCall struct {
	Ctx         context.Context
	Req         CreateEmployeeRequest
	UserID      int32
	File        multipart.File
	Filename    string
	ContentType string
	Size        int64
}

type updateEmployeeCall struct {
	Ctx         context.Context
	EmployeeID  int32
	Req         CreateEmployeeRequest
	File        multipart.File
	Filename    string
	ContentType string
	Size        int64
}

type locationCall struct {
	Ctx        context.Context
	EmployeeID int32
	Req        CreateEmployeeLocationRequest
}

type techCall struct {
	Ctx        context.Context
	EmployeeID int32
	Req        CreateEmployeeTechRequest
}

type availabilityCall struct {
	Ctx        context.Context
	EmployeeID int32
	Req        CreateEmployeeProfileAvailabilityRequest
}

type educationCall struct {
	Ctx        context.Context
	EmployeeID int32
	Req        CreateEmployeeEducationRequest
	Documents  []EducationDocumentUpload
}

func (f *fakeEmployeeService) CreateEmployee(ctx context.Context, employeeReq CreateEmployeeRequest, userID int32, file multipart.File, filename, contentType string, size int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createEmployeeCalls = append(f.createEmployeeCalls, createEmployeeCall{
		Ctx:         ctx,
		Req:         employeeReq,
		UserID:      userID,
		File:        file,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
	})
	return f.createEmployeeErr
}

func (f *fakeEmployeeService) UpdateEmployee(ctx context.Context, employeeID int32, employeeReq CreateEmployeeRequest, file multipart.File, filename, contentType string, size int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateEmployeeCalls = append(f.updateEmployeeCalls, updateEmployeeCall{
		Ctx:         ctx,
		EmployeeID:  employeeID,
		Req:         employeeReq,
		File:        file,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
	})
	return f.updateEmployeeErr
}

func (f *fakeEmployeeService) GetEmployee(ctx context.Context, ID int32) (Employee, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getEmployeeCalls++
	f.getEmployeeID = ID
	return f.getEmployeeData, f.getEmployeeErr
}

func (f *fakeEmployeeService) CreateLocation(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createLocationCalls = append(f.createLocationCalls, locationCall{Ctx: ctx, EmployeeID: employeeID, Req: locationRequest})
	return f.createLocationErr
}

func (f *fakeEmployeeService) UpdateLocation(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateLocationCalls = append(f.updateLocationCalls, locationCall{Ctx: ctx, EmployeeID: employeeID, Req: locationRequest})
	return f.updateLocationErr
}

func (f *fakeEmployeeService) CreateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createTechCalls = append(f.createTechCalls, techCall{Ctx: ctx, EmployeeID: employeeID, Req: techRequest})
	return f.createTechErr
}

func (f *fakeEmployeeService) UpdateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateTechCalls = append(f.updateTechCalls, techCall{Ctx: ctx, EmployeeID: employeeID, Req: techRequest})
	return f.updateTechErr
}

func (f *fakeEmployeeService) CreateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createAvailabilityCalls = append(f.createAvailabilityCalls, availabilityCall{Ctx: ctx, EmployeeID: employeeID, Req: availabilityRequest})
	return f.createAvailabilityErr
}

func (f *fakeEmployeeService) UpdateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateAvailabilityCalls = append(f.updateAvailabilityCalls, availabilityCall{Ctx: ctx, EmployeeID: employeeID, Req: availabilityRequest})
	return f.updateAvailabilityErr
}

func (f *fakeEmployeeService) CreateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest, documents []EducationDocumentUpload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createEducationCalls = append(f.createEducationCalls, educationCall{Ctx: ctx, EmployeeID: employeeID, Req: educationRequest, Documents: documents})
	return f.createEducationErr
}

func (f *fakeEmployeeService) UpdateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest, documents []EducationDocumentUpload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateEducationCalls = append(f.updateEducationCalls, educationCall{Ctx: ctx, EmployeeID: employeeID, Req: educationRequest, Documents: documents})
	return f.updateEducationErr
}

type fakeEmployeeStore struct {
	mu sync.Mutex

	createEmployeeCalls []storeCreateEmployeeCall
	createEmployeeID    int32
	createEmployeeErr   error

	updateEmployeeCalls []storeUpdateEmployeeCall
	updateEmployeeErr   error

	getEmployeeCalls []storeGetEmployeeCall
	getEmployeeData  Employee
	getEmployeeErr   error

	createLocationErr     error
	updateLocationErr     error
	createTechErr         error
	updateTechErr         error
	createAvailabilityErr error
	updateAvailabilityErr error
	createEducationErr    error
	updateEducationErr    error
}

type storeCreateEmployeeCall struct {
	Ctx    context.Context
	Req    CreateEmployeeRequest
	UserID int32
	File   *EmployeeFileMetadata
}

type storeUpdateEmployeeCall struct {
	Ctx        context.Context
	EmployeeID int32
	Req        CreateEmployeeRequest
	File       *EmployeeFileMetadata
}

type storeGetEmployeeCall struct {
	Ctx context.Context
	ID  int32
}

func (f *fakeEmployeeStore) CreateEmployee(ctx context.Context, employee CreateEmployeeRequest, userID int32, file *EmployeeFileMetadata) (int32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createEmployeeCalls = append(f.createEmployeeCalls, storeCreateEmployeeCall{Ctx: ctx, Req: employee, UserID: userID, File: file})
	return f.createEmployeeID, f.createEmployeeErr
}

func (f *fakeEmployeeStore) UpdateEmployee(ctx context.Context, employeeID int32, employee CreateEmployeeRequest, file *EmployeeFileMetadata) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateEmployeeCalls = append(f.updateEmployeeCalls, storeUpdateEmployeeCall{Ctx: ctx, EmployeeID: employeeID, Req: employee, File: file})
	return f.updateEmployeeErr
}

func (f *fakeEmployeeStore) CreateLocationWithConnections(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createLocationErr
}

func (f *fakeEmployeeStore) UpdateLocationWithConnections(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.updateLocationErr
}

func (f *fakeEmployeeStore) CreateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createTechErr
}

func (f *fakeEmployeeStore) UpdateTech(ctx context.Context, employeeID int32, techRequest CreateEmployeeTechRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.updateTechErr
}

func (f *fakeEmployeeStore) CreateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createAvailabilityErr
}

func (f *fakeEmployeeStore) UpdateAvailability(ctx context.Context, employeeID int32, availabilityRequest CreateEmployeeProfileAvailabilityRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.updateAvailabilityErr
}

func (f *fakeEmployeeStore) CreateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createEducationErr
}

func (f *fakeEmployeeStore) UpdateEducation(ctx context.Context, employeeID int32, educationRequest CreateEmployeeEducationRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.updateEducationErr
}

func (f *fakeEmployeeStore) GetEmployee(ctx context.Context, ID int32) (Employee, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getEmployeeCalls = append(f.getEmployeeCalls, storeGetEmployeeCall{Ctx: ctx, ID: ID})
	return f.getEmployeeData, f.getEmployeeErr
}

type fakeUploader struct {
	mu sync.Mutex

	uploadCalls []uploader.UploadInput
	uploadOut   *transfermanager.UploadObjectOutput
	uploadErr   error

	deleteCalls chan deleteCall
	deleteErr   error
}

type deleteCall struct {
	Bucket string
	Key    string
}

func newFakeUploader() *fakeUploader {
	return &fakeUploader{
		deleteCalls: make(chan deleteCall, 10),
		uploadOut: &transfermanager.UploadObjectOutput{
			Bucket:         aws.String("test-bucket"),
			Key:            aws.String("employees/cert.pdf"),
			ChecksumSHA256: aws.String("deadbeef"),
		},
	}
}

func (f *fakeUploader) Upload(ctx context.Context, input uploader.UploadInput) (*transfermanager.UploadObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uploadCalls = append(f.uploadCalls, input)
	if f.uploadErr != nil {
		return nil, f.uploadErr
	}
	return f.uploadOut, nil
}

func (f *fakeUploader) Delete(ctx context.Context, bucket, key string) error {
	f.deleteCalls <- deleteCall{Bucket: bucket, Key: key}
	return f.deleteErr
}
