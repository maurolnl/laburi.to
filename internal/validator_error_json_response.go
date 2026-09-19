package internal

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

// RespondWithValidatorError emite los errores de validación con el mismo contrato JSON
// `{"error":"..."}` que el resto de los errores, en vez del texto plano de
// PrintValidatorError. El handler global del frontend solo interpreta `{error}` y
// `{messages}`; con texto plano degrada a un mensaje genérico. PrintValidatorError sigue
// existiendo para los endpoints que ya dependen de su formato.
func RespondWithValidatorError(w http.ResponseWriter, err error) {
	RespondWithError(w, http.StatusBadRequest, validatorErrorMessage(err))
}

func validatorErrorMessage(err error) string {
	var invalidValidationError *validator.InvalidValidationError
	if errors.As(err, &invalidValidationError) {
		return err.Error()
	}

	var validateErrs validator.ValidationErrors
	if errors.As(err, &validateErrs) {
		errMessages := make([]string, 0, len(validateErrs))
		for _, e := range validateErrs {
			errMessages = append(errMessages, fmt.Sprintf("%s is %s", e.Field(), e.Tag()))
		}
		return strings.Join(errMessages, ", ")
	}

	return err.Error()
}
