package jobposition

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

const (
	testUserID        int32 = 42
	testEmployerID    int32 = 7
	testJobPositionID int32 = 11
)

type fakeJobPositionStore struct {
	employerID    int32
	employerErr   error
	createResult  JobPosition
	createErr     error
	getResult     JobPosition
	getErr        error
	listResult    []JobPosition
	listErr       error
	updateResult  JobPosition
	updateErr     error
	deleteErr     error
	employerCalls int
	createCalls   int
	getCalls      int
	listCalls     int
	updateCalls   int
	deleteCalls   int
}

func (f *fakeJobPositionStore) GetEmployerIDByUserID(context.Context, int32) (int32, error) {
	f.employerCalls++
	return f.employerID, f.employerErr
}

func (f *fakeJobPositionStore) CreateJobPosition(context.Context, int32, CreateJobPositionRequest) (JobPosition, error) {
	f.createCalls++
	return f.createResult, f.createErr
}

func (f *fakeJobPositionStore) GetActiveJobPositionByID(context.Context, int32) (JobPosition, error) {
	f.getCalls++
	return f.getResult, f.getErr
}

func (f *fakeJobPositionStore) ListActiveJobPositionsByEmployer(context.Context, int32) ([]JobPosition, error) {
	f.listCalls++
	return f.listResult, f.listErr
}

func (f *fakeJobPositionStore) UpdateActiveJobPosition(context.Context, int32, UpdateJobPositionRequest) (JobPosition, error) {
	f.updateCalls++
	return f.updateResult, f.updateErr
}

func (f *fakeJobPositionStore) SoftDeleteJobPosition(context.Context, int32) error {
	f.deleteCalls++
	return f.deleteErr
}

func (f *fakeJobPositionStore) writeCalls() int {
	return f.createCalls + f.updateCalls + f.deleteCalls
}

type fakePublisher struct {
	err       error
	published []JobPosition
}

func (f *fakePublisher) JobPositionPublished(_ context.Context, position JobPosition) error {
	f.published = append(f.published, position)
	return f.err
}

func employerPrincipal() auth.Principal {
	return auth.Principal{UserID: testUserID, Role: user.UserRoleEmployer}
}

func employeePrincipal() auth.Principal {
	return auth.Principal{UserID: testUserID, Role: user.UserRoleEmployee}
}

func ownedStore() *fakeJobPositionStore {
	return &fakeJobPositionStore{
		employerID:   testEmployerID,
		createResult: newTestJobPosition(),
		getResult:    newTestJobPosition(),
		updateResult: newTestJobPosition(),
	}
}

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
	foreign := newTestJobPosition()
	foreign.EmployerID = testEmployerID + 1

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
		if len(publisher.published) != 1 || !reflect.DeepEqual(publisher.published[0], newTestJobPosition()) {
			t.Fatalf("published = %#v, want exactly the created job position", publisher.published)
		}
	})

	t.Run("update notifies once", func(t *testing.T) {
		publisher := &fakePublisher{}
		_, err := NewService(ownedStore(), publisher).UpdateJobPosition(context.Background(), testJobPositionID, newTestCreateJobPositionRequest(), employerPrincipal())
		if err != nil {
			t.Fatalf("UpdateJobPosition() error = %v", err)
		}
		if len(publisher.published) != 1 {
			t.Fatalf("published = %#v, want exactly one notification", publisher.published)
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
			store: ownedStore,
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
	if err := (NoopEventPublisher{}).JobPositionPublished(context.Background(), newTestJobPosition()); err != nil {
		t.Fatalf("JobPositionPublished() error = %v, want nil", err)
	}
}
