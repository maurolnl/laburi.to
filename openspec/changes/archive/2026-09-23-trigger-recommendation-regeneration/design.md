## Context

Ver `proposal.md` — Why. El estado del que parte este cambio:

- `internal/recommendation` ya expone el puerto `JobPublisher` y su implementación
  `QueueJobPublisher`, que abre el batch `pending` y publica el evento versionado. Toda la
  semántica de deduplicación y de fallo de publicación vive ahí y no se toca.
- `internal/jobposition` ya define `JobPositionEventPublisher` y lo notifica tras el alta y la
  edición mediante un helper `publish` que registra el error y no lo propaga. Su único cableado
  hoy es `NoopEventPublisher{}`.
- `internal/employee` no tiene ningún puerto de eventos: sus diez escrituras de perfil llaman al
  repositorio y devuelven.
- `internal/recommendation` tiene prohibido importar `internal/employee` e `internal/jobposition`:
  `candidates.go` documenta esa dirección porque invertirla crearía un ciclo cuando los paquetes de
  dominio consuman recomendaciones.
- `RECOMMENDATIONS_QUEUE_ENABLED` está apagado por defecto y en ese caso `cmd` inyecta
  `queue.Disabled`, que falla con `ErrQueueDisabled` en vez de simular éxito.
- El perfil de empleado se persiste en cinco tablas separadas (`employees`, `employee_location`,
  `employee_profile_tech`, `employee_profile_availability`, `employee_education`) y ninguna columna
  registra si el perfil está completo.

## Goals / Non-Goals

**Goals:**

- Conectar los bordes de escritura de `employee` y `jobposition` con `recommendation.JobPublisher`
  sin que ninguno de los tres paquetes importe a otro.
- Resolver la completitud del perfil del empleado con una única consulta, sin recorrer el agregado
  completo ni duplicar la regla en cada servicio.
- Dejar la estrategia ante fallo de publicación escrita y verificable, en lugar de implícita.

**Non-Goals:**

- Reintentar automáticamente una emisión fallida. Un batch `failed` es el registro durable de que
  el sujeto quedó sin regenerar; barrerlos es trabajo posterior.
- Cambiar la deduplicación de `QueueJobPublisher`, el contrato del mensaje o el worker.
- Tocar el contrato HTTP: ninguna ruta, cuerpo ni código de estado cambia.

## Decisions

### Puertos reducidos al identificador del sujeto, sin capa adaptadora

`jobposition.JobPositionEventPublisher` pasa de `JobPositionPublished(ctx, position JobPosition)` a
`JobPositionPublished(ctx, jobPositionID int32)`, y `internal/employee` estrena
`EmployeeEventPublisher` con `EmployeeProfileCompleted(ctx, employeeID int32)`. Con ambas firmas
reducidas a tipos primitivos, un único tipo de `internal/recommendation` —`Trigger`, que envuelve un
`JobPublisher`— satisface los dos puertos por tipado estructural: `recommendation` no importa
`employee` ni `jobposition`, y ninguno de los dos importa `recommendation`.

Alternativas descartadas:

- **Conservar `JobPosition` completo y adaptar en `cmd/api.go`.** Obliga a un adaptador por sujeto
  en el nivel donde no hay tests de paquete, justo el código que el ticket pide cubrir con productor
  falso.
- **Adaptadores en un paquete nuevo `internal/recommendationtrigger`.** Testeable, pero agrega una
  capa para resolver un problema que desaparece angostando dos firmas, contra la preferencia de
  `CLAUDE.md` por cambios locales.
- **Que `recommendation` importe los paquetes de dominio.** Invierte la dirección que
  `candidates.go` fija a propósito.

Reducir la carga útil además es coherente con el mensaje: `JobRequestedEvent` transporta solo
identificadores, así que pasar el puesto entero nunca tuvo consumidor posible.

### Completitud resuelta por una consulta `EXISTS`, no por el agregado

`EmployeeStore` gana `IsProfileComplete(ctx, employeeID int32) (bool, error)`, respaldada por una
consulta sqlc nueva sobre `employees` que combina cinco `EXISTS` —locación, recursos técnicos,
disponibilidad y al menos un título de educación, más la existencia del propio empleado—. No se
reutiliza `GetEmployee` porque toma el `userID`, agrega tres subconsultas JSON y devuelve el perfil
entero para responder una pregunta booleana.

La regla es «la fila de la sección existe», no «la sección tiene datos»: el paso de recursos
técnicos admite `os` y `paid_software` vacíos por validación, así que exigir contenido dejaría a
esos perfiles fuera para siempre. Es la misma lectura que `PairsForEmployee` ya hace con
`has_tech_profile`.

Sin columna derivada ni migración: la completitud es una función del estado y materializarla
agregaría un dato que puede desincronizarse.

### Emisión después de persistir, en el servicio y no en el handler

Cada método de escritura de `employeeService` —creación y edición del registro base, y creación y
edición de locación, recursos, disponibilidad y educación— consulta la completitud después de que
el repositorio confirma, y solo entonces notifica al puerto. `jobposition` ya tiene esa forma; el
helper `publish` de `employee` la replica: registra el error y no lo propaga, porque el cambio ya
está persistido y propagarlo haría que el cliente reintente una escritura que sí ocurrió.

La emisión es sincrónica dentro del request. Publicar es un `INSERT` más un envío a la cola: no
evalúa indicadores, no espera al worker y no justifica una goroutine sin ciclo de vida, que
`CLAUDE.md` desaconseja.

La creación del perfil base también consulta la completitud aunque nunca pueda estar completa en ese
momento. Uniformar evita que la regla viva en nueve lugares con una excepción en el décimo.

### Cableado condicionado al interruptor de la cola

`cmd/api.go` construye `recommendation.NewTrigger(...)` sobre `QueueJobPublisher` cuando
`queueCfg.Enabled`, y sobre `NoopJobPublisher{}` cuando no. Sin esa condición, un entorno con la
cola apagada —el valor por defecto— abriría y marcaría `failed` un batch por cada alta y cada
edición, convirtiendo un apagado deliberado en un rastro de fallos. Es el mismo criterio con el que
`startWorker` decide si consume.

### Fallo de publicación: batch `failed` como registro durable, sin outbox

El orden es persistir dominio → responder al cliente → emitir. Si la emisión falla,
`QueueJobPublisher` ya deja el batch en `failed` y devuelve el error; el borde de escritura lo
registra y no revierte nada. El batch `failed` es la representación durable exigida: queda en la
base, es consultable y —por el índice único parcial, que solo cubre `pending` y `processing`— no
bloquea al sujeto, así que la escritura siguiente vuelve a intentar.

Alternativa descartada: **outbox transaccional**. Garantiza no perder eventos, pero exige migración
nueva, un segundo ciclo de publicación con reintentos y su propio backoff. Es más máquina de la que
esta épica necesita mientras el algoritmo de scoring no exista y todo batch con candidatos termine
igual en `failed`.

### Orden entre escrituras sucesivas

No se agrega ningún mecanismo: el índice único parcial de la migración 0007 ya impide dos batches en
curso para el mismo sujeto, así que las ejecuciones de un sujeto están serializadas y ninguna puede
reemplazar el conjunto dejado por otra posterior. El requisito correspondiente documenta esa
garantía en vez de duplicarla.

## Risks / Trade-offs

- **Una edición que llega mientras el batch está `processing` se deduplica y sus datos pueden no
  quedar reflejados** → El worker lee el estado desde la base al reclamar el batch, así que una
  edición anterior al reclamo sí entra. La ventana real es la edición que llega con el trabajo ya
  reclamado. Mitigación: la escritura siguiente del sujeto abre un batch nuevo. Cerrar la ventana
  exigiría marcar el sujeto como sucio y republicar al cerrar el batch, que cambia la
  deduplicación definida en `recommendation-job-publishing` y pertenece a otro ticket.
- **Una consulta adicional por escritura de perfil** → Es un `SELECT` de `EXISTS` sobre claves
  foráneas indexadas, en endpoints que ya hacen una transacción multi-tabla. El costo es marginal
  frente a la escritura que lo precede.
- **Angostar `JobPositionEventPublisher` es un cambio incompatible del puerto** → Es interno: sus
  únicas implementaciones son `NoopEventPublisher` y el doble de los tests, y ningún consumidor
  externo lo observa. El requisito modificado de `job-position-api` deja constancia.
- **Con la cola apagada nada se emite y ningún batch lo registra** → Es deliberado y ya visible en
  el log de arranque, que nombra el estado del transporte. Registrar cada omisión en la base
  llenaría la tabla de fallos que nadie va a atender.
- **La emisión sincrónica suma la latencia del envío a la cola al request** → Acotada por el timeout
  del cliente SQS y pagada solo por escrituras, no por lecturas. Moverla fuera del request exigiría
  un ciclo de vida propio que este cambio no justifica.

## Migration Plan

Sin migración de base de datos ni cambios de configuración. El despliegue con
`RECOMMENDATIONS_QUEUE_ENABLED` apagado se comporta exactamente como hoy; encenderlo activa los
disparadores. Revertir es volver a la versión anterior: no queda ningún dato con formato nuevo.
