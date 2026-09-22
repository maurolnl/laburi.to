package queue

import "context"

// Disabled es el Client que se inyecta cuando el transporte no está habilitado. Cumple el
// puerto y falla siempre con ErrQueueDisabled.
//
// Existe para que ningún consumidor reciba nil ni tenga que comprobarlo antes de cada
// llamada, igual que scoring.Unavailable. Y falla en vez de devolver nil como
// jobposition.NoopEventPublisher: tragarse el error es una decisión legítima del borde HTTP,
// donde perder un evento no debe hacer fallar un alta ya persistida, pero dentro del
// transporte reportar éxito sin haber enviado nada sería mentir.
type Disabled struct{}

var _ Client = Disabled{}

func (Disabled) Send(context.Context, string) error {
	return ErrQueueDisabled
}

func (Disabled) Receive(context.Context) ([]Message, error) {
	return nil, ErrQueueDisabled
}

func (Disabled) Delete(context.Context, string) error {
	return ErrQueueDisabled
}

func (Disabled) ExtendVisibility(context.Context, string, int32) error {
	return ErrQueueDisabled
}
