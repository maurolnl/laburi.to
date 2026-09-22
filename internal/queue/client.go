package queue

import "context"

// Message es un mensaje recibido del transporte, con tipos propios del paquete: sus
// consumidores no conocen el proveedor de colas.
type Message struct {
	// ID identifica el mensaje dentro de la cola. Sirve para diagnóstico; no es una clave de
	// idempotencia: un redelivery conserva el mismo ID, pero dos envíos del mismo contenido
	// producen IDs distintos.
	ID string

	// Body es el cuerpo tal como lo envió el productor. El transporte no lo interpreta.
	Body string

	// ReceiptHandle identifica esta recepción concreta y es lo que hay que presentar para
	// confirmar el mensaje o extender su plazo. Cambia en cada entrega del mismo mensaje.
	ReceiptHandle string

	// ReceiveCount es la cantidad de veces que el mensaje fue entregado, incluida la actual.
	// Es aproximado: el servicio no lo garantiza exacto. Un valor mayor a uno indica
	// redelivery, que es lo que el worker necesita para decidir cuándo dejar de reintentar.
	ReceiveCount int
}

// Client es el puerto del transporte asíncrono de recomendaciones.
//
// Ninguna firma expone tipos del proveedor de colas: reemplazar la implementación no obliga
// a tocar a sus consumidores. Los parámetros de recepción —cantidad de mensajes y tiempo de
// espera— salen de Config y no de cada llamada, para que la validación de rango del arranque
// no quede sin efecto.
//
// # Garantías
//
// La entrega es al menos una vez, nunca exactamente una vez:
//
//   - Un mensaje recibido queda invisible para otros consumidores hasta que vence su
//     visibility timeout.
//   - Si no se confirma con Delete antes de ese vencimiento, vuelve a entregarse con un
//     ReceiveCount mayor.
//   - Tras agotar el maxReceiveCount configurado en la cola, el mensaje termina en la DLQ.
//     Ese atributo pertenece a la cola en AWS y no se configura desde la aplicación.
//
// Todo consumidor debe ser idempotente: procesar dos veces el mismo mensaje no puede
// duplicar trabajo ni corromper estado.
type Client interface {
	// Send encola un cuerpo ya serializado. Serializar es responsabilidad del productor.
	Send(ctx context.Context, body string) error

	// Receive devuelve hasta MaxMessages mensajes, esperando hasta WaitTimeSeconds a que
	// haya alguno. Un lote vacío sin error significa que no había mensajes, no un fallo.
	Receive(ctx context.Context) ([]Message, error)

	// Delete confirma el procesamiento de un mensaje y lo elimina definitivamente. Sin esta
	// llamada, el mensaje vuelve a entregarse.
	Delete(ctx context.Context, receiptHandle string) error

	// ExtendVisibility corre el vencimiento del plazo de procesamiento de un mensaje ya
	// recibido, para un procesamiento más largo que el visibility timeout por defecto.
	ExtendVisibility(ctx context.Context, receiptHandle string, seconds int32) error
}
