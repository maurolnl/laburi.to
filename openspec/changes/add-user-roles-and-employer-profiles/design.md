## Context

`users` no contiene un rol y `employees.user_id` ya establece una relación uno a uno con borrado en cascada. El repositorio modela dominios cerrados mediante `TEXT` con `CHECK`, listas abiertas mediante `TEXT[]` y genera la capa tipada desde migraciones y queries con sqlc. La migración debe admitir cuentas y perfiles de empleado existentes, aunque el ticket exige asignarles inicialmente el rol `employer`.

LAB-19 incorporará el rol al alta y a la sesión; LAB-20 consumirá las consultas de empleadores. Aplicar esta migración sin LAB-19 vuelve inválido el `INSERT` actual de usuarios porque no suministra el nuevo campo obligatorio.

## Goals / Non-Goals

**Goals:**

- Mantener las invariantes de rol y exclusividad en PostgreSQL, sin depender de que todos los consumidores pasen por una capa de servicio.
- Hacer segura la exclusividad de perfiles frente a transacciones concurrentes.
- Mantener la migración reversible a nivel de esquema y compatible con los perfiles de empleado ya almacenados.

**Non-Goals:**

- Exigir que el rol asignado por el backfill coincida retroactivamente con un perfil de empleado existente.
- Incorporar el rol a DTO, login, JWT o `/auth/me`.
- Crear handlers, servicios o repositorios de dominio para empleadores.
- Validar reglas de presentación de los strings libres, responsabilidad de LAB-20.

## Decisions

### Representar el rol con `TEXT` y `CHECK`

La nueva columna `users.role` usará `TEXT NOT NULL` con una restricción para `employee` y `employer`. Esto sigue el patrón de las migraciones actuales y evita administrar un tipo PostgreSQL global. Se descartó un enum nativo porque no aporta integridad adicional para dos valores y vuelve más costosa su evolución y reversión.

La migración agregará primero la columna nullable, actualizará todas las filas existentes a `employer`, agregará el `CHECK` y finalmente aplicará `NOT NULL`. No conservará un valor por defecto: LAB-19 debe hacer explícita la selección en cada alta nueva.

### Impedir cambios de rol mediante trigger

Un trigger `BEFORE UPDATE OF role` comparará valores con `IS DISTINCT FROM` y abortará cualquier modificación. Una restricción `CHECK` solo valida el dominio y no puede comparar el valor anterior; confiar únicamente en que no exista una query de actualización dejaría la invariante abierta a otros consumidores SQL.

### Modelar empleadores como relación uno a uno

`employers` tendrá `id`, `user_id UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE`, `name`, `industry` y `location` como `TEXT NOT NULL`, `hiring_modalities` como `TEXT[] NOT NULL DEFAULT '{}'`, y timestamps `TIMESTAMPTZ NOT NULL DEFAULT now()`. La unicidad crea el índice necesario para consultas por usuario y el borrado en cascada replica el ciclo de vida de `employees`. Este ticket solo crea y consulta perfiles: ambos timestamps se inicializan al alta y cualquier operación futura de actualización será responsable de avanzar `updated_at`.

La ubicación será texto libre en este ticket. Las modalidades aceptarán cualquier string y podrán formar una lista vacía; LAB-20 validará campos vacíos o elementos no presentables antes de persistirlos.

### Serializar y comprobar la exclusividad entre tablas

Triggers sobre inserciones o cambios de `user_id` en ambas tablas tomarán un advisory lock transaccional derivado del usuario y comprobarán la ausencia de una fila en la tabla opuesta. El lock evita que dos transacciones concurrentes observen ambas tablas vacías e inserten perfiles incompatibles.

Se descartó una comprobación sin lock por su condición de carrera. También se descartó introducir una tabla central de perfiles porque ampliaría el modelo y exigiría migrar las relaciones existentes. Los perfiles de empleado previos se conservan; el backfill a `employer` no los convierte ni permite agregarles un perfil de empleador.

### Limitar sqlc a las operaciones solicitadas

`sql/queries/employers.sql` definirá `CreateEmployer :one` y `GetEmployerByUserID :one`. `sqlc generate` actualizará los modelos de `users` por la nueva columna y generará las operaciones de empleador; no se editará manualmente `internal/database`.

## Risks / Trade-offs

- [La migración y LAB-19 se despliegan por separado] → Tratar el binario compatible de LAB-19 como prerrequisito estricto y aplicar ambos cambios en la misma ventana; no ejecutar la migración mientras una instancia con el `INSERT` anterior pueda atender altas.
- [El backfill `employer` no coincide con perfiles de empleado históricos] → Preservar esos perfiles como excepción histórica y bloquear cualquier segundo perfil; LAB-19 mantiene explícitamente esta compatibilidad.
- [Los advisory locks pueden colisionar si comparten namespace con otras funciones] → Usar una clave de namespace fija junto con `user_id` en la variante de dos enteros.
- [Los triggers agregan lógica transversal a dos tablas] → Nombrarlos explícitamente, cubrir ambos sentidos y documentar su eliminación en `Down`.

## Migration Plan

1. Crear una migración nueva posterior a `0004`; no modificar migraciones aplicadas.
2. Agregar y completar `users.role`, instalar su restricción y el trigger de inmutabilidad.
3. Crear `employers` y luego los triggers simétricos de exclusividad.
4. Regenerar sqlc y preparar junto con LAB-19 un binario cuyo alta suministre `role`; este binario es un prerrequisito estricto para aplicar la migración en cualquier entorno que atienda registro.
5. Validar en PostgreSQL descartable el camino `Up`, las invariantes y el camino `Down`; no ejecutar sobre una base compartida durante desarrollo. LAB-23 incorporará la automatización persistente de estos escenarios a la suite.

El rollback elimina primero triggers y funciones, luego `employers`, la restricción y la columna `users.role`. El valor de backfill se pierde al quitar la columna, sin modificar las cuentas ni perfiles originales.
