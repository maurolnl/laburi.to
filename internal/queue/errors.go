package queue

import "errors"

var (
	// ErrQueueDisabled es el fallo de Disabled. El transporte no está habilitado, así que
	// ninguna operación pudo ocurrir. No es un error de configuración: la configuración es
	// válida y dice que la cola está apagada.
	ErrQueueDisabled = errors.New("recommendation queue is disabled")

	// ErrMissingQueueConfig identifica una variable obligatoria ausente o vacía con la cola
	// habilitada.
	ErrMissingQueueConfig = errors.New("missing required recommendation queue configuration")

	// ErrInvalidQueueConfig identifica una variable declarada con un valor que no se puede
	// interpretar o que cae fuera del rango que admite el servicio de colas. Se distingue de
	// ErrMissingQueueConfig porque el diagnóstico es distinto: una falta, la otra está mal
	// escrita.
	ErrInvalidQueueConfig = errors.New("invalid recommendation queue configuration")
)
