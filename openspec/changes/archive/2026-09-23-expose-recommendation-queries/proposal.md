## Why

La épica LAB-17 ya tiene persistencia, cola, productor, consumidor y disparadores, pero el
conjunto vigente que todo eso genera no es observable: `internal/recommendation` no expone
ninguna ruta HTTP. Hoy un empleado no puede ver sus puestos sugeridos ni un empleador sus
candidatos, y `docs/employee-searching-for-position.md` y
`docs/employer-searching-for-employees.md` siguen describiendo rutas que no existen. Este
cambio aporta el borde de lectura que cierra la épica del lado del backend.

## What Changes

- Dos rutas nuevas, ambas autenticadas por JWT:
  - `GET /employees/{employeeID}/job-recommendations`, que devuelve los puestos sugeridos a un
    empleado;
  - `GET /jobs/{jobPositionID}/employee-recommendations`, que devuelve los candidatos sugeridos
    para un puesto.
- Respuesta única para ambas: estado del batch vigente, lista de items del conjunto vigente y
  metadatos de paginación. El estado y los items provienen de batches distintos —el más
  reciente y el último `completed`— y esa distinción ya la resuelve la persistencia.
- Estado de transporte `none`, fuera del enum de batch, para el sujeto que nunca tuvo un batch.
  Junto con `pending`, `processing`, `completed` y `failed` cubre los cinco estados que el
  frontend necesita distinguir: nunca solicitado, procesando, resultado —vacío o no— y fallo.
- Paginación por `limit` y `offset` en query string, validada en el borde: `limit` acotado a un
  máximo, `offset` no negativo y ambos numéricos. Fuera de rango o no numérico responde `400`.
- Metadato `total` con el tamaño del conjunto vigente ya filtrado, para que el cliente pueda
  paginar sin adivinar cuándo terminó.
- Ownership derivado exclusivamente del JWT: un `employee` solo consulta su propio perfil y un
  `employer` solo sus propios puestos. El identificador del path sirve para detectar el acceso
  ajeno, nunca para resolver identidad.
- Orden y exclusión de puestos eliminados: ya son comportamiento de la persistencia y este
  cambio los expone tal cual, sin reordenar ni filtrar en el borde.

Sin cambios en el frontend: las pantallas que consumen estas rutas son trabajo aparte.

## Capabilities

### New Capabilities
- `recommendation-query-api`: el contrato HTTP de consulta de recomendaciones en ambos
  sentidos —rutas, autenticación, ownership, estados, paginación, orden expuesto y códigos de
  respuesta—.

### Modified Capabilities
- `recommendation-persistence`: la lectura del conjunto vigente pasa a informar también su
  tamaño total ya filtrado, y la capa gana la resolución de la propiedad de un sujeto —qué
  usuario es dueño de un empleado y de un puesto activo— que el borde necesita para autorizar.

## Impact

- **Código nuevo**: `internal/recommendation/handler.go`, `service.go` y sus tests; ampliación
  de `store.go` con los puertos de consulta y de propiedad.
- **Código existente**: `internal/recommendation/repo.go` y `models.go` (tamaño total y
  resolución de propiedad), `cmd/api.go` (registro de las rutas nuevas).
- **Base de datos**: ninguna migración. Cuatro consultas nuevas en
  `sql/queries/recommendations.sql` —dos de conteo y dos de propiedad— más `sqlc generate`.
- **Dependencias**: ninguna nueva. `internal/recommendation` pasa a importar `internal/auth` e
  `internal/user`, como ya hace `internal/jobposition`; sigue sin importar `internal/employee`
  ni `internal/jobposition`.
- **Documentación**: `docs/recommendation-queue.md` del backend gana la sección de consulta;
  los diagramas de secuencia `docs/employee-searching-for-position.md` y
  `docs/employer-searching-for-employees.md` de la raíz se reescriben con las rutas, estados y
  orden reales.
- **Fuera de alcance**: el algoritmo de indicadores —que la épica prohíbe definir—, las
  pantallas del frontend, cualquier filtro o búsqueda sobre el conjunto vigente y la
  paginación por cursor.
