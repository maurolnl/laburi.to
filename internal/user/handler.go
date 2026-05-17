package user

import (
	"encoding/json"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

type UserHandler struct {
	service UserService
}

func NewHandler(userService UserService) *UserHandler {
	return &UserHandler{
		service: userService,
	}
}

func (h *UserHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var user CreateUserReq

	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if creds.Email == "" || creds.Password == "" {
		http.Error(w, ErrEmailOrPasswordRequired.Error(), http.StatusBadRequest)
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
