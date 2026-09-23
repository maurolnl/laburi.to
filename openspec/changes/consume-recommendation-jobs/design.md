## Context

Las cuatro piezas que este worker necesita ya existen y ninguna hay que rediseñarla:

- `queue.Client` (LAB-31) expone `Receive`, `Delete` y `ExtendVisibility` con tipos propios,
  y declara la garantía de entrega al menos una vez. `queuetest.Fake` reproduce la
  invisibilidad temporal, el redelivery y el conteo creciente de recepciones con un reloj
  inyectable.
- `recommendation.JobRequestedEvent` (LAB-32) es el contrato del mensaje: `event_id`,
  `version`, `subject_type`, `subject_id`, `batch_id` y `emitted_at`.
- `RecommendationStore` (LAB-29) ofrece `CreateBatch`, `TransitionBatch` y `CompleteBatch`,
  esta última ya atómica: completa el batch, inserta el conjunto y poda los batches
  anteriores del sujeto en una única transacción.
- `scoring.Scorer` y `scoring.HardFilter` (LAB-30) son los puertos del cálculo, y
  `scoring.Unavailable` es la única implementación de producción: falla con
  `ErrScoringUnavailable` para todo lote no vacío y devuelve un conjunto vacío sin error
  para el lote vacío.

Lo que falta es exactamente lo que este cambio agrega: quién recibe el mensaje, cómo llega
de un `subject_id` al universo de pares a evaluar, y en qué orden se tocan batch, cola y
conjunto vigente para que ningún fallo deje estado inconsistente.

Dos restricciones del código existente condicionan el diseño:

- `internal/recommendation` no importa `internal/jobposition` ni `internal/employee`, y no
  debe hacerlo: invertir esa dirección crearía un ciclo. Lee de la base por
  `internal/database`, como ya hace `repo.go`.
- El índice único parcial `recommendation_batches_*_in_flight_key` admite un solo batch
  `pending` o `processing` por sujeto. Un batch que queda atascado en cualquiera de esos dos
  estados bloquea toda solicitud futura de ese sujeto. Ningún camino de fallo puede dejarlo
  así.

## Goals / Non-Goals

**Goals**

- Procesar un mensaje de punta a punta: interpretarlo, reclamar su batch, resolver los
  candidatos activos, puntuarlos y reemplazar el conjunto vigente en una transacción.
- Ser idempotente frente a redelivery y mensajes duplicados, sin depender de una tabla de
  deduplicación adicional.
- No reconocer ningún mensaje antes de que el batch haya quedado persistido en un estado
  terminal.
- Conservar el conjunto vigente anterior ante cualquier fallo.
- Nunca recomendar un puesto eliminado lógicamente.
- Fallar explícitamente, y de forma distinguible, cuando no hay scoring productivo.

**Non-Goals**

- Definir indicadores, pesos o filtros duros. La épica lo prohíbe y `scoring.Unavailable`
  sigue siendo la implementación inyectada.
- Disparar la regeneración desde el alta o la edición de un perfil o un puesto (LAB-34).
- Exponer los resultados por HTTP (LAB-35).
- Aprovisionar la cola, su DLQ o su `maxReceiveCount` en AWS. Son atributos de la cola, no
  de la aplicación, y `docs/recommendation-queue.md` ya los documenta.
- Procesar sujetos en paralelo dentro de un mismo lote. Un lote se procesa mensaje a
  mensaje; el paralelismo es una optimización sin demanda todavía.

## Decisions

### El worker es una goroutine del mismo binario, detrás de un flag propio

El ticket pide un "worker separado", y separado admite dos lecturas: proceso aparte o
unidad de ejecución aparte. Se elige la segunda.

Un binario nuevo en `cmd/worker` obligaría a un segundo servicio en Railway, a un
`Makefile` con dos targets y a duplicar la carga de configuración y la apertura de la base.
Eso es trabajo de infraestructura que este ticket no pide y que no aporta nada mientras el
scoring no exista y todo batch termine en `failed`.

La goroutine da la separación que importa ahora: el ciclo de consumo no comparte el camino
de request de la API, no puede bloquear un handler, y su ausencia es total con el flag
apagado. La lógica vive en `internal/recommendation` y no en `cmd`, así que extraerla a un
binario propio más adelante es mover un `main`, no rediseñar.

`RECOMMENDATIONS_WORKER_ENABLED` es un flag propio y no reutiliza
`RECOMMENDATIONS_QUEUE_ENABLED`: publicar y consumir son decisiones de despliegue distintas.
Una instancia de API puede querer emitir sin consumir. Con el transporte deshabilitado el
worker no arranca aunque su flag esté encendido, y se registra la razón: consumir de
`queue.Disabled` sería un bucle de `ErrQueueDisabled`.

### El batch se reclama con una transición condicional, no con `TransitionBatch`

`TransitionBatch` es incondicional: mueve el batch al estado pedido sea cual sea el estado
actual. Para el productor alcanza, porque solo cierra un batch que acaba de abrir. Para el
worker no: un redelivery de un mensaje cuyo batch ya está `completed` lo devolvería a
`processing`, y el `CompleteBatch` siguiente borraría y reinsertaría un conjunto vigente que
ya era correcto. Peor: si ese segundo intento falla, el sujeto queda con el batch en
`processing`, bloqueado por el índice único parcial.

Se agrega `ClaimBatch`, una transición condicional que solo prospera si el batch está en
`pending` o `processing`:

```sql
UPDATE recommendation_batches
SET status = 'processing', updated_at = now()
WHERE id = $1 AND status IN ('pending', 'processing')
RETURNING ...
```

Cero filas significa una de dos cosas, y ninguna requiere trabajo: el batch ya es terminal, o
no existe. `GetBatch` distingue los dos casos para el diagnóstico y para decidir el
reconocimiento; en ambos el mensaje se reconoce.

El predicado incluye `processing` y no solo `pending` a propósito. Excluirlo haría el reclamo
estrictamente exclusivo, pero convertiría cualquier caída del worker a mitad de un batch en
un sujeto permanentemente bloqueado: el redelivery no podría reclamarlo y el batch quedaría
`processing` para siempre. Incluirlo acepta que, si el visibility timeout vence mientras un
worker sigue trabajando, dos entregas puedan solaparse. Ese solapamiento es inocuo porque
`CompleteBatch` es un reemplazo atómico completo: el segundo en commitear gana y el conjunto
resultante es coherente en cualquier orden. Para que ocurra lo menos posible, el worker
extiende la visibilidad del mensaje mientras procesa.

### El universo de candidatos se resuelve por un puerto propio, no importando los paquetes de dominio

`internal/recommendation` no puede importar `internal/jobposition` ni `internal/employee`.
Se define un puerto en el propio paquete:

```go
type CandidateSource interface {
    PairsForEmployee(ctx context.Context, employeeID int32) ([]scoring.Pair, error)
    PairsForJobPosition(ctx context.Context, jobPositionID int32) ([]scoring.Pair, error)
}
```

Devuelve `scoring.Pair` ya armados y no modelos de dominio: la traducción de las filas a la
entrada normalizada del scoring es exactamente el trabajo que `scoring/doc.go` asigna a quien
lee de la base. Que el puerto devuelva el tipo del contrato deja al worker sin ninguna
decisión de mapeo.

Lo implementa `RecommendationRepository` con consultas sqlc nuevas sobre tablas existentes.
Importar `internal/scoring` desde `internal/recommendation` es una dirección nueva pero no un
ciclo: `scoring` no importa ningún paquete de dominio.

Ambas direcciones filtran `job_positions.deleted_at IS NULL`, tanto para el candidato como
para el sujeto. Un puesto eliminado no se recomienda a nadie y no recibe candidatos.

El sujeto inexistente —empleado borrado, puesto eliminado lógicamente— no es un fallo de
infraestructura: es un batch que ya no tiene para qué existir. Se completa con lista vacía,
que lo cierra sin bloquear al sujeto y sin dejar recomendaciones de un puesto que no está.

### El perfil del empleado se traduce respetando la ausencia

`scoring.EmployeeProfile` distingue ausencia de valor cero con punteros, porque el perfil se
completa en cinco pasos. La traducción lo respeta:

- `Experience` sale de `employees.years_of_experience`, siempre presente.
- `AvailableHoursPerDay` y `Timezone` son `nil` cuando el empleado no completó los pasos de
  disponibilidad y ubicación: `LEFT JOIN`, no `JOIN`.
- `TechnicalResources` sale de `employee_profile_tech.paid_software`. `nil` cuando no hay
  fila y slice vacío cuando la hay sin software, que es la distinción que el tipo pide.
- `HighestEducation` es el nivel de mayor rango entre las filas de `employee_education`, o
  `nil` si no hay ninguna. El orden se resuelve en Go con `scoring.EducationLevel.Rank()` y
  no con un `CASE` en SQL: el orden ya es parte del contrato de scoring, y duplicarlo en una
  consulta invitaría a que las dos copias se desincronicen.

Un empleado incompleto igual es candidato. Decidir si su perfil alcanza es un filtro duro, y
los filtros duros no están definidos.

### La disponibilidad del scoring se comprueba antes de abrir `processing`

`scoring.Unavailable` solo se delata al ser invocado, y para invocarlo habría que haber
reclamado el batch. El sujeto quedaría un instante en `processing` para un trabajo que no
puede prosperar, y cada fallo de infraestructura en ese tramo lo dejaría bloqueado.

Se agrega al contrato una comprobación opcional:

```go
// Availability la implementa el Scorer que puede declarar por adelantado que no va a
// poder evaluar.
type Availability interface {
    Available(ctx context.Context) error
}
```

`scoring.Unavailable` la implementa devolviendo `ErrScoringUnavailable`. Es opcional —el
worker la detecta con una aserción de tipo— para que una implementación futura que siempre
puede evaluar no tenga que escribir un método que devuelve `nil`.

El orden del worker se desprende de eso:

1. Interpretar el mensaje y validar su versión.
2. Leer el batch. Terminal o inexistente: reconocer y terminar.
3. Resolver los candidatos, con el batch todavía en `pending`. Es una lectura y no cambia
   nada observable.
4. Sin candidatos: reclamar, completar con lista vacía, reconocer. No se consulta el scoring,
   igual que `ScoreAll` con lote vacío no necesita dependencia.
5. Con candidatos y scoring no disponible: `pending` → `failed`, reconocer. `processing` no
   se abre nunca.
6. Con candidatos y scoring disponible: reclamar (`processing`), puntuar, completar,
   reconocer.

Resolver candidatos antes de reclamar deja al sujeto visible como `pending` durante la
lectura en vez de `processing`. Es el precio de no abrir `processing` para un batch
condenado, y es el menor de los dos: `pending` ya significa "solicitado y todavía no
resuelto", que es exactamente lo que está pasando.

### Solo se persiste el resultado puntuado

`scoring.Result` tiene tres desenlaces. Al conjunto vigente van únicamente los pares
elegibles:

- `Eligible: false` no se persiste. Un filtro duro lo descartó; no es una recomendación.
- `Eligible: true` se persiste, con `Total` como `Candidate.Score`. `nil` viaja como `NULL`,
  que es lo que la columna nullable de LAB-29 existe para representar, y el orden de lectura
  ya lo manda al final con `NULLS LAST`.

El worker no ordena ni pagina: `ListJobRecommendationsForEmployee` y su inversa ya resuelven
el orden y la paginación en la lectura. Ordenar también en la escritura sería una segunda
copia de la regla, y las dos copias divergirían.

### Recuperable frente a terminal, y qué significa reconocer

El reconocimiento —`Delete`— es la única señal de que el mensaje no vuelve. Se emite después
de persistir el desenlace del batch, nunca antes, y nunca sin haberlo persistido.

| Situación | Batch | Mensaje |
|---|---|---|
| Cuerpo ilegible o versión desconocida | no se toca (no se sabe cuál es) | reconocer |
| Batch inexistente o terminal | no se toca | reconocer |
| Sujeto inexistente o puesto eliminado | `completed` vacío | reconocer |
| Sin candidatos | `completed` vacío | reconocer |
| Scoring no disponible | `pending` → `failed` | reconocer |
| Par inválido (`ErrInvalidPair`) | `failed` | reconocer |
| Fallo de scoring distinto de no disponible | `failed` | reconocer |
| Fallo de base de datos | como haya quedado | **no** reconocer |
| Fallo de `Delete` | ya persistido | redelivery lo reconoce |

La línea divisoria es si reintentar puede cambiar el desenlace. Un cuerpo ilegible se lee
igual de mal la décima vez; devolverlo a la cola solo consume entregas hasta la DLQ y demora
los mensajes sanos que vienen detrás. Un fallo de base de datos sí puede resolverse solo, así
que el mensaje vuelve, y si nunca se resuelve el `maxReceiveCount` de la cola lo manda a la
DLQ, que es exactamente para lo que la DLQ está configurada desde LAB-31.

Scoring no disponible se reconoce y no se reintenta, aunque a primera vista parezca
recuperable. Mientras el algoritmo no exista, reintentar lleva todo el tráfico a la DLQ y
deja al sujeto esperando; con el batch en `failed` el sujeto queda libre para una solicitud
nueva, y cuando el algoritmo llegue, el disparador de LAB-34 va a emitir una. El rastro del
motivo queda en el batch `failed`, que es donde se puede consultar, y no en una DLQ.

### El worker se apaga por contexto y termina el mensaje en curso

`Receive` con long polling bloquea hasta veinte segundos. El apagado cancela el contexto del
ciclo, así que la recepción en curso corta y no se inicia otra. El mensaje que ya está siendo
procesado se termina con un contexto desprendido de la cancelación, por la misma razón por la
que `QueueJobPublisher.failBatch` usa `context.WithoutCancel`: abandonar a mitad de camino es
justo lo que deja un batch `processing` bloqueando al sujeto.

## Risks / Trade-offs

- **Pares en memoria sin cota.** Un empleado se empareja contra todos los puestos activos.
  Con el volumen actual no es un problema, y acotarlo bien exige los filtros duros que la
  épica prohíbe definir. Se documenta como deuda conocida en vez de inventar un límite
  arbitrario que después habría que justificar.
- **Dos entregas solapadas del mismo batch.** Aceptado a cambio de no bloquear al sujeto ante
  una caída del worker. Es inocuo porque `CompleteBatch` reemplaza el conjunto entero; la
  extensión de visibilidad reduce la ventana.
- **Todo batch con candidatos termina en `failed` hasta que exista el algoritmo.** Es el
  comportamiento que el ticket pide y no un defecto. Los tests lo fijan como tal, de modo que
  el día que llegue el algoritmo el cambio sea visible en la suite.
- **El worker corre en el proceso de la API.** Comparte límites de memoria y de conexiones a
  la base. Con el flag apagado por defecto y sin scoring productivo, la carga real es nula;
  si deja de serlo, la lógica ya está fuera de `cmd` y extraerla es mecánico.

## Migration Plan

Ninguna migración de base de datos: el esquema de LAB-29 alcanza y las consultas nuevas son
sobre tablas existentes.

El despliegue es reversible con una variable: `RECOMMENDATIONS_WORKER_ENABLED` apagado deja
la aplicación idéntica a la actual. Encendido sin `RECOMMENDATIONS_QUEUE_ENABLED` tampoco
arranca el ciclo y lo registra.

Los batches `pending` que hayan quedado de pruebas previas de LAB-32 sin consumidor no
requieren limpieza manual: su mensaje sigue en la cola mientras no venza la retención y el
worker los va a resolver; los que hayan perdido su mensaje quedan `pending` y se pueden
cerrar con una transición manual si estorban, pero eso es dato de entorno de desarrollo y no
parte del cambio.
