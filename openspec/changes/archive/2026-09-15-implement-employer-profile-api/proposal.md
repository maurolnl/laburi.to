## Why

La persistencia y la identidad autenticada de empleadores ya existen, pero la API todavía no permite crear ni consultar su perfil. Este cambio completa el acceso HTTP protegido requerido por LAB-20 y deja un contrato coherente para el futuro formulario frontend.

## What Changes

- Incorporar el flujo `handler -> service -> store/repository` para crear y consultar perfiles de empleador mediante las consultas sqlc existentes.
- Exponer `POST /employers`, derivando usuario y rol exclusivamente del principal autenticado, y responder `201` sin cuerpo cuando se crea el perfil.
- Exponer `GET /users/{userID}/employer`, restringido al propietario, y responder `200` con el perfil en JSON.
- Validar que nombre, industria y ubicación no estén vacíos ni contengan solo espacios; permitir una lista vacía de modalidades libres, pero rechazar elementos vacíos.
- Responder `403` ante rol incorrecto o acceso a otro usuario, `409` ante un perfil duplicado o incompatible y `404` cuando el perfil consultado no exista.
- Cubrir éxito, validación, duplicados, rol incorrecto, ausencia de perfil y acceso ajeno mediante pruebas unitarias de handlers y servicio.
- Actualizar `../docs/create-employer.md` para reflejar las rutas y respuestas definitivas; la documentación vive fuera del repositorio Git del backend.

## Capabilities

### New Capabilities

- `employer-profile-api`: Creación y consulta autenticadas del perfil de empleador, con validación, autorización y clasificación estable de errores HTTP.

### Modified Capabilities

Ninguna.

## Impact

- Nuevo paquete de dominio `internal/employer` y composición de rutas en `cmd`.
- Nuevos contratos HTTP `POST /employers` y `GET /users/{userID}/employer`.
- Reutilización de JWT/principal, autorización por rol, validator, helpers HTTP y consultas sqlc existentes; no requiere migraciones ni regeneración sqlc.
- Actualización posterior de `../docs/create-employer.md`; no requiere cambios en el frontend dentro de LAB-20.
