## Why

La persistencia ya exige un rol inmutable, pero el registro actual no lo suministra y la sesión no lo transporta. Esto impide desplegar con seguridad la migración de LAB-18 y deja la creación de perfiles sin una autorización de dominio basada en el rol autenticado.

## What Changes

- Extender `POST /auth/register` para exigir exactamente `employee` o `employer` y persistir el valor elegido.
- Incorporar el rol al access token, al contexto autenticado, a `POST /auth/login` y a `GET /auth/me`.
- Derivar identidad y rol exclusivamente del JWT/contexto para las operaciones protegidas.
- Rechazar en la capa de negocio la creación de un perfil de tipo opuesto al rol autenticado, aplicándolo al alta de employee existente y dejando la misma regla disponible para la futura API de employer.
- Mantener el rol `employer` asignado por el backfill a las cuentas existentes, sin inferirlo desde perfiles históricos ni permitir cambios posteriores.
- Cubrir registro sin rol o con rol inválido, grants con rol y accesos cruzados mediante pruebas.
- Documentar el contrato coordinado con frontend y `docs/use-cases.md`.

## Capabilities

### New Capabilities
- `role-aware-authentication`: Registro, grants y sesión autenticada con rol obligatorio e inmutable derivado del servidor.

### Modified Capabilities
- `employee-profile-onboarding`: La creación del perfil de employee queda restringida a una identidad autenticada cuyo rol sea `employee`.

## Impact

- Contratos HTTP de `POST /auth/register`, `POST /auth/login` y `GET /auth/me`.
- Modelos, servicio, repositorio y handlers de `internal/user`; claims y validación de `internal/auth`.
- Middleware/contexto autenticado y creación del perfil en `internal/employee`.
- Query `CreateUser` y código sqlc regenerado; no se agrega una migración ni se modifica el backfill de LAB-18.
- Tipos y mapper de autenticación del frontend, más `docs/use-cases.md`, coordinados bajo el mismo ticket.
