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

	// GetBatch recupera un batch por su identificador. Devuelve ErrBatchNotFound cuando no
	// existe.
	GetBatch(ctx context.Context, batchID int32) (Batch, error)

	// TransitionBatch cambia el estado de un batch y refresca su fecha de actualización. Es
	// incondicional: mueve el batch al estado pedido sea cual sea el actual.
	TransitionBatch(ctx context.Context, batchID int32, status BatchStatus) (Batch, error)

	// ClaimBatch toma el trabajo de un batch moviéndolo a processing, y solo prospera si
	// todavía no es terminal. Devuelve ErrBatchNotClaimable cuando el batch ya está completed
	// o failed, y ErrBatchNotFound cuando no existe.
	//
	// Es la operación que hace idempotente al consumidor: un redelivery de un batch ya
	// cerrado no reabre trabajo ni toca el conjunto vigente que ese batch dejó. Un batch en
	// processing sí se reclama, para que un procesamiento interrumpido pueda retomarse en vez
	// de dejar al sujeto bloqueado por el índice único parcial.
	ClaimBatch(ctx context.Context, batchID int32) (Batch, error)

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
