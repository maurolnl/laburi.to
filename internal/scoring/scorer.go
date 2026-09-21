package scoring

import "context"

// HardFilter decide si un par puede recomendarse. No produce puntaje y no persiste nada:
// es una decisión booleana, deliberadamente barata, que puede aplicarse antes de traer
// los datos que el cálculo necesita.
//
// Las reglas concretas no son parte de este contrato. La épica LAB-17 prohíbe definirlas
// mientras no exista el algoritmo.
type HardFilter interface {
	// Eligible devuelve la decisión sobre el par. Un error significa que la decisión no
	// pudo tomarse, y es distinto de haber decidido que el par es inelegible.
	Eligible(ctx context.Context, pair Pair) (Eligibility, error)
}

// Scorer puntúa pares elegibles. Es el puerto que el worker de LAB-33 recibe por
// inyección; ningún consumidor conoce los indicadores, sus pesos ni cómo se combinan.
//
// Toda implementación MUST cumplir tres reglas:
//
//   - ScoreAll devuelve exactamente un resultado por par de entrada, en el mismo orden.
//   - ScoreAll produce para cada par el mismo resultado que Score sobre ese par.
//   - ScoreAll no devuelve resultados parciales: ante un fallo devuelve error y ningún
//     resultado, para que el llamador no complete un batch con lo que alcanzó a llegar.
//
// La segunda regla es la que permite que una implementación trivial sea un loop y una
// optimizada batchee o vectorice, sin que el llamador note la diferencia.
type Scorer interface {
	Score(ctx context.Context, pair Pair) (Result, error)
	ScoreAll(ctx context.Context, pairs []Pair) ([]Result, error)
}
