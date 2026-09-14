## Why

La base de datos no distingue la identidad de dominio de cada cuenta ni permite persistir perfiles de empleador. LAB-18 establece esas invariantes antes de incorporar el rol al contrato de autenticación y exponer los endpoints de empleadores.

## What Changes

- **BREAKING** Agregar a `users` un rol obligatorio, limitado a `employee` o `employer`, sin valor predeterminado permanente; migrar las cuentas existentes a `employer` e impedir cambios posteriores.
- Crear `employers` con relación uno a uno con `users`, nombre, industria, ubicación, modalidades de contratación abiertas y timestamps.
- Garantizar desde PostgreSQL que una cuenta no pueda tener simultáneamente perfiles `employee` y `employer`.
- Incorporar consultas sqlc para crear un empleador y obtenerlo por usuario, y regenerar los modelos tipados.
- Dejar fuera de alcance los cambios del registro y la sesión, los endpoints HTTP de empleadores y el frontend, cubiertos por LAB-19, LAB-20, LAB-21 y LAB-22.

## Capabilities

### New Capabilities

- `user-role-employer-persistence`: Persistencia del rol inmutable de usuario, exclusividad entre perfiles y almacenamiento tipado del perfil de empleador.

### Modified Capabilities


## Impact

- Nueva migración PostgreSQL en `sql/schema/`, sin modificar migraciones aplicadas.
- Nuevas consultas en `sql/queries/employers.sql` y código regenerado en `internal/database/`.
- La migración requiere coordinación obligatoria con LAB-19: no podrá aplicarse en un entorno que atienda altas hasta que el binario compatible que proporciona un rol válido esté listo para desplegarse en la misma ventana.
- No cambia rutas, DTO HTTP ni código frontend en este ticket.
