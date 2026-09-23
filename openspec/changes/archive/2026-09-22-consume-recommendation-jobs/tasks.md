## 1. Comprobación previa de disponibilidad del scoring

- [x] 1.1 Agregar a `internal/scoring/scorer.go` la interfaz opcional `Availability`
  (`Available(ctx) error`), documentando por qué es opcional y por qué su fallo es el mismo
  `ErrScoringUnavailable` que el de la evaluación; verificar con `go build ./...`.
- [x] 1.2 Implementar `Available` en `scoring.Unavailable` devolviendo `ErrScoringUnavailable`,
  con la aserción de compilación `var _ Availability = Unavailable{}`; verificar con un test
  que `errors.Is` lo reconoce y que la comprobación no invoca `Score` ni `ScoreAll`.
- [x] 1.3 Cubrir con un test que un `Scorer` que no implementa `Availability` sigue cumpliendo
  el contrato: la aserción de tipo falla y no hay obligación de escribir el método.
- [x] 1.4 Ampliar `internal/scoring/doc.go` para nombrar la comprobación previa como cuarto
  desenlace distinguible de los tres de `Result`.

## 2. Reclamo condicional y lectura del batch

- [x] 2.1 Agregar a `sql/queries/recommendations.sql` la consulta `GetRecommendationBatch`
  por identificador, con el comentario que explique que el worker necesita distinguir batch
  inexistente de batch terminal; regenerar con `sqlc generate` y verificar con `go build ./...`.
- [x] 2.2 Agregar a `sql/queries/recommendations.sql` la consulta `ClaimRecommendationBatch`
  con `WHERE id = $1 AND status IN ('pending', 'processing')`, documentando por qué el
  predicado incluye `processing` —un procesamiento interrumpido no debe bloquear al sujeto—
  y por qué no puede usarse `TransitionRecommendationBatch`; regenerar con `sqlc generate`.
- [x] 2.3 Agregar `ErrBatchNotClaimable` a `internal/recommendation/errors.go`, distinto de
  `ErrBatchNotFound`, documentando que uno significa "ya cerrado" y el otro "no existe".
- [x] 2.4 Agregar `GetBatch(ctx, batchID) (Batch, error)` y
  `ClaimBatch(ctx, batchID) (Batch, error)` a la interfaz `RecommendationStore` en
  `internal/recommendation/store.go` y a `RecommendationRepository` en `repo.go`; verificar
  con `go build ./...`.
- [x] 2.5 Cubrir con tests contra la base real de `internal/testsupport` que el reclamo
  prospera desde `pending` y desde `processing`, y que no prospera desde `completed` ni desde
  `failed`, devolviendo `ErrBatchNotClaimable`.
- [x] 2.6 Cubrir con un test contra la base real que un reclamo fallido sobre un batch
  `completed` deja intacto su conjunto de recomendaciones, que es la garantía que impide que
  un redelivery destruya el conjunto vigente.
- [x] 2.7 Cubrir con un test que `GetBatch` sobre un identificador inexistente devuelve
  `ErrBatchNotFound` y no un batch cero.

## 3. Resolución de candidatos activos

- [x] 3.1 Agregar a `sql/queries/recommendations.sql` la consulta que resuelve el perfil de
  scoring de un empleado —experiencia, disponibilidad, timezone, recursos técnicos y niveles
  educativos— con `LEFT JOIN` sobre `employee_profile_availability`, `employee_location` y
  `employee_profile_tech`, de modo que un paso sin completar produzca `NULL` y no ausencia de
  fila; regenerar con `sqlc generate`.
- [x] 3.2 Agregar la consulta que lista todos los perfiles de scoring de empleados, con la
  misma forma que 3.1, para el sujeto puesto.
- [x] 3.3 Agregar las consultas que resuelven un puesto vigente por identificador y que listan
  todos los puestos vigentes, ambas con `deleted_at IS NULL`, con el comentario que las ate al
  criterio "los puestos eliminados nunca son recomendados"; regenerar con `sqlc generate`.
- [x] 3.4 Crear `internal/recommendation/candidates.go` con el puerto `CandidateSource`
  (`PairsForEmployee`, `PairsForJobPosition`), documentando por qué devuelve `scoring.Pair` ya
  armados y no modelos de dominio, y por qué el paquete puede importar `internal/scoring` pero
  sigue sin importar `internal/jobposition` ni `internal/employee`; verificar con
  `go list -deps ./internal/recommendation | grep -E 'internal/(jobposition|employee)'` sin
  coincidencias.
- [x] 3.5 Implementar `CandidateSource` en `RecommendationRepository`, traduciendo las filas a
  `scoring.EmployeeProfile` y `scoring.JobRequirements` con punteros y slices que conserven la
  ausencia; resolver el nivel educativo más alto en Go con `scoring.EducationLevel.Rank()`.
- [x] 3.6 Implementar la señal de sujeto inexistente: `ErrSubjectNotFound` cuando el empleado
  no existe o el puesto no existe o está eliminado lógicamente, distinguible de un universo
  vacío legítimo.
- [x] 3.7 Cubrir con tests contra la base real, en ambos sentidos: universo completo, universo
  vacío, puesto eliminado excluido como candidato, puesto eliminado como sujeto,
  empleado con perfil incompleto presente en el universo, y `nil` frente a slice vacío en
  recursos técnicos.
- [x] 3.8 Cubrir con un test que los pares devueltos superan `Pair.Validate()`, de modo que
  la traducción no produzca entradas que el contrato de scoring rechace.

## 4. Worker: ciclo de vida y configuración

- [x] 4.1 Agregar a `internal/queue/config.go` la constante y la carga de
  `RECOMMENDATIONS_WORKER_ENABLED` como flag independiente de `RECOMMENDATIONS_QUEUE_ENABLED`,
  documentando por qué son dos decisiones de despliegue distintas; verificar con tests de
  configuración que cubren las cuatro combinaciones.
  Resuelto con `WorkerConfig`/`LoadWorkerConfig` aparte de `Config`/`LoadConfig`, y no como un
  campo más: meterlo en `LoadConfig` rompía la garantía de LAB-31 de que con la cola apagada no
  se lee ninguna otra variable. La decisión de arranque vive en `WorkerConfig.ShouldConsume`,
  que es lo que cubre las cuatro combinaciones sin montar la aplicación.
- [x] 4.2 Crear `internal/recommendation/worker.go` con el struct del worker, sus dependencias
  —`RecommendationStore`, `CandidateSource`, `queue.Client`, `scoring.Scorer`— y su
  constructor; verificar con `go build ./...`.
- [x] 4.3 Implementar el ciclo `Run(ctx)`: recibir lotes, procesar mensaje a mensaje y volver a
  recibir, terminando cuando el contexto se cancela; verificar con un test que usa
  `queuetest.Fake` y comprueba que `Run` retorna al cancelar y que no inicia procesamiento
  nuevo después.
- [x] 4.4 Implementar el tratamiento del error de recepción: registrar y reintentar sin
  terminar el ciclo, para que un fallo transitorio de red no apague el worker; verificar con un
  test que fuerza `FailReceive` y luego lo restaura.
- [x] 4.5 Implementar el procesamiento del mensaje en curso con un contexto desprendido de la
  cancelación, por la misma razón que `QueueJobPublisher.failBatch`; verificar con un test que
  cancela durante el procesamiento y comprueba que el batch igual queda en estado terminal.
- [x] 4.6 Implementar la extensión periódica de visibilidad mientras un mensaje se procesa, y
  verificar con un test que el fake recibió al menos una extensión en un procesamiento que
  supera el plazo configurado.
- [x] 4.7 Cablear el worker en `cmd/api.go` y `cmd/main.go` detrás del flag, inyectando
  `scoring.Unavailable{}`; implementar que con el transporte deshabilitado el ciclo no arranque
  y quede constancia del motivo; verificar con un test de arranque para las combinaciones de
  flags.

## 5. Worker: interpretación del mensaje y reclamo idempotente

- [x] 5.1 Implementar la deserialización del cuerpo a `JobRequestedEvent` y el rechazo de un
  cuerpo ilegible como fallo terminal: sin tocar ningún batch, con reconocimiento del mensaje y
  una línea de diagnóstico que no incluya el cuerpo; verificar con un test.
- [x] 5.2 Implementar el rechazo de una versión distinta de `JobRequestedVersion`, también
  terminal; verificar con un test que envía un cuerpo con versión superior y comprueba que el
  mensaje se reconoce y ningún batch cambia.
- [x] 5.3 Implementar la resolución del batch con `GetBatch`: inexistente o terminal
  —`completed`, `failed`— reconoce el mensaje y termina sin trabajo; verificar con un test por
  cada uno de los tres casos.
- [x] 5.4 Cubrir con un test de idempotencia extremo a extremo: procesar dos veces el mismo
  mensaje mediante el redelivery de `queuetest.Fake` deja al sujeto con un único conjunto
  vigente coherente y sin recomendaciones duplicadas.
- [x] 5.5 Cubrir con un test que un batch que quedó en `processing` por un procesamiento
  interrumpido se reclama de nuevo en la entrega siguiente y no deja al sujeto bloqueado.

## 6. Worker: generación y reemplazo del conjunto

- [x] 6.1 Implementar el orden del procesamiento definido en `design.md`: resolver candidatos
  con el batch todavía en `pending`, y solo después decidir entre completar vacío, fallar por
  scoring no disponible o reclamar y puntuar; verificar con un test que observa el estado del
  batch en cada punto.
- [x] 6.2 Implementar el camino de universo vacío: completar el batch con lista vacía sin
  consultar el scoring; verificar con un test por tipo de sujeto que usa un scorer doble que
  falla la prueba si se lo invoca.
- [x] 6.3 Implementar el camino de scoring no disponible con candidatos: transicionar de
  `pending` a `failed` sin pasar por `processing` y reconocer el mensaje; verificar con un test
  que afirma que `processing` nunca se persistió y que no hay ninguna recomendación.
- [x] 6.4 Implementar el camino completo: reclamar, invocar `ScoreAll`, descartar los resultados
  inelegibles, traducir los elegibles a `Candidate` conservando `Total` nil como ausencia de
  puntaje, y completar con `CompleteBatch`; verificar con un test que afirma el conjunto
  resultante.
- [x] 6.5 Cubrir con un test que los resultados inelegibles no se persisten y que los elegibles
  sin puntaje sí, con puntaje nulo.
- [x] 6.6 Cubrir con un test contra la base real que el conjunto vigente anterior sobrevive a un
  batch posterior fallido, y otro que un batch completado lo reemplaza entero sin dejar
  recomendaciones del anterior.
- [x] 6.7 Cubrir con un test que un puesto eliminado lógicamente entre la emisión y el consumo
  no aparece en el conjunto generado para ningún empleado.
- [x] 6.8 Implementar el camino de sujeto inexistente o eliminado: completar el batch con lista
  vacía y reconocer; verificar con un test por cada caso.

## 7. Worker: clasificación de errores y reconocimiento

- [x] 7.1 Implementar la clasificación recuperable/terminal según la tabla de `design.md`, con
  una función propia y testeable que mapee el error a la decisión de reconocimiento; verificar
  con un test tabular que cubre cada fila de esa tabla.
- [x] 7.2 Implementar el no reconocimiento ante fallo de persistencia: el mensaje queda sin
  borrar; verificar con un test que fuerza el fallo del store y comprueba que
  `queuetest.Fake` sigue teniendo el mensaje pendiente y lo vuelve a entregar tras avanzar el
  reloj.
- [x] 7.3 Cubrir con un test que un fallo de `Delete` sobre un batch ya persistido no altera el
  estado persistido y que el redelivery se resuelve sin rehacer el trabajo.
- [x] 7.4 Cubrir con un test que un mensaje envenenado —cuerpo ilegible— no impide que los
  mensajes siguientes del mismo lote se procesen.
- [x] 7.5 Cubrir con un test que el agotamiento de reintentos es observable: tras N entregas sin
  reconocimiento el conteo de recepciones crece, que es lo que la cola usa para derivar a la
  DLQ; dejar explícito en el comentario que el `maxReceiveCount` es atributo de AWS y no de la
  aplicación.

## 8. Desacople y documentación

- [x] 8.1 Cubrir con un test que el worker no conoce el algoritmo: sustituir el scorer por un
  doble que puntúa cambia el conjunto persistido y no requiere ningún cambio en el worker.
- [x] 8.2 Verificar que el worker no menciona ningún tipo del SDK de SQS, con
  `grep -r "aws-sdk-go-v2" internal/recommendation` sin coincidencias.
- [x] 8.3 Ampliar `internal/recommendation/doc.go` para declarar el alcance del consumidor, el
  orden de procesamiento y por qué el reclamo es condicional.
- [x] 8.4 Actualizar `docs/recommendation-queue.md`: mover el consumo de "no implementado" a
  "implementado en LAB-33", documentar `RECOMMENDATIONS_WORKER_ENABLED`, la tabla de
  clasificación de errores y que todo batch con candidatos termina en `failed` hasta que exista
  el algoritmo; verificar leyendo el documento que ninguna URL de cola ni identificador de
  cuenta aparece.
- [x] 8.5 Actualizar `.env.example` con el flag del worker, sin ningún valor real.

## 9. Cierre

- [x] 9.1 Verificar el cambio completo con `gofmt -l .` sin salida, `go vet ./...` sin hallazgos
  y `go test ./...` en verde.
- [x] 9.2 Comprobar explícitamente cada criterio de aceptación del ticket contra un test o un
  comando concreto, y dejar constancia del mapeo en el commit de implementación:
  idempotencia ante redelivery y duplicados, errores recuperables que vuelven a la cola y
  agotados que terminan en DLQ, resultado vacío que completa el batch con lista vacía, fallos
  que conservan el conjunto vigente anterior, puestos eliminados nunca recomendados, y batch
  fallido explícito sin scoring productivo.
