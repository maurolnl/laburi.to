package user

import (
	"context"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/cmd/middleware"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
)

type userIDContextKey int

const userIDKey userIDContextKey = iota

func UserIDFromContext(ctx context.Context) (int32, bool) {
	userID, ok := ctx.Value(userIDKey).(int32)
	return userID, ok
}

func AuthenticatedUser(secretKey string) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accessToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				internal.RespondWithError(w, http.StatusUnauthorized, err.Error())
				return
			}

			userID, err := auth.ValidateJWT(accessToken, secretKey)
			if err != nil {
				internal.RespondWithError(w, http.StatusUnauthorized, err.Error())
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
