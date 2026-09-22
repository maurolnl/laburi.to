package queue

import (
	"fmt"
	"strconv"
	"strings"
)

// Nombres de las variables de entorno. Se declaran como constantes porque los errores las
// nombran y los tests las referencian: un typo debe romper la compilación, no producir un
// mensaje que apunte a una variable inexistente.
const (
	EnvEnabled           = "RECOMMENDATIONS_QUEUE_ENABLED"
	EnvRegion            = "AWS_REGION"
	EnvQueueURL          = "AWS_SQS_RECOMMENDATIONS_QUEUE_URL"
	EnvDLQURL            = "AWS_SQS_RECOMMENDATIONS_DLQ_URL"
	EnvVisibilityTimeout = "AWS_SQS_VISIBILITY_TIMEOUT_SECONDS"
	EnvWaitTime          = "AWS_SQS_WAIT_TIME_SECONDS"
	EnvMaxMessages       = "AWS_SQS_MAX_MESSAGES"
	EnvMaxRetryAttempts  = "AWS_SQS_MAX_RETRY_ATTEMPTS"
	EnvEndpointURL       = "AWS_SQS_ENDPOINT_URL"
)

// Valores por defecto de los parámetros opcionales.
const (
	// DefaultVisibilityTimeoutSeconds da al worker un minuto para completar un batch antes
	// de que el mensaje vuelva a entregarse. LAB-33 puede extenderlo por mensaje con
	// Client.ExtendVisibility.
	DefaultVisibilityTimeoutSeconds = 60
	// DefaultWaitTimeSeconds es el máximo que admite SQS: long polling en vez de recepción
	// inmediata, que implicaría polling vacío constante y costo por request.
	DefaultWaitTimeSeconds = 20
	// DefaultMaxMessages es el máximo que admite SQS por recepción.
	DefaultMaxMessages = 10
	// DefaultMaxRetryAttempts son los reintentos del propio SDK ante fallos de red o
	// throttling. No tiene relación con el maxReceiveCount de la cola, que es un atributo
	// de AWS y no se configura desde la aplicación.
	DefaultMaxRetryAttempts = 3
)

// Rangos admitidos. Los de SQS son los que documenta el servicio; el de reintentos es una
// cota propia para que un valor absurdo no convierta un fallo en una espera larga.
const (
	minVisibilityTimeoutSeconds = 0
	maxVisibilityTimeoutSeconds = 43200 // 12 horas
	minWaitTimeSeconds          = 0
	maxWaitTimeSeconds          = 20
	minMaxMessages              = 1
	maxMaxMessages              = 10
	minMaxRetryAttempts         = 1
	maxMaxRetryAttempts         = 10
)

// Lookup resuelve una variable de entorno y distingue ausente de presente y vacía, igual
// que os.LookupEnv. Es un parámetro y no una llamada directa a os para que la validación se
// pueda ejercitar sin tocar el entorno del proceso que corre los tests.
type Lookup func(key string) (string, bool)

// Config es la configuración resuelta del transporte. Con Enabled en false el resto de los
// campos carece de significado: LoadConfig ni siquiera los leyó.
type Config struct {
	Enabled bool

	Region   string
	QueueURL string
	// DLQURL no se usa en runtime todavía. Es obligatoria para que un despliegue sin destino
	// para los mensajes que agotan sus reintentos falle en el arranque, en vez de
	// descubrirse cuando un mensaje envenenado circula sin salida.
	DLQURL string

	VisibilityTimeoutSeconds int32
	WaitTimeSeconds          int32
	MaxMessages              int32
	MaxRetryAttempts         int

	// EndpointURL apunta el cliente a un SQS local (LocalStack, ElasticMQ). Vacío significa
	// resolver el endpoint real de AWS por región.
	EndpointURL string
}

// LoadConfig resuelve y valida la configuración del transporte.
//
// Con el flag apagado devuelve Config{Enabled: false} sin error y sin consultar ninguna
// otra variable. Con el flag encendido, toda variable obligatoria ausente o vacía produce
// un error que envuelve ErrMissingQueueConfig, y todo valor numérico ilegible o fuera de
// rango uno que envuelve ErrInvalidQueueConfig.
//
// Los mensajes nombran la variable responsable y nunca incluyen su valor: los valores
// vienen del entorno y un secreto pegado en la variable equivocada terminaría en los logs.
func LoadConfig(lookup Lookup) (Config, error) {
	enabled, err := lookupBool(lookup, EnvEnabled)
	if err != nil {
		return Config{}, err
	}
	if !enabled {
		return Config{Enabled: false}, nil
	}

	cfg := Config{Enabled: true}

	for _, required := range []struct {
		key    string
		target *string
	}{
		{EnvRegion, &cfg.Region},
		{EnvQueueURL, &cfg.QueueURL},
		{EnvDLQURL, &cfg.DLQURL},
	} {
		value, err := lookupRequired(lookup, required.key)
		if err != nil {
			return Config{}, err
		}
		*required.target = value
	}

	for _, numeric := range []struct {
		key      string
		fallback int
		lower    int
		upper    int
		target   *int32
	}{
		{EnvVisibilityTimeout, DefaultVisibilityTimeoutSeconds, minVisibilityTimeoutSeconds, maxVisibilityTimeoutSeconds, &cfg.VisibilityTimeoutSeconds},
		{EnvWaitTime, DefaultWaitTimeSeconds, minWaitTimeSeconds, maxWaitTimeSeconds, &cfg.WaitTimeSeconds},
		{EnvMaxMessages, DefaultMaxMessages, minMaxMessages, maxMaxMessages, &cfg.MaxMessages},
	} {
		value, err := lookupBoundedInt(lookup, numeric.key, numeric.fallback, numeric.lower, numeric.upper)
		if err != nil {
			return Config{}, err
		}
		*numeric.target = int32(value)
	}

	retries, err := lookupBoundedInt(lookup, EnvMaxRetryAttempts, DefaultMaxRetryAttempts, minMaxRetryAttempts, maxMaxRetryAttempts)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxRetryAttempts = retries

	cfg.EndpointURL = strings.TrimSpace(lookupOptional(lookup, EnvEndpointURL))

	return cfg, nil
}

// lookupBool interpreta el flag de habilitación. Ausente o vacío significa deshabilitado;
// cualquier valor que strconv no entienda es un error y no un silencioso "false": un typo
// en el flag no debe apagar la cola sin avisar.
func lookupBool(lookup Lookup, key string) (bool, error) {
	raw, ok := lookup(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return false, nil
	}

	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%w: %s must be a boolean", ErrInvalidQueueConfig, key)
	}

	return value, nil
}

func lookupRequired(lookup Lookup, key string) (string, error) {
	raw, ok := lookup(key)
	value := strings.TrimSpace(raw)
	if !ok || value == "" {
		return "", fmt.Errorf("%w: %s", ErrMissingQueueConfig, key)
	}

	return value, nil
}

func lookupOptional(lookup Lookup, key string) string {
	raw, _ := lookup(key)
	return raw
}

func lookupBoundedInt(lookup Lookup, key string, fallback, lower, upper int) (int, error) {
	raw, ok := lookup(key)
	trimmed := strings.TrimSpace(raw)
	if !ok || trimmed == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer", ErrInvalidQueueConfig, key)
	}
	if value < lower || value > upper {
		return 0, fmt.Errorf("%w: %s must be between %d and %d", ErrInvalidQueueConfig, key, lower, upper)
	}

	return value, nil
}
