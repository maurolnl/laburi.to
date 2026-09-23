# Cola de recomendaciones (Amazon SQS)

Configuración del transporte asíncrono que lleva las solicitudes de recomendación entre el
productor y el worker. El código vive en `internal/queue`; la composición, en `cmd`.

Este documento **no contiene secretos**: ninguna URL real de cola, identificador de cuenta
ni credencial. Los valores concretos viven en el `.env` local, que no se versiona, y en las
variables del entorno desplegado.

## Estado actual

Implementado en LAB-31: configuración validada, puerto `queue.Client`, implementación sobre
SQS, implementación deshabilitada y doble en memoria para tests.

Implementado en LAB-32: el productor. `recommendation.QueueJobPublisher` abre el batch
`pending` del sujeto y publica el mensaje que lo referencia; `recommendation.JobPublisher` es
el puerto con el que el dominio lo consume.

Implementado en LAB-33: el consumidor. `recommendation.Worker` recibe los mensajes, resuelve
el universo de candidatos activos, invoca el contrato de scoring y reemplaza el conjunto
vigente en una transacción, reconociendo el mensaje solo después de persistir el desenlace.
Arranca como goroutine del mismo binario, detrás de `RECOMMENDATIONS_WORKER_ENABLED`.

Todavía **no** implementados: los disparadores del dominio (LAB-34) y los endpoints de
consulta (LAB-35). El productor existe pero **nadie lo llama**: `cmd/api.go` sigue cableando
`jobposition.NoopEventPublisher{}`, así que hoy ninguna ruta publica nada.

**Mientras el algoritmo de indicadores no exista, todo batch con candidatos termina en
`failed`.** La única implementación de producción del contrato de scoring es
`scoring.Unavailable`, y es la que `cmd/api.go` le inyecta al worker. Es el comportamiento
que la épica LAB-17 pide —la generación está bloqueada hasta que exista el algoritmo— y no un
defecto. Un sujeto sin candidatos sí completa, con conjunto vacío.

## Variables de entorno

| Variable | Obligatoria | Default | Rango |
| --- | --- | --- | --- |
| `RECOMMENDATIONS_QUEUE_ENABLED` | no | `false` | booleano que entienda Go (`true`, `false`, `1`, `0`); ilegible ⇒ `false` con advertencia |
| `RECOMMENDATIONS_WORKER_ENABLED` | no | `false` | ídem |
| `AWS_REGION` | sí, con la cola habilitada | — | no vacío |
| `AWS_SQS_RECOMMENDATIONS_QUEUE_URL` | sí, con la cola habilitada | — | no vacío |
| `AWS_SQS_RECOMMENDATIONS_DLQ_URL` | sí, con la cola habilitada | — | no vacío |
| `AWS_SQS_VISIBILITY_TIMEOUT_SECONDS` | no | `60` | 0–43200 |
| `AWS_SQS_WAIT_TIME_SECONDS` | no | `20` | 0–20 |
| `AWS_SQS_MAX_MESSAGES` | no | `10` | 1–10 |
| `AWS_SQS_MAX_RETRY_ATTEMPTS` | no | `3` | 1–10 |
| `AWS_SQS_ENDPOINT_URL` | no | — | URL de un SQS local |

Notas:

- **Un valor vacío o solo con espacios equivale a ausente.** Para las obligatorias eso aborta
  el arranque; para las opcionales, aplica el default.
- **Los dos interruptores nunca abortan el arranque.** Un valor que no se pueda interpretar
  —`si`, `yes`, `on`— los deja **apagados** y produce una advertencia en el log que nombra la
  variable. Describen *si* la funcionalidad participa, no *cómo*, y una funcionalidad apagada
  no debe impedir que la aplicación levante. El resto sí aborta: con la cola habilitada, una
  variable obligatoria ausente o un valor numérico fuera de rango describen cómo participa un
  transporte encendido, y adivinarlos haría desaparecer mensajes.
- **`AWS_REGION` se comparte con S3.** `internal/uploader` la trata como opcional y cae a
  `us-east-2`; acá es obligatoria con la cola habilitada. La divergencia es deliberada: un
  bucket en la región equivocada da error visible, pero una cola en la región equivocada
  hace desaparecer mensajes.
- **`AWS_SQS_MAX_RETRY_ATTEMPTS` son los reintentos del SDK** ante fallos de red o
  throttling. No tiene relación con `maxReceiveCount`, que es un atributo de la cola en AWS.
- **No hay credenciales nuevas.** El SDK resuelve las mismas que ya usa S3 por la cadena
  estándar (`AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`, perfil, o rol de la instancia).
- **`RECOMMENDATIONS_WORKER_ENABLED` es independiente de `RECOMMENDATIONS_QUEUE_ENABLED`.**
  Publicar y consumir son decisiones de despliegue distintas: una instancia puede emitir sin
  dedicar su proceso a procesar. Se resuelve con `queue.LoadWorkerConfig`, aparte de
  `queue.LoadConfig`, porque con el transporte apagado `LoadConfig` no lee ninguna otra
  variable. Con el worker encendido y el transporte apagado el ciclo **no arranca** y el
  arranque lo registra: no hay de dónde consumir.

## Arranque local

### Sin cola

Es el modo por defecto. Con `RECOMMENDATIONS_QUEUE_ENABLED` ausente o en `false`, el proceso
no lee ninguna otra variable del transporte, no construye ningún cliente de AWS y arranca
con el contrato HTTP actual sin cambios. Toda operación sobre la cola falla con
`queue.ErrQueueDisabled`.

```bash
go run ./cmd
# Recommendation queue enabled: false
```

### Con un SQS local

`AWS_SQS_ENDPOINT_URL` apunta el cliente a LocalStack o ElasticMQ en lugar del servicio real.
Con LocalStack:

```bash
# 1. Levantar LocalStack con SQS
docker run --rm -p 4566:4566 -e SERVICES=sqs localstack/localstack

# 2. Crear la DLQ y la cola principal con su redrive policy
awslocal sqs create-queue --queue-name laburito-recommendations-dlq

awslocal sqs create-queue \
  --queue-name laburito-recommendations \
  --attributes VisibilityTimeout=60,ReceiveMessageWaitTimeSeconds=20,RedrivePolicy='{"deadLetterTargetArn":"<arn-de-la-dlq>","maxReceiveCount":"3"}'
```

Y en el `.env` local:

```
RECOMMENDATIONS_QUEUE_ENABLED=true
AWS_REGION=us-east-2
AWS_SQS_ENDPOINT_URL=http://localhost:4566
AWS_SQS_RECOMMENDATIONS_QUEUE_URL=<url que devolvió create-queue>
AWS_SQS_RECOMMENDATIONS_DLQ_URL=<url que devolvió create-queue de la dlq>
```

LocalStack acepta credenciales de prueba; no hace falta ninguna credencial real.

> Los tests **nunca** necesitan esto. `internal/queue/queuetest` provee un doble en memoria
> con las mismas garantías observables, y `go test ./...` corre sin red ni credenciales.

## Arranque desplegado

1. Aprovisionar la DLQ primero y después la cola principal con la redrive policy apuntando a
   su ARN (ver atributos más abajo).
2. Declarar las variables en el entorno del servicio. Si falta alguna obligatoria, el proceso
   **no arranca** y el despliegue anterior sigue en pie.
3. Encender `RECOMMENDATIONS_QUEUE_ENABLED`.

El rol o las credenciales del servicio necesitan, sobre la cola principal:
`sqs:SendMessage`, `sqs:ReceiveMessage`, `sqs:DeleteMessage`,
`sqs:ChangeMessageVisibility` y `sqs:GetQueueAttributes`. Sobre la DLQ, solo lectura para
inspección.

Rollback: apagar el flag. Este cambio no persiste ningún estado.

## Atributos que deben tener las colas

La aplicación **no** crea ni configura las colas: estos atributos se definen al
aprovisionarlas y quedan del lado de AWS.

### Cola principal

| Atributo | Valor recomendado | Por qué |
| --- | --- | --- |
| `VisibilityTimeout` | `60` | Debe coincidir con `AWS_SQS_VISIBILITY_TIMEOUT_SECONDS` o superarlo. El worker puede extenderlo por mensaje con `ExtendVisibility`. |
| `ReceiveMessageWaitTimeSeconds` | `20` | Long polling también a nivel de cola, para que un consumidor mal configurado no haga polling vacío. |
| `MessageRetentionPeriod` | `345600` (4 días) | Margen para reprocesar tras una caída del worker. |
| `RedrivePolicy.deadLetterTargetArn` | ARN de la DLQ | Destino de los mensajes que agotan sus reintentos. |
| `RedrivePolicy.maxReceiveCount` | `3` | Cuántas entregas fallidas se toleran antes de mandar el mensaje a la DLQ. |

Cola **estándar**, no FIFO: no se necesita orden total, y la idempotencia ya está garantizada
en la base por el índice único parcial que impide dos batches `pending` o `processing` para
el mismo sujeto.

### Cola de mensajes fallidos

| Atributo | Valor recomendado | Por qué |
| --- | --- | --- |
| `MessageRetentionPeriod` | `1209600` (14 días) | Máximo de SQS: un mensaje envenenado debe poder inspeccionarse mucho después. |

La DLQ no tiene su propia redrive policy.

## Garantía de entrega: al menos una vez

SQS entrega **al menos una vez**, nunca exactamente una vez. En concreto:

- Un mensaje recibido queda invisible para otros consumidores mientras dure su visibility
  timeout.
- Si no se confirma con `Delete` antes de que venza, vuelve a entregarse con un
  `ReceiveCount` mayor y un receipt handle nuevo.
- Tras `maxReceiveCount` entregas sin confirmación, el mensaje termina en la DLQ.

Consecuencias para quien consuma la cola:

- **El procesamiento debe ser idempotente.** Procesar dos veces el mismo mensaje no puede
  duplicar recomendaciones ni corromper un batch.
- **Confirmar recién al terminar.** Un `Delete` antes de completar el trabajo convierte un
  fallo en una pérdida silenciosa.
- **Extender la visibilidad en trabajos largos**, en lugar de subir el visibility timeout
  global de la cola.
- **`ReceiveCount` es aproximado.** Sirve para detectar un redelivery, no para contar con
  exactitud.

`internal/recommendation` ya está preparado para esto: `CreateBatch` devuelve
`ErrBatchAlreadyInFlight` cuando el sujeto ya tiene un batch `pending` o `processing`, que es
como la base absorbe un mensaje duplicado sin generar trabajo paralelo.

## Sustitución en tests

`internal/queue/queuetest` provee `Fake`, un `queue.Client` en memoria con reloj inyectable:

```go
fake := queuetest.New().WithVisibilityTimeout(30 * time.Second)

_ = fake.Send(ctx, `{"subject_type":"employee","employee_id":7}`)

messages, _ := fake.Receive(ctx)   // ReceiveCount = 1
fake.Advance(31 * time.Second)     // vence la visibilidad
messages, _ = fake.Receive(ctx)    // el mismo mensaje, ReceiveCount = 2

fake.Sent()                        // todos los cuerpos publicados
fake.FailSend(errors.New("boom"))  // fuerza el fallo de una operación
```

Es un paquete normal y no un `_test.go` porque sus consumidores viven en otros paquetes. No
entra al binario: ninguna ruta de producción lo importa.

## Contrato del mensaje

El transporte no interpreta el cuerpo: lo define `internal/recommendation`. Un mensaje es un
**sobre de ruteo**, no una copia del dominio.

```json
{
  "event_id": "11111111-1111-1111-1111-111111111111",
  "version": 1,
  "subject_type": "employee",
  "subject_id": 12,
  "batch_id": 77,
  "emitted_at": "2026-09-22T15:34:56Z"
}
```

| Campo | Para qué |
| --- | --- |
| `event_id` | Identifica esta emisión. Distinto en cada una, incluso para el mismo sujeto; dos entregas del mismo mensaje lo conservan. |
| `version` | Versión del contrato. Un consumidor que no la implementa rechaza el mensaje en vez de interpretarlo mal. Entero monótono: un cambio incompatible lo sube. |
| `subject_type` | `employee` o `job_position`. Es lo que da sentido a `subject_id`. |
| `subject_id` | Identificador del empleado o del puesto. Uno solo: a diferencia del batch en la base, el mensaje no repite el par excluyente. |
| `batch_id` | Identifica el trabajo a ejecutar. Le alcanza a un consumidor idempotente para decidir si ya lo hizo. |
| `emitted_at` | Instante de emisión, siempre en UTC. |

**El mensaje no lleva datos personales, credenciales ni el perfil del empleado o la
descripción del puesto.** El consumidor resuelve desde la base todo lo que necesite a partir
de los identificadores. Agregar un campo acá es agregarlo a algo que viaja fuera del proceso
y queda en la cola.

### Desenlaces de una emisión

| Situación | Qué pasa |
| --- | --- |
| El sujeto no tiene trabajo en curso | Se abre un batch `pending` y se publica su mensaje. |
| El sujeto ya tiene un batch `pending` o `processing` | No se publica nada y la emisión se resuelve **sin error**: el trabajo en curso ya va a producir el conjunto. Queda una línea de diagnóstico. |
| Falla la apertura del batch | No se publica nada y el error se propaga. |
| Falla la publicación | El batch recién abierto pasa a `failed` y el error se propaga. El sujeto no queda bloqueado: la próxima solicitud abre un batch nuevo. |
| El transporte está deshabilitado | Igual que un fallo de publicación. Reportar éxito sin haber enviado nada sería mentir. |

El orden importa: **primero el batch, después el mensaje.** Ningún mensaje puede referirse a
un batch inexistente. No hay reintento de publicación: un fallo cierra el batch en `failed` y
termina ahí.

Queda una ventana conocida sin cubrir: si el proceso muere entre abrir el batch y publicar,
ese batch queda `pending` sin mensaje y bloquea al sujeto. Cerrarla exige un outbox
transaccional, desproporcionado para el volumen actual.

## Consumo del mensaje

`recommendation.Worker` procesa un mensaje en este orden. No es arbitrario: los candidatos se
resuelven con el batch todavía en `pending` para que los dos desenlaces que no prosperan
—universo vacío y scoring no disponible— no abran `processing`. Abrirlo dejaría al sujeto
bloqueado por el índice único parcial ante cualquier fallo posterior.

1. Interpretar el cuerpo y validar su `version`.
2. Leer el batch. Inexistente o ya terminal: reconocer sin trabajo.
3. Resolver el universo de candidatos activos del sujeto.
4. Universo vacío: reclamar, completar con lista vacía, reconocer. **No se consulta el
   scoring.**
5. Con candidatos y scoring no disponible: `pending` → `failed`, reconocer.
6. Con candidatos y scoring disponible: reclamar (`processing`), puntuar, reemplazar el
   conjunto, reconocer.

### Idempotencia

El reclamo del batch es condicional: solo prospera desde `pending` o `processing`. Un
redelivery de un batch ya `completed` o `failed` no lo reabre, así que el conjunto vigente que
dejó queda intacto.

El predicado incluye `processing` a propósito. Excluirlo haría el reclamo estrictamente
exclusivo, pero convertiría cualquier caída del worker a mitad de un batch en un sujeto
bloqueado para siempre. Que dos entregas se solapen es inocuo: completar un batch es un
reemplazo atómico completo y el último en commitear gana. El worker renueva la visibilidad del
mensaje mientras procesa, lo que reduce esa ventana.

### Clasificación de errores

| Situación | Batch | Mensaje |
| --- | --- | --- |
| Cuerpo ilegible o versión desconocida | no se toca | reconocer |
| Batch inexistente o ya terminal | no se toca | reconocer |
| Sujeto inexistente o puesto eliminado | `completed` vacío | reconocer |
| Sin candidatos | `completed` vacío | reconocer |
| Scoring no disponible | `pending` → `failed` | reconocer |
| Par inválido o fallo de scoring | `failed` | reconocer |
| Fallo de base de datos | como haya quedado | **no** reconocer |
| Fallo del propio reconocimiento | ya persistido | el redelivery lo reconoce |

La línea divisoria es si reintentar puede cambiar el desenlace. Un cuerpo ilegible se lee
igual de mal la décima vez: devolverlo a la cola solo consume entregas hasta la DLQ y demora
los mensajes sanos que vienen detrás. Un fallo de base de datos sí puede resolverse solo, así
que el mensaje vuelve, y si nunca se resuelve el `maxReceiveCount` de la cola lo deriva a la
DLQ.

**Scoring no disponible se reconoce y no se reintenta**, aunque parezca recuperable. Mientras
el algoritmo no exista, reintentar llevaría todo el tráfico a la DLQ y dejaría al sujeto
esperando; con el batch en `failed` el sujeto queda libre para una solicitud nueva, y el
rastro del motivo queda donde se puede consultar.

### Deuda conocida

Un empleado se empareja contra **todos** los puestos vigentes, y un puesto contra todos los
empleados, sin cota. Con el volumen actual no es un problema, y acotarlo bien exige los
filtros duros que la épica prohíbe definir mientras no exista el algoritmo.

## Referencias

- Épica LAB-17 y tickets LAB-31, LAB-32 y LAB-33.
- Cambios OpenSpec `configure-recommendation-queue`, `publish-recommendation-jobs` y
  `consume-recommendation-jobs`; capacidades `recommendation-queue-configuration`,
  `recommendation-job-publishing` y `recommendation-job-consumption`.
