// Package timezone provides functionality to manage timezones in the application.
// It includes a handler for HTTP requests, a service layer for business logic,
// and a repository for database interactions.
package timezone

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/cmd/middleware"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type Timezone struct {
	Name      string `json:"name"`
	Abbrev    string `json:"abbrev"`
	UTCOffset string `json:"utc_offset"`
	IsDST     bool   `json:"is_dst"`
}

type TimezoneHandler struct {
	service TimezoneService
}

func NewHandler(service TimezoneService) *TimezoneHandler {
	return &TimezoneHandler{
		service: service,
	}
}

type TimezoneService interface {
	GetTimezones(ctx context.Context) ([]Timezone, error)
}

type timezoneService struct {
	repo TimezoneStore
}

func NewService(repo TimezoneStore) TimezoneService {
	return &timezoneService{
		repo: repo,
	}
}

type TimezoneStore interface {
	GetTimezones(ctx context.Context) ([]Timezone, error)
}

type TimezoneRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *TimezoneRepository {
	return &TimezoneRepository{
		db: db,
	}
}

func (h *TimezoneHandler) RegisterRoutes(router *http.ServeMux, secretKey string) {
	router.HandleFunc("GET /timezones", h.GetTimezones)

	authenticatedMiddleware := user.AuthenticatedUser(secretKey)

	middlewareStack := middleware.CreateStack(
		middleware.Logger,
		authenticatedMiddleware,
	)

	middlewareStack(router)
}

func (h *TimezoneHandler) GetTimezones(w http.ResponseWriter, r *http.Request) {
	timezones, err := h.service.GetTimezones(r.Context())
	if err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := internal.RespondWithJSON(w, http.StatusOK, timezones); err != nil {
		internal.RespondWithError(w, http.StatusInternalServerError, err.Error())
	}
}

func (s *timezoneService) GetTimezones(ctx context.Context) ([]Timezone, error) {
	return s.repo.GetTimezones(ctx)
}

func (r *TimezoneRepository) GetTimezones(ctx context.Context) ([]Timezone, error) {
	const query = `
		SELECT name, abbrev, utc_offset::text, is_dst
		FROM pg_timezone_names
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	timezones := []Timezone{}
	for rows.Next() {
		var tz Timezone
		if err := rows.Scan(&tz.Name, &tz.Abbrev, &tz.UTCOffset, &tz.IsDST); err != nil {
			return nil, err
		}
		timezones = append(timezones, tz)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return timezones, nil
}
