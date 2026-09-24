package employee

import (
	"context"
	"errors"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

// testTriggerEmployeeID es el empleado sobre el que operan estos tests. Coincide con
// createEmployeeID del store falso para que el alta notifique ese mismo identificador.
const testTriggerEmployeeID int32 = 7

// profileWrite es una de las diez escrituras de perfil que pueden disparar la regeneración.
// failStore configura el fallo de persistencia de esa operación concreta, que es lo que
// distingue "no notificó porque el perfil está incompleto" de "no notificó porque no hubo
// cambio".
type profileWrite struct {
	name      string
	run       func(*employeeService) error
	failStore func(*fakeEmployeeStore, error)
}

func profileWrites() []profileWrite {
	principal := auth.Principal{UserID: 1, Role: user.UserRoleEmployee}

	return []profileWrite{
		{
			name: "create employee",
			run: func(s *employeeService) error {
				return s.CreateEmployee(context.Background(), CreateEmployeeRequest{}, principal, nil, "", "", 0)
			},
			failStore: func(store *fakeEmployeeStore, err error) { store.createEmployeeErr = err },
		},
		{
			name: "update employee",
			run: func(s *employeeService) error {
				return s.UpdateEmployee(context.Background(), testTriggerEmployeeID, CreateEmployeeRequest{}, nil, "", "", 0)
			},
			failStore: func(store *fakeEmployeeStore, err error) { store.updateEmployeeErr = err },
		},
		{
			name: "create location",
			run: func(s *employeeService) error {
				return s.CreateLocation(context.Background(), testTriggerEmployeeID, CreateEmployeeLocationRequest{})
			},
			failStore: func(store *fakeEmployeeStore, err error) { store.createLocationErr = err },
		},
		{
			name: "update location",
			run: func(s *employeeService) error {
				return s.UpdateLocation(context.Background(), testTriggerEmployeeID, CreateEmployeeLocationRequest{})
			},
			failStore: func(store *fakeEmployeeStore, err error) { store.updateLocationErr = err },
		},
		{
			name: "create tech",
			run: func(s *employeeService) error {
				return s.CreateTech(context.Background(), testTriggerEmployeeID, CreateEmployeeTechRequest{})
			},
			failStore: func(store *fakeEmployeeStore, err error) { store.createTechErr = err },
		},
		{
			name: "update tech",
			run: func(s *employeeService) error {
				return s.UpdateTech(context.Background(), testTriggerEmployeeID, CreateEmployeeTechRequest{})
			},
			failStore: func(store *fakeEmployeeStore, err error) { store.updateTechErr = err },
		},
		{
			name: "create availability",
			run: func(s *employeeService) error {
				return s.CreateAvailability(context.Background(), testTriggerEmployeeID, CreateEmployeeProfileAvailabilityRequest{})
			},
			failStore: func(store *fakeEmployeeStore, err error) { store.createAvailabilityErr = err },
		},
		{
			name: "update availability",
			run: func(s *employeeService) error {
				return s.UpdateAvailability(context.Background(), testTriggerEmployeeID, CreateEmployeeProfileAvailabilityRequest{})
			},
			failStore: func(store *fakeEmployeeStore, err error) { store.updateAvailabilityErr = err },
		},
		{
			name: "create education",
			run: func(s *employeeService) error {
				return s.CreateEducation(context.Background(), testTriggerEmployeeID, CreateEmployeeEducationRequest{}, nil)
			},
			failStore: func(store *fakeEmployeeStore, err error) { store.createEducationErr = err },
		},
		{
			name: "update education",
			run: func(s *employeeService) error {
				return s.UpdateEducation(context.Background(), testTriggerEmployeeID, CreateEmployeeEducationRequest{}, nil)
			},
			failStore: func(store *fakeEmployeeStore, err error) { store.updateEducationErr = err },
		},
	}
}

// newTriggerService arma el servicio con el store falso ya apuntando al empleado de estos
// tests, de modo que el alta devuelva el mismo identificador que las demás operaciones reciben
// por parámetro.
func newTriggerService(t *testing.T) (*employeeService, *fakeEmployeeStore, *fakeEmployeePublisher) {
	t.Helper()
	service, store, _, publisher := newTestServiceWithPublisher(t)
	store.createEmployeeID = testTriggerEmployeeID
	return service, store, publisher
}

// TestProfileWritesNotifyWhenProfileIsComplete cubre el disparador en las diez escrituras de
// perfil: cada una notifica exactamente una vez, con el identificador del empleado que acaba de
// cambiar.
func TestProfileWritesNotifyWhenProfileIsComplete(t *testing.T) {
	for _, write := range profileWrites() {
		t.Run(write.name, func(t *testing.T) {
			service, store, publisher := newTriggerService(t)
			store.profileComplete = true

			if err := write.run(service); err != nil {
				t.Fatalf("%s error = %v", write.name, err)
			}

			published := publisher.calls()
			if len(published) != 1 || published[0] != testTriggerEmployeeID {
				t.Fatalf("published = %v, want exactly employee %d", published, testTriggerEmployeeID)
			}
		})
	}
}

// TestProfileWritesDoNotNotifyWhenProfileIsIncomplete es el criterio de aceptación de perfiles
// incompletos: la escritura se persiste igual, pero no genera trabajo.
func TestProfileWritesDoNotNotifyWhenProfileIsIncomplete(t *testing.T) {
	for _, write := range profileWrites() {
		t.Run(write.name, func(t *testing.T) {
			service, store, publisher := newTriggerService(t)
			store.profileComplete = false

			if err := write.run(service); err != nil {
				t.Fatalf("%s error = %v", write.name, err)
			}

			if published := publisher.calls(); len(published) != 0 {
				t.Fatalf("published = %v, want no notification for an incomplete profile", published)
			}
			if len(store.profileCompleteCalls) != 1 {
				t.Fatalf("completeness resolved %d times, want exactly once", len(store.profileCompleteCalls))
			}
		})
	}
}

// TestProfileWritesDoNotNotifyOnPersistenceFailure comprueba que una escritura rechazada no
// solicita nada y ni siquiera pregunta por la completitud: sin cambio persistido no hay nada que
// regenerar.
func TestProfileWritesDoNotNotifyOnPersistenceFailure(t *testing.T) {
	cause := errors.New("persistence failed")

	for _, write := range profileWrites() {
		t.Run(write.name, func(t *testing.T) {
			service, store, publisher := newTriggerService(t)
			store.profileComplete = true
			write.failStore(store, cause)

			if err := write.run(service); !errors.Is(err, cause) {
				t.Fatalf("%s error = %v, want %v", write.name, err, cause)
			}

			if published := publisher.calls(); len(published) != 0 {
				t.Fatalf("published = %v, want no notification for a rejected write", published)
			}
			if len(store.profileCompleteCalls) != 0 {
				t.Fatalf("completeness resolved %d times for a rejected write, want none", len(store.profileCompleteCalls))
			}
		})
	}
}

// TestProfileWritesSurvivePublisherFailure es la estrategia ante fallo de emisión: el cambio ya
// está confirmado, así que el llamador recibe éxito y nada se revierte. La representación del
// fallo vive del lado del productor, que deja el batch del sujeto en failed.
func TestProfileWritesSurvivePublisherFailure(t *testing.T) {
	for _, write := range profileWrites() {
		t.Run(write.name, func(t *testing.T) {
			service, store, publisher := newTriggerService(t)
			store.profileComplete = true
			publisher.err = errors.New("queue unavailable")

			if err := write.run(service); err != nil {
				t.Fatalf("%s error = %v, want the write to survive the publisher failure", write.name, err)
			}

			if published := publisher.calls(); len(published) != 1 {
				t.Fatalf("published = %v, want exactly one attempted notification", published)
			}
		})
	}
}

// TestProfileWritesSurviveCompletenessFailure cubre el mismo criterio para el otro desenlace que
// ocurre después del cambio confirmado: si no se puede resolver la completitud, no se notifica y
// tampoco se convierte la escritura en un error.
func TestProfileWritesSurviveCompletenessFailure(t *testing.T) {
	for _, write := range profileWrites() {
		t.Run(write.name, func(t *testing.T) {
			service, store, publisher := newTriggerService(t)
			store.profileComplete = true
			store.profileCompleteErr = errors.New("database unavailable")

			if err := write.run(service); err != nil {
				t.Fatalf("%s error = %v, want the write to survive the completeness failure", write.name, err)
			}

			if published := publisher.calls(); len(published) != 0 {
				t.Fatalf("published = %v, want no notification when completeness is unknown", published)
			}
		})
	}
}

// TestEmployeeEventPublisherPortIsAlwaysADouble documenta que la suite no habla con SQS: el
// puerto se satisface con el doble del paquete y con la implementación inerte, y ninguna de las
// dos abre red ni lee entorno. Un servicio sin publicador tampoco debe romperse.
func TestEmployeeEventPublisherPortIsAlwaysADouble(t *testing.T) {
	var _ EmployeeEventPublisher = &fakeEmployeePublisher{}
	var _ EmployeeEventPublisher = NoopEventPublisher{}

	if err := (NoopEventPublisher{}).EmployeeProfileCompleted(context.Background(), testTriggerEmployeeID); err != nil {
		t.Fatalf("EmployeeProfileCompleted() error = %v, want nil", err)
	}

	store := &fakeEmployeeStore{profileComplete: true}
	service := NewService(store, newFakeUploader(), &fakeRecommendationAccess{}, nil).(*employeeService)
	if err := service.CreateTech(context.Background(), testTriggerEmployeeID, CreateEmployeeTechRequest{}); err != nil {
		t.Fatalf("CreateTech() with a nil publisher error = %v", err)
	}
	if len(store.profileCompleteCalls) != 0 {
		t.Fatalf("resolved completeness without a publisher to notify: %v", store.profileCompleteCalls)
	}
}
