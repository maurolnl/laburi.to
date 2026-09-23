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

	// EnvWorkerEnabled decide si esta instancia consume la cola. Es un flag propio y no
	// reutiliza EnvEnabled porque publicar y consumir son decisiones de despliegue
	// distintas: una instancia de API puede querer emitir solicitudes sin dedicar su
	// proceso a procesarlas.
	EnvWorkerEnabled = "RECOMMENDATIONS_WORKER_ENABLED"
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

	// Warnings son las degradaciones que hay que registrar en el arranque: hoy, un
	// interruptor ilegible que dejó el transporte apagado.
	//
	// Viajan en el resultado y no en un log de este paquete a propósito. La configuración se
	// resuelve con un Lookup inyectable justamente para ejercitarla sin tocar el entorno ni
	// la salida del proceso de tests; un log.Printf acá ensuciaría toda la suite y haría que
	// comprobar la advertencia exigiera capturar salida.
	Warnings []string
}

// WorkerConfig decide si este proceso consume la cola.
//
// Es un tipo aparte y no un campo de Config porque se resuelve aparte: Config describe el
// transporte, y con el transporte apagado LoadConfig no lee ninguna otra variable. Consumir
// no es transporte, es qué hace esta instancia con él, y su flag tiene que poder leerse en
// las cuatro combinaciones para que quien monta el worker pueda explicar por qué no arranca
// en vez de que el flag desaparezca en silencio.
type WorkerConfig struct {
	Enabled bool

	// Warnings tiene el mismo significado y la misma razón de ser que Config.Warnings.
	Warnings []string
}

// LoadWorkerConfig resuelve el flag del consumidor. Un valor que no se entienda deja el
// worker apagado y produce una advertencia, igual que el flag del transporte: apagar es la
// degradación segura y la advertencia evita que un typo pase inadvertido.
//
// Devuelve error para no cerrarle la puerta a una validación futura, aunque hoy ningún
// camino lo produzca.
//
// Habilitado no alcanza para arrancar: sin transporte no hay de dónde consumir. Esa
// comprobación pertenece a quien monta el worker, que es el único que ve las dos
// configuraciones.
func LoadWorkerConfig(lookup Lookup) (WorkerConfig, error) {
	enabled, warning := lookupSwitch(lookup, EnvWorkerEnabled)

	cfg := WorkerConfig{Enabled: enabled}
	if warning != "" {
		cfg.Warnings = append(cfg.Warnings, warning)
	}

	return cfg, nil
}

// LoadConfig resuelve y valida la configuración del transporte.
//
// Con el flag apagado devuelve Config{Enabled: false} sin error y sin consultar ninguna
// otra variable. Un flag ilegible cuenta como apagado y suma una advertencia, en vez de
// abortar: es un interruptor, y una funcionalidad apagada no debe impedir que la aplicación
// arranque. Con el flag encendido, toda variable obligatoria ausente o vacía produce un
// error que envuelve ErrMissingQueueConfig, y todo valor numérico ilegible o fuera de rango
// uno que envuelve ErrInvalidQueueConfig: esos describen cómo participa un transporte ya
// encendido, y adivinarlos haría desaparecer mensajes.
//
// Los mensajes nombran la variable responsable y nunca incluyen su valor: los valores
// vienen del entorno y un secreto pegado en la variable equivocada terminaría en los logs.
func LoadConfig(lookup Lookup) (Config, error) {
	enabled, warning := lookupSwitch(lookup, EnvEnabled)

	var warnings []string
	if warning != "" {
		warnings = append(warnings, warning)
	}

	if !enabled {
		return Config{Enabled: false, Warnings: warnings}, nil
	}

	cfg := Config{Enabled: true, Warnings: warnings}

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

// lookupSwitch interpreta un interruptor de habilitación. Ausente, vacío o ilegible
// significan deshabilitado; el segundo valor es la advertencia a registrar, vacía cuando no
// hay nada que corregir.
//
// Un valor ilegible degrada a apagado y no aborta el arranque porque el interruptor describe
// si una funcionalidad participa, no cómo participa. Apagar es la única degradación segura:
// encender por error un transporte cuya configuración nadie revisó llevaría a fallos contra
// una cola que quizá no existe, mientras que apagado lo peor que pasa es que no se generen
// recomendaciones, que es el estado actual del producto.
//
// La advertencia conserva la intención original —un typo no debe apagar la cola en
// silencio— sin el precio de que una funcionalidad apagada impida levantar la aplicación.
// Nombra la variable y nunca incluye su valor, igual que los errores: los valores vienen del
// entorno y un secreto pegado en la variable equivocada no debe terminar en un log.
func lookupSwitch(lookup Lookup, key string) (enabled bool, warning string) {
	raw, ok := lookup(key)
	trimmed := strings.TrimSpace(raw)
	if !ok || trimmed == "" {
		return false, ""
	}

	value, err := strconv.ParseBool(trimmed)
	if err != nil {
		return false, fmt.Sprintf("%s is not a boolean: leaving it disabled", key)
	}

	return value, ""
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

// ShouldConsume informa si esta instancia debe iniciar el ciclo de consumo, y con qué motivo
// cuando no.
//
// Existe como función y no como una condición suelta en cmd porque la decisión combina las
// dos configuraciones y tiene tres desenlaces, no dos: apagado a propósito, encendido sin
// transporte —que es un despliegue mal armado y debe ser visible— y encendido.
//
// El motivo es para una línea de log: no nombra ninguna URL, región ni credencial.
func (w WorkerConfig) ShouldConsume(transport Config) (bool, string) {
	switch {
	case !w.Enabled:
		return false, "recommendation worker disabled"
	case !transport.Enabled:
		return false, "recommendation worker enabled but the recommendation queue is disabled: nothing to consume"
	default:
		return true, "recommendation worker enabled"
	}
}
