package user

import (
	"encoding/json"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

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

	internal.RespondWithJSON(w, http.StatusAccepted, UserRes{
		ID:           userID,
		Email:        creds.Email,
		Token:        token,
		RefreshToken: refreshToken,
	})
}
