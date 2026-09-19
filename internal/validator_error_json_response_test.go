package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

type validatorErrorSample struct {
	Name string `validate:"required"`
	Age  int    `validate:"min=1"`
}

func TestRespondWithValidatorErrorUsesJSONContract(t *testing.T) {
	err := validator.New().Struct(validatorErrorSample{})
	if err == nil {
		t.Fatal("expected validation error")
	}

	recorder := httptest.NewRecorder()
	RespondWithValidatorError(recorder, err)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, recorder.Body.String())
	}
	message, ok := body["error"]
	if !ok {
		t.Fatalf("response body = %s, want an \"error\" key", recorder.Body.String())
	}
	if !strings.Contains(message, "Name is required") || !strings.Contains(message, "Age is min") {
		t.Fatalf("error message = %q, want the aggregated field failures", message)
	}
}

func TestRespondWithValidatorErrorHandlesNonValidationErrors(t *testing.T) {
	recorder := httptest.NewRecorder()
	RespondWithValidatorError(recorder, validator.New().Struct("not a struct"))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v (%s)", err, recorder.Body.String())
	}
	if body["error"] == "" {
		t.Fatalf("response body = %s, want a non-empty error message", recorder.Body.String())
	}
}
