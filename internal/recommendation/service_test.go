package recommendation

import (
	"context"
	"errors"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

const (
	queryEmployeeID    int32 = 11
	queryJobPositionID int32 = 22
	queryOwnerUserID   int32 = 33
	queryOtherUserID   int32 = 44
	queryEmployerID    int32 = 55
)

// queryStore es un doble del conjunto vigente. Solo implementa las dos lecturas: el resto del
// puerto existe para el worker y ningún escenario de consulta lo ejerce.
type queryStore struct {
	jobResult      JobRecommendations
	jobErr         error
	employeeResult EmployeeRecommendations
	employeeErr    error

	jobCalls      int
	employeeCalls int
	lastPage      Page
	lastSubjectID int32
}

var _ RecommendationStore = (*queryStore)(nil)

func (s *queryStore) CreateBatch(context.Context, Subject) (Batch, error) { return Batch{}, nil }
func (s *queryStore) GetBatch(context.Context, int32) (Batch, error)      { return Batch{}, nil }
func (s *queryStore) TransitionBatch(context.Context, int32, BatchStatus) (Batch, error) {
	return Batch{}, nil
}
func (s *queryStore) ClaimBatch(context.Context, int32) (Batch, error) { return Batch{}, nil }
func (s *queryStore) CompleteBatch(context.Context, int32, []Candidate) (Batch, error) {
	return Batch{}, nil
}

func (s *queryStore) JobRecommendationsForEmployee(_ context.Context, employeeID int32, page Page) (JobRecommendations, error) {
	s.jobCalls++
	s.lastSubjectID = employeeID
	s.lastPage = page
	return s.jobResult, s.jobErr
}

func (s *queryStore) EmployeeRecommendationsForJobPosition(_ context.Context, jobPositionID int32, page Page) (EmployeeRecommendations, error) {
	s.employeeCalls++
	s.lastSubjectID = jobPositionID
	s.lastPage = page
	return s.employeeResult, s.employeeErr
}

type queryOwnership struct {
	employeeOwner int32
	employeeErr   error
	jobOwner      JobPositionOwner
	jobErr        error
}

var _ SubjectOwnership = (*queryOwnership)(nil)

func (o *queryOwnership) EmployeeOwner(context.Context, int32) (int32, error) {
	return o.employeeOwner, o.employeeErr
}

func (o *queryOwnership) JobPositionOwner(context.Context, int32) (JobPositionOwner, error) {
	return o.jobOwner, o.jobErr
}

func ownedByPrincipal() *queryOwnership {
	return &queryOwnership{
		employeeOwner: queryOwnerUserID,
		jobOwner:      JobPositionOwner{EmployerID: queryEmployerID, UserID: queryOwnerUserID},
	}
}

func employeePrincipal() auth.Principal {
	return auth.Principal{UserID: queryOwnerUserID, Role: user.UserRoleEmployee}
}

func employerPrincipal() auth.Principal {
	return auth.Principal{UserID: queryOwnerUserID, Role: user.UserRoleEmployer}
}

func testPage() Page { return Page{Limit: 20, Offset: 0} }

func TestJobRecommendationsAutorizacion(t *testing.T) {
	ctx := context.Background()

	t.Run("perfil propio", func(t *testing.T) {
		store := &queryStore{jobResult: JobRecommendations{Status: BatchCompleted, Items: []JobRecommendation{{JobPositionID: queryJobPositionID}}, Total: 1}}
		service := NewQueryService(store, ownedByPrincipal())

		response, err := service.JobRecommendations(ctx, queryEmployeeID, testPage(), employeePrincipal())
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}
		if response.Status != StatusCompleted || len(response.Items) != 1 || response.Page.Total != 1 {
			t.Fatalf("respuesta inesperada: %+v", response)
		}
		if store.lastSubjectID != queryEmployeeID || store.lastPage != testPage() {
			t.Fatalf("el store recibió %d con la página %+v", store.lastSubjectID, store.lastPage)
		}
	})

	t.Run("perfil ajeno", func(t *testing.T) {
		store := &queryStore{}
		ownership := ownedByPrincipal()
		ownership.employeeOwner = queryOtherUserID
		service := NewQueryService(store, ownership)

		_, err := service.JobRecommendations(ctx, queryEmployeeID, testPage(), employeePrincipal())
		if !errors.Is(err, ErrRecommendationsForbidden) {
			t.Fatalf("se esperaba ErrRecommendationsForbidden, se obtuvo %v", err)
		}
		if store.jobCalls != 0 {
			t.Fatalf("un perfil ajeno no debería llegar al conjunto vigente, hubo %d lecturas", store.jobCalls)
		}
	})

	t.Run("rol incorrecto", func(t *testing.T) {
		store := &queryStore{}
		service := NewQueryService(store, ownedByPrincipal())

		_, err := service.JobRecommendations(ctx, queryEmployeeID, testPage(), employerPrincipal())
		if !errors.Is(err, user.ErrProfileRoleForbidden) {
			t.Fatalf("se esperaba ErrProfileRoleForbidden, se obtuvo %v", err)
		}
		if store.jobCalls != 0 {
			t.Fatalf("un rol incorrecto no debería llegar al conjunto vigente, hubo %d lecturas", store.jobCalls)
		}
	})

	t.Run("empleado inexistente", func(t *testing.T) {
		ownership := ownedByPrincipal()
		ownership.employeeErr = ErrSubjectNotFound
		service := NewQueryService(&queryStore{}, ownership)

		_, err := service.JobRecommendations(ctx, queryEmployeeID, testPage(), employeePrincipal())
		if !errors.Is(err, ErrSubjectNotFound) {
			t.Fatalf("se esperaba ErrSubjectNotFound, se obtuvo %v", err)
		}
	})
}

func TestEmployeeRecommendationsAutorizacion(t *testing.T) {
	ctx := context.Background()

	t.Run("puesto propio", func(t *testing.T) {
		store := &queryStore{employeeResult: EmployeeRecommendations{Status: BatchCompleted, Items: []EmployeeRecommendation{{EmployeeID: queryEmployeeID}}, Total: 1}}
		service := NewQueryService(store, ownedByPrincipal())

		response, err := service.EmployeeRecommendations(ctx, queryJobPositionID, testPage(), employerPrincipal())
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}
		if response.Status != StatusCompleted || len(response.Items) != 1 || response.Page.Total != 1 {
			t.Fatalf("respuesta inesperada: %+v", response)
		}
	})

	t.Run("puesto ajeno", func(t *testing.T) {
		store := &queryStore{}
		ownership := ownedByPrincipal()
		ownership.jobOwner = JobPositionOwner{EmployerID: queryEmployerID, UserID: queryOtherUserID}
		service := NewQueryService(store, ownership)

		_, err := service.EmployeeRecommendations(ctx, queryJobPositionID, testPage(), employerPrincipal())
		if !errors.Is(err, ErrRecommendationsForbidden) {
			t.Fatalf("se esperaba ErrRecommendationsForbidden, se obtuvo %v", err)
		}
		if store.employeeCalls != 0 {
			t.Fatalf("un puesto ajeno no debería llegar al conjunto vigente, hubo %d lecturas", store.employeeCalls)
		}
	})

	t.Run("rol incorrecto", func(t *testing.T) {
		service := NewQueryService(&queryStore{}, ownedByPrincipal())

		_, err := service.EmployeeRecommendations(ctx, queryJobPositionID, testPage(), employeePrincipal())
		if !errors.Is(err, user.ErrProfileRoleForbidden) {
			t.Fatalf("se esperaba ErrProfileRoleForbidden, se obtuvo %v", err)
		}
	})

	// Un empleador sin perfil creado no es dueño de ningún puesto, así que cae en el mismo 403
	// que el puesto ajeno sin necesitar una comprobación aparte.
	t.Run("empleador sin perfil de empleador", func(t *testing.T) {
		ownership := ownedByPrincipal()
		ownership.jobOwner = JobPositionOwner{EmployerID: queryEmployerID, UserID: queryOtherUserID}
		service := NewQueryService(&queryStore{}, ownership)

		principal := auth.Principal{UserID: 999, Role: user.UserRoleEmployer}
		if _, err := service.EmployeeRecommendations(ctx, queryJobPositionID, testPage(), principal); !errors.Is(err, ErrRecommendationsForbidden) {
			t.Fatalf("se esperaba ErrRecommendationsForbidden, se obtuvo %v", err)
		}
	})

	t.Run("puesto eliminado o inexistente", func(t *testing.T) {
		ownership := ownedByPrincipal()
		ownership.jobErr = ErrSubjectNotFound
		service := NewQueryService(&queryStore{}, ownership)

		_, err := service.EmployeeRecommendations(ctx, queryJobPositionID, testPage(), employerPrincipal())
		if !errors.Is(err, ErrSubjectNotFound) {
			t.Fatalf("se esperaba ErrSubjectNotFound, se obtuvo %v", err)
		}
	})
}

func TestEstadosExpuestosAlCliente(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		batch BatchStatus
		want  QueryStatus
	}{
		{name: "pendiente", batch: BatchPending, want: StatusPending},
		{name: "procesando", batch: BatchProcessing, want: StatusProcessing},
		{name: "completado", batch: BatchCompleted, want: StatusCompleted},
		{name: "fallido", batch: BatchFailed, want: StatusFailed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &queryStore{jobResult: JobRecommendations{Status: test.batch, Items: []JobRecommendation{}}}
			service := NewQueryService(store, ownedByPrincipal())

			response, err := service.JobRecommendations(ctx, queryEmployeeID, testPage(), employeePrincipal())
			if err != nil {
				t.Fatalf("no se esperaba error: %v", err)
			}
			if response.Status != test.want {
				t.Fatalf("se esperaba %q, se obtuvo %q", test.want, response.Status)
			}
		})
	}

	// Sin batch alguno el sujeto no está procesando ni falló: nunca se le pidió nada.
	t.Run("sin ningún batch", func(t *testing.T) {
		store := &queryStore{jobErr: ErrNoCurrentBatch}
		service := NewQueryService(store, ownedByPrincipal())

		response, err := service.JobRecommendations(ctx, queryEmployeeID, testPage(), employeePrincipal())
		if err != nil {
			t.Fatalf("la ausencia de batch no debería ser error: %v", err)
		}
		if response.Status != StatusNone || len(response.Items) != 0 || response.Page.Total != 0 {
			t.Fatalf("respuesta inesperada: %+v", response)
		}
	})

	t.Run("sin ningún batch en el sentido del puesto", func(t *testing.T) {
		store := &queryStore{employeeErr: ErrNoCurrentBatch}
		service := NewQueryService(store, ownedByPrincipal())

		response, err := service.EmployeeRecommendations(ctx, queryJobPositionID, testPage(), employerPrincipal())
		if err != nil {
			t.Fatalf("la ausencia de batch no debería ser error: %v", err)
		}
		if response.Status != StatusNone || response.Items == nil || len(response.Items) != 0 {
			t.Fatalf("respuesta inesperada: %+v", response)
		}
	})

	// El completado sin resultados y el fallido se distinguen por el estado y no por la
	// cantidad de items, que en ambos casos es cero.
	t.Run("completado sin resultados frente a fallido", func(t *testing.T) {
		service := NewQueryService(&queryStore{jobResult: JobRecommendations{Status: BatchCompleted}}, ownedByPrincipal())
		empty, err := service.JobRecommendations(ctx, queryEmployeeID, testPage(), employeePrincipal())
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}

		service = NewQueryService(&queryStore{jobResult: JobRecommendations{Status: BatchFailed}}, ownedByPrincipal())
		failed, err := service.JobRecommendations(ctx, queryEmployeeID, testPage(), employeePrincipal())
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}

		if len(empty.Items) != 0 || len(failed.Items) != 0 {
			t.Fatalf("ambos casos deberían llegar sin items: %+v y %+v", empty, failed)
		}
		if empty.Status == failed.Status {
			t.Fatalf("el completado vacío y el fallido deberían distinguirse, ambos son %q", empty.Status)
		}
	})

	// Un conjunto nulo del store no puede salir como null: el cliente distingue [] de null.
	t.Run("items nunca es nil", func(t *testing.T) {
		service := NewQueryService(&queryStore{jobResult: JobRecommendations{Status: BatchCompleted, Items: nil}}, ownedByPrincipal())

		response, err := service.JobRecommendations(ctx, queryEmployeeID, testPage(), employeePrincipal())
		if err != nil {
			t.Fatalf("no se esperaba error: %v", err)
		}
		if response.Items == nil {
			t.Fatalf("items no debería ser nil")
		}
	})
}

// El estado sale del batch más reciente y los items del último completado: el servicio los
// transporta sin mezclarlos.
func TestGeneracionEnCursoConservaElConjuntoPrevio(t *testing.T) {
	store := &queryStore{jobResult: JobRecommendations{
		Status: BatchProcessing,
		Items:  []JobRecommendation{{JobPositionID: queryJobPositionID}},
		Total:  1,
	}}
	service := NewQueryService(store, ownedByPrincipal())

	response, err := service.JobRecommendations(context.Background(), queryEmployeeID, testPage(), employeePrincipal())
	if err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
	if response.Status != StatusProcessing || len(response.Items) != 1 {
		t.Fatalf("respuesta inesperada: %+v", response)
	}
}
