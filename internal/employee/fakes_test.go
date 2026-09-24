package employee

import (
	"context"
	"mime/multipart"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
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

	profileData  EmployeeProfileResponse
	profileErr   error
	profileCalls []int32

	downloadData  DownloadURLResponse
	downloadErr   error
	downloadCalls []storeFileCall

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
	Principal   auth.Principal
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

func (f *fakeEmployeeService) CreateEmployee(ctx context.Context, employeeReq CreateEmployeeRequest, principal auth.Principal, file multipart.File, filename, contentType string, size int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createEmployeeCalls = append(f.createEmployeeCalls, createEmployeeCall{
		Ctx:         ctx,
		Req:         employeeReq,
		Principal:   principal,
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

// Los tres métodos de lectura por identificador existen para satisfacer la interfaz. Los tests
// de esas rutas no usan este doble: arman el handler sobre el servicio real con store, uploader
// y acceso falsos, porque lo que verifican es justamente la autorización y el presignado, que
// viven en el servicio.
func (f *fakeEmployeeService) GetEmployeeProfile(_ context.Context, employeeID int32, _ auth.Principal) (EmployeeProfileResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profileCalls = append(f.profileCalls, employeeID)
	return f.profileData, f.profileErr
}

func (f *fakeEmployeeService) CertificateDownloadURL(_ context.Context, employeeID, fileID int32, _ auth.Principal) (DownloadURLResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downloadCalls = append(f.downloadCalls, storeFileCall{EmployeeID: employeeID, FileID: fileID})
	return f.downloadData, f.downloadErr
}

func (f *fakeEmployeeService) EducationDocumentDownloadURL(_ context.Context, employeeID, educationID int32, _ auth.Principal) (DownloadURLResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downloadCalls = append(f.downloadCalls, storeFileCall{EmployeeID: employeeID, FileID: educationID})
	return f.downloadData, f.downloadErr
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

	// profileComplete es la respuesta de IsProfileComplete y profileCompleteCalls registra por
	// qué empleado se preguntó. El valor por defecto es false: un test que espera notificación
	// tiene que declararlo, así que no hay disparo accidental.
	profileComplete      bool
	profileCompleteErr   error
	profileCompleteCalls []int32

	// El perfil por identificador, el certificado y el documento de título se configuran por
	// separado porque los tests de autorización y los de entrega miran cosas distintas: unos,
	// que la lectura ni siquiera ocurra; otros, qué archivo se firmó.
	ownerData  Employee
	ownerErr   error
	ownerCalls []int32

	profileData  EmployeeProfile
	profileErr   error
	profileCalls []int32

	fileData  StoredFile
	fileErr   error
	fileCalls []storeFileCall

	educationDocumentData  StoredFile
	educationDocumentErr   error
	educationDocumentCalls []storeFileCall
}

// storeFileCall registra el par con el que se pidió un archivo. El par completo importa: el
// filtro por empleado es lo que impide entregar el archivo de otro.
type storeFileCall struct {
	EmployeeID int32
	FileID     int32
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

func (f *fakeEmployeeStore) IsProfileComplete(_ context.Context, employeeID int32) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profileCompleteCalls = append(f.profileCompleteCalls, employeeID)
	return f.profileComplete, f.profileCompleteErr
}

func (f *fakeEmployeeStore) GetEmployee(ctx context.Context, ID int32) (Employee, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getEmployeeCalls = append(f.getEmployeeCalls, storeGetEmployeeCall{Ctx: ctx, ID: ID})
	return f.getEmployeeData, f.getEmployeeErr
}

// GetEmployeeByID es la lectura mínima con la que el servicio resuelve la propiedad. El doble
// la separa del perfil completo a propósito: los tests de autorización comprueban que la
// decisión se toma con esta y no leyendo el perfil entero de alguien ajeno.
func (f *fakeEmployeeStore) GetEmployeeByID(_ context.Context, employeeID int32) (Employee, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ownerCalls = append(f.ownerCalls, employeeID)
	return f.ownerData, f.ownerErr
}

func (f *fakeEmployeeStore) GetEmployeeProfileByID(_ context.Context, employeeID int32) (EmployeeProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profileCalls = append(f.profileCalls, employeeID)
	return f.profileData, f.profileErr
}

func (f *fakeEmployeeStore) GetEmployeeFile(_ context.Context, employeeID, fileID int32) (StoredFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fileCalls = append(f.fileCalls, storeFileCall{EmployeeID: employeeID, FileID: fileID})
	return f.fileData, f.fileErr
}

func (f *fakeEmployeeStore) GetEmployeeEducationDocument(_ context.Context, employeeID, educationID int32) (StoredFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.educationDocumentCalls = append(f.educationDocumentCalls, storeFileCall{EmployeeID: employeeID, FileID: educationID})
	return f.educationDocumentData, f.educationDocumentErr
}

type fakeUploader struct {
	mu sync.Mutex

	uploadCalls []uploader.UploadInput
	uploadOut   *transfermanager.UploadObjectOutput
	uploadErr   error

	deleteCalls chan deleteCall
	deleteErr   error

	presignCalls []uploader.PresignInput
	presignErr   error
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

// PresignGetObject registra con qué bucket, clave, nombre y plazo se firmó, y devuelve una URL
// determinística derivada de la clave. Es lo que permite verificar «la URL corresponde al
// archivo pedido» y «el plazo es el declarado» sin firmar de verdad ni abrir red.
func (f *fakeUploader) PresignGetObject(_ context.Context, input uploader.PresignInput) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.presignCalls = append(f.presignCalls, input)
	if f.presignErr != nil {
		return "", f.presignErr
	}
	return "https://s3.test/signed/" + input.Key, nil
}

func (f *fakeUploader) lastPresign() uploader.PresignInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.presignCalls) == 0 {
		return uploader.PresignInput{}
	}
	return f.presignCalls[len(f.presignCalls)-1]
}

// fakeRecommendationAccess es el doble del puerto que decide si un empleador puede mirar un
// perfil ajeno. Registra el par consultado para que los tests puedan comprobar que la
// autorización preguntó por el principal del JWT y no por un identificador del path.
type fakeRecommendationAccess struct {
	mu sync.Mutex

	allowed bool
	err     error
	calls   []accessCall
}

type accessCall struct {
	EmployeeID     int32
	EmployerUserID int32
}

func (f *fakeRecommendationAccess) EmployerHasCurrentRecommendation(_ context.Context, employeeID, employerUserID int32) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, accessCall{EmployeeID: employeeID, EmployerUserID: employerUserID})
	return f.allowed, f.err
}

// fakeEmployeePublisher es el doble del puerto de recomendaciones. Toda la suite lo usa en
// lugar de SQS: ninguna prueba del paquete abre red, lee entorno ni necesita credenciales.
//
// Registra identificadores y no perfiles porque eso es todo lo que el puerto transporta: si
// alguna vez recibiera el perfil, este doble dejaría de compilar.
type fakeEmployeePublisher struct {
	mu sync.Mutex

	err       error
	published []int32
}

var _ EmployeeEventPublisher = (*fakeEmployeePublisher)(nil)

func (f *fakeEmployeePublisher) EmployeeProfileCompleted(_ context.Context, employeeID int32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, employeeID)
	return f.err
}

func (f *fakeEmployeePublisher) calls() []int32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int32(nil), f.published...)
}
