## Why

La migración `0006_job_positions.sql` y sus queries sqlc ya existen, pero ningún
empleador puede crear, consultar, editar ni eliminar un puesto: `internal/jobposition`
contiene solo `doc.go` y la prueba de contrato de la migración. Sin este contrato HTTP
el frontend de la épica (LAB-26 y LAB-27) no tiene con qué trabajar, y la épica de
recomendaciones no tiene dónde engancharse cuando un puesto se publica o se edita.

## What Changes

- Contrato HTTP nuevo, todo autenticado y restringido al rol `employer`:
  - `POST /employers/{employerID}/jobs` → `201` con el puesto creado.
  - `GET /employers/{employerID}/jobs` → `200` con la lista de puestos activos.
  - `GET /jobs/{jobPositionID}` → `200` con el puesto.
  - `PUT /jobs/{jobPositionID}` → `200` con el puesto actualizado.
  - `DELETE /jobs/{jobPositionID}` → `204` sin cuerpo.
- Ownership derivado siempre del JWT: el `employerID` efectivo se resuelve desde
  `userID` del principal vía `employers.user_id`. El identificador que llega por path
  se usa para detectar un intento de acceso ajeno, nunca como fuente de identidad.
- Validación de DTO en el borde: `position`, `role`, `required_experience`,
  `required_education_level`, `available_hours_per_day` y `timezone` obligatorios;
  `technical_resources` opcional y siempre normalizado a lista. Los enums replican los
  dominios ya vigentes en el perfil de empleado y en los `CHECK` de la tabla.
- `timezone` se valida contra `pg_timezone_names`, siguiendo el precedente de
  `employee_location`.
- Publicación inmediata: crear un puesto lo deja publicado. No hay borrador, ni
  transición de estado, ni reapertura.
- Eliminar es soft delete; el puesto eliminado desaparece de listados y consultas
  individuales, que responden `404`.
- Costura de recomendaciones sin acoplamiento a SQS: el servicio depende de un puerto
  `JobPositionEventPublisher` invocado tras crear y tras editar. Este cambio cablea una
  implementación no-op; la épica de recomendaciones sustituye solo esa implementación.
- **Desviación deliberada del precedente**: los errores de validación de DTO de estos
  endpoints responden JSON `{"error":"..."}` en vez del texto plano que emite
  `internal.PrintValidatorError`. El `MutationCache.onError` del frontend solo entiende
  `{error}` y `{messages}`; el texto plano degrada a un mensaje genérico. Se agrega un
  helper de respuesta JSON para errores de validación, usado por ahora solo aquí.
- `docs/create-job-position.md` (carpeta suelta de la raíz, sin Git propio) queda
  alineado: `POST api/employer/jobs` pasa a `POST /employers/{employerID}/jobs`.

Sin cambios BREAKING: todas las rutas son nuevas y ningún contrato vigente se modifica.

## Capabilities

### New Capabilities

- `job-position-api`: contrato HTTP protegido para crear, listar, obtener, editar y
  eliminar lógicamente puestos de trabajo; autorización por rol `employer`, ownership
  derivado del JWT, dominio de los campos del puesto, exclusión inmediata de los
  puestos eliminados y notificación del puerto de recomendaciones.

### Modified Capabilities

Ninguna. `job-position-persistence` describe la integridad de la tabla y sus queries y
no cambia; `employer-profile-api`, `role-aware-authentication`,
`user-role-employer-persistence` y `employee-profile-onboarding` se mantienen intactos.

## Impact

- `internal/jobposition/`: paquete completado con `models.go`, `errors.go`, `store.go`,
  `service.go`, `repo.go`, `handler.go`, `events.go` y sus tests.
- `internal/` (helpers compartidos): helper nuevo de respuesta JSON para errores de
  validación. No se modifica `PrintValidatorError`, que sigue sirviendo a los endpoints
  existentes.
- `cmd/api.go`: composición del repositorio, el publicador no-op y el registro de rutas.
- `sql/`: sin cambios. El esquema y las queries tipadas llegaron con LAB-24.
- Frontend: no se toca en este cambio, pero LAB-26 y LAB-27 consumen exactamente este
  contrato.
- `../docs/create-job-position.md`: actualización documental fuera de este repositorio.
