## 1. Migración de esquema

- [x] 1.1 Crear `sql/schema/0006_job_positions.sql` con la tabla `job_positions`:
      `id SERIAL PRIMARY KEY`, `employer_id INTEGER NOT NULL REFERENCES employers(id)
      ON DELETE CASCADE`, `position TEXT NOT NULL`, `role TEXT NOT NULL`,
      `required_experience TEXT NOT NULL`, `required_education_level TEXT NOT NULL`,
      `available_hours_per_day SMALLINT NOT NULL`, `timezone TEXT NOT NULL`,
      `technical_resources TEXT[] NOT NULL DEFAULT '{}'`, `created_at`, `updated_at` y
      `deleted_at TIMESTAMPTZ`. Verificar leyendo el archivo: no existe columna de
      estado ni de publicación.
- [x] 1.2 Agregar los checks nombrados `job_positions_required_experience_check`
      (`less_1y`, `1y`, `2_to_5y`, `5_to_10y`, `more_10y`),
      `job_positions_required_education_level_check` (`university`, `postgraduate`,
      `high-school-orientation`, `tertiary`) y
      `job_positions_available_hours_per_day_check` (`BETWEEN 1 AND 8`). Verificar que
      los valores coinciden carácter por carácter con los de
      `0001_employees.sql`.
- [x] 1.3 Agregar los índices parciales `WHERE deleted_at IS NULL`:
      `job_positions_employer_id_idx` sobre `employer_id`, y los índices de búsqueda
      sobre `required_experience`, `required_education_level` y `timezone`. Verificar
      que ninguno omite la cláusula parcial.
- [x] 1.4 Escribir la sección `-- +goose Down` que elimina índices y tabla, y verificar
      que no toca objetos de migraciones anteriores.

## 2. Acceso tipado

- [x] 2.1 Crear `sql/queries/job_positions.sql` con `CreateJobPosition :one`,
      `GetActiveJobPositionByID :one`, `ListActiveJobPositionsByEmployer :many`,
      `UpdateActiveJobPosition :one` y `SoftDeleteJobPosition :one`. Verificar que
      toda query de lectura y actualización incluye `deleted_at IS NULL`.
- [x] 2.2 Implementar `SoftDeleteJobPosition` como
      `UPDATE ... SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL
      RETURNING ...`, y verificar que una segunda ejecución no devuelve filas.
- [x] 2.3 Ejecutar `sqlc generate` y verificar que `internal/database/` compila con
      `go build ./...` y que los tipos generados reflejan la nullability de
      `deleted_at`.

## 3. Prueba de contrato

- [x] 3.1 Crear `internal/jobposition/migration_0006_contract_test.go` siguiendo el
      patrón de `internal/employer/migration_0005_contract_test.go`: leer el SQL,
      recortar la sección `-- +goose Up` y verificar por regex la clave foránea con
      `ON DELETE CASCADE`, los tres checks nombrados, el default de
      `technical_resources` y la nullability de `deleted_at`. Verificar con
      `go test ./internal/jobposition`.
- [x] 3.2 Agregar al mismo test la verificación de que los cuatro índices existen y
      todos incluyen `WHERE deleted_at IS NULL`, y de que el SQL no contiene ninguna
      columna llamada `status`, `published` ni `state`.
- [x] 3.3 Agregar la verificación de que los dominios de experiencia y nivel educativo
      declarados en `0006` coinciden con los de `0001_employees.sql`, leyendo ambos
      archivos, para que una divergencia futura falle en CI.

## 4. Validación final

- [x] 4.1 Ejecutar `gofmt -l .` y verificar que no lista archivos.
- [x] 4.2 Ejecutar `go vet ./...` y `go test ./...` y verificar que ambos pasan.
- [x] 4.3 Comprobar explícitamente cada criterio de aceptación del ticket LAB-24 contra
      los tests ejecutados, y dejar asentado que "los puestos eliminados quedan fuera
      de recomendaciones" solo se cumple en su parte verificable: ninguna query activa
      los devuelve, porque el módulo de recomendaciones todavía no existe.
