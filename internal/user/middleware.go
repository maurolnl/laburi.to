package user

import (
	"context"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/cmd/middleware"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
)

type principalContextKey int

const principalKey principalContextKey = iota

func PrincipalFromContext(ctx context.Context) (auth.Principal, bool) {
	principal, ok := ctx.Value(principalKey).(auth.Principal)
	return principal, ok && principal.Role.Valid()
}

func AuthenticatedUser(secretKey string) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accessToken, err := auth.GetBearerToken(r.Header)
			if err != nil {
				internal.RespondWithError(w, http.StatusUnauthorized, err.Error())
				return
			}

			principal, err := auth.ValidateJWT(accessToken, secretKey)
			if err != nil {
				internal.RespondWithError(w, http.StatusUnauthorized, err.Error())
				return
			}

			ctx := context.WithValue(r.Context(), principalKey, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
