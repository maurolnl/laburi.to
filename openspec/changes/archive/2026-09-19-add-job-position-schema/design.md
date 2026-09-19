## Context

Ver `proposal.md` - Why. El esquema actual llega hasta la migración
`0005_add_user_roles_and_employers.sql`, que introdujo el rol inmutable del usuario,
la tabla `employers` y la exclusividad entre perfiles. El proyecto usa goose para las
migraciones, sqlc para el acceso tipado y checks textuales en lugar de tipos `ENUM` de
PostgreSQL. No hay infraestructura de recomendaciones ni colas; cualquier referencia a
`LaunchJobPositionRecommendations` pertenece a una épica posterior.

Restricción operativa: las migraciones ya aplicadas son inmutables. Este cambio agrega
`0006_job_positions.sql` y no toca las anteriores.

## Goals / Non-Goals

**Goals:**

- Esquema mínimo que soporte el CRUD con ownership de LAB-25 sin cambios posteriores
  de estructura.
- Dominios comparables con el perfil de empleado, para que las recomendaciones futuras
  no necesiten una tabla de equivalencias.
- Exclusión de puestos eliminados garantizada por índices y queries, no por disciplina
  del llamador.

**Non-Goals:**

- Handlers, servicios, repositorios y validación HTTP: son LAB-25.
- Factories y cobertura de servicio del puesto: son LAB-28.
- Disparo asincrónico de recomendaciones.
- Historial de cambios o versionado del puesto.

## Decisions

### Checks textuales en lugar de tipos ENUM

Se replican los `CHECK (columna IN (...))` usados en `employees` y `employers`.
Alternativa considerada: `CREATE TYPE ... AS ENUM`. Se descarta porque agregar un
valor a un ENUM en PostgreSQL es más rígido en migraciones y porque introduciría un
segundo estilo de modelado de dominios en el mismo esquema.

### Reutilización literal de los dominios del empleado

`required_experience` y `required_education_level` copian los valores exactos de
`employee_years_of_experience_check` y `employee_education_type_check`. Alternativa
considerada: dominios propios del puesto. Se descarta porque obligaría a una tabla de
mapeo en la épica de recomendaciones, que es precisamente el consumidor de estos
campos.

Consecuencia asumida: si un dominio cambia, hay que migrar ambas tablas en el mismo
cambio. Queda registrado como riesgo.

### Rango 1..8 para las horas disponibles

`employee_profile_availability` declara dos checks contradictorios: uno inline
`BETWEEN 0 AND 8` y uno nombrado `BETWEEN 0 AND 24`. El inline es el que efectivamente
restringe. El puesto adopta 1..8 para quedar en la misma escala y además excluye el 0,
que no tiene sentido funcional en una oferta laboral. No se corrige el check
contradictorio de `employees`: es deuda preexistente y ajena a este ticket.

### Timezone como TEXT validado en la aplicación

`employee_location.timezone` ya es `TEXT NOT NULL` sin check, y la validación real se
hace contra `pg_timezone_names` desde el repositorio. Se mantiene ese precedente.
Alternativa considerada: `CHECK` con la lista de zonas. Se descarta porque
`pg_timezone_names` es una vista y no puede usarse dentro de un `CHECK`, y hardcodear
la lista la deja desactualizada ante cambios de la base de datos de zonas horarias.

### Soft delete con `deleted_at` e índices parciales

`deleted_at TIMESTAMPTZ NULL`; una fila con `deleted_at IS NULL` es un puesto
publicado. Los índices de consulta son parciales con `WHERE deleted_at IS NULL`, lo
que reduce su tamaño y hace que el plan de ejecución solo alcance puestos activos.

Las queries sqlc de lectura y actualización incluyen `deleted_at IS NULL` en su
cláusula `WHERE`. La eliminación usa `UPDATE ... SET deleted_at = now() WHERE id = $1
AND deleted_at IS NULL`, lo que la vuelve idempotente: una segunda eliminación no
afecta filas y conserva la marca original.

Alternativa considerada: tabla de archivo separada. Se descarta por complejidad
desproporcionada para el volumen esperado.

### Sin estado de publicación

No se agrega columna de estado. La publicación es implícita en la existencia de la
fila activa, tal como pide la épica. Esto elimina de raíz la posibilidad de estados
inconsistentes entre `status` y `deleted_at`.

### Prueba de contrato sobre la migración

Se agrega `internal/jobposition/migration_0006_contract_test.go` siguiendo el patrón
introducido para la migración 0005: lee el archivo SQL, separa la sección `-- +goose
Up` y verifica por expresión regular que las constraints, triggers e índices críticos
sigan presentes. Es la única forma de cubrir el esquema sin una base de datos en el
entorno de test.

Alternativa considerada: tests de integración contra PostgreSQL. Se descarta en este
cambio porque el proyecto no tiene todavía infraestructura de base de datos efímera en
tests; agregarla es un cambio propio.

## Risks / Trade-offs

- [Los dominios quedan duplicados entre `employees` y `job_positions`] → La prueba de
  contrato de la migración fija los valores esperados, de modo que una divergencia
  falla en CI en lugar de descubrirse al calcular recomendaciones.
- [El paquete `internal/jobposition` nace conteniendo solo un test] → Es transitorio;
  LAB-25 lo completa con handler, service y repo. La alternativa, ubicar el test en
  `internal/employer`, mezclaría dominios.
- [La prueba de contrato valida texto SQL, no comportamiento real de la base] → Cubre
  regresiones de esquema, que es el riesgo concreto de este ticket. El comportamiento
  se cubrirá cuando exista infraestructura de test con base de datos.
- [El número de migración `0006` queda reservado] → Si otra rama toma el mismo número,
  hay conflicto al mergear. Se mitiga avisando en el PR; no hay mecanismo automático
  en el proyecto.
- [El criterio de aceptación "los puestos eliminados quedan fuera de recomendaciones"
  no es verificable hoy] → No existe módulo de recomendaciones. Se cumple la parte
  verificable: ninguna query activa devuelve puestos eliminados. Queda asentado para
  la épica correspondiente.

## Migration Plan

1. Agregar `sql/schema/0006_job_positions.sql` con secciones `-- +goose Up` y
   `-- +goose Down`.
2. Agregar `sql/queries/job_positions.sql`.
3. Ejecutar `sqlc generate` y versionar la salida en `internal/database/`.
4. Ejecutar `gofmt`, `go vet ./...` y `go test ./...`.

Rollback: la sección `-- +goose Down` elimina los índices y la tabla. Al no modificar
objetos existentes, el rollback no afecta datos de empleados, empleadores ni usuarios.

No se ejecutan migraciones sobre ninguna base compartida como parte de este cambio.
