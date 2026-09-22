package queue

import (
	"errors"
	"fmt"
	"testing"
)

// El arranque reacciona distinto según el error: una variable faltante se corrige
// declarándola, una inválida se corrige arreglando su valor, y la cola deshabilitada no es
// un error de configuración en absoluto. El contrato debe permitir separarlos sin
// inspeccionar el texto del error.
func TestSentinelErrorsAreMutuallyDistinguishable(t *testing.T) {
	missing := fmt.Errorf("%w: %s", ErrMissingQueueConfig, EnvQueueURL)
	invalid := fmt.Errorf("%w: %s must be an integer", ErrInvalidQueueConfig, EnvWaitTime)

	if !errors.Is(missing, ErrMissingQueueConfig) {
		t.Error("a missing value must be classifiable as ErrMissingQueueConfig")
	}
	if errors.Is(missing, ErrInvalidQueueConfig) {
		t.Error("a missing value must not be confused with an invalid one")
	}
	if errors.Is(missing, ErrQueueDisabled) {
		t.Error("a configuration error must not be confused with a disabled queue")
	}

	if !errors.Is(invalid, ErrInvalidQueueConfig) {
		t.Error("an invalid value must be classifiable as ErrInvalidQueueConfig")
	}
	if errors.Is(invalid, ErrMissingQueueConfig) {
		t.Error("an invalid value must not be confused with a missing one")
	}

	if errors.Is(ErrQueueDisabled, ErrMissingQueueConfig) || errors.Is(ErrQueueDisabled, ErrInvalidQueueConfig) {
		t.Error("a disabled queue is not a configuration error")
	}
}
