## Context

`internal/recommendation` ya tiene el repositorio completo del lado de lectura:
`JobRecommendationsForEmployee` y `EmployeeRecommendationsForJobPosition` resuelven el batch
vigente, el último batch completado, el orden, el desempate, la exclusión de puestos
eliminados y el tramo `LIMIT/OFFSET`. Lo que falta es el borde HTTP y la autorización. El
paquete no tiene handler ni service, y hoy no importa `internal/employee` ni
`internal/jobposition` —una propiedad que el cambio anterior dejó verificada explícitamente—.

El motivo del cambio está en `proposal.md - Why`; los requisitos, en
`specs/recommendation-query-api/spec.md`.

## Goals / Non-Goals

**Goals:**

- Un único borde de lectura para los dos sentidos, con la misma forma de respuesta y el mismo
  vocabulario de estados.
- Autorización derivada del JWT sin que `internal/recommendation` dependa de los paquetes de
  dominio de empleado ni de puesto.
- Paginación cuyo contrato el cliente pueda recorrer sin adivinar el final.

**Non-Goals:**

- Cambiar el orden, el desempate o el filtrado, que son comportamiento ya especificado de la
  persistencia.
- Cachear, materializar o precalcular el conjunto vigente.
- Unificar el formato de error del resto de los endpoints viejos del backend.

## Decisions

### Los endpoints viven en `internal/recommendation`

Alternativa considerada: repartirlos, la consulta del empleado en `internal/employee` y la del
puesto en `internal/jobposition`, reusando el ownership que cada paquete ya tiene —
`AuthenticatedEmployeeMiddleWare` y `resolveOwnedPosition`—.

Se descarta porque partiría en dos una capacidad única: la forma de la respuesta, el
vocabulario de estados y las reglas de paginación son las mismas en ambos sentidos, y
mantenerlas sincronizadas entre dos paquetes que no se conocen es trabajo permanente. Además
obligaría a los dos paquetes de dominio a importar `internal/recommendation` para leer el
conjunto vigente, invirtiendo la dirección de dependencias que el cambio anterior estableció.

El costo es duplicar la resolución de propiedad, y se paga con dos consultas SQL nuevas —no con
lógica nueva—.

### Ownership resuelto con consultas propias, no importando los paquetes de dominio

`internal/recommendation` gana dos puertos de lectura, `EmployeeOwner` y `JobPositionOwner`,
satisfechos por su propio repositorio sobre `sql/queries/recommendations.sql`. La alternativa
—importar `employee.EmployeeStore` y `jobposition.JobPositionStore`— traería consigo todo el
dominio de escritura de esos paquetes para usar un campo de cada uno.

`internal/recommendation` sí pasa a importar `internal/auth` e `internal/user`, igual que
`internal/jobposition`: son los paquetes de identidad y no de dominio, y la dirección de
dependencias se conserva.

### Autorización en el service y no en un middleware propio

Se sigue el patrón de `internal/jobposition`: `user.AuthenticatedUser` deja el principal en el
contexto, y el service resuelve rol y propiedad. Se descarta replicar el
`AuthenticatedEmployeeMiddleWare` de `internal/employee` porque ese middleware no verifica el
rol del JWT, y acá el rol es parte del contrato: el sentido de la consulta lo determina.

La distinción entre `403` y `404` se hereda de `jobposition`: el sujeto inexistente es `404` y
el ajeno es `403`, con un mensaje que no revela a quién pertenece. Un `employer` sin perfil de
empleador creado también es `403`: no puede operar, y distinguirlo filtraría información.

### `limit` y `offset` en vez de cursor opaco

El conjunto vigente es inmutable entre batches: no hay inserciones intercaladas que corran la
ventana, que es el problema que un cursor resuelve. Un reemplazo de batch invalida la
paginación en curso, pero también invalidaría un cursor. Con `LIMIT/OFFSET` ya implementado y
apoyado en el índice `(batch_id, score)`, el cursor solo agregaría keyset sobre tres columnas,
consultas nuevas y codificación, sin beneficio observable.

Los valores fuera de rango responden `400` en vez de recortarse en silencio: un `limit=1000`
recortado a 100 le hace creer al cliente que llegó al final del conjunto.

### `none` como estado de transporte, fuera del enum de batch

`recommendation.BatchStatus` replica exactamente el check de la migración 0007 y no debe ganar
un valor que la base no conoce. El estado del transporte HTTP es un tipo aparte que incluye los
cuatro estados de batch más `none`, que traduce `ErrNoCurrentBatch`.

Alternativas descartadas: responder `pending` —miente sobre trabajo que nadie encoló— y
responder `404` —obliga al frontend a tratar como error el caso más común de un usuario nuevo—.

### `total` atado al mismo batch que los items

El total es una consulta `COUNT` separada, con los mismos joins y filtros que la lista. Para que
total y tramo no puedan describir conjuntos distintos, el repositorio resuelve el último batch
completado **una sola vez** y pasa ese identificador a las dos consultas. Sin transacción: las
dos consultas están ancladas al mismo `batch_id`, y un batch nuevo que lo reemplace entre ambas
lo borra, con lo que el conteo devuelve cero y la lista queda vacía —coherente entre sí, aunque
desactualizado respecto del conjunto nuevo—.

## Risks / Trade-offs

- **Un reemplazo de batch a mitad de una paginación desplaza los items** → El `status` que
  acompaña a cada página informa que hubo una generación posterior, y el conjunto vigente de un
  sujeto es corto por naturaleza. No se agrega versionado de páginas.
- **`total` y `items` son dos consultas y no una transacción** → Se mitiga anclando ambas al
  mismo `batch_id` ya resuelto; el peor caso observable es un conjunto vacío con total cero, no
  una inconsistencia entre ambos.
- **Duplicación de la resolución de propiedad respecto de `employee` y `jobposition`** → Es
  duplicación de dos consultas SQL, no de reglas: la regla de autorización vive una sola vez en
  el service de recomendaciones. Un test de integración contra el esquema real protege las
  consultas.
- **Con `scoring.Unavailable` cableado, todo batch con candidatos termina en `failed`** → Es el
  comportamiento pedido por la épica, no un defecto de este cambio: las rutas devolverán
  `failed` hasta que exista el algoritmo de indicadores, y los tests cubren los cinco estados
  con datos preparados en la base.

## Migration Plan

Sin migración de base ni de datos. Las rutas son nuevas y ningún contrato existente cambia, así
que el despliegue es aditivo y el rollback es revertir el binario.
