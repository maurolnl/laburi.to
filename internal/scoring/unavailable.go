package scoring

import "context"

// Unavailable es la única implementación de producción mientras el algoritmo de
// indicadores no exista. Rechaza toda evaluación con ErrScoringUnavailable.
//
// Falla en vez de devolver un resultado sin puntaje a propósito. Un Result con Total nil
// significa "este par es elegible y no tiene puntaje", y un batch entero de esos se
// completaría como completed: el usuario vería recomendaciones reales sin ordenar. La
// épica pide lo contrario, que la generación esté bloqueada, y el estado que reserva para
// eso es failed. Devolver error es lo que hace que el worker llegue ahí.
//
// Tampoco es un noop al estilo de jobposition.NoopEventPublisher: perder un evento no
// corrompe nada, inventar un puntaje sí.
type Unavailable struct{}

var (
	_ Scorer       = Unavailable{}
	_ HardFilter   = Unavailable{}
	_ Availability = Unavailable{}
)

// Available declara por adelantado lo que Score y ScoreAll confirmarían al ser invocados.
// Devuelve el mismo error para que el llamador no tenga que distinguir dos formas de decir
// que no hay algoritmo.
//
// No consulta nada y no evalúa ningún par: es lo que permite al worker resolver el batch sin
// abrir processing.
func (Unavailable) Available(context.Context) error {
	return ErrScoringUnavailable
}

func (Unavailable) Score(context.Context, Pair) (Result, error) {
	return Result{}, ErrScoringUnavailable
}

// ScoreAll falla entera y no devuelve resultados parciales: un lote a medias tentaría al
// llamador a completar el batch con lo que alcanzó a llegar.
//
// Un lote vacío es la única excepción y devuelve un conjunto vacío sin error: no hay nada
// que puntuar, así que no hace falta ninguna dependencia. Es el caso del sujeto sin
// candidatos, que la épica define como estado vacío y no como fallo.
func (Unavailable) ScoreAll(_ context.Context, pairs []Pair) ([]Result, error) {
	if len(pairs) == 0 {
		return []Result{}, nil
	}
	return nil, ErrScoringUnavailable
}

// Eligible falla en lugar de declarar el par elegible o inelegible: sin algoritmo, los
// filtros duros tampoco están definidos.
func (Unavailable) Eligible(context.Context, Pair) (Eligibility, error) {
	return Eligibility{}, ErrScoringUnavailable
}
