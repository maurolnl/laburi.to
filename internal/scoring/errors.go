package scoring

import "errors"

var (
	// ErrScoringUnavailable es el fallo de la implementación de producción mientras el
	// algoritmo de indicadores no exista. El llamador lo clasifica con errors.Is y lo
	// traduce a un batch failed; nunca a un batch completed sin puntajes.
	ErrScoringUnavailable = errors.New("scoring implementation is not available")

	// ErrInvalidPair y los errores de nivel señalan una entrada mal construida, que es un
	// problema del llamador y no de la dependencia. Existen para que el worker distinga
	// "no puedo puntuar porque no hay algoritmo" de "me pasaron un par inválido".
	ErrInvalidPair            = errors.New("invalid scoring pair")
	ErrInvalidExperienceLevel = errors.New("invalid experience level")
	ErrInvalidEducationLevel  = errors.New("invalid education level")
)
