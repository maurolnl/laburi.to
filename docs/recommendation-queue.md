# Cola de recomendaciones (Amazon SQS)

Configuración del transporte asíncrono que lleva las solicitudes de recomendación entre el
productor y el worker. El código vive en `internal/queue`; la composición, en `cmd`.

Este documento **no contiene secretos**: ninguna URL real de cola, identificador de cuenta
ni credencial. Los valores concretos viven en el `.env` local, que no se versiona, y en las
variables del entorno desplegado.

## Estado actual

Implementado en LAB-31: configuración validada, puerto `queue.Client`, implementación sobre
SQS, implementación deshabilitada y doble en memoria para tests.

Todavía **no** implementado: quién publica (LAB-32), quién consume y completa batches
(LAB-33) y los disparadores del dominio (LAB-34). Hoy el cliente se construye en el arranque
y no tiene consumidor.

## Variables de entorno

| Variable | Obligatoria | Default | Rango |
| --- | --- | --- | --- |
| `RECOMMENDATIONS_QUEUE_ENABLED` | no | `false` | booleano que entienda Go (`true`, `false`, `1`, `0`) |
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
- **`AWS_REGION` se comparte con S3.** `internal/uploader` la trata como opcional y cae a
  `us-east-2`; acá es obligatoria con la cola habilitada. La divergencia es deliberada: un
  bucket en la región equivocada da error visible, pero una cola en la región equivocada
  hace desaparecer mensajes.
- **`AWS_SQS_MAX_RETRY_ATTEMPTS` son los reintentos del SDK** ante fallos de red o
  throttling. No tiene relación con `maxReceiveCount`, que es un atributo de la cola en AWS.
- **No hay credenciales nuevas.** El SDK resuelve las mismas que ya usa S3 por la cadena
  estándar (`AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`, perfil, o rol de la instancia).

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

## Referencias

- Épica LAB-17 y ticket LAB-31.
- Cambio OpenSpec `configure-recommendation-queue`, capacidad
  `recommendation-queue-configuration`.
