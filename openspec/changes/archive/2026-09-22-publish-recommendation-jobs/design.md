## Context

Ver `proposal.md` — Why. Lo que condiciona el diseño es lo que ya existe:

- **`queue.Client` recibe un cuerpo ya serializado**: `Send(ctx, body string)`. El
  transporte no interpreta el mensaje, así que el formato lo define este cambio
  (`internal/queue/doc.go`).
- **La base ya garantiza un único batch en curso por sujeto**: un índice único parcial
  sobre `pending`/`processing` hace que `RecommendationStore.CreateBatch` devuelva
  `ErrBatchAlreadyInFlight`. La deduplicación no depende de una comprobación previa del
  productor.
- **`RecommendationStore` ya tiene todo lo necesario**: `CreateBatch` abre el batch
  `pending` y `TransitionBatch` lo mueve a otro estado. Este cambio no amplía esa interfaz.
- **El puerto del consumidor ya existe del lado del dominio**:
  `jobposition.JobPositionEventPublisher`, cableado hoy con `NoopEventPublisher{}` en
  `cmd/api.go:75`. LAB-34 lo conecta; este cambio no lo toca.
- **`queuetest.Fake` existe justamente para esto**: es un paquete normal, importable desde
  otro paquete, que conserva los cuerpos enviados y permite forzar un error de envío.

## Goals / Non-Goals

**Goals:**

- Un puerto de emisión que `jobposition` y `employee` puedan implementar o consumir en
  LAB-34 sin conocer SQS.
- Un contrato de mensaje estable, versionado y verificable byte a byte en un test.
- Un orden de operaciones —batch primero, mensaje después— que no deje mensajes apuntando a
  batches inexistentes ni batches en curso sin mensaje.

**Non-Goals:**

- Cablear el productor en `cmd/api.go` ni sustituir `NoopEventPublisher`: es LAB-34.
- Definir cómo el worker consume el mensaje, transiciona a `processing` o completa el batch:
  es LAB-33. Este cambio solo se compromete a que el mensaje tenga lo que ese worker
  necesita.
- Cualquier reintento de publicación. Un fallo deja el batch en `failed` y termina ahí; el
  reintento es una decisión del llamador, que hoy no existe.

## Decisions

### El productor vive en `internal/recommendation`

Necesita las dos mitades a la vez: `RecommendationStore` para abrir el batch y
`queue.Client` para publicarlo. `internal/recommendation` ya define la primera, así que
ubicarlo ahí evita exportar nada nuevo y deja la dirección de importación en un solo
sentido: `recommendation → queue`.

Alternativas descartadas:

- **`internal/jobposition`**, que es donde LAB-31 lo anticipaba. No sirve: el productor
  atiende los dos sujetos, y un empleado no tiene por qué pasar por el paquete de puestos.
- **Un paquete nuevo `internal/recommendation/publisher`**. Obligaría a exportar detalles de
  la persistencia solo para que el productor los use, sin ganar nada: no hay un tercer
  consumidor que justifique la separación.

`internal/recommendation` MUST seguir sin importar `jobposition` ni `employee`. LAB-34
conecta los disparadores con adaptadores que viven del lado del dominio o en `cmd`, que es
lo que evita el ciclo latente que ya se identificó en LAB-30.

### El puerto recibe `Subject` y devuelve solo `error`

```go
type JobPublisher interface {
    PublishJob(ctx context.Context, subject Subject) error
}
```

`Subject` ya existe y sus constructores —`NewEmployeeSubject`, `NewJobPositionSubject`—
impiden armar una combinación inválida, así que el puerto no necesita dos operaciones ni un
parámetro de tipo suelto.

Devolver solo `error` y no el `Batch` es deliberado: el llamador es un borde de escritura al
que el batch no le sirve para nada, y devolverlo lo tentaría a esperar su resultado, que es
exactamente lo que el criterio de aceptación prohíbe.

### El mensaje lleva un único `subject_id`, no dos campos excluyentes

```go
const JobRequestedVersion = 1

type JobRequestedEvent struct {
    EventID     string      `json:"event_id"`
    Version     int         `json:"version"`
    SubjectType SubjectType `json:"subject_type"`
    SubjectID   int32       `json:"subject_id"`
    BatchID     int32       `json:"batch_id"`
    EmittedAt   time.Time   `json:"emitted_at"`
}
```

`Batch` tiene `EmployeeID` y `JobPositionID` excluyentes porque la base necesita dos claves
foráneas distintas. El mensaje no: `subject_type` ya desambigua, y repetir la exclusividad
en un lugar donde ninguna restricción la puede hacer cumplir solo agrega una combinación
inválida representable. El consumidor traduce el par `(subject_type, subject_id)` al
`Subject` del dominio con una función que ya existe.

`Version` es un entero monótono y no una cadena tipo semver: el consumidor compara contra el
único valor que entiende, y una comparación de cadenas invita a errores de orden.

`EmittedAt` se normaliza a UTC antes de serializar. `time.Time` se serializa como RFC 3339
con offset; forzar UTC hace que el cuerpo no dependa de la zona del proceso, que es lo que
permite comparar el mensaje byte a byte en un test.

### Reloj e identificador de evento inyectables

La implementación guarda dos funciones no exportadas, con `time.Now` y `uuid.NewString` por
defecto. Los tests del paquete las sustituyen por un reloj fijo y un contador, que es lo que
permite afirmar el cuerpo completo del mensaje en vez de comprobar campo por campo que "algo
hay". No se exponen como opciones públicas: nadie fuera del paquete necesita cambiarlas.

`uuid` ya está en `go.mod`; no se agrega ninguna dependencia.

### `ErrBatchAlreadyInFlight` se resuelve sin error y sin publicar

Es el caso normal de dos ediciones seguidas, no una anomalía. El batch en curso va a generar
el conjunto del sujeto, así que publicar un segundo mensaje solo agregaría trabajo duplicado
que el worker tendría que descartar.

Alternativas descartadas:

- **Propagar el error**: convierte una operación exitosa del usuario en una línea de error en
  cada alta y edición, y obliga a cada llamador a distinguir un centinela del paquete.
- **Republicar sobre el batch en curso**: duplica mensajes por diseño y contradice el
  criterio de deduplicación.

Se acepta un riesgo, anotado abajo: una edición hecha mientras el batch corre puede no
quedar reflejada hasta el siguiente disparo.

### El fallo de publicación deja el batch en `failed`

El batch ya está abierto cuando se publica, así que un fallo del transporte dejaría al sujeto
con un batch `pending` que nadie va a procesar y que, por el índice único parcial, bloquearía
toda solicitud futura de ese sujeto. Transicionar a `failed` cierra el ciclo con el estado
que la épica reserva para eso y deja al sujeto disponible para la próxima solicitud.

Alternativas descartadas:

- **Borrar el batch**: exige un `DeleteBatch` que hoy no existe ni en `RecommendationStore`
  ni en `sql/queries/recommendations.sql`, y ampliaría el contrato de persistencia de LAB-29
  para un caso de error.
- **Dejarlo `pending`**: estado zombi que solo se resuelve a mano.

El error que se devuelve envuelve el del transporte con `%w`, de modo que un llamador pueda
reconocer `queue.ErrQueueDisabled` si le importa. Si además falla la transición a `failed`,
se reporta la causa original —el fallo de publicación— y la segunda se registra en el log:
cambiar el error devuelto ocultaría por qué no hay mensaje.

### El transporte deshabilitado también es un fallo

`queue.Disabled` devuelve `ErrQueueDisabled` en `Send`, y el productor lo trata como
cualquier otro fallo: batch a `failed` y error propagado. Es coherente con `queue/doc.go`,
que ya define que reportar éxito sin haber enviado nada sería mentir. Tragarse ese error es
una decisión legítima del borde HTTP —`jobposition.publish` ya lo hace— pero pertenece a
LAB-34, no al productor.

### Implementación inerte del puerto

`NoopJobPublisher` no abre batch, no publica y devuelve `nil`. Existe para que ningún
llamador reciba `nil` ni tenga que comprobarlo, igual que `queue.Disabled` y
`scoring.Unavailable`. A diferencia de esas dos, devuelve `nil` y no error: la ausencia de
disparo es un estado de despliegue legítimo, y el criterio de "no mentir" aplica a la
implementación que sí dice haber publicado.

## Risks / Trade-offs

- **Una edición durante un batch en curso puede no reflejarse** → Se acepta. La alternativa
  —encolar un segundo mensaje— duplica trabajo por un caso de borde, y LAB-33 puede resolverlo
  mejor releyendo el estado al procesar, que es cuando el dato ya es el definitivo.
- **Entre abrir el batch y publicar hay una ventana sin transacción**: si el proceso muere
  justo ahí, el batch queda `pending` sin mensaje y bloquea al sujeto → No se mitiga en este
  cambio. Un outbox transaccional es la solución correcta y es desproporcionada para el
  volumen actual; la ventana es de milisegundos y solo la abre una caída del proceso, no un
  error del transporte, que sí está cubierto.
- **El formato del mensaje queda congelado en cuanto LAB-33 lo consuma** → Mitigado por el
  campo `Version`: un cambio incompatible sube la versión y el consumidor rechaza lo que no
  entiende en vez de malinterpretarlo.
- **El productor no tiene ningún llamador hasta LAB-34**, así que ninguna ruta de producción
  lo ejercita → Mitigado con tests que lo ejercitan contra `queuetest.Fake` y un store doble,
  incluidos el error de envío y el reintento posterior.

## Migration Plan

No aplica. No hay migración de base de datos, ninguna variable de entorno nueva y ningún
comportamiento observable cambia: sin llamadores, el código nuevo no se ejecuta en
producción. El despliegue es el binario nuevo y el rollback, el anterior.
