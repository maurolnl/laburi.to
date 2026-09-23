package jobposition

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

// operation ejerce cada punto de entrada del servicio con la misma firma para poder
// recorrer las cinco operaciones en las tablas de autorización y ownership.
type operation struct {
	name string
	run  func(JobPositionService, auth.Principal) error
}

func allOperations() []operation {
	request := newTestCreateJobPositionRequest()
	return []operation{
		{"create", func(s JobPositionService, p auth.Principal) error {
			_, err := s.CreateJobPosition(context.Background(), testEmployerID, request, p)
			return err
		}},
		{"list", func(s JobPositionService, p auth.Principal) error {
			_, err := s.ListJobPositions(context.Background(), testEmployerID, p)
			return err
		}},
		{"get", func(s JobPositionService, p auth.Principal) error {
			_, err := s.GetJobPosition(context.Background(), testJobPositionID, p)
			return err
		}},
		{"update", func(s JobPositionService, p auth.Principal) error {
			_, err := s.UpdateJobPosition(context.Background(), testJobPositionID, request, p)
			return err
		}},
		{"delete", func(s JobPositionService, p auth.Principal) error {
			return s.DeleteJobPosition(context.Background(), testJobPositionID, p)
		}},
	}
}

func TestJobPositionServiceRejectsNonEmployerRole(t *testing.T) {
	for _, op := range allOperations() {
		t.Run(op.name, func(t *testing.T) {
			store := ownedStore()
			err := op.run(NewService(store, &fakePublisher{}), employeePrincipal())

			if !errors.Is(err, user.ErrProfileRoleForbidden) {
				t.Fatalf("%s error = %v, want %v", op.name, err, user.ErrProfileRoleForbidden)
			}
			if store.employerCalls != 0 || store.writeCalls() != 0 || store.getCalls != 0 || store.listCalls != 0 {
				t.Fatalf("%s reached the store with an employee principal: %#v", op.name, store)
			}
		})
	}
}

func TestJobPositionServiceRejectsPrincipalWithoutEmployerProfile(t *testing.T) {
	for _, op := range allOperations() {
		t.Run(op.name, func(t *testing.T) {
			store := ownedStore()
			store.employerErr = ErrEmployerProfileRequired
			err := op.run(NewService(store, &fakePublisher{}), employerPrincipal())

			if !errors.Is(err, ErrEmployerProfileRequired) {
				t.Fatalf("%s error = %v, want %v", op.name, err, ErrEmployerProfileRequired)
			}
			if store.writeCalls() != 0 || store.getCalls != 0 || store.listCalls != 0 {
				t.Fatalf("%s operated on job positions without an employer profile: %#v", op.name, store)
			}
		})
	}
}

func TestJobPositionServiceRejectsForeignCollection(t *testing.T) {
	store := ownedStore()
	store.employerID = testEmployerID + 1
	service := NewService(store, &fakePublisher{})

	if _, err := service.CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest(), employerPrincipal()); !errors.Is(err, ErrJobPositionForbidden) {
		t.Fatalf("CreateJobPosition() error = %v, want %v", err, ErrJobPositionForbidden)
	}
	if _, err := service.ListJobPositions(context.Background(), testEmployerID, employerPrincipal()); !errors.Is(err, ErrJobPositionForbidden) {
		t.Fatalf("ListJobPositions() error = %v, want %v", err, ErrJobPositionForbidden)
	}
	if store.createCalls != 0 || store.listCalls != 0 {
		t.Fatalf("store was reached for a foreign employer: %#v", store)
	}
}

func TestJobPositionServiceRejectsForeignPosition(t *testing.T) {
	foreign := newTestJobPosition(withTestJobPositionEmployerID(testEmployerID + 1))

	for _, op := range allOperations()[2:] {
		t.Run(op.name, func(t *testing.T) {
			store := ownedStore()
			store.getResult = foreign
			err := op.run(NewService(store, &fakePublisher{}), employerPrincipal())

			if !errors.Is(err, ErrJobPositionForbidden) {
				t.Fatalf("%s error = %v, want %v", op.name, err, ErrJobPositionForbidden)
			}
			if store.writeCalls() != 0 {
				t.Fatalf("%s wrote a job position owned by another employer: %#v", op.name, store)
			}
		})
	}
}

func TestJobPositionServiceReportsDeletedPositionAsNotFound(t *testing.T) {
	for _, op := range allOperations()[2:] {
		t.Run(op.name, func(t *testing.T) {
			store := ownedStore()
			store.getErr = ErrJobPositionNotFound
			err := op.run(NewService(store, &fakePublisher{}), employerPrincipal())

			if !errors.Is(err, ErrJobPositionNotFound) {
				t.Fatalf("%s error = %v, want %v", op.name, err, ErrJobPositionNotFound)
			}
			if store.writeCalls() != 0 {
				t.Fatalf("%s wrote a deleted job position: %#v", op.name, store)
			}
		})
	}
}

func TestJobPositionServiceCreateAndUpdateSucceed(t *testing.T) {
	store := ownedStore()
	service := NewService(store, &fakePublisher{})

	created, err := service.CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest(), employerPrincipal())
	if err != nil || !reflect.DeepEqual(created, newTestJobPosition()) {
		t.Fatalf("CreateJobPosition() = %#v, %v", created, err)
	}

	updated, err := service.UpdateJobPosition(context.Background(), testJobPositionID, newTestCreateJobPositionRequest(), employerPrincipal())
	if err != nil || !reflect.DeepEqual(updated, newTestJobPosition()) {
		t.Fatalf("UpdateJobPosition() = %#v, %v", updated, err)
	}

	if err := service.DeleteJobPosition(context.Background(), testJobPositionID, employerPrincipal()); err != nil {
		t.Fatalf("DeleteJobPosition() error = %v", err)
	}
	if store.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", store.deleteCalls)
	}
}

func TestJobPositionServiceListNeverReturnsNil(t *testing.T) {
	store := ownedStore()
	store.listResult = nil
	got, err := NewService(store, &fakePublisher{}).ListJobPositions(context.Background(), testEmployerID, employerPrincipal())

	if err != nil {
		t.Fatalf("ListJobPositions() error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("ListJobPositions() = %#v, want an empty non-nil slice", got)
	}
}

func TestJobPositionServiceNotifiesPublisherOnlyOnSuccess(t *testing.T) {
	t.Run("create notifies once", func(t *testing.T) {
		publisher := &fakePublisher{}
		_, err := NewService(ownedStore(), publisher).CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest(), employerPrincipal())
		if err != nil {
			t.Fatalf("CreateJobPosition() error = %v", err)
		}
		if len(publisher.published) != 1 || publisher.published[0] != newTestJobPosition().ID {
			t.Fatalf("published = %#v, want exactly the created job position id", publisher.published)
		}
	})

	t.Run("update notifies once", func(t *testing.T) {
		publisher := &fakePublisher{}
		_, err := NewService(ownedStore(), publisher).UpdateJobPosition(context.Background(), testJobPositionID, newTestCreateJobPositionRequest(), employerPrincipal())
		if err != nil {
			t.Fatalf("UpdateJobPosition() error = %v", err)
		}
		if len(publisher.published) != 1 || publisher.published[0] != newTestJobPosition().ID {
			t.Fatalf("published = %#v, want exactly the updated job position id", publisher.published)
		}
	})

	t.Run("delete does not notify", func(t *testing.T) {
		publisher := &fakePublisher{}
		if err := NewService(ownedStore(), publisher).DeleteJobPosition(context.Background(), testJobPositionID, employerPrincipal()); err != nil {
			t.Fatalf("DeleteJobPosition() error = %v", err)
		}
		if len(publisher.published) != 0 {
			t.Fatalf("published = %#v, want no notification", publisher.published)
		}
	})

	rejections := []struct {
		name  string
		store func() *fakeJobPositionStore
		run   func(JobPositionService) error
	}{
		{
			name:  "rejected by role",
			store: func() *fakeJobPositionStore { return ownedStore() },
			run: func(s JobPositionService) error {
				_, err := s.CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest(), employeePrincipal())
				return err
			},
		},
		{
			name: "rejected by ownership",
			store: func() *fakeJobPositionStore {
				store := ownedStore()
				store.employerID = testEmployerID + 1
				return store
			},
			run: func(s JobPositionService) error {
				_, err := s.CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest(), employerPrincipal())
				return err
			},
		},
		{
			name: "rejected by persistence",
			store: func() *fakeJobPositionStore {
				store := ownedStore()
				store.createErr = errors.New("database unavailable")
				return store
			},
			run: func(s JobPositionService) error {
				_, err := s.CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest(), employerPrincipal())
				return err
			},
		},
	}

	for _, tt := range rejections {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &fakePublisher{}
			if err := tt.run(NewService(tt.store(), publisher)); err == nil {
				t.Fatal("expected the operation to be rejected")
			}
			if len(publisher.published) != 0 {
				t.Fatalf("published = %#v, want no notification for a rejected operation", publisher.published)
			}
		})
	}
}

func TestJobPositionServiceSurvivesPublisherFailure(t *testing.T) {
	publisher := &fakePublisher{err: errors.New("queue unavailable")}
	got, err := NewService(ownedStore(), publisher).CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest(), employerPrincipal())

	if err != nil {
		t.Fatalf("CreateJobPosition() error = %v, want the persisted position despite the publisher failure", err)
	}
	if !reflect.DeepEqual(got, newTestJobPosition()) {
		t.Fatalf("CreateJobPosition() = %#v, want the persisted position", got)
	}
}

func TestNoopEventPublisherSucceeds(t *testing.T) {
	if err := (NoopEventPublisher{}).JobPositionPublished(context.Background(), testJobPositionID); err != nil {
		t.Fatalf("JobPositionPublished() error = %v, want nil", err)
	}
}

// TestJobPositionServiceDerivesEmployerFromPrincipal comprueba que el empleador con el que
// se persiste el puesto es el resuelto desde el JWT, y que la request llega al store tal
// como la recibió el servicio, incluidos los recursos técnicos.
func TestJobPositionServiceDerivesEmployerFromPrincipal(t *testing.T) {
	option := withTestJobPositionResources([]string{"Laptop", "VPN", "Monitor"})
	store := ownedStore(option)
	request := newTestCreateJobPositionRequest(option)

	if _, err := NewService(store, &fakePublisher{}).CreateJobPosition(context.Background(), testEmployerID, request, employerPrincipal()); err != nil {
		t.Fatalf("CreateJobPosition() error = %v", err)
	}
	if store.createEmployerID != testEmployerID {
		t.Fatalf("store employerID = %d, want %d resolved from the principal", store.createEmployerID, testEmployerID)
	}
	if !reflect.DeepEqual(store.createRequest, request) {
		t.Fatalf("store request = %#v, want %#v", store.createRequest, request)
	}

	updateStore := ownedStore(option)
	if _, err := NewService(updateStore, &fakePublisher{}).UpdateJobPosition(context.Background(), testJobPositionID, request, employerPrincipal()); err != nil {
		t.Fatalf("UpdateJobPosition() error = %v", err)
	}
	if !reflect.DeepEqual(updateStore.updateRequest, request) {
		t.Fatalf("store update request = %#v, want %#v", updateStore.updateRequest, request)
	}
}

// TestJobPositionEventPublisherPortIsAlwaysADouble documenta que la suite no habla con SQS:
// el puerto se satisface con el doble del paquete y con la implementación vigente del
// binario, y ninguna de las dos abre red ni lee entorno.
func TestJobPositionEventPublisherPortIsAlwaysADouble(t *testing.T) {
	var _ JobPositionEventPublisher = &fakePublisher{}
	var _ JobPositionEventPublisher = NoopEventPublisher{}

	publisher := &fakePublisher{}
	if err := publisher.JobPositionPublished(context.Background(), testJobPositionID); err != nil {
		t.Fatalf("fake publisher error = %v", err)
	}
	if len(publisher.published) != 1 || publisher.published[0] != testJobPositionID {
		t.Fatalf("published = %#v, want the notified job position id", publisher.published)
	}

	// Un servicio sin publicador no debe romperse: el puerto es opcional mientras la épica
	// de recomendaciones no exista.
	if _, err := NewService(ownedStore(), nil).CreateJobPosition(context.Background(), testEmployerID, newTestCreateJobPositionRequest(), employerPrincipal()); err != nil {
		t.Fatalf("CreateJobPosition() with a nil publisher error = %v", err)
	}
}
