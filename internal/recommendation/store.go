package recommendation

import "context"

// RecommendationStore es la interfaz de consumidor de la persistencia de recomendaciones.
// La implementan el repositorio real y los fakes de los tests de servicio que llegarán con
// LAB-33 y LAB-35.
type RecommendationStore interface {
	// CreateBatch abre una ejecución en estado pending. Devuelve ErrBatchAlreadyInFlight
	// cuando el sujeto ya tiene un batch pending o processing, que es la garantía de
	// idempotencia frente a redelivery y mensajes duplicados.
	CreateBatch(ctx context.Context, subject Subject) (Batch, error)

	// TransitionBatch cambia el estado de un batch y refresca su fecha de actualización.
	TransitionBatch(ctx context.Context, batchID int32, status BatchStatus) (Batch, error)

	// CompleteBatch completa un batch, persiste su conjunto de recomendaciones y descarta
	// los batches anteriores del mismo sujeto, todo en una única transacción. Ante
	// cualquier fallo conserva intactos el estado previo del batch y el conjunto vigente
	// anterior.
	CompleteBatch(ctx context.Context, batchID int32, candidates []Candidate) (Batch, error)

	// JobRecommendationsForEmployee resuelve el estado vigente y el conjunto vigente de
	// puestos recomendados a un empleado. El llamador no necesita conocer la regla que
	// los distingue.
	JobRecommendationsForEmployee(ctx context.Context, employeeID int32, page Page) (JobRecommendations, error)

	// EmployeeRecommendationsForJobPosition hace lo propio en el sentido inverso.
	EmployeeRecommendationsForJobPosition(ctx context.Context, jobPositionID int32, page Page) (EmployeeRecommendations, error)
}

// JobRecommendations combina el estado vigente del empleado con su conjunto vigente de
// puestos. Status sale del batch más reciente; Items, del último batch completado.
type JobRecommendations struct {
	Status BatchStatus         `json:"status"`
	Items  []JobRecommendation `json:"items"`
}

// EmployeeRecommendations es el equivalente para un puesto.
type EmployeeRecommendations struct {
	Status BatchStatus              `json:"status"`
	Items  []EmployeeRecommendation `json:"items"`
}
