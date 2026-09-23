package recommendation

import (
	"context"
	"errors"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

// QueryService es el borde de lectura de las recomendaciones en ambos sentidos. Resuelve la
// autorización y traduce la ausencia de batch a un estado observable; el orden, el desempate y
// la exclusión de puestos eliminados ya son comportamiento de la persistencia y este servicio
// no los toca.
type QueryService interface {
	JobRecommendations(ctx context.Context, employeeID int32, page Page, principal auth.Principal) (JobRecommendationsResponse, error)
	EmployeeRecommendations(ctx context.Context, jobPositionID int32, page Page, principal auth.Principal) (EmployeeRecommendationsResponse, error)
}

type queryService struct {
	store     RecommendationStore
	ownership SubjectOwnership
}

func NewQueryService(store RecommendationStore, ownership SubjectOwnership) QueryService {
	return &queryService{store: store, ownership: ownership}
}

// JobRecommendations devuelve los puestos sugeridos a un empleado.
//
// El employeeID del path no resuelve identidad: la identidad sale del JWT y el path solo sirve
// para detectar que alguien pidió el perfil de otro.
func (s *queryService) JobRecommendations(ctx context.Context, employeeID int32, page Page, principal auth.Principal) (JobRecommendationsResponse, error) {
	if err := s.authorizeEmployee(ctx, employeeID, principal); err != nil {
		return JobRecommendationsResponse{}, err
	}

	result, err := s.store.JobRecommendationsForEmployee(ctx, employeeID, page)
	if errors.Is(err, ErrNoCurrentBatch) {
		return JobRecommendationsResponse{Status: StatusNone, Items: []JobRecommendation{}, Page: pageInfo(page, 0)}, nil
	}
	if err != nil {
		return JobRecommendationsResponse{}, err
	}

	items := result.Items
	if items == nil {
		items = []JobRecommendation{}
	}

	return JobRecommendationsResponse{
		Status: queryStatusFrom(result.Status),
		Items:  items,
		Page:   pageInfo(page, result.Total),
	}, nil
}

// EmployeeRecommendations devuelve los candidatos sugeridos para un puesto propio.
func (s *queryService) EmployeeRecommendations(ctx context.Context, jobPositionID int32, page Page, principal auth.Principal) (EmployeeRecommendationsResponse, error) {
	if err := s.authorizeJobPosition(ctx, jobPositionID, principal); err != nil {
		return EmployeeRecommendationsResponse{}, err
	}

	result, err := s.store.EmployeeRecommendationsForJobPosition(ctx, jobPositionID, page)
	if errors.Is(err, ErrNoCurrentBatch) {
		return EmployeeRecommendationsResponse{Status: StatusNone, Items: []EmployeeRecommendation{}, Page: pageInfo(page, 0)}, nil
	}
	if err != nil {
		return EmployeeRecommendationsResponse{}, err
	}

	items := result.Items
	if items == nil {
		items = []EmployeeRecommendation{}
	}

	return EmployeeRecommendationsResponse{
		Status: queryStatusFrom(result.Status),
		Items:  items,
		Page:   pageInfo(page, result.Total),
	}, nil
}

// authorizeEmployee exige rol employee y propiedad del perfil. El rol se comprueba antes que la
// existencia del sujeto: un empleador no debe poder usar este endpoint ni siquiera para
// averiguar qué identificadores de empleado existen.
func (s *queryService) authorizeEmployee(ctx context.Context, employeeID int32, principal auth.Principal) error {
	if err := user.AuthorizeProfileRole(principal.Role, user.UserRoleEmployee); err != nil {
		return err
	}

	ownerUserID, err := s.ownership.EmployeeOwner(ctx, employeeID)
	if err != nil {
		return err
	}
	if ownerUserID != principal.UserID {
		return ErrRecommendationsForbidden
	}

	return nil
}

// authorizeJobPosition exige rol employer y propiedad del puesto. La resolución de propiedad
// excluye los puestos eliminados lógicamente, así que un puesto eliminado llega acá como
// ErrSubjectNotFound y el handler lo traduce al mismo 404 que un identificador inexistente.
func (s *queryService) authorizeJobPosition(ctx context.Context, jobPositionID int32, principal auth.Principal) error {
	if err := user.AuthorizeProfileRole(principal.Role, user.UserRoleEmployer); err != nil {
		return err
	}

	owner, err := s.ownership.JobPositionOwner(ctx, jobPositionID)
	if err != nil {
		return err
	}
	if owner.UserID != principal.UserID {
		return ErrRecommendationsForbidden
	}

	return nil
}

// pageInfo devuelve los parámetros efectivamente aplicados y no los pedidos: son los mismos,
// porque un valor fuera de rango ya fue rechazado en el borde en vez de recortarse.
func pageInfo(page Page, total int32) PageInfo {
	return PageInfo{Limit: page.Limit, Offset: page.Offset, Total: total}
}
