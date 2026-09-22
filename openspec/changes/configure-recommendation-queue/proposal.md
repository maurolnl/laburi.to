## Why

La épica LAB-17 procesa las recomendaciones de forma asíncrona sobre Amazon SQS, pero
hoy el backend no tiene ninguna noción de cola: `jobposition.NoopEventPublisher` descarta
cada evento y `cmd/api.go` no conoce ningún transporte. El productor (LAB-32) y el worker
(LAB-33) no pueden escribirse mientras no exista un cliente configurado, validado y
sustituible.

Además, este repositorio no tiene hoy ningún mecanismo de configuración: `main.go` lee
variables sueltas con `os.Getenv` sin validarlas, y `internal/uploader` carga la
configuración de AWS dentro del paquete y aborta con `log.Fatal` desde ahí. Un valor
faltante se manifiesta recién en la primera llamada a AWS, en producción. Una cola mal
configurada es peor que un bucket mal configurado: los mensajes se pierden sin respuesta
de error que el usuario pueda ver.

## What Changes

- Nuevo paquete `internal/queue`: configuración, construcción del cliente SQS y el puerto
  que LAB-32 y LAB-33 consumirán. No publica mensajes, no consume mensajes, no toca la
  base de datos y no expone rutas HTTP.
- **Configuración explícita y validada** en `queue.Config`, resuelta desde variables de
  entorno mediante una función de lookup inyectable (no `os.Getenv` directo), de modo que
  la validación sea testeable sin tocar el entorno del proceso:
  - `RECOMMENDATIONS_QUEUE_ENABLED` decide si la cola participa del arranque;
  - con la cola habilitada: `AWS_REGION`, `AWS_SQS_RECOMMENDATIONS_QUEUE_URL` y
    `AWS_SQS_RECOMMENDATIONS_DLQ_URL` son obligatorias;
  - con valores por defecto y rango validado: visibility timeout, tiempo de long polling,
    cantidad máxima de mensajes por recepción y máximo de reintentos del SDK;
  - `AWS_SQS_ENDPOINT_URL` opcional para apuntar a un SQS local (LocalStack/ElasticMQ).
- **Fail-fast con mensajes seguros**: toda variable obligatoria faltante, vacía o fuera de
  rango aborta el arranque con un error sentinela que nombra la variable y **nunca**
  incluye su valor, credenciales AWS ni la URL completa de la cola.
- **Cola deshabilitada es un estado legítimo**: sin `RECOMMENDATIONS_QUEUE_ENABLED` la
  aplicación arranca igual y el resto del contrato HTTP no cambia. Hoy no existe ningún
  productor ni worker, así que obligar a toda persona que corra `go run ./cmd` a tener una
  cola aprovisionada sería un costo sin beneficio.
- **Puerto `queue.Client`** con las cuatro operaciones que el productor y el worker
  necesitan —enviar, recibir, borrar y extender la visibilidad de un mensaje— expresadas
  con tipos propios del paquete. El puerto no filtra tipos del AWS SDK hacia sus
  consumidores: sustituir SQS por otro transporte no obliga a tocar LAB-32 ni LAB-33.
- **Implementación real** sobre `aws-sdk-go-v2/service/sqs`, construida con
  `config.LoadDefaultConfig` y la cadena de credenciales estándar. Devuelve error en vez de
  abortar: quien decide terminar el proceso es `cmd`, no el paquete.
- **Implementación `Disabled`**: cumple `queue.Client` y falla siempre con
  `ErrQueueDisabled`. Es el equivalente de `scoring.Unavailable` y evita que un consumidor
  reciba `nil` cuando la cola no está habilitada.
- **Doble para tests en `internal/queue/queuetest`**: un fake en memoria con las mismas
  garantías que SQS —entrega al menos una vez, mensajes invisibles hasta su visibility
  timeout, redelivery y conteo de recepciones— importable desde los tests de LAB-32 y
  LAB-33. Es un paquete aparte, no un `_test.go`, precisamente porque los consumidores
  viven en otros paquetes; ninguna ruta de producción lo importa.
- **Composición en `cmd` sin globals**: `main.go` carga la configuración y falla temprano;
  `api.go` construye el cliente una vez y lo guarda en `application`. No hay variable de
  paquete, no hay `init()` y no hay `log.Fatal` dentro de `internal/queue`.
- **Documentación**: `.env.example` con las variables nuevas y sin valores, y una sección
  en `docs/` con el setup local y el desplegado, incluidos los atributos que la cola debe
  tener del lado de AWS —redrive policy hacia la DLQ, `maxReceiveCount`, visibility
  timeout y long polling— y por qué el procesamiento asume entrega al menos una vez.

Fuera de alcance, explícitamente: publicar el evento de `jobposition` (LAB-32), consumir
mensajes y completar batches (LAB-33), disparar la regeneración desde employee y puestos
(LAB-34), endpoints HTTP (LAB-35), aprovisionar la cola y la DLQ en AWS, y cualquier
observabilidad más allá de los logs mínimos descritos.

Sin cambios BREAKING: no se modifica ningún contrato HTTP, ninguna migración, ninguna
interfaz existente ni el comportamiento del binario cuando la cola está deshabilitada.

## Capabilities

### New Capabilities

- `recommendation-queue-configuration`: la configuración del transporte asíncrono de
  recomendaciones y el acceso a él — qué valores son obligatorios y cuándo, cómo se
  validan, cómo falla el arranque sin filtrar secretos, qué estado tiene el sistema con la
  cola deshabilitada, qué operaciones expone el puerto de cola, y qué garantías de entrega
  y de reintento asumen sus consumidores.

### Modified Capabilities

Ninguna. `recommendation-persistence` y `recommendation-scoring` no cambian ninguno de sus
requirements: este cambio no persiste nada ni puntúa nada. `job-position-api` tampoco:
`NoopEventPublisher` sigue siendo la implementación cableada hasta LAB-32.

## Impact

- Código nuevo: `internal/queue/` (configuración, errores, puerto, implementación SQS e
  implementación deshabilitada) y `internal/queue/queuetest/` (doble de test).
- Código modificado: `cmd/main.go` y `cmd/config.go` (carga y fail-fast de la
  configuración), `cmd/api.go` (construcción del cliente y su lugar en `application`).
- Dependencia nueva: `github.com/aws/aws-sdk-go-v2/service/sqs`. El SDK v2, su `config` y
  sus credenciales ya son dependencias directas por `internal/uploader`.
- Configuración: siete variables de entorno nuevas, todas documentadas en `.env.example`
  sin valores. Ningún secreto nuevo: las credenciales AWS son las que ya usa S3.
- Infraestructura: la cola principal y su DLQ **no** se crean en este cambio. El ticket
  deja documentados los atributos esperados para aprovisionarlas cuando el toolkit/MCP de
  AWS esté disponible.
- Tickets habilitados: LAB-32 puede implementar `JobPositionEventPublisher` contra
  `queue.Client`; LAB-33 puede escribir el worker y sus tests contra `queuetest` sin hablar
  con AWS.
- Divergencia deliberada con `internal/uploader`: allí `AWS_REGION` ausente cae a un
  default y el paquete aborta con `log.Fatal`. Aquí la región es obligatoria cuando la cola
  está habilitada y el paquete devuelve error. No se modifica `uploader` en este cambio;
  migrarlo es trabajo aparte.
