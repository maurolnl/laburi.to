## Why

El paso base del perfil de empleado guarda `certifications` como lista de nombres y acepta un
único PDF (`certifications_file`), que queda en `employee_files` sin vínculo con ningún nombre.
Con dos o más certificaciones no se puede saber a cuál corresponde el archivo, ni subir uno
por cada una, y el empleador no puede verificar una certificación puntual (LAB-40; en prod,
`emp01.completo@laburito.test`, employee 4, tiene dos certificaciones y un PDF).

## What Changes

- Migración `0008`: `employee_files.certification_name TEXT NULL` e índice único parcial por
  (`employee_id`, `certification_name`) entre los archivos activos. Los registros existentes
  quedan con `NULL` (certificado sin asociar). Idempotente (`IF NOT EXISTS`) y aplicada por
  `./out migrate` en el pre-deploy.
- **BREAKING** `POST /employees` y `PUT /employees/{employeeID}` reciben `certifications` como
  un único campo JSON `[{ "name", "document"?, "document_id"? }]` y N PDFs, uno por clave
  referenciada en `document`. Se eliminan `certifications[]` y `certifications_file`.
- Validaciones con `400 {"error": "..."}`: solo PDF, ≤ 5 MB por archivo, nombre no vacío y
  único por perfil, ningún archivo sin certificación que lo referencie, clave referenciada
  inexistente, `document` y `document_id` a la vez, `document_id` ajeno o inexistente, JSON
  inválido o formato viejo. La validación del DTO pasa de texto plano
  (`PrintValidatorError`) a `RespondWithValidatorError`.
- `PUT` reemplaza el conjunto: cada PDF asociado que deja de estar referenciado (reemplazado,
  quitado o de una certificación eliminada) pasa a `status = 'deleted'` en la transacción y su
  objeto se borra de S3 después del commit.
- **BREAKING** `GET /employees/{employeeID}` y `GET /users/{userID}/employee` devuelven
  `certifications: [{ "name", "document_id" }]` (`null` sin PDF) y `files: [{ "id", "title" }]`
  solo con los certificados subidos sin asociar.
- `employees.certifications TEXT[]` sigue siendo la fuente de los nombres: el scoring y la
  consulta de recomendaciones no cambian.

## Capabilities

### New Capabilities
<!-- ninguna -->

### Modified Capabilities
- `employee-profile-onboarding`: el certificado único opcional pasa a un PDF opcional por
  certificación, con contrato multipart, validaciones, reemplazo y baja de archivos.
- `employee-profile-access`: el perfil identifica el documento de cada certificación y separa
  los certificados sin asociar.

## Impact

- Código: `internal/employee/` (`crud_employee.go`, `models.go`, `repo.go`, `get_employee.go`,
  `get_employee_profile.go`, `config.go`, `errors.go`), `sql/queries/employees.sql` + sqlc
  regenerado, `sql/schema/0008_employee_file_certification_name.sql`.
- API: cambio de contrato coordinado con el cambio homónimo de `bolsa-de-trabajo`; se
  despliegan juntos. Descarga sin cambios
  (`GET /employees/{employeeID}/files/{fileID}/download-url`).
- Dependencia: se basa en el trabajo de `./out migrate` (`cmd/migrate.go`, `sql/schema/embed.go`,
  `preDeployCommand` en `railway.json`) de la rama `fix/availability-zero-and-deploy-migrations`.
- S3: borrado de objetos reemplazados (`uploader.Delete`, ya existente).
