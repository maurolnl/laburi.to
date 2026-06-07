package user

import (
	"encoding/json"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

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
