// Package queuetest provee un doble en memoria de queue.Client para las pruebas de los
// paquetes que lo consumen.
//
// Es un paquete normal y no un archivo _test.go porque sus consumidores viven en otros
// paquetes —el productor de LAB-32 en internal/jobposition, el worker de LAB-33— y un
// _test.go no es importable. No entra al binario: ninguna ruta de producción lo importa, y
// `go list -deps ./cmd | grep queuetest` no devuelve nada.
//
// El doble reproduce las garantías observables de SQS, no su implementación: entrega al
// menos una vez, invisibilidad temporal del mensaje recibido, redelivery al vencer ese plazo
// y conteo de recepciones creciente. No promete orden, porque SQS estándar tampoco lo
// promete, y un test que dependa del orden pasaría acá y fallaría en producción.
package queuetest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/queue"
)

// DefaultVisibilityTimeout es el plazo que el doble aplica si no se configura otro.
const DefaultVisibilityTimeout = time.Minute

type fakeMessage struct {
	id            string
	body          string
	receiptHandle string
	receiveCount  int
	// invisibleUntil es el instante a partir del cual el mensaje vuelve a entregarse. El
	// cero significa nunca recibido y, por lo tanto, visible.
	invisibleUntil time.Time
}

// Fake es un queue.Client en memoria. Su cero no es utilizable: construirlo con New.
type Fake struct {
	mu       sync.Mutex
	messages []*fakeMessage
	// sent conserva todos los cuerpos enviados, incluso los ya borrados, para que un test
	// pueda comprobar qué publicó el productor.
	sent      []string
	nextID    int
	now       time.Time
	visibleIn time.Duration

	sendErr    error
	receiveErr error
	deleteErr  error
	extendErr  error
}

var _ queue.Client = (*Fake)(nil)

// New crea un doble vacío con un reloj propio que solo avanza con Advance. Un reloj
// inyectable es lo que permite comprobar el redelivery sin esperas reales.
func New() *Fake {
	return &Fake{
		now:       time.Unix(0, 0).UTC(),
		visibleIn: DefaultVisibilityTimeout,
	}
}

// WithVisibilityTimeout cambia el plazo que el doble aplica a cada recepción.
func (f *Fake) WithVisibilityTimeout(d time.Duration) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.visibleIn = d
	return f
}

// Advance corre el reloj del doble, que es como vence la visibilidad de los mensajes
// recibidos y no confirmados.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.now = f.now.Add(d)
}

// FailSend, FailReceive, FailDelete y FailExtend fuerzan el error de una operación, para
// cubrir los caminos de fallo de quien consume la cola. Pasar nil restaura el éxito.
func (f *Fake) FailSend(err error) { f.mu.Lock(); f.sendErr = err; f.mu.Unlock() }

func (f *Fake) FailReceive(err error) { f.mu.Lock(); f.receiveErr = err; f.mu.Unlock() }

func (f *Fake) FailDelete(err error) { f.mu.Lock(); f.deleteErr = err; f.mu.Unlock() }

func (f *Fake) FailExtend(err error) { f.mu.Lock(); f.extendErr = err; f.mu.Unlock() }

// Sent devuelve los cuerpos enviados, en orden de envío, incluidos los ya procesados.
func (f *Fake) Sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.sent...)
}

// Pending devuelve la cantidad de mensajes que siguen en la cola, visibles o no. Un mensaje
// confirmado con Delete no cuenta.
func (f *Fake) Pending() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.messages)
}

func (f *Fake) Send(_ context.Context, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.sendErr != nil {
		return f.sendErr
	}

	f.nextID++
	f.messages = append(f.messages, &fakeMessage{
		id:   fmt.Sprintf("message-%d", f.nextID),
		body: body,
	})
	f.sent = append(f.sent, body)

	return nil
}

// Receive entrega los mensajes visibles y los vuelve invisibles hasta que venza su plazo. El
// receipt handle es nuevo en cada entrega, igual que en SQS: el de una entrega anterior deja
// de servir.
func (f *Fake) Receive(_ context.Context) ([]queue.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.receiveErr != nil {
		return nil, f.receiveErr
	}

	received := make([]queue.Message, 0, len(f.messages))
	for _, message := range f.messages {
		if message.invisibleUntil.After(f.now) {
			continue
		}

		message.receiveCount++
		message.receiptHandle = fmt.Sprintf("%s-receipt-%d", message.id, message.receiveCount)
		message.invisibleUntil = f.now.Add(f.visibleIn)

		received = append(received, queue.Message{
			ID:            message.id,
			Body:          message.body,
			ReceiptHandle: message.receiptHandle,
			ReceiveCount:  message.receiveCount,
		})
	}

	return received, nil
}

func (f *Fake) Delete(_ context.Context, receiptHandle string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.deleteErr != nil {
		return f.deleteErr
	}

	for i, message := range f.messages {
		if message.receiptHandle != receiptHandle {
			continue
		}

		f.messages = append(f.messages[:i], f.messages[i+1:]...)
		return nil
	}

	// SQS acepta un receipt handle vencido sin error. El doble hace lo mismo: un consumidor
	// que confirma tarde no debe ver un fallo que en producción no ocurre.
	return nil
}

func (f *Fake) ExtendVisibility(_ context.Context, receiptHandle string, seconds int32) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.extendErr != nil {
		return f.extendErr
	}

	for _, message := range f.messages {
		if message.receiptHandle == receiptHandle {
			message.invisibleUntil = f.now.Add(time.Duration(seconds) * time.Second)
			return nil
		}
	}

	return nil
}
