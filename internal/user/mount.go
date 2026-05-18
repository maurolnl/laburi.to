package user

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

func BuildHandlers(psqlDB *database.Queries, secretKey string, validate *validator.Validate) *UserHandler {
	userRepo := NewRepository(psqlDB)
	userService := NewService(userRepo, secretKey)
	userHandler := NewHandler(userService, validate)

	return userHandler
}

func RegisterRoutes(mux *http.ServeMux, h *UserHandler) {
	mux.HandleFunc("POST /auth/register", h.RegisterUser)
	mux.HandleFunc("POST /auth/login", h.Login)
}
