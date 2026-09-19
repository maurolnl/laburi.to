## Why

La épica de gestión de puestos de trabajo no tiene base persistente: hoy existen
usuarios con rol, perfiles de empleado y perfiles de empleador, pero ninguna tabla
representa un puesto publicado. Sin ese esquema no puede construirse el CRUD con
ownership ni, más adelante, el cálculo de recomendaciones entre puesto y candidato.

## What Changes

- Nueva tabla `job_positions` vinculada a `employers` mediante clave foránea, con
  borrado en cascada del empleador.
- Campos obligatorios: `position`, `role`, `required_experience`,
  `required_education_level`, `available_hours_per_day` y `timezone`.
- Campo opcional `technical_resources` como `TEXT[] NOT NULL DEFAULT '{}'`.
- `required_experience` y `required_education_level` reutilizan exactamente los
  dominios ya vigentes en el perfil de empleado, de modo que puesto y candidato sean
  comparables sin tabla de equivalencias.
- `available_hours_per_day` es `SMALLINT` restringido al rango 1..8, la misma escala
  efectiva que rige hoy en `employee_profile_availability`.
- `timezone` es `TEXT NOT NULL` validado en la aplicación contra `pg_timezone_names`,
  siguiendo el precedente de `employee_location`.
- Publicación inmediata: no existe columna de estado, borrador ni reapertura. Una fila
  presente y no eliminada es un puesto publicado.
- Soft delete mediante `deleted_at TIMESTAMPTZ NULL`; las consultas activas excluyen
  las filas eliminadas.
- Queries sqlc tipadas para crear, obtener, listar por empleador, actualizar y
  eliminar lógicamente un puesto.
- Índices que soportan el listado de puestos activos por empleador y la futura
  búsqueda de candidatos por experiencia, educación y timezone.

Sin cambios BREAKING: la migración solo agrega objetos nuevos.

## Capabilities

### New Capabilities

- `job-position-persistence`: integridad persistente del puesto de trabajo, su
  relación con el empleador, el dominio de sus campos, la exclusión de puestos
  eliminados y el acceso tipado vía sqlc.

### Modified Capabilities

Ninguna. Los requisitos de `user-role-employer-persistence`,
`employer-profile-api`, `role-aware-authentication` y `employee-profile-onboarding`
se mantienen sin cambios.

## Impact

- `sql/schema/`: migración nueva `0006_job_positions.sql` con su sección `-- +goose Down`.
- `sql/queries/`: archivo nuevo `job_positions.sql`.
- `internal/database/`: código regenerado por `sqlc generate`.
- `internal/jobposition/`: paquete nuevo, limitado en este cambio a la prueba de
  contrato de la migración. Los handlers, servicios y repositorios llegan con LAB-25.
- Sin impacto en el frontend: este cambio no expone rutas HTTP.
- `docs/create-job-position.md` describe la ruta inconsistente `POST api/employer/jobs`;
  su corrección pertenece a LAB-25, que es quien define el contrato HTTP.
