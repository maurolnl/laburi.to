package employee

import (
	"context"
	"log"
)

// EmployeeEventPublisher es el puerto por el que la épica de recomendaciones se entera de que
// un perfil de empleado completo cambió. El contrato no menciona ninguna tecnología de cola:
// sustituir la implementación no altera rutas, cuerpos ni códigos de estado.
//
// Transporta el identificador del empleado y nada más. El consumidor resuelve desde la base
// todo dato que necesite, así que pasarle el perfil no tendría lector; mantener la firma en
// tipos primitivos, además, permite que un adaptador de internal/recommendation la satisfaga
// sin que ninguno de los dos paquetes importe al otro.
type EmployeeEventPublisher interface {
	EmployeeProfileCompleted(ctx context.Context, employeeID int32) error
}

// NoopEventPublisher es la implementación inerte: no emite nada y no falla. Existe para que el
// servicio nunca reciba nil y para cablear un entorno que deliberadamente no emite.
type NoopEventPublisher struct{}

var _ EmployeeEventPublisher = NoopEventPublisher{}

func (NoopEventPublisher) EmployeeProfileCompleted(context.Context, int32) error {
	return nil
}

// publishIfComplete notifica al puerto solo cuando el perfil quedó completo tras una escritura
// ya persistida. Es el único punto donde vive la condición: las diez escrituras de perfil la
// invocan igual, sin repetir la regla ni exceptuar a ninguna.
//
// Ningún desenlace de esta función se propaga al llamador. El cambio de dominio ya está
// confirmado y devolver un error haría que el cliente reintente una escritura que sí ocurrió;
// el fallo de la emisión queda representado del lado del productor —el batch del sujeto pasa a
// failed— y acá solo se registra, nombrando al empleado por su identificador.
//
// Un fallo al resolver la completitud tampoco se propaga por la misma razón: se registra y no
// se notifica, porque no se puede afirmar que corresponda.
func publishIfComplete(ctx context.Context, store EmployeeStore, publisher EmployeeEventPublisher, employeeID int32) {
	if publisher == nil {
		return
	}

	complete, err := store.IsProfileComplete(ctx, employeeID)
	if err != nil {
		log.Printf("employee %d profile completeness could not be resolved: %v", employeeID, err)
		return
	}
	if !complete {
		return
	}

	if err := publisher.EmployeeProfileCompleted(ctx, employeeID); err != nil {
		log.Printf("employee %d profile completed event failed: %v", employeeID, err)
	}
}
