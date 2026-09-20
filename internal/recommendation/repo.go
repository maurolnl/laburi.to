package recommendation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/lib/pq"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

// scoreScale es la escala declarada en NUMERIC(7,4). Formatear con exactamente esa
// cantidad de decimales evita que el redondeo de PostgreSQL cambie el valor al leerlo.
const scoreScale = 4

type recommendationQueries interface {
	CreateRecommendationBatch(ctx context.Context, arg database.CreateRecommendationBatchParams) (database.RecommendationBatch, error)
	TransitionRecommendationBatch(ctx context.Context, arg database.TransitionRecommendationBatchParams) (database.RecommendationBatch, error)
	GetCurrentBatchByEmployee(ctx context.Context, employeeID sql.NullInt32) (database.RecommendationBatch, error)
	GetCurrentBatchByJobPosition(ctx context.Context, jobPositionID sql.NullInt32) (database.RecommendationBatch, error)
	GetLastCompletedBatchByEmployee(ctx context.Context, employeeID sql.NullInt32) (database.RecommendationBatch, error)
	GetLastCompletedBatchByJobPosition(ctx context.Context, jobPositionID sql.NullInt32) (database.RecommendationBatch, error)
	ListJobRecommendationsForEmployee(ctx context.Context, arg database.ListJobRecommendationsForEmployeeParams) ([]database.ListJobRecommendationsForEmployeeRow, error)
	ListEmployeeRecommendationsForJobPosition(ctx context.Context, arg database.ListEmployeeRecommendationsForJobPositionParams) ([]database.ListEmployeeRecommendationsForJobPositionRow, error)
}

type RecommendationRepository struct {
	db      *sql.DB
	queries recommendationQueries
}

func NewRepository(db *sql.DB) *RecommendationRepository {
	return &RecommendationRepository{db: db, queries: database.New(db)}
}

func (r *RecommendationRepository) CreateBatch(ctx context.Context, subject Subject) (Batch, error) {
	if !subject.Valid() {
		return Batch{}, ErrInvalidSubject
	}

	row, err := r.queries.CreateRecommendationBatch(ctx, database.CreateRecommendationBatchParams{
		SubjectType:   string(subject.Type),
		EmployeeID:    nullInt32(subject.EmployeeID),
		JobPositionID: nullInt32(subject.JobPositionID),
	})
	if err != nil {
		return Batch{}, classifyCreateBatchError(err)
	}

	return batchFromDatabase(row), nil
}

func (r *RecommendationRepository) TransitionBatch(ctx context.Context, batchID int32, status BatchStatus) (Batch, error) {
	if !status.Valid() {
		return Batch{}, ErrInvalidBatchStatus
	}

	row, err := r.queries.TransitionRecommendationBatch(ctx, database.TransitionRecommendationBatchParams{
		ID:     batchID,
		Status: string(status),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Batch{}, ErrBatchNotFound
	}
	if err != nil {
		return Batch{}, fmt.Errorf("transition recommendation batch: %w", err)
	}

	return batchFromDatabase(row), nil
}

// CompleteBatch es el reemplazo atómico. Los tres pasos comparten una única transacción:
// completar el batch, insertar el conjunto nuevo y descartar los batches anteriores del
// sujeto. Si cualquiera falla, el rollback deja el batch en su estado previo y el conjunto
// vigente anterior intacto.
func (r *RecommendationRepository) CompleteBatch(ctx context.Context, batchID int32, candidates []Candidate) (Batch, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Batch{}, fmt.Errorf("begin complete batch transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := database.New(r.db).WithTx(tx)

	row, err := qtx.TransitionRecommendationBatch(ctx, database.TransitionRecommendationBatchParams{
		ID:     batchID,
		Status: string(BatchCompleted),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Batch{}, ErrBatchNotFound
	}
	if err != nil {
		return Batch{}, fmt.Errorf("complete recommendation batch: %w", err)
	}

	for _, candidate := range candidates {
		if _, err := qtx.InsertRecommendation(ctx, database.InsertRecommendationParams{
			BatchID:       batchID,
			EmployeeID:    candidate.EmployeeID,
			JobPositionID: candidate.JobPositionID,
			Score:         scoreToDatabase(candidate.Score),
		}); err != nil {
			return Batch{}, classifyInsertRecommendationError(err)
		}
	}

	// El sujeto sale del batch recién completado, no de un parámetro: así no puede
	// desalinearse con la fila que efectivamente se está reemplazando.
	switch {
	case row.EmployeeID.Valid:
		err = qtx.DeleteOtherBatchesForEmployee(ctx, database.DeleteOtherBatchesForEmployeeParams{
			EmployeeID: row.EmployeeID,
			ID:         batchID,
		})
	case row.JobPositionID.Valid:
		err = qtx.DeleteOtherBatchesForJobPosition(ctx, database.DeleteOtherBatchesForJobPositionParams{
			JobPositionID: row.JobPositionID,
			ID:            batchID,
		})
	default:
		return Batch{}, ErrInvalidSubject
	}
	if err != nil {
		return Batch{}, fmt.Errorf("prune superseded batches: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Batch{}, fmt.Errorf("commit complete batch transaction: %w", err)
	}

	return batchFromDatabase(row), nil
}

func (r *RecommendationRepository) JobRecommendationsForEmployee(ctx context.Context, employeeID int32, page Page) (JobRecommendations, error) {
	subject := nullInt32(&employeeID)

	current, err := r.queries.GetCurrentBatchByEmployee(ctx, subject)
	if errors.Is(err, sql.ErrNoRows) {
		return JobRecommendations{}, ErrNoCurrentBatch
	}
	if err != nil {
		return JobRecommendations{}, fmt.Errorf("get current batch by employee: %w", err)
	}

	result := JobRecommendations{
		Status: BatchStatus(current.Status),
		Items:  []JobRecommendation{},
	}

	completed, err := r.queries.GetLastCompletedBatchByEmployee(ctx, subject)
	if errors.Is(err, sql.ErrNoRows) {
		// El sujeto nunca tuvo un batch completado: el estado ya quedó resuelto y el
		// conjunto vigente está vacío.
		return result, nil
	}
	if err != nil {
		return JobRecommendations{}, fmt.Errorf("get last completed batch by employee: %w", err)
	}

	rows, err := r.queries.ListJobRecommendationsForEmployee(ctx, database.ListJobRecommendationsForEmployeeParams{
		BatchID: completed.ID,
		Limit:   page.Limit,
		Offset:  page.Offset,
	})
	if err != nil {
		return JobRecommendations{}, fmt.Errorf("list job recommendations: %w", err)
	}

	for _, row := range rows {
		score, err := scoreFromDatabase(row.Score)
		if err != nil {
			return JobRecommendations{}, err
		}

		item := JobRecommendation{
			RecommendationID:       row.ID,
			JobPositionID:          row.JobPositionID,
			EmployerID:             row.EmployerID,
			Position:               row.Position,
			Role:                   row.Role,
			RequiredExperience:     row.RequiredExperience,
			RequiredEducationLevel: row.RequiredEducationLevel,
			AvailableHoursPerDay:   row.AvailableHoursPerDay,
			Timezone:               row.Timezone,
			TechnicalResources:     row.TechnicalResources,
			Score:                  score,
			PublishedAt:            row.CreatedAt,
			UpdatedAt:              row.UpdatedAt,
		}
		if item.TechnicalResources == nil {
			item.TechnicalResources = []string{}
		}

		result.Items = append(result.Items, item)
	}

	return result, nil
}

func (r *RecommendationRepository) EmployeeRecommendationsForJobPosition(ctx context.Context, jobPositionID int32, page Page) (EmployeeRecommendations, error) {
	subject := nullInt32(&jobPositionID)

	current, err := r.queries.GetCurrentBatchByJobPosition(ctx, subject)
	if errors.Is(err, sql.ErrNoRows) {
		return EmployeeRecommendations{}, ErrNoCurrentBatch
	}
	if err != nil {
		return EmployeeRecommendations{}, fmt.Errorf("get current batch by job position: %w", err)
	}

	result := EmployeeRecommendations{
		Status: BatchStatus(current.Status),
		Items:  []EmployeeRecommendation{},
	}

	completed, err := r.queries.GetLastCompletedBatchByJobPosition(ctx, subject)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return EmployeeRecommendations{}, fmt.Errorf("get last completed batch by job position: %w", err)
	}

	rows, err := r.queries.ListEmployeeRecommendationsForJobPosition(ctx, database.ListEmployeeRecommendationsForJobPositionParams{
		BatchID: completed.ID,
		Limit:   page.Limit,
		Offset:  page.Offset,
	})
	if err != nil {
		return EmployeeRecommendations{}, fmt.Errorf("list employee recommendations: %w", err)
	}

	for _, row := range rows {
		score, err := scoreFromDatabase(row.Score)
		if err != nil {
			return EmployeeRecommendations{}, err
		}

		item := EmployeeRecommendation{
			RecommendationID:  row.ID,
			EmployeeID:        row.EmployeeID,
			UserID:            row.UserID,
			Position:          row.Position,
			Role:              row.Role,
			YearsOfExperience: row.YearsOfExperience,
			Certifications:    row.Certifications,
			Score:             score,
			CreatedAt:         row.CreatedAt,
			ProfileUpdatedAt:  row.UpdatedAt,
		}
		if item.Certifications == nil {
			item.Certifications = []string{}
		}
		if row.PortfolioUrl.Valid {
			portfolio := row.PortfolioUrl.String
			item.PortfolioURL = &portfolio
		}

		result.Items = append(result.Items, item)
	}

	return result, nil
}

func classifyCreateBatchError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch string(pqErr.Code) {
		case "23505":
			return ErrBatchAlreadyInFlight
		case "23503":
			return ErrSubjectNotFound
		case "23514":
			return ErrInvalidSubject
		}
	}

	return fmt.Errorf("create recommendation batch: %w", err)
}

func classifyInsertRecommendationError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		if string(pqErr.Code) == "23503" {
			return ErrSubjectNotFound
		}
	}

	return fmt.Errorf("insert recommendation: %w", err)
}

func batchFromDatabase(row database.RecommendationBatch) Batch {
	batch := Batch{
		ID:          row.ID,
		SubjectType: SubjectType(row.SubjectType),
		Status:      BatchStatus(row.Status),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	if row.EmployeeID.Valid {
		employeeID := row.EmployeeID.Int32
		batch.EmployeeID = &employeeID
	}
	if row.JobPositionID.Valid {
		jobPositionID := row.JobPositionID.Int32
		batch.JobPositionID = &jobPositionID
	}

	return batch
}

// scoreToDatabase y scoreFromDatabase median entre el *float64 del dominio y el NUMERIC de
// PostgreSQL, que sqlc con database/sql expone como sql.NullString. El texto es el formato
// exacto de la base: convertirlo aquí mantiene la ausencia de puntaje distinguible del
// puntaje cero.
func scoreToDatabase(score *float64) sql.NullString {
	if score == nil {
		return sql.NullString{}
	}

	return sql.NullString{
		String: strconv.FormatFloat(*score, 'f', scoreScale, 64),
		Valid:  true,
	}
}

func scoreFromDatabase(score sql.NullString) (*float64, error) {
	if !score.Valid {
		return nil, nil
	}

	value, err := strconv.ParseFloat(score.String, 64)
	if err != nil {
		return nil, fmt.Errorf("parse recommendation score %q: %w", score.String, err)
	}

	return &value, nil
}

func nullInt32(value *int32) sql.NullInt32 {
	if value == nil {
		return sql.NullInt32{}
	}

	return sql.NullInt32{Int32: *value, Valid: true}
}
