// Package user contains the HTTP handler logic for user-related operations, such as registration and login.
// It defines the UserHandler struct, which has methods to handle incoming HTTP requests for user authentication.
// The BuildHandlers function initializes
// The UserHandler with the necessary dependencies,
// and the RegisterRoutes function registers the appropriate routes for user authentication endpoints.
package user

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
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

func BuildHandlers(psqlDB *database.Queries, secretKey string, validate *validator.Validate) *UserHandler {
	userRepo := NewRepository(psqlDB)
	userService := NewService(userRepo, secretKey)
	userHandler := NewHandler(userService, validate)

	return userHandler
}

func RegisterRoutes(mux *http.ServeMux, h *UserHandler, secretKey string) {
	authMiddleware := AuthenticatedUser(secretKey)

	mux.HandleFunc("POST /auth/register", h.RegisterUser)
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.Handle("POST /auth/me", authMiddleware(http.HandlerFunc(h.GetCurrentUser)))
}
