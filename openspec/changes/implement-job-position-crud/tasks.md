## 1. Modelos, errores y helper de respuesta

- [x] 1.1 Crear `internal/jobposition/models.go` con `CreateJobPositionRequest`,
  `UpdateJobPositionRequest` y `JobPosition` (JSON `snake_case`), tags `validate` para
  los campos obligatorios y los enums de experiencia, educación y rango 1..8, y
  `Normalize()` que recorta espacios y convierte `technical_resources` nulo en lista
  vacía; verificar con un test de tabla sobre `Normalize` y sobre el unmarshal de
  `technical_resources: null`.
- [x] 1.2 Crear `internal/jobposition/errors.go` con los sentinelas
  `ErrJobPositionNotFound`, `ErrJobPositionForbidden`, `ErrEmployerProfileRequired`,
  `ErrInvalidJSON`, `ErrInvalidJobPositionID`, `ErrInvalidTimezone` y los internos de
  cada operación; verificar que compila con `go build ./...`.
- [x] 1.3 Agregar a `internal/` un helper que responda los errores de validación como
  JSON `{"error":"..."}` reusando el texto agregado de `validator.ValidationErrors`, sin
  modificar `PrintValidatorError`; verificar con un test que un `ValidationErrors`
  produce `400`, `Content-Type: application/json` y un cuerpo con clave `error`.

## 2. Store y repositorio

- [x] 2.1 Definir `JobPositionStore` en `internal/jobposition/store.go` con
  `GetEmployerIDByUserID`, `CreateJobPosition`, `GetActiveJobPositionByID`,
  `ListActiveJobPositionsByEmployer`, `UpdateActiveJobPosition` y
  `SoftDeleteJobPosition`; verificar que `go build ./...` pasa.
- [x] 2.2 Implementar `internal/jobposition/repo.go` sobre las queries sqlc de LAB-24,
  con mapeo `database.JobPosition -> JobPosition`, traducción de `sql.ErrNoRows` a
  `ErrJobPositionNotFound` y de `ErrEmployerNotFound` a `ErrEmployerProfileRequired`;
  verificar con un test de mapeo usando un `employerQueries` falso, como en
  `internal/employer/repo_test.go`.
- [x] 2.3 Validar `timezone` en el repositorio contra `pg_timezone_names` replicando
  `validateTimezone` de `internal/employee/repo.go`, devolviendo `ErrInvalidTimezone`;
  verificar que la función se invoca en el alta y en la edición y que el error se
  propaga sin envolver detalles de PostgreSQL.

## 3. Puerto de recomendaciones

- [x] 3.1 Crear `internal/jobposition/events.go` con la interfaz
  `JobPositionEventPublisher` (`JobPositionPublished(ctx, JobPosition) error`) y una
  implementación `NoopEventPublisher`; verificar que el no-op devuelve `nil` en un test.

## 4. Servicio: autorización, ownership y orquestación

- [x] 4.1 Implementar `internal/jobposition/service.go` con `NewService(store, publisher)`
  y las cinco operaciones, resolviendo el empleador con
  `AuthorizeProfileRole(principal.Role, user.UserRoleEmployer)` seguido de
  `GetEmployerIDByUserID(principal.UserID)`; verificar con tests que un principal
  `employee` obtiene `user.ErrProfileRoleForbidden` y uno sin perfil
  `ErrEmployerProfileRequired`, sin tocar el store de puestos.
- [x] 4.2 Contrastar el `employerID` del path contra el empleador derivado del JWT en
  las operaciones de colección y el `EmployerID` del puesto cargado en las operaciones
  individuales, devolviendo `ErrJobPositionForbidden`; verificar con tests de un puesto
  ajeno en get, update y delete que el store de escritura nunca se invoca.
- [x] 4.3 Invocar `JobPositionPublished` solo tras un alta o edición persistida con
  éxito, registrando y descartando el error del publicador; verificar con un publicador
  falso que cuenta llamadas: una en alta válida, una en edición válida, cero cuando la
  operación es rechazada, y que un publicador que falla no altera el resultado.
- [x] 4.4 Garantizar que el listado devuelve siempre un slice no nulo y que get, update
  y delete sobre un puesto eliminado devuelven `ErrJobPositionNotFound`; verificar con
  tests de servicio sobre un store falso.

## 5. Handler y rutas

- [x] 5.1 Implementar `internal/jobposition/handler.go` con `NewHandler`,
  `BuildHandlers` y `RegisterRoutes` registrando `POST|GET /employers/{employerID}/jobs`
  y `GET|PUT|DELETE /jobs/{jobPositionID}` detrás de `user.AuthenticatedUser`;
  verificar con un test que cada ruta sin `Authorization` responde `401`.
- [x] 5.2 Decodificar el cuerpo rechazando JSON malformado y contenido extra tras el
  objeto (patrón de `employer.CreateEmployer`), normalizar, validar y responder los
  errores de validación con el helper JSON de 1.3; verificar con tests de handler que
  cuerpo inválido, enum fuera de dominio, horas fuera de rango y campo faltante
  responden `400` con cuerpo `{"error":...}`.
- [x] 5.3 Traducir los sentinelas a status: `403` para rol o ownership y para perfil de
  empleador ausente, `404` para puesto inexistente o eliminado, `400` para identificador
  de path inválido y timezone desconocida, `500` genérico para el resto; verificar con
  un test de tabla que cubre cada mapeo.
- [x] 5.4 Responder `201` con el puesto creado, `200` en listado, consulta y edición, y
  `204` sin cuerpo en la eliminación; verificar con tests de handler que comprueban
  status, `Content-Type` y la forma `snake_case` del JSON.

## 6. Composición

- [x] 6.1 Cablear en `cmd/api.go` el repositorio de puestos, el `NoopEventPublisher` y
  `jobposition.RegisterRoutes` dentro de `mountFeatureRoutes`; verificar con
  `go build ./...` y arrancando el servidor con `make build`.
- [x] 6.2 Actualizar el `doc.go` de `internal/jobposition` para que describa el paquete
  ya completo en vez de anunciar que los handlers llegan después; verificar leyendo el
  archivo.

## 7. Validación y cierre

- [x] 7.1 Ejecutar `gofmt -l .`, `go vet ./...` y `go test ./...` y dejar las tres
  salidas limpias.
- [x] 7.2 Comprobar uno por uno los criterios de aceptación de LAB-25 contra los tests y
  el código: preparación del productor sin acoplarse a SQS, exclusión inmediata al
  eliminar, listado sin eliminados, errores y status consistentes, y tests de handlers y
  servicios en verde.
- [x] 7.3 Actualizar `../docs/create-job-position.md` reemplazando
  `POST api/employer/jobs` por `POST /employers/{employerID}/jobs` y describiendo el
  resto de las rutas del CRUD; verificar que el diagrama Mermaid sigue siendo válido.
  Este archivo vive en la carpeta suelta `docs/` de la raíz `laburi.to/`, sin Git
  propio: no entra en ningún commit de este repositorio.

## 8. Actualización de la documentación del repositorio

- [x] 8.1 Agregar las cinco rutas nuevas a la sección "Contrato HTTP actual" de
  `CLAUDE.md` y `AGENTS.md`, indicando que `jobposition` responde los errores de
  validación en JSON; verificar que ambos archivos quedan alineados entre sí.
