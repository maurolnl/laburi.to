package recommendation

import "errors"

var (
	ErrBatchNotFound = errors.New("recommendation batch not found")
	// ErrBatchAlreadyInFlight traduce la violación del índice único parcial: el sujeto ya
	// tiene un batch pending o processing. Es la forma en que la base garantiza que un
	// redelivery o un mensaje duplicado no generen trabajo paralelo.
	ErrBatchAlreadyInFlight = errors.New("the subject already has a recommendation batch in flight")
	ErrSubjectNotFound      = errors.New("recommendation batch subject not found")
	// ErrBatchNotClaimable significa que el batch existe pero ya llegó a un estado terminal,
	// así que su trabajo está hecho. Se distingue de ErrBatchNotFound, que significa que el
	// batch no existe: el primero es un redelivery de trabajo ya cerrado y el segundo un
	// mensaje que sobrevivió a su batch. Los dos se reconocen sin trabajo, pero un worker que
	// los confunda no puede diagnosticar cuál de las dos cosas está pasando.
	ErrBatchNotClaimable  = errors.New("the recommendation batch already reached a terminal state")
	ErrInvalidSubject     = errors.New("a recommendation batch must have exactly one subject")
	ErrInvalidBatchStatus = errors.New("invalid recommendation batch status")
	ErrNoCurrentBatch     = errors.New("the subject has no recommendation batch")
)
