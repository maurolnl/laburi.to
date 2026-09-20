## Why

La épica LAB-17 necesita persistir recomendaciones en ambos sentidos: puestos
sugeridos a un empleado y candidatos sugeridos para un puesto. Hoy no existe ninguna
tabla que represente ni una recomendación ni el proceso asincrónico que la genera. Sin
ese esquema no pueden construirse el productor SQS (LAB-32), el consumidor con
reemplazo atómico (LAB-33) ni las consultas paginadas (LAB-35), que son todos
consumidores directos de estas tablas.

Este cambio es exclusivamente de persistencia: no expone rutas HTTP, no publica
mensajes y no calcula puntajes.

## What Changes

- Nueva tabla `recommendation_batches`: una ejecución de generación de recomendaciones
  para un sujeto, con estados `pending | processing | completed | failed`.
- El sujeto del batch es un empleado **o** un puesto, nunca ambos. Se modela con dos
  claves foráneas nullable (`employee_id`, `job_position_id`) más un `subject_type` y
  un CHECK de exclusividad mutua, de modo que la integridad referencial y el borrado en
  cascada siguen siendo reales.
- Nueva tabla `recommendations`: relación many-to-many entre `employees` y
  `job_positions`, siempre perteneciente a un batch, con `score NUMERIC NULL` hasta que
  LAB-30 defina los indicadores.
- Distinción explícita entre **estado vigente** y **conjunto vigente**: el estado se lee
  del batch más reciente del sujeto; el conjunto de recomendaciones se lee del batch
  `completed` más reciente. Esto permite que un batch `failed` se exponga como tal
  (LAB-35) sin destruir el conjunto anterior (LAB-33).
- Retención acotada: por sujeto se conservan a lo sumo el último batch `completed` y el
  último batch no completado. Índices únicos parciales garantizan que no haya dos
  batches en curso para el mismo sujeto, lo que vuelve idempotente el reprocesamiento.
- Un batch `completed` sin filas en `recommendations` es un resultado vacío legítimo y
  se distingue de `failed` por su `status`, no por la cantidad de filas.
- Los puestos con `deleted_at IS NOT NULL` quedan excluidos: las queries de lectura
  filtran contra `job_positions.deleted_at IS NULL` y el borrado físico de un puesto
  arrastra sus recomendaciones en cascada.
- Timestamps, constraints e índices que soportan el orden pedido por la épica: puntaje
  descendente con desempate por `job_positions.created_at` en un sentido y por
  `employees.updated_at` en el otro.
- Queries sqlc tipadas para crear un batch, transicionar su estado, resolver el batch
  vigente de cada tipo, reemplazar atómicamente el conjunto de recomendaciones y
  listarlo paginado en ambos sentidos.
- Nueva infraestructura de test con PostgreSQL efímero vía `testcontainers-go`, para
  probar de verdad la atomicidad del reemplazo, la unicidad del batch en curso y los
  CHECK del esquema.

Sin cambios BREAKING: la migración solo agrega objetos nuevos.

## Capabilities

### New Capabilities

- `recommendation-persistence`: integridad persistente del batch de recomendaciones,
  su sujeto excluyente, su ciclo de estados, la relación many-to-many entre empleados
  y puestos, la resolución del batch vigente, el reemplazo atómico del conjunto, la
  exclusión de puestos eliminados y el acceso tipado vía sqlc.
- `database-integration-testing`: disponibilidad de una base PostgreSQL efímera en la
  suite de tests, con el esquema migrado, aislamiento entre tests y degradación
  explícita cuando no hay entorno de contenedores.

### Modified Capabilities

Ninguna. Los requisitos de `job-position-persistence`,
`employee-profile-onboarding`, `employer-profile-api`,
`user-role-employer-persistence` y `role-aware-authentication` se mantienen sin
cambios. Este cambio solo agrega tablas que referencian las existentes.

## Impact

- `sql/schema/`: migración nueva `0007_recommendations.sql` con su sección
  `-- +goose Down`.
- `sql/queries/`: archivo nuevo `recommendations.sql`.
- `internal/database/`: código regenerado por `sqlc generate`.
- `internal/recommendation/`: paquete nuevo, limitado en este cambio al repositorio de
  persistencia, sus factories y sus tests. Productor, consumidor, scoring y handlers
  llegan con LAB-30 a LAB-35.
- `internal/testsupport/`: paquete nuevo con el arranque del PostgreSQL efímero y la
  aplicación de las migraciones.
- `go.mod` / `go.sum`: se incorpora `testcontainers-go` y su driver de PostgreSQL como
  dependencias de test.
- Sin impacto en el frontend: este cambio no expone rutas HTTP.
- `docs/employee-searching-for-position.md` y
  `docs/employer-searching-for-employees.md` describen el flujo de consulta pero no el
  esquema; su actualización pertenece a LAB-35, que es quien define el contrato HTTP.
