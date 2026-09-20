## 1. Migración de esquema

- [ ] 1.1 Crear `sql/schema/0007_recommendations.sql` con la tabla
      `recommendation_batches`: `id SERIAL PRIMARY KEY`, `subject_type TEXT NOT NULL`,
      `employee_id INTEGER REFERENCES employees(id) ON DELETE CASCADE`,
      `job_position_id INTEGER REFERENCES job_positions(id) ON DELETE CASCADE`,
      `status TEXT NOT NULL DEFAULT 'pending'`, `created_at` y `updated_at`
      `TIMESTAMPTZ NOT NULL DEFAULT now()`. Verificar leyendo el archivo que ninguna de
      las dos claves foráneas es `NOT NULL`.
- [ ] 1.2 Agregar el check `recommendation_batches_status_check` con exactamente
      `pending`, `processing`, `completed` y `failed`, y el check
      `recommendation_batches_subject_check` que exige que `subject_type = 'employee'`
      implique `employee_id IS NOT NULL AND job_position_id IS NULL` y que
      `subject_type = 'job_position'` implique lo inverso. Verificar con un `INSERT`
      manual en la base efímera que los cuatro casos inválidos (dos sujetos, ningún
      sujeto, sujeto cruzado con el tipo, estado desconocido) son rechazados.
- [ ] 1.3 Agregar los índices únicos parciales
      `recommendation_batches_employee_in_flight_key` sobre `employee_id` y
      `recommendation_batches_job_position_in_flight_key` sobre `job_position_id`, ambos
      con `WHERE <columna> IS NOT NULL AND status IN ('pending', 'processing')`.
      Verificar que ninguno omite la cláusula parcial y que ambos son `UNIQUE`.
- [ ] 1.4 Agregar el índice `recommendation_batches_employee_recent_idx` sobre
      `(employee_id, created_at DESC)` y su equivalente sobre `job_position_id`, que
      resuelven la búsqueda del batch más reciente y del último `completed` por sujeto.
- [ ] 1.5 Agregar en la misma migración la tabla `recommendations`:
      `id SERIAL PRIMARY KEY`,
      `batch_id INTEGER NOT NULL REFERENCES recommendation_batches(id) ON DELETE CASCADE`,
      `employee_id INTEGER NOT NULL REFERENCES employees(id) ON DELETE CASCADE`,
      `job_position_id INTEGER NOT NULL REFERENCES job_positions(id) ON DELETE CASCADE`,
      `score NUMERIC(6,4)` nullable y `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`.
      Verificar que `score` no tiene `NOT NULL` ni `DEFAULT`.
- [ ] 1.6 Agregar la constraint `recommendations_unique_pair_per_batch`
      `UNIQUE (batch_id, employee_id, job_position_id)` y los índices
      `recommendations_batch_score_idx` sobre `(batch_id, score DESC)`,
      `recommendations_employee_id_idx` y `recommendations_job_position_id_idx`.
- [ ] 1.7 Escribir la sección `-- +goose Down` que elimina índices y tablas en orden
      inverso, y verificar que no toca objetos de migraciones anteriores.
- [ ] 1.8 Aplicar la migración y su rollback contra la base efímera y verificar que
      `up` seguido de `down` deja el esquema exactamente como estaba.

## 2. Infraestructura de test con PostgreSQL efímero

- [ ] 2.1 Agregar `github.com/testcontainers/testcontainers-go` y su módulo de
      PostgreSQL a `go.mod`, ejecutar `go mod tidy` y verificar que ninguna de las dos
      dependencias aparece importada fuera de archivos `_test.go` o de
      `internal/testsupport`.
- [ ] 2.2 Crear `internal/testsupport/postgres.go` con un helper que levante el
      contenedor una única vez por ejecución, aplique en orden todos los archivos de
      `sql/schema` recortando su sección `-- +goose Up`, y devuelva un `*sql.DB`.
      Verificar que un segundo llamado reutiliza la instancia en lugar de crear otra.
- [ ] 2.3 Implementar la degradación: si el entorno de contenedores no está disponible,
      el helper hace `t.Skip` con un motivo explícito. Verificar ejecutando
      `go test ./...` con el entorno de contenedores detenido y comprobando que la suite
      completa termina sin fallos.
- [ ] 2.4 Implementar el aislamiento entre tests: transacción con rollback por defecto y
      un modo alternativo que permita commits reales limpiando las tablas al terminar.
      Verificar con dos tests que escriben las mismas tablas y no se observan entre sí.
- [ ] 2.5 Verificar que el helper ignora cualquier `DB_URL` del entorno y que nunca
      aplica migraciones sobre una base preexistente, leyendo el código y comprobando
      que no consulta esa variable.

## 3. Acceso tipado

- [ ] 3.1 Crear `sql/queries/recommendations.sql` con `CreateRecommendationBatch :one`,
      `TransitionRecommendationBatch :one`, `GetCurrentBatchByEmployee :one`,
      `GetCurrentBatchByJobPosition :one`, `GetLastCompletedBatchByEmployee :one`,
      `GetLastCompletedBatchByJobPosition :one`, `InsertRecommendation :one`,
      `DeleteOtherBatchesForEmployee :exec`, `DeleteOtherBatchesForJobPosition :exec`,
      `ListJobRecommendationsForEmployee :many` y
      `ListEmployeeRecommendationsForJobPosition :many`.
- [ ] 3.2 Implementar las dos queries de listado con `JOIN job_positions` y
      `job_positions.deleted_at IS NULL`, `ORDER BY score DESC NULLS LAST` más el
      desempate correspondiente (`job_positions.created_at DESC` en un sentido,
      `employees.updated_at DESC` en el otro) y `LIMIT`/`OFFSET`. Verificar leyendo cada
      query que ninguna omite el filtro de soft delete ni `NULLS LAST`.
- [ ] 3.3 Implementar `GetCurrentBatch*` como el batch más reciente por `created_at`
      cualquiera sea su estado, y `GetLastCompletedBatch*` restringido a
      `status = 'completed'`. Verificar que son dos queries distintas y que ninguna
      asume que coinciden.
- [ ] 3.4 Ejecutar `sqlc generate` y verificar que `internal/database/` compila con
      `go build ./...`, que `EmployeeID` y `JobPositionID` del batch se generan como
      nullable y que `Score` refleja la nullability de la columna.

## 4. Repositorio de recomendaciones

- [ ] 4.1 Crear `internal/recommendation/models.go` con los tipos de dominio del batch,
      su estado y la recomendación, siguiendo el estilo de `internal/jobposition`.
      Usar `employee` y `employer` como términos; no introducir `employeer`.
- [ ] 4.2 Crear `internal/recommendation/store.go` con la interfaz de consumidor y
      `internal/recommendation/repo.go` con la implementación sobre
      `database.Queries`, clasificando los errores de `pq` a errores de dominio como
      hace `internal/employer/repo.go` (violación del índice único en curso, sujeto
      inexistente, batch inexistente).
- [ ] 4.3 Implementar el reemplazo atómico en una única operación transaccional con
      `BeginTx`, `defer tx.Rollback()` y `database.New(db).WithTx(tx)`: transiciona el
      batch a `completed`, inserta el conjunto y borra los demás batches del sujeto.
      Verificar leyendo el código que los tres pasos comparten la misma transacción.
- [ ] 4.4 Implementar la resolución del estado vigente y del conjunto vigente como una
      operación que combina `GetCurrentBatch*` y `GetLastCompletedBatch*`, de modo que el
      llamador no tenga que conocer la regla. Verificar que devuelve estado y conjunto
      por separado.
- [ ] 4.5 Crear `internal/recommendation/factories_test.go` con el patrón de opciones de
      `internal/employer/factories_test.go`, cubriendo batches por empleado y por
      puesto, y recomendaciones con y sin puntaje.

## 5. Tests de esquema y transacción

- [ ] 5.1 Test de sujeto excluyente: verificar contra la base efímera que se aceptan los
      batches por empleado y por puesto, y que se rechazan los cuatro casos inválidos de
      la tarea 1.2.
- [ ] 5.2 Test de ciclo de estados: verificar que los cuatro estados se aceptan, que un
      estado desconocido se rechaza, que la creación sin estado deja `pending` y que una
      transición refresca `updated_at`.
- [ ] 5.3 Test de unicidad del batch en curso: verificar que un segundo batch `pending`
      para el mismo sujeto es rechazado, que se acepta cuando el anterior está
      `completed` o `failed`, y que dos sujetos distintos pueden tener batches en curso
      simultáneos.
- [ ] 5.4 Test de la relación many-to-many: varios empleados para un puesto, varios
      puestos para un empleado, dupla repetida rechazada dentro del mismo batch, misma
      dupla aceptada en batches distintos, y cascada al borrar el batch.
- [ ] 5.5 Test de puntaje: ausencia de puntaje, puntaje persistido sin pérdida de
      precisión, y puntaje cero distinguible de puntaje ausente al leerlo.
- [ ] 5.6 Test de reemplazo atómico con commit real: verificar que al completar un batch
      el conjunto vigente pasa a ser el nuevo y los batches anteriores del sujeto
      desaparecen junto con sus recomendaciones.
- [ ] 5.7 Test de fallo en el reemplazo: forzar un error en medio de la transacción
      (por ejemplo una recomendación hacia un empleado inexistente) y verificar que el
      batch conserva su estado previo, que no quedó ninguna recomendación nueva y que el
      conjunto vigente anterior sigue completo. Este es el test que cubre
      "reprocesar es idempotente y no deja conjuntos parciales".
- [ ] 5.8 Test de retención: tras varias ejecuciones sucesivas, verificar que el sujeto
      conserva a lo sumo el último `completed` y el último no completado.
- [ ] 5.9 Test de estado vigente contra conjunto vigente: cubrir los cinco escenarios del
      spec, en particular que un batch `failed` posterior a un `completed` expone estado
      `failed` y conserva el conjunto anterior.
- [ ] 5.10 Test de resultado vacío: un batch `completed` sin recomendaciones se
      distingue de uno `failed` por su estado y no por la cantidad de filas.
- [ ] 5.11 Test de exclusión de puestos eliminados: eliminar lógicamente un puesto ya
      recomendado y verificar que desaparece de la lectura por empleado, que la lectura
      por ese puesto no devuelve nada, y que los puestos activos del mismo conjunto
      siguen apareciendo.
- [ ] 5.12 Test de orden y paginación en ambos sentidos: orden por puntaje descendente,
      desempate por publicación del puesto, desempate por actualización del perfil,
      `NULLS LAST` y tramos con límite y desplazamiento.

## 6. Validación final

- [ ] 6.1 Ejecutar `gofmt -l .` y verificar que no lista archivos.
- [ ] 6.2 Ejecutar `go vet ./...` y verificar que pasa.
- [ ] 6.3 Ejecutar `go test ./...` con entorno de contenedores disponible y verificar que
      toda la suite pasa, incluidos los tests de integración.
- [ ] 6.4 Ejecutar `go test ./...` con el entorno de contenedores detenido y verificar
      que los tests de integración se omiten con motivo y la suite no falla.
- [ ] 6.5 Comprobar explícitamente cada criterio de aceptación de LAB-29 contra los tests
      ejecutados: un job admite múltiples empleados y un employee múltiples jobs (5.4);
      solo el batch más reciente se expone, sin historial consultable (5.8, 5.9);
      reprocesar es idempotente y no deja conjuntos parciales (5.3, 5.7); empty result y
      failed se distinguen (5.10); migraciones, queries sqlc y tests de transacción
      incluidos (1.x, 3.x, 5.6 y 5.7).
- [ ] 6.6 Dejar asentado en el PR que el número de migración `0007` queda reservado y
      que el repositorio no tiene CI, de modo que los tests de integración dependen hoy
      de que se ejecuten localmente.
