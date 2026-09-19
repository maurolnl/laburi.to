## Context

Ver `proposal.md` — Why. El esquema, las queries sqlc y `database.JobPosition` ya
existen desde LAB-24; `internal/jobposition` solo tiene `doc.go` y la prueba de
contrato de la migración.

Restricciones que condicionan el diseño:

- El flujo obligatorio del repositorio es `route -> middleware -> handler -> service ->
  store/repository`. `internal/employer` es el precedente más cercano y reciente.
- El JWT transporta `UserID` y `Role`; `user.AuthenticatedUser` inyecta el principal en
  el contexto y `user.AuthorizeProfileRole` autoriza el tipo de perfil. No existe
  ningún claim con el `employerID`.
- Las queries de LAB-24 son `GetActiveJobPositionByID`,
  `ListActiveJobPositionsByEmployer`, `UpdateActiveJobPosition` y
  `SoftDeleteJobPosition`: filtran por `id` y por `employer_id`, pero ninguna filtra
  por ambos a la vez.
- `internal.PrintValidatorError` responde texto plano vía `http.Error`, formato que el
  `MutationCache.onError` del frontend no sabe interpretar.
- La épica de recomendaciones y su infraestructura de cola no existen todavía.

## Goals / Non-Goals

**Goals:**

- Un único punto de verdad para la identidad del empleador: el JWT.
- Verificación de ownership imposible de saltear desde el handler, ubicada en la capa
  de servicio.
- Reutilizar las queries de LAB-24 sin migraciones nuevas.
- Dejar la costura de recomendaciones verificable con un fake, sin arrastrar SQS.

**Non-Goals:**

- No se implementa el productor real ni ninguna integración con colas.
- No se toca `internal.PrintValidatorError` ni los endpoints que ya lo usan.
- No se agregan paginación, filtros ni ordenamiento configurable al listado.
- No se elimina ninguna recomendación: la tabla todavía no existe. La exclusión
  inmediata se logra porque todo consumo futuro parte de las queries activas.
- No se implementa el frontend (LAB-26 y LAB-27).

## Decisions

### Resolver el empleador desde el JWT en el servicio, no en el handler

El servicio recibe `auth.Principal`, llama a `AuthorizeProfileRole(role, employer)` y
luego resuelve el empleador con `GetEmployerByUserID(principal.UserID)`. El
`employerID` del path llega al servicio solo como valor a contrastar.

Alternativas descartadas:

- *Confiar en el `employerID` del path*: rompe la regla del repositorio de derivar
  identidad del JWT y convierte un path enumerable en un acceso directo a datos ajenos.
- *Agregar el `employerID` como claim del JWT*: el token se emite en el login, cuando el
  perfil de empleador puede no existir todavía, y quedaría desactualizado al crearse.
  Además obligaría a modificar `role-aware-authentication`.

El costo es una consulta extra a `employers` por petición. Es aceptable: el endpoint no
es de alto volumen y la alternativa es un agujero de autorización.

### Un usuario con rol `employer` sin perfil responde `403`, no `404`

`GetEmployerByUserID` devuelve `ErrEmployerNotFound`. Desde la perspectiva de estos
endpoints el principal simplemente no está autorizado a operar sobre puestos, y un
`404` sobre una colección que el cliente sí solicitó resultaría ambiguo respecto del
`404` de un puesto inexistente. Se traduce a `403`.

### Ownership verificado en la aplicación, no agregando queries sqlc

Para `GET|PUT|DELETE /jobs/{jobPositionID}` el servicio primero hace
`GetActiveJobPositionByID`, compara `EmployerID` contra el empleador del principal y
recién entonces actualiza o elimina.

Alternativa descartada: agregar queries `...ByIDAndEmployer` a `sql/queries/`. Sería
una consulta menos, pero colapsa "no existe" y "es de otro" en el mismo resultado
vacío, lo que impide distinguir `404` de `403` y contradice los escenarios del spec.
También exigiría una migración de queries y `sqlc generate` en un ticket que la épica
definió como puramente de aplicación.

La condición de carrera entre el `SELECT` y el `UPDATE`/`DELETE` es benigna: ambas
queries llevan `AND deleted_at IS NULL`, así que una eliminación concurrente degrada a
`sql.ErrNoRows` y el handler responde `404`. No se abre transacción: cada operación
toca una sola tabla, y la regla de atomicidad del repositorio aplica a operaciones
multi-tabla.

### `JobPositionEventPublisher` como puerto con implementación no-op

```go
type JobPositionEventPublisher interface {
    JobPositionPublished(ctx context.Context, position JobPosition) error
}
```

El servicio lo invoca tras persistir un alta o una edición exitosa. `cmd/api.go` cablea
un `NoopEventPublisher`. Un error del publicador se registra y se descarta: el puesto ya
está persistido y devolver `500` haría que el cliente reintente un alta que sí ocurrió.

Alternativas descartadas:

- *Solo un comentario en el servicio*: no deja nada verificable para el criterio de
  aceptación del ticket.
- *Publicar en una goroutine*: el repositorio prohíbe goroutines sin lifecycle, y el
  no-op actual no justifica asincronía. La implementación real de la épica de
  recomendaciones decidirá su propio modelo de concurrencia detrás de esta interfaz.

### Errores de validación en JSON solo en este paquete

Se agrega a `internal/` un helper que emite `{"error":"..."}` con el mismo texto
agregado que hoy produce `PrintValidatorError`. `PrintValidatorError` no se modifica.

El repositorio no tiene un formato de error único y estandarizarlo entero es un cambio
de contrato que afectaría a todos los endpoints vigentes y a sus tests de frontend. Este
cambio introduce rutas nuevas, sin consumidores previos: es el único lugar donde adoptar
el formato correcto no rompe nada. La inconsistencia resultante es transitoria y queda
documentada en el spec.

### Estructura del paquete

`internal/jobposition/`: `models.go` (DTOs, `JobPosition`, `Normalize`), `errors.go`
(sentinelas), `store.go` (interfaz), `repo.go` (adaptador sqlc y mapeo),
`service.go` (autorización, ownership, orquestación del publicador), `events.go`
(puerto y no-op) y `handler.go` (`RegisterRoutes` + adaptación HTTP). Espeja
`internal/employer` para que el paquete se lea igual que el resto del código.

El store del paquete expone también `GetEmployerIDByUserID`, implementado con la query
`GetEmployerByUserID` ya generada. Así `jobposition` no depende del paquete `employer`
y el servicio se testea con un único store falso.

### Validación de `timezone`

Se replica el precedente de `internal/employee/repo.go`: consulta directa contra
`pg_timezone_names` desde el repositorio. `CLAUDE.md` nombra explícitamente esa
validación como el caso sancionado de SQL directo. Mantenerla en el repositorio la deja
fuera de los tests de servicio, que usan un store falso.

## Risks / Trade-offs

- **Tres consultas en el peor caso de una edición** (empleador, puesto, update) →
  Aceptable para el volumen esperado; todas son lookups por clave primaria o por índice
  único. Si alguna vez pesa, se resuelve con queries compuestas, no cambiando el modelo
  de autorización.
- **Formato de error divergente entre `jobposition` y el resto del backend** →
  Documentado en el spec y visible en la propuesta; el camino de convergencia es migrar
  los endpoints antiguos a JSON, no volver atrás este cambio.
- **`ErrEmployerNotFound` traducido a `403`** puede confundir a un empleador que aún no
  creó su perfil → El frontend ya distingue ese estado con `GET /users/{userID}/employer`
  y tiene guards para el onboarding del empleador.
- **La exclusión de "recomendaciones vigentes" no se puede verificar hoy** → No existe
  tabla de recomendaciones. El spec lo expresa como obligación sobre el consumo futuro;
  la épica de recomendaciones deberá partir de las queries activas.
- **Un fallo silencioso del publicador no-op es invisible** → Se registra el error; con
  la implementación real, la épica de recomendaciones definirá reintentos y
  observabilidad.

## Migration Plan

No aplica: no hay migraciones de base de datos ni cambios de datos. El despliegue solo
agrega rutas nuevas; el rollback es revertir el commit.
