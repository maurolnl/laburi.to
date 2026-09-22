## Context

Ver `proposal.md` — Why para la motivación y `specs/recommendation-queue-configuration/spec.md`
para los requirements.

Restricciones del código actual que condicionan el enfoque:

- **No hay ninguna capa de configuración.** `cmd/main.go` construye `appConfig` leyendo
  `os.Getenv` sin validar, `cmd/api.go` vuelve a leer `SPA_URL` y `DB_URL` por su cuenta, y
  `internal/uploader` carga la configuración de AWS dentro del paquete y aborta con
  `log.Fatal`. Este cambio introduce el primer componente con configuración validada; el
  patrón que elija va a ser copiado.
- **El SDK de AWS v2 ya es dependencia directa** (`aws-sdk-go-v2`, `config`, `credentials`,
  `service/s3`). Falta únicamente `service/sqs`.
- **El puerto del productor ya existe**: `jobposition.JobPositionEventPublisher`, cableado
  hoy con `NoopEventPublisher{}` en `cmd/api.go:75`. LAB-32 lo reemplaza; este cambio no lo
  toca.
- **No hay consumidor todavía.** El cliente que este cambio construye no se usa en ninguna
  ruta de ejecución hasta LAB-32. Eso obliga a decidir qué hace el arranque cuando la cola
  no está configurada.
- **Precedente de `internal/scoring`** (LAB-30): contrato en el paquete, implementación de
  producción que falla explícitamente en vez de simular, doble determinista para tests.
  Este cambio sigue esa forma.

## Goals / Non-Goals

**Goals:**

- Un único punto de resolución y validación de la configuración de cola, testeable sin
  tocar el entorno del proceso.
- Un puerto de cola que LAB-32 y LAB-33 puedan consumir sin importar el SDK de AWS.
- Un doble en memoria con las garantías observables de SQS, usable desde otros paquetes.
- Composición explícita en `cmd`, sin globals ni `init()`.

**Non-Goals:**

- Migrar `internal/uploader` ni el resto de `cmd` al patrón de configuración nuevo. Se deja
  la divergencia documentada.
- Reintentos de aplicación, backoff propio, circuit breaker o métricas. El SDK ya reintenta
  y SQS ya redriveia; agregar una capa encima sin un consumidor real sería especulativo.
- Serialización del mensaje (qué va en el cuerpo). Es una decisión de LAB-32.
- Aprovisionar cola y DLQ.

## Decisions

### D1: `internal/queue` como paquete propio, no dentro de `internal/recommendation`

El transporte no es persistencia. `internal/recommendation` ya tiene su repositorio, sus
modelos y su store; meterle un cliente de AWS lo obligaría a depender del SDK para
compilar sus tests de base de datos. Además LAB-32 publica desde `jobposition` y LAB-34
desde `employee`: ninguno de los dos debería importar el paquete de persistencia de
recomendaciones solo para acceder a la cola.

*Alternativa descartada:* `internal/recommendation/queue`. Ata el transporte a un paquete
con el que no comparte ciclo de vida ni dependencias.

### D2: Flag de habilitación en vez de configuración siempre obligatoria

`RECOMMENDATIONS_QUEUE_ENABLED` decide si la cola participa del arranque. Con la cola
deshabilitada no se lee ninguna otra variable y no se construye ningún cliente de AWS.

El criterio de aceptación pide fallo temprano ante un valor requerido faltante; el flag
define *cuándo* un valor es requerido, no lo relaja: con la cola habilitada, toda variable
obligatoria faltante aborta el arranque.

La alternativa —exigir siempre las variables— rompería `go run ./cmd` y `go test ./...`
para cualquiera que no tenga una cola aprovisionada, a cambio de ningún beneficio: hoy no
hay productor ni worker. La otra alternativa —inferir el estado de la presencia de
`AWS_SQS_RECOMMENDATIONS_QUEUE_URL`— convierte un typo en la variable en "cola
silenciosamente deshabilitada", que es exactamente el modo de fallo que este ticket quiere
evitar.

### D3: La habilitación se representa como un campo de `Config`, no como dos tipos

`queue.Config` lleva `Enabled bool`. `LoadConfig` devuelve `Config{Enabled: false}` sin
error cuando el flag está apagado, y valida todo lo demás cuando está encendido. `cmd`
decide con ese campo si construye el cliente real o el deshabilitado.

*Alternativa descartada:* devolver `(Config, bool, error)` o un tipo opcional. Agrega
ceremonia en el único llamador sin ganar nada.

### D4: Lookup inyectable en vez de `os.Getenv` directo

```go
type Lookup func(key string) (string, bool)
func LoadConfig(lookup Lookup) (Config, error)
```

`cmd` pasa `os.LookupEnv`; los tests pasan un mapa. Es lo que permite probar la matriz
completa de faltantes, vacíos y fuera de rango sin `t.Setenv` ni estado compartido entre
tests paralelos. `os.LookupEnv` distingue "ausente" de "presente y vacío", distinción que
el spec necesita.

### D5: Puerto `queue.Client` con cuatro operaciones y tipos propios

```go
type Message struct {
    ID            string
    Body          string
    ReceiptHandle string
    ReceiveCount  int
}

type Client interface {
    Send(ctx context.Context, body string) error
    Receive(ctx context.Context) ([]Message, error)
    Delete(ctx context.Context, receiptHandle string) error
    ExtendVisibility(ctx context.Context, receiptHandle string, seconds int32) error
}
```

Las cuatro son exactamente las que LAB-32 y LAB-33 necesitan. `Send` recibe un `string` y
no un tipo de dominio: el paquete transporta, no serializa; qué contiene el cuerpo lo
decide LAB-32.

`ReceiveCount` se expone porque el worker de LAB-33 necesita distinguir un primer intento
de un redelivery para decidir cuándo llevar un batch a `failed`. Viene del atributo
`ApproximateReceiveCount` de SQS; el nombre del campo omite el "Approximate" pero la
documentación del tipo aclara que es aproximado.

`Receive` no toma parámetros de lote ni de espera: salen de `Config`. Un consumidor que
pudiera elegirlos por llamada volvería inútil la validación de rango del arranque.

*Alternativa descartada:* exponer `*sqs.Client` y que cada consumidor arme sus inputs.
Filtra el SDK a `jobposition` y a `employee`, y obliga a cada test de esos paquetes a
mockear la API de AWS.

### D6: `Disabled` en vez de `nil` — mismo patrón que `scoring.Unavailable`

Con la cola deshabilitada, `cmd` inyecta `queue.Disabled{}`, que implementa `Client` y
devuelve `ErrQueueDisabled` en las cuatro operaciones. Nadie recibe `nil` ni tiene que
comprobarlo antes de cada llamada.

Diferencia deliberada con `jobposition.NoopEventPublisher`, que devuelve `nil`: un noop es
correcto en el borde HTTP, donde perder un evento no debe hacer fallar un alta ya
persistida. Dentro del transporte, devolver éxito sin enviar nada es mentir. Quien decide
tragarse el error sigue siendo `jobposition.publish`, que ya lo hace y lo registra.

### D7: Doble de test en `internal/queue/queuetest`, no en un `_test.go`

`internal/scoring` puso su fake en `fake_test.go` porque su único consumidor futuro
—LAB-33— iba a estar en otro paquete pero el contrato se ejercita dentro del propio
paquete. Acá los consumidores son `jobposition` (LAB-32) y el worker (LAB-33), ambos en
paquetes distintos, y ambos necesitan el mismo doble. Un `_test.go` no es importable.

`queuetest` es un paquete normal, como `net/http/httptest`. No entra al binario porque
ninguna ruta de producción lo importa, y una tarea lo verifica explícitamente con
`go list -deps ./cmd`.

El doble reproduce lo observable de SQS, no su implementación: entrega al menos una vez,
mensaje invisible tras recibirse, redelivery al vencer la visibilidad y `ReceiveCount`
creciente. Para que el redelivery sea comprobable sin esperas reales, el reloj del doble es
inyectable.

### D8: Variables de entorno y defaults

| Variable | Obligatoria | Default | Rango |
| --- | --- | --- | --- |
| `RECOMMENDATIONS_QUEUE_ENABLED` | no | `false` | `true`/`false` |
| `AWS_REGION` | sí, si habilitada | — | no vacío |
| `AWS_SQS_RECOMMENDATIONS_QUEUE_URL` | sí, si habilitada | — | no vacío |
| `AWS_SQS_RECOMMENDATIONS_DLQ_URL` | sí, si habilitada | — | no vacío |
| `AWS_SQS_VISIBILITY_TIMEOUT_SECONDS` | no | `60` | 0–43200 |
| `AWS_SQS_WAIT_TIME_SECONDS` | no | `20` | 0–20 |
| `AWS_SQS_MAX_MESSAGES` | no | `10` | 1–10 |
| `AWS_SQS_MAX_RETRY_ATTEMPTS` | no | `3` | 1–10 |
| `AWS_SQS_ENDPOINT_URL` | no | — | URL |

Notas:

- **Prefijo `AWS_SQS_`** para los parámetros del transporte y `RECOMMENDATIONS_` para el
  flag de feature. `AWS_REGION` se reutiliza tal cual: ya la lee `internal/uploader` y
  tener dos regiones distintas para el mismo proyecto sería un accidente, no una feature.
- **`AWS_REGION` es obligatoria acá y opcional en `uploader`**, que cae a `us-east-2`. Un
  bucket en la región equivocada da error; una cola en la región equivocada, con la URL
  apuntando a otra, es un fallo confuso. Se prefiere fallar. La divergencia queda
  documentada en `CLAUDE.md`; unificar `uploader` es trabajo aparte.
- **`AWS_SQS_WAIT_TIME_SECONDS` default 20**: long polling en el máximo que admite SQS.
  Recepción inmediata implica polling vacío constante y costo por request.
- **`AWS_SQS_MAX_RETRY_ATTEMPTS`** configura los reintentos del propio SDK ante fallos de
  red o throttling (`config.WithRetryMaxAttempts`). Es distinto de `maxReceiveCount`, que
  es un atributo de la cola en AWS y **no** se configura desde la aplicación.
- **`AWS_SQS_RECOMMENDATIONS_DLQ_URL` es obligatoria aunque el proceso no la use en runtime
  hoy.** Es la forma de que un despliegue sin DLQ falle en el arranque en vez de descubrirse
  cuando un mensaje envenenado circula sin destino. LAB-33 la usará para herramientas de
  inspección.
- **Sin credenciales nuevas.** La cadena estándar del SDK resuelve las mismas que ya usa S3.

### D9: Errores sentinela clasificables

```go
var (
    ErrQueueDisabled      = errors.New("recommendation queue is disabled")
    ErrMissingQueueConfig = errors.New("missing required recommendation queue configuration")
    ErrInvalidQueueConfig = errors.New("invalid recommendation queue configuration")
)
```

Los errores de carga envuelven `ErrMissingQueueConfig` o `ErrInvalidQueueConfig` y nombran
la variable (`AWS_SQS_WAIT_TIME_SECONDS`), nunca su valor. Es la misma regla que ya aplica
`internal/scoring` con `ErrScoringUnavailable` / `ErrInvalidPair`, y la que el spec exige
para no clasificar por texto.

Incluir el valor inválido en el mensaje ayudaría a depurar, pero los valores llegan del
entorno: un secreto pegado en la variable equivocada terminaría en los logs. No se incluyen.

### D10: Composición en `cmd`

```
main.go   loadEnv → queue.LoadConfig(os.LookupEnv) → error ⇒ logErrorAndFail
api.go    appConfig.queue → queue.New(ctx, cfg) | queue.Disabled{} → application.queueClient
```

`queue.New` devuelve `(Client, error)`; no hay `log.Fatal` dentro de `internal/queue`. El
cliente vive como campo de `application`, igual que el resto de la configuración. Hasta
LAB-32 nadie lo lee: el arranque solo registra una línea con el estado —habilitada o no— y
sin la URL.

Dejar el campo sin consumir es deliberado y es lo que el ticket pide ("componer
dependencias en `cmd` sin globals"). La alternativa —construir el cliente recién en LAB-32—
dejaría este ticket sin nada que componer y sin forma de verificar el fail-fast.

## Risks / Trade-offs

- **Configuración muerta hasta LAB-32** → El cliente se construye y no se usa. Mitigación:
  el arranque registra el estado del transporte, y un test cubre que con la cola habilitada
  y configuración válida el cliente se construye sin error. Si LAB-32 se demora, lo peor que
  pasa es una conexión ociosa que el SDK no abre hasta la primera llamada.
- **El flag permite desplegar producción con la cola apagada por error** → Nadie se entera,
  porque no hay alerta. Mitigación: el log de arranque dice explícitamente que el transporte
  está deshabilitado; LAB-32 y LAB-33 deben tratar `ErrQueueDisabled` como fallo visible y
  no como silencio.
- **El doble de `queuetest` puede divergir de SQS** → Un test verde contra un doble que
  miente. Mitigación: el doble reproduce solo garantías observables y documentadas, no
  timing ni ordenamiento; SQS estándar no garantiza orden y el doble tampoco lo promete.
- **Dos formas de configurar AWS en el mismo repositorio** (`uploader` con `log.Fatal` y
  default de región, `queue` con error y región obligatoria) → Confusión para quien agregue
  el tercer servicio. Mitigación: la divergencia y su motivo quedan escritos en `CLAUDE.md`
  y `AGENTS.md`; migrar `uploader` es un ticket aparte, no un efecto colateral de este.
- **`maxReceiveCount` y la redrive policy no son verificables desde la aplicación** → Una
  cola creada sin DLQ arranca igual mientras las variables estén declaradas. Mitigación:
  documentar los atributos requeridos; la verificación real llega con el aprovisionamiento
  por toolkit/MCP de AWS, que el ticket deja explícitamente para después.

## Migration Plan

No hay migración de datos ni cambio de contrato. Despliegue:

1. Desplegar con `RECOMMENDATIONS_QUEUE_ENABLED` ausente: comportamiento idéntico al
   actual.
2. Aprovisionar cola y DLQ con los atributos documentados (fuera de este cambio).
3. Declarar las variables y encender el flag. Si falta alguna, el proceso no arranca y el
   despliegue anterior sigue en pie.

Rollback: apagar el flag. No queda estado persistido por este cambio.
