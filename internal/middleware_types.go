package internal

import "net/http"

type AuthMiddleware func(
	next func(w http.ResponseWriter, r *http.Request, userID int32),
) func(http.ResponseWriter, *http.Request)
