package jobposition

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type JobPositionHandler struct {
	service  JobPositionService
	validate *validator.Validate
}

func NewHandler(service JobPositionService, validate *validator.Validate) *JobPositionHandler {
	return &JobPositionHandler{service: service, validate: validate}
}

func BuildHandlers(store JobPositionStore, publisher JobPositionEventPublisher, validate *validator.Validate) *JobPositionHandler {
	return NewHandler(NewService(store, publisher), validate)
}

func RegisterRoutes(mux *http.ServeMux, handler *JobPositionHandler, secretKey string) {
	authMiddleware := user.AuthenticatedUser(secretKey)

	mux.Handle("POST /employers/{employerID}/jobs", authMiddleware(http.HandlerFunc(handler.CreateJobPosition)))
	mux.Handle("GET /employers/{employerID}/jobs", authMiddleware(http.HandlerFunc(handler.ListJobPositions)))
	mux.Handle("GET /jobs/{jobPositionID}", authMiddleware(http.HandlerFunc(handler.GetJobPosition)))
	mux.Handle("PUT /jobs/{jobPositionID}", authMiddleware(http.HandlerFunc(handler.UpdateJobPosition)))
	mux.Handle("DELETE /jobs/{jobPositionID}", authMiddleware(http.HandlerFunc(handler.DeleteJobPosition)))
}

func (h *JobPositionHandler) CreateJobPosition(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	principal, ok := principalFrom(w, r)
	if !ok {
		return
	}

	employerID, ok := pathID(w, r, "employerID", ErrInvalidEmployerID)
	if !ok {
		return
	}

	request, ok := decodeRequest(w, r, h.validate)
	if !ok {
		return
	}

	position, err := h.service.CreateJobPosition(r.Context(), employerID, request, principal)
	if err != nil {
		respondWithServiceError(w, err, ErrInternalErrorCreatingJobPosition)
		return
	}

	respondWithPosition(w, http.StatusCreated, position, ErrInternalErrorCreatingJobPosition)
}

func (h *JobPositionHandler) ListJobPositions(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	principal, ok := principalFrom(w, r)
	if !ok {
		return
	}

	employerID, ok := pathID(w, r, "employerID", ErrInvalidEmployerID)
	if !ok {
		return
	}

	positions, err := h.service.ListJobPositions(r.Context(), employerID, principal)
	if err != nil {
		respondWithServiceError(w, err, ErrInternalErrorListingJobPositions)
		return
	}

	if err := internal.RespondWithJSON(w, http.StatusOK, positions); err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, ErrInternalErrorListingJobPositions.Error())
	}
}

func (h *JobPositionHandler) GetJobPosition(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	principal, ok := principalFrom(w, r)
	if !ok {
		return
	}

	jobPositionID, ok := pathID(w, r, "jobPositionID", ErrInvalidJobPositionID)
	if !ok {
		return
	}

	position, err := h.service.GetJobPosition(r.Context(), jobPositionID, principal)
	if err != nil {
		respondWithServiceError(w, err, ErrInternalErrorGettingJobPosition)
		return
	}

	respondWithPosition(w, http.StatusOK, position, ErrInternalErrorGettingJobPosition)
}

func (h *JobPositionHandler) UpdateJobPosition(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	principal, ok := principalFrom(w, r)
	if !ok {
		return
	}

	jobPositionID, ok := pathID(w, r, "jobPositionID", ErrInvalidJobPositionID)
	if !ok {
		return
	}

	request, ok := decodeRequest(w, r, h.validate)
	if !ok {
		return
	}

	position, err := h.service.UpdateJobPosition(r.Context(), jobPositionID, request, principal)
	if err != nil {
		respondWithServiceError(w, err, ErrInternalErrorUpdatingJobPosition)
		return
	}

	respondWithPosition(w, http.StatusOK, position, ErrInternalErrorUpdatingJobPosition)
}

func (h *JobPositionHandler) DeleteJobPosition(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	principal, ok := principalFrom(w, r)
	if !ok {
		return
	}

	jobPositionID, ok := pathID(w, r, "jobPositionID", ErrInvalidJobPositionID)
	if !ok {
		return
	}

	if err := h.service.DeleteJobPosition(r.Context(), jobPositionID, principal); err != nil {
		respondWithServiceError(w, err, ErrInternalErrorDeletingJobPosition)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

func decodeRequest(w http.ResponseWriter, r *http.Request, validate *validator.Validate) (CreateJobPositionRequest, bool) {
	var request CreateJobPositionRequest

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrInvalidJSON.Error())
		return CreateJobPositionRequest{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		internal.RespondWithError(w, http.StatusBadRequest, ErrInvalidJSON.Error())
		return CreateJobPositionRequest{}, false
	}

	request.Normalize()
	if err := validate.Struct(request); err != nil {
		internal.RespondWithValidatorError(w, err)
		return CreateJobPositionRequest{}, false
	}

	return request, true
}

// respondWithServiceError traduce los sentinelas del dominio a status HTTP. El rol
// incorrecto, el perfil de empleador ausente y el recurso ajeno comparten el 403: los
// tres significan que el principal no puede operar, y distinguirlos filtraría
// información sobre puestos de otros empleadores.
func respondWithServiceError(w http.ResponseWriter, err error, internalErr error) {
	switch {
	case errors.Is(err, user.ErrProfileRoleForbidden),
		errors.Is(err, ErrEmployerProfileRequired),
		errors.Is(err, ErrJobPositionForbidden):
		internal.RespondWithError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, ErrJobPositionNotFound):
		internal.RespondWithError(w, http.StatusNotFound, ErrJobPositionNotFound.Error())
	case errors.Is(err, ErrInvalidTimezone):
		internal.RespondWithError(w, http.StatusBadRequest, ErrInvalidTimezone.Error())
	default:
		internal.RespondWithError(w, http.StatusInternalServerError, internalErr.Error())
	}
}

func respondWithPosition(w http.ResponseWriter, code int, position JobPosition, internalErr error) {
	if err := internal.RespondWithJSON(w, code, position); err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, internalErr.Error())
	}
}
