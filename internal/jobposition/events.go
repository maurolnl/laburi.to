package jobposition

import (
	"context"
	"log"
)

// JobPositionEventPublisher es el puerto que la épica de recomendaciones implementará.
// El contrato no menciona ninguna tecnología de cola: sustituir la implementación no
// altera rutas, cuerpos ni códigos de estado.
type JobPositionEventPublisher interface {
	JobPositionPublished(ctx context.Context, position JobPosition) error
}

// NoopEventPublisher es la implementación vigente mientras la infraestructura de
// recomendaciones no existe.
type NoopEventPublisher struct{}

func (NoopEventPublisher) JobPositionPublished(context.Context, JobPosition) error {
	return nil
}

// publish notifica el puerto y descarta su error: el puesto ya está persistido, y
// propagar el fallo haría que el cliente reintente un alta o una edición que sí ocurrió.
func publish(ctx context.Context, publisher JobPositionEventPublisher, position JobPosition) {
	if publisher == nil {
		return
	}
	if err := publisher.JobPositionPublished(ctx, position); err != nil {
		log.Printf("job position %d published event failed: %v", position.ID, err)
	}
}
