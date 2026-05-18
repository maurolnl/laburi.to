package user

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

type UserHandler struct {
	service  UserService
	validate *validator.Validate
}

func NewHandler(userService UserService, validate *validator.Validate) *UserHandler {
	return &UserHandler{
		service:  userService,
		validate: validate,
	}
}

func (h *UserHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var user CreateUserReq

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.validate.Struct(user); err != nil {
		internal.PrintValidatorError(w, err)
		return
	}

	if err := h.service.SaveUser(r.Context(), user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	internal.RespondWithNoBody(w, http.StatusOK)
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var creds LoginUserRequest
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Could not decode request body", http.StatusBadRequest)
		return
	}

	if err := h.validate.Struct(creds); err != nil {
		internal.PrintValidatorError(w, err)
		return
	}

	userID, token, refreshToken, err := h.service.Login(r.Context(), creds.Email, creds.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	internal.RespondWithJson(w, http.StatusAccepted, UserRes{
		ID:           userID,
		Email:        creds.Email,
		Token:        token,
		RefreshToken: refreshToken,
	})
}
