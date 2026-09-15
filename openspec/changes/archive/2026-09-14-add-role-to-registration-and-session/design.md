## Context

La migración de LAB-18 agregó `users.role` como `NOT NULL`, restringió sus valores y bloqueó actualizaciones, pero `CreateUser` todavía no inserta la columna. La autenticación actual firma únicamente el id como `sub`, los middlewares reconstruyen solo ese id y los DTO de login y `/auth/me` omiten el rol. La creación de employee recibe el id autenticado, pero no puede verificar el tipo de perfil permitido.

El backfill de LAB-18 asignó `employer` a todos los usuarios preexistentes y preservó perfiles de employee históricos como excepción explícita. Este cambio debe respetar ese dato almacenado sin recalcularlo.

## Goals / Non-Goals

**Goals:**
- Representar `employee` y `employer` mediante un tipo de rol de cuenta validable y distinto de la especialidad profesional del employee.
- Propagar un principal autenticado con id y rol desde el JWT hasta servicios protegidos.
- Aplicar la restricción de rol antes de ejecutar efectos de creación de perfil.
- Mantener compatibles el casing y los códigos exitosos de los contratos HTTP actuales.

**Non-Goals:**
- Agregar endpoints para actualizar roles.
- Crear la API de employer de LAB-20.
- Implementar selector o guards de navegación de LAB-21.
- Corregir perfiles históricos cuyo usuario fue marcado como `employer` por el backfill.

## Decisions

### Usar un tipo de rol de cuenta explícito

Se definirá `UserRole` como un tipo cerrado con `employee` y `employer`, junto con validación explícita. DTO, claims, repositorio y reglas de perfil usarán ese tipo en vez de strings dispersos. El nombre lo diferencia del campo `role` del perfil employee, que representa una especialidad profesional y no participa de la autorización. Se descarta inferir el rol de cuenta por existencia de perfiles porque contradice el backfill acordado y permitiría divergencias entre autorización y sesión.

### Firmar id y rol dentro del access token

El access token incluirá el rol persistido al momento del login. Su validación devolverá un principal con id y rol, y rechazará claims incompletos o fuera del dominio. Esto satisface que identidad y rol provengan del JWT/contexto y evita consultas adicionales en cada middleware. La alternativa de consultar `users` por petición mantendría el token sin rol, pero no cumple el contrato solicitado y agrega acceso a DB a todos los endpoints protegidos.

El payload firmado conservará los registered claims actuales (`iss`, `sub`, `iat`, `exp`) y agregará el claim obligatorio `role` como string. Por ejemplo: `{"iss":"chirpy-access","sub":"1","iat":...,"exp":...,"role":"employee"}`. La validación MUST exigir que `sub` sea un int32 válido y que `role` sea exactamente `employee` o `employer` antes de construir el principal.

El rol es inmutable, por lo que no existe riesgo de que un token válido conserve un permiso anterior después de un cambio legítimo.

### Mantener los contratos JSON aditivos

Registro recibirá `role`; login agregará `role` junto a sus campos camelCase actuales; `/auth/me` conservará `ID` y `Email` y agregará `Role` para no introducir un cambio de casing ajeno al ticket. El frontend mapeará explícitamente ambos contratos.

Los contratos coordinados serán:

- `POST /auth/register`, `Content-Type: application/json`: `{"email":"user@example.com","password":"secret123","role":"employee"}` → `200` sin cuerpo; rol ausente o inválido → `400`.
- `POST /auth/login`, `Content-Type: application/json`: request actual de email/password → `202` con `{"id":1,"email":"user@example.com","role":"employee","token":"...","refreshToken":"..."}`.
- `GET /auth/me`, `Authorization: Bearer`: → `200` con `{"ID":1,"Email":"user@example.com","Role":"employee"}`; token sin id o rol válido → `401` con `{"error":"..."}`.
- `POST /employees`, `multipart/form-data`: el `role` del formulario sigue siendo la especialidad profesional; un principal con rol de cuenta `employer` → `403` con `{"error":"..."}`.

### Autorizar la creación de perfil en la capa de negocio

Los servicios que crean perfiles recibirán el principal autenticado, no ids o roles de cuenta tomados del payload. Una regla compartida recibirá `UserRole` y el tipo de perfil objetivo, y devolverá un error de autorización estable cuando no coincidan. El handler mapeará ese error a `403`; el servicio ejecutará la regla antes de subir archivos o escribir en DB. El alta de employee existente adoptará la regla ahora; LAB-20 deberá usarla al incorporar el alta de employer. Ambas direcciones se cubrirán en pruebas de la política, y los tests del endpoint existente comprobarán que el rechazo de employer a employee no invoca uploader ni store.

Se mantiene además la exclusividad transaccional de LAB-18 como defensa final ante datos históricos, concurrencia u otros consumidores SQL.

### Regenerar sqlc desde la query fuente

`CreateUser` insertará el rol explícitamente y luego se regenerará sqlc. No se editará código generado ni se agregará un default a la columna.

## Risks / Trade-offs

- [Los access tokens emitidos antes del despliegue no contienen rol] → Se rechazarán y el usuario deberá iniciar sesión nuevamente; no se aceptará un rol implícito inseguro.
- [El backfill marca como employer usuarios con perfil histórico de employee] → Se devolverá el rol persistido y se conservará el perfil histórico sin habilitar nuevas creaciones cruzadas.
- [Cambiar la firma de validación JWT afecta varios middlewares] → Actualizar todos los consumidores y sus pruebas en el mismo cambio.
- [La API de employer aún no existe] → Probar la política en ambas direcciones y exigir su reutilización en LAB-20, sin crear endpoints fuera de alcance.

## Migration Plan

1. Ejecutar una ventana coordinada sin tráfico de registro y detener todas las instancias antiguas antes de cambiar el esquema.
2. Aplicar la migración de LAB-18 y desplegar inmediatamente el binario compatible antes de reabrir el registro; no existe un orden online seguro porque cada versión del binario requiere un esquema diferente.
3. Invalidar de hecho los access tokens anteriores al exigir el claim de rol; los refresh tokens no se modifican porque no existe endpoint de renovación activo.
4. Ante rollback, volver a detener registros y revertir conjuntamente binario y migración; nunca ejecutar el binario anterior mientras `users.role` sea obligatorio.
