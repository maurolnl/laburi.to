package main

import (
	"fmt"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
)

func (app *application) authenticatedUserMiddleWare(next func(w http.ResponseWriter, r *http.Request, userID int32)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		accessToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			internal.RespondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Error reading access token: %v", err))
			return
		}

		userID, err := auth.ValidateJWT(accessToken, app.config.secretKey)
		if err != nil {
			internal.RespondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Error validating access token: %v", err))
			return
		}

		next(w, r, userID)
	}
}
