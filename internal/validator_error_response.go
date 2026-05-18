package internal

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

func PrintValidatorError(w http.ResponseWriter, err error) {
	var invalidValidationError *validator.InvalidValidationError
	if errors.As(err, &invalidValidationError) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var validateErrs validator.ValidationErrors
	if errors.As(err, &validateErrs) {
		errMessages := make([]string, 0, len(validateErrs))
		for _, e := range validateErrs {
			errMessages = append(errMessages, fmt.Sprintf("%s is %s", e.Field(), e.Tag()))
		}
		http.Error(w, strings.Join(errMessages, ", "), http.StatusBadRequest)
		return
	}

	http.Error(w, err.Error(), http.StatusBadRequest)
}
