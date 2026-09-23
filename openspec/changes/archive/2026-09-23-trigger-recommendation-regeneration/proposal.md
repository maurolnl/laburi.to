## Why

El productor de LAB-32 y el consumidor de LAB-33 ya existen, pero nadie los invoca: `cmd/api.go`
sigue cableando `jobposition.NoopEventPublisher{}` y `internal/employee` no tiene siquiera un
puerto de eventos. Hoy ningún cambio de dominio abre un batch, así que la cola nunca recibe
trabajo y las consultas de LAB-35 no tendrían conjunto vigente que devolver. Este cambio aporta
la mitad que falta: conectar las escrituras de dominio con la emisión ya construida.

## What Changes

- Nueva capacidad de disparadores: qué operación de dominio solicita la regeneración de qué
  sujeto, en qué orden respecto de la persistencia y qué operaciones explícitamente **no**
  disparan nada.
- Regla de perfil completo para el empleado: solo un perfil con sus cinco pasos presentes
  —registro base, locación, recursos técnicos, disponibilidad y al menos un título de
  educación— genera trabajo. Mientras falte un paso, ninguna escritura publica.
- Nuevo puerto de eventos en `internal/employee`, simétrico al que `internal/jobposition` ya
  tiene, notificado después de cada escritura de perfil que deje al empleado completo.
- **BREAKING** (interno, sin efecto en el contrato HTTP): `jobposition.JobPositionEventPublisher`
  pasa a recibir el identificador del puesto en lugar de su representación completa. Con los dos
  puertos reducidos a identificadores, un único tipo de `internal/recommendation` los satisface
  por tipado estructural y ningún paquete de dominio necesita importar a otro ni existe capa
  adaptadora nueva.
- Cableado real en `cmd/api.go`: con la cola habilitada se inyecta el productor sobre
  `queue.Client`; con la cola deshabilitada se inyecta `recommendation.NoopJobPublisher{}`, para
  no marcar como `failed` un batch por cada escritura en un entorno que deliberadamente no emite.
- Estrategia ante fallo de publicación, que el ticket pide definir acá: persistir el cambio de
  dominio, responder al cliente y recién después emitir. Si la emisión falla, el batch recién
  abierto queda en `failed` —registro durable de que ese sujeto quedó sin regenerar— y el fallo se
  registra, sin revertir el alta ni la edición ya confirmadas. Sin outbox ni tabla nueva.

Sin cambios en el contrato HTTP: ninguna ruta, cuerpo ni código de estado se modifica, y el
frontend no se toca.

## Capabilities

### New Capabilities
- `recommendation-regeneration-triggers`: qué cambios de dominio solicitan regenerar las
  recomendaciones de un sujeto, la condición de perfil completo del empleado, el orden entre
  persistencia y emisión, el tratamiento del fallo de emisión y el conjunto de operaciones que no
  disparan nada.

### Modified Capabilities
- `job-position-api`: el requisito «Notificación del proceso de recomendaciones» pasa a describir
  una notificación que transporta el identificador del puesto y no su representación completa. Las
  rutas, cuerpos y códigos de estado no cambian.

## Impact

- **Código nuevo**: adaptador de disparadores en `internal/recommendation`, puerto de eventos e
  inspección de completitud en `internal/employee`, y sus tests con productor falso.
- **Código existente**: `internal/jobposition` (firma del puerto y su llamador),
  `internal/employee` (servicio y store), `cmd/api.go` (construcción del productor real).
- **Base de datos**: ninguna migración. Una consulta nueva en `sql/queries/employees.sql` para
  resolver la completitud del perfil, más `sqlc generate`.
- **Dependencias**: ninguna nueva.
- **Documentación**: `docs/recommendation-queue.md` gana la lista de disparadores y la estrategia
  ante fallo de publicación.
- **Fuera de alcance**: los endpoints HTTP de consulta de recomendaciones (LAB-35), el algoritmo de
  indicadores —que la épica prohíbe definir— y cualquier reintento automático de las emisiones
  fallidas.
