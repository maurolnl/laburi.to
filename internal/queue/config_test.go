package queue

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// envLookup arma un Lookup sobre un mapa. Es lo que permite ejercitar la matriz completa de
// configuración sin t.Setenv ni estado compartido entre tests paralelos.
func envLookup(values map[string]string) Lookup {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

// enabledEnv es una configuración habilitada y válida, con solo lo obligatorio declarado.
func enabledEnv() map[string]string {
	return map[string]string{
		EnvEnabled:  "true",
		EnvRegion:   "us-east-2",
		EnvQueueURL: "http://localhost:4566/000000000000/recommendations",
		EnvDLQURL:   "http://localhost:4566/000000000000/recommendations-dlq",
	}
}

func TestLoadConfigDisabledDoesNotReadAnyOtherVariable(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
	}{
		{name: "flag absent", set: false},
		{name: "flag empty", value: "", set: true},
		{name: "flag blank", value: "   ", set: true},
		{name: "flag false", value: "false", set: true},
		{name: "flag zero", value: "0", set: true},
		// Un interruptor ilegible cuenta como apagado, así que tampoco habilita la lectura
		// del resto de la configuración.
		{name: "flag unreadable", value: "yes", set: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) {
				if key == EnvEnabled {
					if !tt.set {
						return "", false
					}
					return tt.value, true
				}
				t.Fatalf("a disabled queue must not read %s", key)
				return "", false
			}

			cfg, err := LoadConfig(lookup)
			if err != nil {
				t.Fatalf("LoadConfig() = %v, want nil", err)
			}
			if cfg.Enabled {
				t.Error("the queue must be disabled")
			}
		})
	}
}

// Un typo en el flag no debe apagar la cola en silencio: ese es exactamente el modo de fallo
// que el ticket quiere evitar.
// Un interruptor ilegible describe si la funcionalidad participa, no cómo: degrada a apagado
// y no derriba el arranque. La advertencia es lo que evita que el typo pase inadvertido.
func TestLoadConfigLeavesAnUnreadableSwitchDisabled(t *testing.T) {
	cfg, err := LoadConfig(envLookup(map[string]string{EnvEnabled: "yes"}))
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, an unreadable switch must not abort startup", err)
	}
	if cfg.Enabled {
		t.Fatal("an unreadable switch must leave the transport disabled")
	}
	if len(cfg.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want exactly one", cfg.Warnings)
	}
	if !strings.Contains(cfg.Warnings[0], EnvEnabled) {
		t.Fatalf("the warning must name %s, got %q", EnvEnabled, cfg.Warnings[0])
	}
}

func TestLoadWorkerConfigLeavesAnUnreadableSwitchDisabled(t *testing.T) {
	cfg, err := LoadWorkerConfig(envLookup(map[string]string{EnvWorkerEnabled: "si"}))
	if err != nil {
		t.Fatalf("LoadWorkerConfig() error = %v, an unreadable switch must not abort startup", err)
	}
	if cfg.Enabled {
		t.Fatal("an unreadable switch must leave the worker disabled")
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], EnvWorkerEnabled) {
		t.Fatalf("Warnings = %v, want one naming %s", cfg.Warnings, EnvWorkerEnabled)
	}
}

// Apagar a propósito no es una degradación: no hay nada que corregir y no debe advertirse.
func TestDeliberatelyDisabledSwitchesWarnAboutNothing(t *testing.T) {
	for _, value := range []struct {
		name string
		env  map[string]string
	}{
		{name: "absent", env: map[string]string{}},
		{name: "empty", env: map[string]string{EnvEnabled: "", EnvWorkerEnabled: ""}},
		{name: "blank", env: map[string]string{EnvEnabled: "   ", EnvWorkerEnabled: "   "}},
		{name: "false", env: map[string]string{EnvEnabled: "false", EnvWorkerEnabled: "0"}},
	} {
		t.Run(value.name, func(t *testing.T) {
			cfg, err := LoadConfig(envLookup(value.env))
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			workerCfg, err := LoadWorkerConfig(envLookup(value.env))
			if err != nil {
				t.Fatalf("LoadWorkerConfig() error = %v", err)
			}
			if len(cfg.Warnings) != 0 || len(workerCfg.Warnings) != 0 {
				t.Fatalf("no warning expected, got %v and %v", cfg.Warnings, workerCfg.Warnings)
			}
		})
	}
}

func TestLoadConfigRequiresMandatoryValues(t *testing.T) {
	for _, required := range []string{EnvRegion, EnvQueueURL, EnvDLQURL} {
		for _, state := range []struct {
			name  string
			value string
			set   bool
		}{
			{name: "absent", set: false},
			{name: "empty", value: "", set: true},
			{name: "blank", value: "  \t ", set: true},
		} {
			t.Run(required+"/"+state.name, func(t *testing.T) {
				env := enabledEnv()
				if state.set {
					env[required] = state.value
				} else {
					delete(env, required)
				}

				_, err := LoadConfig(envLookup(env))
				if !errors.Is(err, ErrMissingQueueConfig) {
					t.Fatalf("LoadConfig() = %v, want ErrMissingQueueConfig", err)
				}
				if !strings.Contains(err.Error(), required) {
					t.Errorf("the error must name %s, got %q", required, err)
				}
			})
		}

		t.Run(required+"/present", func(t *testing.T) {
			if _, err := LoadConfig(envLookup(enabledEnv())); err != nil {
				t.Fatalf("LoadConfig() = %v, want nil", err)
			}
		})
	}
}

// Un despliegue sin DLQ debe fallar en el arranque y no descubrirse cuando un mensaje
// envenenado ya está circulando sin destino.
func TestLoadConfigRequiresDeadLetterQueue(t *testing.T) {
	env := enabledEnv()
	delete(env, EnvDLQURL)

	_, err := LoadConfig(envLookup(env))
	if !errors.Is(err, ErrMissingQueueConfig) {
		t.Fatalf("LoadConfig() = %v, want ErrMissingQueueConfig", err)
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	cfg, err := LoadConfig(envLookup(enabledEnv()))
	if err != nil {
		t.Fatalf("LoadConfig() = %v, want nil", err)
	}

	if cfg.VisibilityTimeoutSeconds != DefaultVisibilityTimeoutSeconds {
		t.Errorf("VisibilityTimeoutSeconds = %d, want %d", cfg.VisibilityTimeoutSeconds, DefaultVisibilityTimeoutSeconds)
	}
	if cfg.WaitTimeSeconds != DefaultWaitTimeSeconds {
		t.Errorf("WaitTimeSeconds = %d, want %d", cfg.WaitTimeSeconds, DefaultWaitTimeSeconds)
	}
	if cfg.MaxMessages != DefaultMaxMessages {
		t.Errorf("MaxMessages = %d, want %d", cfg.MaxMessages, DefaultMaxMessages)
	}
	if cfg.MaxRetryAttempts != DefaultMaxRetryAttempts {
		t.Errorf("MaxRetryAttempts = %d, want %d", cfg.MaxRetryAttempts, DefaultMaxRetryAttempts)
	}
}

// Recepción inmediata implica polling vacío constante y costo por request; el default debe
// ser long polling.
func TestLoadConfigEnablesLongPollingByDefault(t *testing.T) {
	cfg, err := LoadConfig(envLookup(enabledEnv()))
	if err != nil {
		t.Fatalf("LoadConfig() = %v, want nil", err)
	}

	if cfg.WaitTimeSeconds <= 0 {
		t.Fatalf("WaitTimeSeconds = %d, want long polling enabled", cfg.WaitTimeSeconds)
	}
}

func TestLoadConfigValidatesNumericRanges(t *testing.T) {
	bounds := []struct {
		key   string
		lower int
		upper int
		read  func(Config) int
	}{
		{EnvVisibilityTimeout, minVisibilityTimeoutSeconds, maxVisibilityTimeoutSeconds, func(c Config) int { return int(c.VisibilityTimeoutSeconds) }},
		{EnvWaitTime, minWaitTimeSeconds, maxWaitTimeSeconds, func(c Config) int { return int(c.WaitTimeSeconds) }},
		{EnvMaxMessages, minMaxMessages, maxMaxMessages, func(c Config) int { return int(c.MaxMessages) }},
		{EnvMaxRetryAttempts, minMaxRetryAttempts, maxMaxRetryAttempts, func(c Config) int { return c.MaxRetryAttempts }},
	}

	for _, bound := range bounds {
		accepted := []struct {
			name  string
			value int
		}{
			{"lower bound", bound.lower},
			{"upper bound", bound.upper},
		}
		for _, tt := range accepted {
			t.Run(bound.key+"/"+tt.name, func(t *testing.T) {
				env := enabledEnv()
				env[bound.key] = strconv.Itoa(tt.value)

				cfg, err := LoadConfig(envLookup(env))
				if err != nil {
					t.Fatalf("LoadConfig() = %v, want nil", err)
				}
				if got := bound.read(cfg); got != tt.value {
					t.Errorf("%s = %d, want %d", bound.key, got, tt.value)
				}
			})
		}

		rejected := []struct {
			name  string
			value string
		}{
			{"below lower bound", strconv.Itoa(bound.lower - 1)},
			{"above upper bound", strconv.Itoa(bound.upper + 1)},
			{"not a number", "twenty"},
			{"decimal", "1.5"},
		}
		for _, tt := range rejected {
			t.Run(bound.key+"/"+tt.name, func(t *testing.T) {
				env := enabledEnv()
				env[bound.key] = tt.value

				_, err := LoadConfig(envLookup(env))
				if !errors.Is(err, ErrInvalidQueueConfig) {
					t.Fatalf("LoadConfig() = %v, want ErrInvalidQueueConfig", err)
				}
				if !strings.Contains(err.Error(), bound.key) {
					t.Errorf("the error must name %s, got %q", bound.key, err)
				}
			})
		}

		t.Run(bound.key+"/blank falls back to the default", func(t *testing.T) {
			env := enabledEnv()
			env[bound.key] = "   "

			if _, err := LoadConfig(envLookup(env)); err != nil {
				t.Fatalf("LoadConfig() = %v, want nil", err)
			}
		})
	}
}

func TestLoadConfigEndpointIsOptional(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		cfg, err := LoadConfig(envLookup(enabledEnv()))
		if err != nil {
			t.Fatalf("LoadConfig() = %v, want nil", err)
		}
		if cfg.EndpointURL != "" {
			t.Errorf("EndpointURL = %q, want empty", cfg.EndpointURL)
		}
	})

	t.Run("present", func(t *testing.T) {
		env := enabledEnv()
		env[EnvEndpointURL] = " http://localhost:4566 "

		cfg, err := LoadConfig(envLookup(env))
		if err != nil {
			t.Fatalf("LoadConfig() = %v, want nil", err)
		}
		if cfg.EndpointURL != "http://localhost:4566" {
			t.Errorf("EndpointURL = %q, want the trimmed endpoint", cfg.EndpointURL)
		}
	})
}

// Los valores vienen del entorno: un secreto pegado en la variable equivocada no debe
// terminar en un log de arranque.
func TestLoadConfigErrorsNeverLeakValues(t *testing.T) {
	const secret = "AKIAIOSFODNN7EXAMPLE-super-secret"

	// La advertencia del interruptor sigue la misma regla que los errores: nombra la variable
	// y nunca su valor.
	t.Run("switch warning", func(t *testing.T) {
		cfg, err := LoadConfig(envLookup(map[string]string{EnvEnabled: secret}))
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if len(cfg.Warnings) != 1 {
			t.Fatalf("Warnings = %v, want exactly one", cfg.Warnings)
		}
		if strings.Contains(cfg.Warnings[0], secret) {
			t.Fatalf("the warning leaked the value: %q", cfg.Warnings[0])
		}
		if !strings.Contains(cfg.Warnings[0], EnvEnabled) {
			t.Fatalf("the warning must name %s, got %q", EnvEnabled, cfg.Warnings[0])
		}
	})

	tests := []struct {
		name string
		env  map[string]string
	}{
		{
			name: "invalid numeric value",
			env: func() map[string]string {
				env := enabledEnv()
				env[EnvWaitTime] = secret
				return env
			}(),
		},
		{
			name: "out of range numeric value",
			env: func() map[string]string {
				env := enabledEnv()
				env[EnvMaxMessages] = "99"
				return env
			}(),
		},
		{
			name: "missing required value",
			env: func() map[string]string {
				env := enabledEnv()
				delete(env, EnvQueueURL)
				return env
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(envLookup(tt.env))
			if err == nil {
				t.Fatal("LoadConfig() = nil, want an error")
			}

			message := err.Error()
			if strings.Contains(message, secret) {
				t.Errorf("the error leaked a value: %q", message)
			}
			for _, url := range []string{enabledEnv()[EnvQueueURL], enabledEnv()[EnvDLQURL]} {
				if strings.Contains(message, url) {
					t.Errorf("the error leaked a queue URL: %q", message)
				}
			}
		})
	}
}

func TestLoadConfigKeepsRequiredValues(t *testing.T) {
	env := enabledEnv()
	env[EnvRegion] = " us-east-2 "

	cfg, err := LoadConfig(envLookup(env))
	if err != nil {
		t.Fatalf("LoadConfig() = %v, want nil", err)
	}

	if !cfg.Enabled {
		t.Error("the queue must be enabled")
	}
	if cfg.Region != "us-east-2" {
		t.Errorf("Region = %q, want the trimmed region", cfg.Region)
	}
	if cfg.QueueURL != enabledEnv()[EnvQueueURL] {
		t.Errorf("QueueURL = %q, want the declared queue URL", cfg.QueueURL)
	}
	if cfg.DLQURL != enabledEnv()[EnvDLQURL] {
		t.Errorf("DLQURL = %q, want the declared dead letter queue URL", cfg.DLQURL)
	}
}

// El flag del worker es independiente del del transporte: una instancia puede querer emitir
// sin consumir, y la configuración tiene que poder expresar las cuatro combinaciones.
func TestLoadWorkerConfigIsIndependentOfTheTransport(t *testing.T) {
	tests := []struct {
		name         string
		transport    string
		worker       string
		wantEnabled  bool
		wantWorkerOn bool
	}{
		{name: "both off", transport: "false", worker: "false"},
		{name: "transport on, worker off", transport: "true", worker: "false", wantEnabled: true},
		{name: "transport on, worker on", transport: "true", worker: "true", wantEnabled: true, wantWorkerOn: true},
		// Consumir sin transporte es una combinación declarable y sin efecto: quien monta el
		// worker comprueba las dos cosas y explica por qué no arranca.
		{name: "transport off, worker on", transport: "false", worker: "true", wantWorkerOn: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := enabledEnv()
			env[EnvEnabled] = test.transport
			env[EnvWorkerEnabled] = test.worker

			cfg, err := LoadConfig(envLookup(env))
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			workerCfg, err := LoadWorkerConfig(envLookup(env))
			if err != nil {
				t.Fatalf("LoadWorkerConfig() error = %v", err)
			}

			if cfg.Enabled != test.wantEnabled {
				t.Fatalf("Enabled = %v, want %v", cfg.Enabled, test.wantEnabled)
			}
			if workerCfg.Enabled != test.wantWorkerOn {
				t.Fatalf("WorkerConfig.Enabled = %v, want %v", workerCfg.Enabled, test.wantWorkerOn)
			}
		})
	}
}

func TestLoadWorkerConfigDefaultsToOff(t *testing.T) {
	cfg, err := LoadWorkerConfig(envLookup(enabledEnv()))
	if err != nil {
		t.Fatalf("LoadWorkerConfig() error = %v", err)
	}
	if cfg.Enabled {
		t.Fatal("the worker must stay off unless it is explicitly enabled")
	}
}

// Las tres decisiones de arranque del consumidor, sin necesidad de montar la aplicación.
func TestShouldConsumeCoversEveryFlagCombination(t *testing.T) {
	tests := []struct {
		name      string
		transport bool
		worker    bool
		want      bool
	}{
		{name: "both off", transport: false, worker: false, want: false},
		{name: "transport on, worker off", transport: true, worker: false, want: false},
		{name: "transport off, worker on", transport: false, worker: true, want: false},
		{name: "both on", transport: true, worker: true, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			should, reason := WorkerConfig{Enabled: test.worker}.ShouldConsume(Config{Enabled: test.transport})
			if should != test.want {
				t.Fatalf("ShouldConsume() = %v, want %v", should, test.want)
			}
			if reason == "" {
				t.Fatal("every decision must carry a reason for the startup log")
			}
		})
	}
}

// Encendido sin transporte no es lo mismo que apagado: el primero es un despliegue mal
// armado y su motivo tiene que decirlo.
func TestShouldConsumeDistinguishesADisabledWorkerFromAMissingTransport(t *testing.T) {
	_, off := WorkerConfig{Enabled: false}.ShouldConsume(Config{Enabled: true})
	_, orphan := WorkerConfig{Enabled: true}.ShouldConsume(Config{Enabled: false})

	if off == orphan {
		t.Fatalf("both reasons read the same: %q", off)
	}
	if !strings.Contains(orphan, "queue is disabled") {
		t.Fatalf("the reason must name the missing transport, got %q", orphan)
	}
}
