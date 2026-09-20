## Context

Ver `proposal.md` - Why. El esquema llega hasta `0006_job_positions.sql`, que introdujo
el puesto publicado con soft delete. El proyecto usa goose para migraciones, sqlc para
acceso tipado, `lib/pq` como driver, IDs `SERIAL` (int32), `TIMESTAMPTZ` en todos los
timestamps y checks textuales en lugar de tipos `ENUM`. Los nullables se generan como
`sql.NullX` y los arrays se pasan con `pq.Array`.

La suite de tests actual no toca ninguna base de datos: los repositorios se prueban
contra fakes y las migraciones contra tests de contrato que leen el SQL como texto. Este
cambio introduce la primera base real en tests, decisión tomada explícitamente con el
usuario.

Restricción operativa: las migraciones ya aplicadas son inmutables. Este cambio agrega
`0007_recommendations.sql` y no toca las anteriores.

Los consumidores de este esquema todavía no existen. El diseño se valida contra los
criterios de aceptación de LAB-32, LAB-33 y LAB-35, no contra código presente.

## Goals / Non-Goals

**Goals:**

- Esquema que soporte productor, consumidor y consultas de la épica sin cambios
  posteriores de estructura.
- Reprocesamiento idempotente garantizado por constraints, no por disciplina del
  llamador.
- Reemplazo del conjunto vigente atómico y sin estados parciales observables.
- `empty result` y `failed` distinguibles sin ambigüedad.
- Orden y paginación de la épica resolubles por índice.

**Non-Goals:**

- Contrato de scoring e indicadores: es LAB-30.
- Cliente y configuración SQS: es LAB-31.
- Productor y publicación de eventos: es LAB-32.
- Worker, transiciones de estado en ejecución y política de reintentos: es LAB-33.
- Disparadores desde perfiles y puestos: es LAB-34.
- Endpoints HTTP, autorización y serialización: es LAB-35.
- Factories y cobertura integral de la épica: es LAB-39.
- Historial consultable de batches: la épica lo descarta explícitamente.

## Decisions

### Un solo batch por sujeto excluyente, con claves foráneas reales

`recommendation_batches` tiene `subject_type TEXT NOT NULL` con check
`IN ('employee', 'job_position')`, más `employee_id` y `job_position_id` nullable, ambos
con `REFERENCES ... ON DELETE CASCADE`, y un check que obliga a que exactamente uno esté
presente y coincida con `subject_type`.

Alternativa considerada: dos tablas tipadas separadas. Se descarta porque duplicaría
queries sqlc, índices y lógica de worker para cada sentido, cuando el ciclo de estados
es idéntico.

Alternativa considerada: polimorfismo puro con `subject_id INTEGER` sin clave foránea.
Se descarta porque perdería integridad referencial y el borrado en cascada, y obligaría
a limpiar batches huérfanos a mano. El esquema actual nunca usa referencias sin FK.

Consecuencia asumida: el modelo generado por sqlc expone `EmployeeID sql.NullInt32` y
`JobPositionID sql.NullInt32`, de modo que el código Go debe desempaquetarlos. Se
acepta a cambio de la integridad garantizada por la base.

### Estado vigente y conjunto vigente son dos punteros distintos

La épica pide tres cosas que no se satisfacen con un único "batch vigente": `failed` se
distingue (LAB-35), un fallo conserva el conjunto anterior (LAB-33) y durante la
ejecución se muestra "procesando" (LAB-17).

Se resuelve separando:

- **Estado vigente**: el batch más reciente del sujeto por `created_at`, cualquiera sea
  su estado. Es lo que determina si la UI muestra procesando, completado, vacío o error.
- **Conjunto vigente**: las filas de `recommendations` del batch `completed` más
  reciente del sujeto.

Cuando el batch más reciente está `completed`, ambos punteros coinciden y el
comportamiento es el literal de la épica. Cuando el más reciente está `pending`,
`processing` o `failed`, el conjunto anterior sigue disponible y el estado refleja la
realidad.

Alternativa considerada: interpretación literal, donde el batch más reciente es el único
vigente y un fallo deja al sujeto sin recomendaciones. Se descarta porque contradice el
criterio de LAB-33.

Alternativa considerada: no exponer los batches fallidos. Se descarta porque contradice
el criterio de LAB-35.

### Retención de a lo sumo dos batches por sujeto

No hay historial consultable, pero tampoco se puede borrar el batch anterior al crear
uno nuevo, porque entonces un fallo destruiría el conjunto vigente.

La regla es: al completar un batch, se borran los demás batches del mismo sujeto
distintos del recién completado. El `ON DELETE CASCADE` de `recommendations` arrastra
sus filas. En estado estable cada sujeto tiene un batch `completed`; mientras hay
trabajo en curso o fallido, tiene dos.

Esta poda ocurre dentro de la misma transacción que marca el batch como `completed`, de
modo que nunca se observa un estado intermedio.

Alternativa considerada: conservar historial completo con una columna `is_current`. Se
descarta porque la épica dice que no se requiere historial y porque agrega una columna
cuya consistencia hay que mantener a mano.

### Unicidad del batch en curso mediante índice único parcial

Dos índices únicos parciales, uno por tipo de sujeto:

```
CREATE UNIQUE INDEX recommendation_batches_employee_in_flight_key
    ON recommendation_batches(employee_id)
    WHERE employee_id IS NOT NULL AND status IN ('pending', 'processing');
```

y su equivalente sobre `job_position_id`. Con esto la base rechaza un segundo batch en
curso para el mismo sujeto: un mensaje duplicado o un redelivery no crea trabajo
paralelo, y la idempotencia queda garantizada por constraint y no por una comprobación
previa sujeta a carrera.

No se restringe la cantidad de batches `completed` o `failed`: esos los acota la poda
descrita arriba.

### Score nullable y numérico

`score NUMERIC(6,4) NULL`. Nullable porque LAB-30 todavía no define los indicadores y la
épica prohíbe inventar un algoritmo temporal; una recomendación puede existir sin
puntaje. `NUMERIC` en lugar de `DOUBLE PRECISION` para que el orden sea determinista y
reproducible, requisito de los tests de orden y desempate de LAB-35.

El orden de la épica es puntaje descendente, de modo que los `NULL` deben ir al final:
las queries usan `ORDER BY score DESC NULLS LAST, <desempate>`.

Alternativa considerada: `score` no nullable con default `0`. Se descarta porque
confundiría "sin puntaje calculado" con "puntaje cero", que son estados distintos.

### Par empleado-puesto único dentro de un batch

`CONSTRAINT recommendations_unique_pair_per_batch UNIQUE (batch_id, employee_id,
job_position_id)`. Impide que una misma dupla aparezca dos veces en el mismo conjunto,
lo que es un error del worker y no un estado válido.

La relación many-to-many pedida por el ticket queda satisfecha: un puesto puede aparecer
con muchos empleados y un empleado con muchos puestos; lo único que se prohíbe es la
repetición exacta dentro de un batch.

### Exclusión de puestos eliminados en lectura, no en escritura

No se agrega un check que impida insertar una recomendación hacia un puesto eliminado:
un puesto puede borrarse lógicamente después de generado el conjunto, y un check no
retroactivo daría falsa seguridad.

La exclusión se garantiza en las queries de lectura, que hacen `JOIN job_positions` con
`job_positions.deleted_at IS NULL`. Es el mismo criterio que ya rige en
`job-position-persistence`: la exclusión la garantizan las queries y los índices, no la
disciplina del llamador.

### Índices de orden y paginación

Para el sentido empleado a puestos, el orden es puntaje descendente con desempate por
fecha de publicación del puesto; para el sentido puesto a empleados, puntaje descendente
con desempate por última actualización del perfil. Ambos desempates viven en tablas
vecinas (`job_positions.created_at`, `employees.updated_at`), así que el índice de
`recommendations` cubre el filtro por batch y el puntaje, y el desempate se resuelve con
el join.

Se crea `recommendations(batch_id, score DESC)` para eso, más
`recommendations(employee_id)` y `recommendations(job_position_id)` para las búsquedas
inversas que necesitará LAB-36 al autorizar el acceso al perfil completo.

La paginación es por `LIMIT`/`OFFSET` sobre ese orden. El cursor opaco, si se decide, es
una decisión del contrato HTTP y pertenece a LAB-35.

### Reemplazo atómico como una sola transacción

El repositorio expone una operación que, en una transacción: marca el batch como
`completed`, inserta el conjunto nuevo y borra los batches anteriores del sujeto. Se usa
el patrón vigente `database.New(db).WithTx(tx)` con `defer tx.Rollback()`, idéntico al
de `internal/employee/repo.go`.

Un fallo en cualquier paso deja el batch en su estado previo y el conjunto anterior
intacto, que es exactamente el criterio "los fallos conservan el conjunto vigente
anterior".

### PostgreSQL efímero en tests con testcontainers-go

Decisión tomada con el usuario. El criterio de aceptación pide tests de transacción, y
un fake de `DBTX` no puede probar atomicidad real, unicidad bajo índice parcial ni el
rechazo de los CHECK: probaría el mock, no el esquema.

Se agrega `internal/testsupport` con un helper que levanta un contenedor PostgreSQL una
vez por ejecución, aplica las migraciones de `sql/schema` en orden y entrega una
conexión. Cada test corre dentro de su propia transacción con rollback al terminar, o
sobre un esquema propio cuando necesita probar commits reales.

Degradación explícita: si no hay entorno de contenedores disponible, los tests que lo
requieren hacen `t.Skip` con un mensaje claro en lugar de fallar. El repositorio no
tiene CI configurado hoy, de modo que `go test ./...` debe seguir siendo ejecutable en
una máquina sin Docker.

Alternativa considerada: `go-sqlmock`. Se descarta porque verifica el SQL emitido, no su
efecto, y el riesgo concreto de este ticket es el comportamiento del esquema.

Alternativa considerada: una base de test compartida vía `DB_URL`. Se descarta porque
obliga a un recurso externo y porque el `CLAUDE.md` del proyecto prohíbe correr
migraciones sobre bases compartidas.

## Risks / Trade-offs

- [testcontainers-go es la primera dependencia de test pesada del repositorio y arrastra
  el cliente de Docker] → Queda acotada a `internal/testsupport` y a los tests que la
  usan; el resto de la suite sigue sin dependencias. El skip explícito evita que una
  máquina sin Docker quede bloqueada.
- [Los tests con contenedor son órdenes de magnitud más lentos que los actuales] → El
  contenedor se levanta una vez por ejecución y se reutiliza; los tests unitarios
  existentes no se tocan.
- [Sin CI configurado, nadie garantiza que los tests de integración se ejecuten] → Se
  deja asentado como deuda. Configurar CI excede este ticket y debería ser un cambio
  propio.
- [La regla de retención vive en el repositorio, no en la base] → No hay constraint que
  impida acumular batches `completed` si un llamador futuro omite la poda. Se mitiga
  concentrando el reemplazo en una única operación transaccional del repositorio y
  cubriéndola con test.
- [El check de exclusividad del sujeto duplica información entre `subject_type` y las
  dos claves foráneas] → Es redundancia deliberada: `subject_type` hace las queries
  legibles y el check impide que las tres columnas se desincronicen.
- [`score` nullable obliga a `NULLS LAST` en toda query de orden] → Se cubre con test
  de orden; cuando LAB-30 entregue el algoritmo, volver nullable a no nullable es una
  migración menor.
- [El número de migración `0007` queda reservado] → Si otra rama toma el mismo número,
  hay conflicto al mergear. Se avisa en el PR; no hay mecanismo automático en el
  proyecto.
- [Varios criterios de aceptación de la épica solo son verificables parcialmente aquí]
  → Redelivery, DLQ y batch obsoleto dependen del worker de LAB-33. Este cambio cubre la
  parte que le corresponde: la base rechaza un segundo batch en curso y el reemplazo es
  atómico.

## Migration Plan

1. Agregar `sql/schema/0007_recommendations.sql` con secciones `-- +goose Up` y
   `-- +goose Down`.
2. Agregar `sql/queries/recommendations.sql`.
3. Ejecutar `sqlc generate` y versionar la salida en `internal/database/`.
4. Agregar `internal/testsupport` y las dependencias de test en `go.mod` / `go.sum`.
5. Agregar `internal/recommendation` con repositorio, factories y tests.
6. Ejecutar `gofmt -l .`, `go vet ./...` y `go test ./...`.

Rollback: la sección `-- +goose Down` elimina índices y tablas en orden inverso. Al no
modificar objetos existentes, el rollback no afecta empleados, empleadores, usuarios ni
puestos.

No se ejecutan migraciones sobre ninguna base compartida como parte de este cambio.
