// Package employer provides HTTP handlers and business logic for employer profiles.
package employer

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type EmployerHandler struct {
	service  EmployerService
	validate *validator.Validate
}

func NewHandler(service EmployerService, validate *validator.Validate) *EmployerHandler {
	return &EmployerHandler{service: service, validate: validate}
}

func BuildHandlers(store EmployerStore, validate *validator.Validate) *EmployerHandler {
	return NewHandler(NewService(store), validate)
}

func RegisterRoutes(mux *http.ServeMux, handler *EmployerHandler, secretKey string) {
	authMiddleware := user.AuthenticatedUser(secretKey)
	mux.Handle("POST /employers", authMiddleware(http.HandlerFunc(handler.CreateEmployer)))
	mux.Handle("GET /users/{userID}/employer", authMiddleware(http.HandlerFunc(handler.GetEmployer)))
}

func (h *EmployerHandler) CreateEmployer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	principal, ok := user.PrincipalFromContext(r.Context())
	if !ok {
		internal.RespondWithError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var request CreateEmployerRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrInvalidJSON.Error())
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		internal.RespondWithError(w, http.StatusBadRequest, ErrInvalidJSON.Error())
		return
	}

	request.Normalize()
	if err := h.validate.Struct(request); err != nil {
		internal.PrintValidatorError(w, err)
		return
	}

	if err := h.service.CreateEmployer(r.Context(), request, principal); err != nil {
		switch {
		case errors.Is(err, user.ErrProfileRoleForbidden):
			internal.RespondWithError(w, http.StatusForbidden, "forbidden")
		case errors.Is(err, ErrEmployerAlreadyExists), errors.Is(err, ErrEmployerProfileConflict):
			internal.RespondWithError(w, http.StatusConflict, err.Error())
		default:
			internal.RespondWithError(w, http.StatusInternalServerError, ErrInternalErrorCreatingEmployer.Error())
		}
		return
	}

	internal.RespondWithNoBody(w, http.StatusCreated)
}

func (h *EmployerHandler) GetEmployer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	principal, ok := user.PrincipalFromContext(r.Context())
	if !ok {
		internal.RespondWithError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	pathUserID, err := internal.GetPathValueAsInt(r, "userID")
	if err != nil || pathUserID <= 0 {
		internal.RespondWithError(w, http.StatusBadRequest, ErrInvalidUserID.Error())
		return
	}
	if principal.UserID != pathUserID {
		internal.RespondWithError(w, http.StatusForbidden, "forbidden")
		return
	}

	employer, err := h.service.GetEmployer(r.Context(), pathUserID, principal)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrProfileRoleForbidden):
			internal.RespondWithError(w, http.StatusForbidden, "forbidden")
		case errors.Is(err, ErrEmployerNotFound):
			internal.RespondWithError(w, http.StatusNotFound, ErrEmployerNotFound.Error())
		default:
			internal.RespondWithError(w, http.StatusInternalServerError, ErrInternalErrorGettingEmployer.Error())
		}
		return
	}

	employer.Normalize()
	if err := internal.RespondWithJSON(w, http.StatusOK, employer); err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, ErrInternalErrorGettingEmployer.Error())
	}
}
