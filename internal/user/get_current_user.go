package user

import (
	"context"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
)

func (h *UserHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		internal.RespondWithError(w, http.StatusBadRequest, ErrUserNotFound.Error())
		return
	}

	user, err := h.service.GetCurrentUser(r.Context(), userID)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrUserNotFound.Error())
		return
	}

	internal.RespondWithJSON(w, http.StatusOK, user)
}

func (s *userService) GetCurrentUser(ctx context.Context, userID int32) (User, error) {
	user, err := s.repo.GetCurrentUser(ctx, userID)
	if err != nil {
		return User{}, err
	}

	return user, nil
}
