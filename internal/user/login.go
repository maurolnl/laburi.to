package user

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

var errCouldNotLogin = errors.New("could not log in")

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var creds LoginUserRequest
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, "could not decode request body")
		return
	}

	if err := h.validate.Struct(creds); err != nil {
		internal.RespondWithValidatorError(w, err)
		return
	}

	userID, role, token, refreshToken, err := h.service.Login(r.Context(), creds.Email, creds.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		internal.RespondWithError(w, http.StatusUnauthorized, ErrInvalidCredentials.Error())
		return
	}
	if err != nil {
		// El detalle queda en el log: la respuesta no debe filtrar errores de DB ni de firma.
		log.Printf("login failed: %v", err)
		internal.RespondWithError(w, http.StatusInternalServerError, errCouldNotLogin.Error())
		return
	}

	internal.RespondWithJSON(w, http.StatusAccepted, UserRes{
		ID:           userID,
		Email:        creds.Email,
		Role:         role,
		Token:        token,
		RefreshToken: refreshToken,
	})
}
