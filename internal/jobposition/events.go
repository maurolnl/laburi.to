package jobposition

import (
	"context"
	"log"
)

// JobPositionEventPublisher es el puerto por el que la épica de recomendaciones se entera de
// que un puesto cambió. El contrato no menciona ninguna tecnología de cola: sustituir la
// implementación no altera rutas, cuerpos ni códigos de estado.
//
// Transporta el identificador del puesto y nada más. El consumidor resuelve desde la base todo
// dato que necesite, así que pasarle la representación completa no tendría lector; mantener la
// firma en tipos primitivos, además, permite que un adaptador de internal/recommendation la
// satisfaga sin que ninguno de los dos paquetes importe al otro.
type JobPositionEventPublisher interface {
	JobPositionPublished(ctx context.Context, jobPositionID int32) error
}

// NoopEventPublisher es la implementación inerte: no emite nada y no falla. Existe para que el
// servicio nunca reciba nil y para cablear un entorno que deliberadamente no emite.
type NoopEventPublisher struct{}

var _ JobPositionEventPublisher = NoopEventPublisher{}

func (NoopEventPublisher) JobPositionPublished(context.Context, int32) error {
	return nil
}

// publish notifica el puerto y descarta su error: el puesto ya está persistido, y propagar el
// fallo haría que el cliente reintente un alta o una edición que sí ocurrió. El fallo queda
// representado del lado de la emisión —el batch del sujeto pasa a failed— y acá solo se
// registra, nombrando al puesto por su identificador.
func publish(ctx context.Context, publisher JobPositionEventPublisher, jobPositionID int32) {
	if publisher == nil {
		return
	}
	if err := publisher.JobPositionPublished(ctx, jobPositionID); err != nil {
		log.Printf("job position %d published event failed: %v", jobPositionID, err)
	}
}
