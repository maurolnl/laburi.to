package recommendation

import (
	"errors"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

var (
	errInternalQueryingJobRecommendations      = errors.New("internal error querying job recommendations")
	errInternalQueryingEmployeeRecommendations = errors.New("internal error querying employee recommendations")
)

type QueryHandler struct {
	service QueryService
}

func NewQueryHandler(service QueryService) *QueryHandler {
	return &QueryHandler{service: service}
}

func BuildQueryHandlers(store RecommendationStore, ownership SubjectOwnership) *QueryHandler {
	return NewQueryHandler(NewQueryService(store, ownership))
}

// RegisterQueryRoutes monta las dos consultas detrás de la autenticación común. La
// autorización por rol y propiedad vive en el servicio y no en un middleware: el rol determina
// el sentido de la consulta, y un middleware genérico que solo validara el JWT dejaría esa
// regla fuera del único lugar donde se puede verificar junto con la propiedad.
func RegisterQueryRoutes(mux *http.ServeMux, handler *QueryHandler, secretKey string) {
	authMiddleware := user.AuthenticatedUser(secretKey)

	mux.Handle("GET /employees/{employeeID}/job-recommendations", authMiddleware(http.HandlerFunc(handler.JobRecommendations)))
	mux.Handle("GET /jobs/{jobPositionID}/employee-recommendations", authMiddleware(http.HandlerFunc(handler.EmployeeRecommendations)))
}

func (h *QueryHandler) JobRecommendations(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	principal, ok := principalFrom(w, r)
	if !ok {
		return
	}

	employeeID, ok := pathID(w, r, "employeeID", ErrInvalidEmployeeID)
	if !ok {
		return
	}

	page, ok := pageFrom(w, r)
	if !ok {
		return
	}

	response, err := h.service.JobRecommendations(r.Context(), employeeID, page, principal)
	if err != nil {
		respondWithQueryError(w, err, errInternalQueryingJobRecommendations)
		return
	}

	if err := internal.RespondWithJSON(w, http.StatusOK, response); err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, errInternalQueryingJobRecommendations.Error())
	}
}

func (h *QueryHandler) EmployeeRecommendations(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	principal, ok := principalFrom(w, r)
	if !ok {
		return
	}

	jobPositionID, ok := pathID(w, r, "jobPositionID", ErrInvalidJobPositionID)
	if !ok {
		return
	}

	page, ok := pageFrom(w, r)
	if !ok {
		return
	}

	response, err := h.service.EmployeeRecommendations(r.Context(), jobPositionID, page, principal)
	if err != nil {
		respondWithQueryError(w, err, errInternalQueryingEmployeeRecommendations)
		return
	}

	if err := internal.RespondWithJSON(w, http.StatusOK, response); err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, errInternalQueryingEmployeeRecommendations.Error())
	}
}

func principalFrom(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	principal, ok := user.PrincipalFromContext(r.Context())
	if !ok {
		internal.RespondWithError(w, http.StatusUnauthorized, "unauthorized")
		return auth.Principal{}, false
	}

	return principal, true
}

func pathID(w http.ResponseWriter, r *http.Request, key string, invalidErr error) (int32, bool) {
	id, err := internal.GetPathValueAsInt(r, key)
	if err != nil || id <= 0 {
		internal.RespondWithError(w, http.StatusBadRequest, invalidErr.Error())
		return 0, false
	}

	return id, true
}

func pageFrom(w http.ResponseWriter, r *http.Request) (Page, bool) {
	page, err := parsePage(r.URL.Query())
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return Page{}, false
	}

	return page, true
}

// respondWithQueryError traduce los sentinelas del dominio a status HTTP. El rol incorrecto y
// el sujeto ajeno comparten el 403 con un mensaje único: distinguirlos le diría a un tercero si
// el sujeto consultado existe y a quién pertenece.
func respondWithQueryError(w http.ResponseWriter, err error, internalErr error) {
	switch {
	case errors.Is(err, user.ErrProfileRoleForbidden), errors.Is(err, ErrRecommendationsForbidden):
		internal.RespondWithError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, ErrSubjectNotFound):
		internal.RespondWithError(w, http.StatusNotFound, ErrSubjectNotFound.Error())
	default:
		internal.RespondWithError(w, http.StatusInternalServerError, internalErr.Error())
	}
}
