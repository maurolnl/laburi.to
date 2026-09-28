## 1. Esquema y consultas

- [ ] 1.1 Crear `sql/schema/0008_employee_file_certification_name.sql` (goose Up/Down, `ADD COLUMN IF NOT EXISTS certification_name TEXT`, índice único parcial `IF NOT EXISTS`); verificar corriendo `./out migrate` dos veces contra una base local
- [ ] 1.2 Actualizar `sql/queries/employees.sql`: insertar archivo con `certification_name`, marcar `deleted` los no conservados, liberar/reasignar nombres, leer archivos del empleado por id, y agregar a `GetEmployee` y `GetEmployeeProfileByID` `certifications` (`name`, `document_id`) y `files` sin asociar; regenerar con `sqlc generate` y verificar `go build ./...`

## 2. Parseo y validación

- [ ] 2.1 Implementar el parseo de `certifications` (JSON de ítems, recorte, unicidad sin mayúsculas, `document` xor `document_id`, claves únicas, archivos no referenciados, formato viejo) con `files.GetPDF` por clave; verificar con tests de tabla que reemplacen `certifications_parsing_test.go`
- [ ] 2.2 Cambiar `CreateEmployee`/`UpdateEmployee` para usar el parseo, responder `400 {"error"}` y `RespondWithValidatorError` en el DTO; verificar con tests de handler (no PDF, > 5 MB, archivo sin certificación, nombre vacío, repetido)

## 3. Servicio y repositorio

- [ ] 3.1 `POST`: subir N PDFs y crear empleado + archivos en una transacción, con limpieza compensatoria; verificar con test de servicio con fakes
- [ ] 3.2 `PUT`: subir nuevos, transacción (validar `document_id` del empleado, actualizar base, `deleted` a los no conservados, reasignar nombres, insertar nuevos), y borrar de S3 los dados de baja después del commit; verificar con tests de servicio (reemplazo, conservar, quitar, eliminar certificación, legacy intacto, `document_id` ajeno → 400)

## 4. Lectura del perfil

- [ ] 4.1 Actualizar modelos y respuestas de `GET /users/{userID}/employee` y `GET /employees/{employeeID}` (`certifications: [{name, document_id}]`, `files` sin asociar, nunca `null`); verificar con tests de respuesta y de `profile_queries_test.go`
- [ ] 4.2 Verificar que la descarga de un certificado asociado funciona para dueño y empleador con vínculo, y que uno `deleted` responde `404` (`profile_access_test.go`)

## 5. Validación

- [ ] 5.1 Ejecutar `gofmt -l .`, `go vet ./...` y `go test ./...` en verde
- [ ] 5.2 Comprobar explícitamente los criterios de aceptación 1 a 8 de LAB-40 contra una base local con el perfil de ejemplo (2 certificaciones + 1 PDF viejo)
