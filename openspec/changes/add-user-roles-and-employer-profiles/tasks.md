## 1. Migración de roles y empleadores

- [ ] 1.1 Crear una migración nueva que agregue `users.role`, complete usuarios existentes con `employer`, restrinja los valores permitidos y bloquee actualizaciones posteriores; verificar en PostgreSQL descartable el backfill, `NOT NULL`, el `CHECK` y el rechazo de cambios.
- [ ] 1.2 Incorporar `employers` con relación única y cascade a `users`, campos obligatorios, `hiring_modalities TEXT[]` abierta y timestamps; verificar mediante operaciones SQL que funcionen la creación válida, la unicidad, la FK, el borrado en cascada y modalidades arbitrarias.
- [ ] 1.3 Agregar triggers simétricos con advisory lock transaccional para impedir perfiles employee/employer simultáneos; ejecutar de forma repetible en PostgreSQL descartable verificaciones de ambos órdenes de inserción y de dos creaciones concurrentes para el mismo usuario, dejando su automatización permanente para LAB-23.
- [ ] 1.4 Completar el bloque `Down` eliminando triggers, funciones, tabla, restricción y columna en orden seguro; verificar un ciclo completo de migración `Up` y `Down` sin modificar migraciones anteriores.

## 2. Consultas y generación sqlc

- [ ] 2.1 Crear `sql/queries/employers.sql` con `CreateEmployer :one` y `GetEmployerByUserID :one`; verificar que ambas consultas devuelvan todos los campos definidos por la migración.
- [ ] 2.2 Ejecutar `sqlc generate`, revisar los cambios de `User` y el nuevo modelo/queries de empleador, y verificar que solo se modifiquen archivos generados esperados sin edición manual.

## 3. Validación final

- [ ] 3.1 Ejecutar `gofmt` sobre cualquier archivo Go afectado y `go test ./...`; verificar que la nueva salida sqlc compile con el backend existente.
- [ ] 3.2 Ejecutar `go vet ./...` y `openspec validate add-user-roles-and-employer-profiles --strict`; documentar cualquier fallo preexistente separado del cambio.
- [ ] 3.3 Revisar los criterios de LAB-18 y confirmar que la rama no introduce cambios HTTP/frontend ni un default permanente de rol; documentar como bloqueo de despliegue que la migración no puede aplicarse mientras una instancia sin el alta compatible de LAB-19 atienda registros.
