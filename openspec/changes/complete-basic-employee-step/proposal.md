## Why

El primer paso del perfil de empleado está implementado parcialmente, pero falta cerrar su contrato con cobertura verificable para archivos PDF, asociación segura con el usuario autenticado y actualizaciones por sección. Completarlo evita inconsistencias entre autenticación, persistencia multipart y el asistente por pasos.

## What Changes

- Consolidar la creación del empleado a partir del usuario autenticado, sin aceptar un `user_id` controlado por el cliente.
- Validar y persistir opcionalmente un PDF enviado como `certifications_file`, manteniendo la limpieza compensatoria si falla la persistencia.
- Garantizar que la consulta protegida del empleado devuelva el correo real mediante la relación con `users`.
- Mantener contratos de actualización separados para datos base, locación, recursos técnicos, disponibilidad y educación, con comprobación de propiedad.
- Auditar las reglas de validación de los DTO y retirar `omitempty` donde permita omitir una validación aplicable, conservándolo en campos realmente opcionales.
- Agregar pruebas enfocadas en multipart con y sin archivo, identidad/propiedad, correo relacionado y validación de los contratos por sección.

## Capabilities

### New Capabilities

- `employee-profile-onboarding`: Creación, consulta y actualización protegida del perfil de empleado por pasos, incluida la certificación PDF opcional.

### Modified Capabilities


## Impact

- Código backend en `internal/employee`, validación de archivos y consultas sqlc relacionadas.
- Contratos HTTP autenticados bajo `/employees` y `/users/{userID}/employee`.
- PostgreSQL y S3 participan en la creación con archivo; no se prevén migraciones de esquema.
- El frontend queda fuera de alcance: LAB-14 es responsable de alinear su payload al campo multipart singular `certifications_file`.
