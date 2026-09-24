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

	// JobRecommendationsForEmployee resuelve el estado vigente, el tramo pedido del conjunto
	// vigente de puestos recomendados a un empleado y el tamaño total de ese conjunto ya
	// filtrado. El llamador no necesita conocer la regla que distingue estado de conjunto.
	//
	// El tramo y el total se resuelven contra el mismo batch completado, así que no pueden
	// describir conjuntos distintos.
	JobRecommendationsForEmployee(ctx context.Context, employeeID int32, page Page) (JobRecommendations, error)

	// EmployeeRecommendationsForJobPosition hace lo propio en el sentido inverso.
	EmployeeRecommendationsForJobPosition(ctx context.Context, jobPositionID int32, page Page) (EmployeeRecommendations, error)
}

// JobRecommendations combina el estado vigente del empleado con su conjunto vigente de
// puestos. Status sale del batch más reciente; Items y Total, del último batch completado.
type JobRecommendations struct {
	Status BatchStatus         `json:"status"`
	Items  []JobRecommendation `json:"items"`
	Total  int32               `json:"total"`
}

// EmployeeRecommendations es el equivalente para un puesto.
type EmployeeRecommendations struct {
	Status BatchStatus              `json:"status"`
	Items  []EmployeeRecommendation `json:"items"`
	Total  int32                    `json:"total"`
}

// SubjectOwnership resuelve a quién pertenece un sujeto. Es un puerto separado de
// RecommendationStore porque responde a una pregunta distinta —quién puede consultar— y
// quien solo lee recomendaciones no necesita depender de él.
//
// Ambas operaciones devuelven ErrSubjectNotFound cuando el sujeto no existe. Para un puesto,
// «no existe» incluye el eliminado lógicamente: a efectos de autorización son la misma cosa,
// y distinguirlos le revelaría a un tercero que alguna vez hubo un puesto con ese
// identificador.
type SubjectOwnership interface {
	EmployeeOwner(ctx context.Context, employeeID int32) (int32, error)
	JobPositionOwner(ctx context.Context, jobPositionID int32) (JobPositionOwner, error)
}

// JobPositionOwner son las dos identidades que cuelgan de un puesto activo: el empleador al
// que pertenece y el usuario dueño de ese empleador. La autorización compara el usuario; el
// empleador queda disponible para diagnóstico.
type JobPositionOwner struct {
	EmployerID int32
	UserID     int32
}

// ProfileAccess responde si un empleador tiene, hoy, una recomendación vigente que lo vincule
// con un empleado. Es un tercer puerto y no un método de SubjectOwnership porque responde a una
// pregunta distinta: no quién es dueño de un sujeto propio, sino si un tercero puede mirar un
// sujeto ajeno.
//
// Lo consume internal/employee, que expone el perfil y no importa este paquete. Devuelve un
// booleano y no el conjunto: quien pregunta solo necesita autorizar.
type ProfileAccess interface {
	// EmployerHasCurrentRecommendation responde si el conjunto vigente del empleado o el de
	// alguno de los puestos activos del empleador contiene el par. Un empleado inexistente, un
	// empleador sin puestos y un par sin vínculo devuelven false sin error propio: para quien
	// autoriza los tres significan lo mismo, y distinguirlos revelaría si el empleado existe.
	EmployerHasCurrentRecommendation(ctx context.Context, employeeID, employerUserID int32) (bool, error)
}
