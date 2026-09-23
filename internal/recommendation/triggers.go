package recommendation

import "context"

// Trigger traduce un cambio de dominio ya persistido en una solicitud de regeneración. Es lo
// único que los bordes de escritura de internal/employee e internal/jobposition necesitan
// conocer de este paquete.
//
// Existe para que ningún paquete importe a otro. Los puertos de esos dos paquetes reciben el
// identificador del sujeto y nada más, así que este tipo los satisface por tipado estructural:
// recommendation no importa employee ni jobposition —doc.go explica por qué esa dirección no
// se invierte— y ninguno de los dos importa recommendation. La composición ocurre en cmd, que
// es el único lugar que conoce a los tres.
//
// No vive del lado del dominio porque atiende los dos sujetos a la vez: ubicarlo en jobposition
// obligaría a un empleado a pasar por el paquete de puestos, y duplicarlo en cada uno repetiría
// la traducción de identificador a Subject.
//
// Su cero es utilizable solo si el publisher embebido no es nil: construirlo con NewTrigger.
type Trigger struct {
	publisher JobPublisher
}

func NewTrigger(publisher JobPublisher) Trigger {
	return Trigger{publisher: publisher}
}

// EmployeeProfileCompleted solicita regenerar los puestos recomendados a un empleado. Quien la
// invoca ya comprobó que el perfil está completo y que su escritura quedó persistida: este
// método no vuelve a mirar el perfil.
func (t Trigger) EmployeeProfileCompleted(ctx context.Context, employeeID int32) error {
	return t.publisher.PublishJob(ctx, NewEmployeeSubject(employeeID))
}

// JobPositionPublished solicita regenerar los empleados recomendados para un puesto, tras un
// alta o una edición ya persistida.
func (t Trigger) JobPositionPublished(ctx context.Context, jobPositionID int32) error {
	return t.publisher.PublishJob(ctx, NewJobPositionSubject(jobPositionID))
}
