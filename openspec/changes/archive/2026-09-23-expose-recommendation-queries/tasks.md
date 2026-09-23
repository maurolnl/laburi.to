## 1. Propiedad del sujeto en la persistencia

- [x] 1.1 Agregar a `sql/queries/recommendations.sql` las consultas `GetEmployeeOwner :one`
  —usuario dueño de un empleado— y `GetJobPositionOwner :one` —empleador y usuario dueños de un
  puesto con `deleted_at IS NULL`—; documentar en un comentario por qué el puesto eliminado es
  inexistente a efectos de autorización.
- [x] 1.2 Ejecutar `sqlc generate` y revisar la salida en `internal/database` sin editarla a
  mano; verificar con `go build ./...`.
- [x] 1.3 Declarar en `internal/recommendation/store.go` el puerto `SubjectOwnership` con
  `EmployeeOwner(ctx, employeeID int32) (int32, error)` y
  `JobPositionOwner(ctx, jobPositionID int32) (JobPositionOwner, error)`, más el tipo
  `JobPositionOwner{EmployerID, UserID int32}`; documentar que ambos devuelven
  `ErrSubjectNotFound` cuando el sujeto no existe o está eliminado.
- [x] 1.4 Implementar los dos métodos en `internal/recommendation/repo.go` sobre las consultas
  generadas, traduciendo `sql.ErrNoRows` a `ErrSubjectNotFound`, y sumar la aserción de
  interfaz `var _ SubjectOwnership = (*RecommendationRepository)(nil)`; verificar con
  `go build ./...`.
- [x] 1.5 Cubrir con un test de integración sobre `testsupport.PostgresTx` los cuatro escenarios
  del requisito «Resolución de la propiedad de un sujeto»: empleado existente, empleado
  inexistente, puesto activo y puesto eliminado lógicamente; verificar con
  `go test ./internal/recommendation`.

## 2. Tamaño total del conjunto vigente

- [x] 2.1 Agregar a `sql/queries/recommendations.sql` las consultas
  `CountJobRecommendationsForEmployee :one` y `CountEmployeeRecommendationsForJobPosition :one`,
  con exactamente los mismos joins y filtros que sus listados, incluida la exclusión de puestos
  eliminados; ejecutar `sqlc generate` y verificar con `go build ./...`.
- [x] 2.2 Agregar el campo `Total int32` a `JobRecommendations` y `EmployeeRecommendations` en
  `internal/recommendation/store.go`, documentando que se resuelve contra el mismo batch
  completado que los items.
- [x] 2.3 Ajustar `JobRecommendationsForEmployee` y `EmployeeRecommendationsForJobPosition` en
  `internal/recommendation/repo.go` para resolver el último batch completado una sola vez y
  pasar ese identificador tanto al listado como al conteo; verificar con `go build ./...`.
- [x] 2.4 Extender los tests de `internal/recommendation/repo_test.go` con los cuatro escenarios
  del requisito «Tamaño total del conjunto vigente»: total sobre un conjunto paginado, total sin
  puestos eliminados, sujeto sin batch completado y desplazamiento más allá del conjunto;
  verificar con `go test ./internal/recommendation`.

## 3. Paginación validada en el borde

- [x] 3.1 Crear en `internal/recommendation/models.go` el tipo de respuesta de paginación
  `PageInfo{Limit, Offset, Total int32}` con etiquetas JSON `limit`, `offset` y `total`.
- [x] 3.2 Implementar en `internal/recommendation` el parseo de `limit` y `offset` desde la query
  string, con `defaultPageLimit` y `maxPageLimit` declarados como constantes; devolver error
  tipado ante valor no numérico, `limit` menor o igual a cero, `limit` mayor al máximo y
  `offset` negativo, y aplicar los valores por defecto cuando el parámetro está ausente.
- [x] 3.3 Cubrir el parseo con tests de tabla que afirmen cada caso del requisito «Validación de
  límite y desplazamiento», incluido que un valor fuera de rango es error y no un recorte
  silencioso; verificar con `go test ./internal/recommendation`.

## 4. Estado de transporte y errores

- [x] 4.1 Definir en `internal/recommendation/models.go` el tipo `QueryStatus` con los valores
  `none`, `pending`, `processing`, `completed` y `failed`, y la función que traduce un
  `BatchStatus` a `QueryStatus`; documentar por qué `none` no puede vivir en `BatchStatus` —el
  check de la migración 0007 no lo conoce—.
- [x] 4.2 Cubrir con un test que cada `BatchStatus` válido se traduce a su `QueryStatus`
  homónimo y que `ErrNoCurrentBatch` produce `none`; verificar con
  `go test ./internal/recommendation`.
- [x] 4.3 Agregar a `internal/recommendation/errors.go` los sentinelas del borde:
  `ErrInvalidEmployeeID`, `ErrInvalidJobPositionID`, `ErrInvalidPagination` y
  `ErrRecommendationsForbidden`, con sus mensajes en inglés como el resto del paquete. El
  empleador sin perfil creado no necesita sentinela propio: no puede ser dueño de ningún puesto,
  así que ya cae en `ErrRecommendationsForbidden`.

## 5. Service de consulta

- [x] 5.1 Crear `internal/recommendation/service.go` con la interfaz `QueryService`
  —`JobRecommendations(ctx, employeeID int32, page Page, principal auth.Principal)` y
  `EmployeeRecommendations(ctx, jobPositionID int32, page Page, principal auth.Principal)`— y su
  implementación sobre `RecommendationStore` y `SubjectOwnership`; verificar con
  `go build ./...`.
- [x] 5.2 Implementar la autorización del sentido empleado: `user.AuthorizeProfileRole` contra
  `user.UserRoleEmployee`, resolución del dueño del empleado y comparación contra
  `principal.UserID`, devolviendo `ErrSubjectNotFound` cuando el empleado no existe y
  `ErrRecommendationsForbidden` cuando es ajeno; documentar que el identificador del path solo
  sirve para detectar el acceso ajeno.
- [x] 5.3 Implementar la autorización del sentido puesto: `user.AuthorizeProfileRole` contra
  `user.UserRoleEmployer`, resolución del dueño del puesto activo y comparación contra
  `principal.UserID`, con `ErrSubjectNotFound` para el puesto inexistente o eliminado y
  `ErrRecommendationsForbidden` para el ajeno.
- [x] 5.4 Traducir `ErrNoCurrentBatch` a una respuesta con estado `none`, items vacíos y total
  cero en ambos sentidos, en lugar de propagarlo como error.
- [x] 5.5 Cubrir con tests de servicio y dobles todos los escenarios del requisito «Ownership
  derivado del JWT»: perfil ajeno, puesto ajeno, rol incorrecto en cada sentido, empleador sin
  perfil de empleador, empleado inexistente y puesto eliminado; verificar con
  `go test ./internal/recommendation`.
- [x] 5.6 Cubrir con tests que los cinco estados llegan intactos al llamador y que `items` nunca
  es `nil`, incluidos el completado sin resultados y el fallido sobre un conjunto previo;
  verificar con `go test ./internal/recommendation`.

## 6. Handler y rutas

- [x] 6.1 Crear `internal/recommendation/handler.go` con `QueryHandler`, `NewQueryHandler`,
  `BuildQueryHandlers(store, ownership)` y `RegisterQueryRoutes(mux, handler, secretKey)`,
  registrando `GET /employees/{employeeID}/job-recommendations` y
  `GET /jobs/{jobPositionID}/employee-recommendations` detrás de `user.AuthenticatedUser`;
  verificar con `go build ./...`.
- [x] 6.2 Implementar los dos handlers: principal desde el contexto, identificador del path
  validado como entero positivo, paginación parseada, y respuesta `200` con `status`, `items` y
  `page`; responder siempre en JSON con `internal.RespondWithJSON`.
- [x] 6.3 Implementar la traducción de errores a HTTP: `401` sin principal, `400` para
  identificador o paginación inválidos, `403` para rol incorrecto, sujeto ajeno y empleador sin
  perfil, `404` para sujeto inexistente o eliminado, y `500` por defecto; todos con
  `internal.RespondWithError` en formato `{"error": "..."}` y sin revelar a quién pertenece el
  sujeto.
- [x] 6.4 Cubrir con tests de handler sobre `httptest` y un service falso: los códigos de cada
  caso anterior, la forma exacta del cuerpo de éxito y que `items` serializa como `[]` y no
  `null` cuando está vacío; verificar con `go test ./internal/recommendation`.
- [x] 6.5 Cubrir con un test de recorrido de páginas —dos páginas sucesivas del mismo conjunto—
  que ningún item se repite ni se omite y que el orden global se conserva; verificar con
  `go test ./internal/recommendation`.

## 7. Cableado y verificación de arquitectura

- [x] 7.1 En `cmd/api.go`, construir el handler de consulta sobre
  `recommendation.NewRepository(psqlDB)` y registrar sus rutas dentro de
  `mountFeatureRoutes`; verificar con `go build ./...` y `go vet ./...`.
- [x] 7.2 Verificar que la dirección de dependencias se conserva:
  `go list -deps ./internal/recommendation | grep -E 'internal/(jobposition|employee)'` sin
  coincidencias.
- [x] 7.3 Actualizar `internal/recommendation/doc.go` para declarar el borde de consulta, los
  cinco estados expuestos y la regla de ownership.

## 8. Documentación y validación final

- [x] 8.1 Agregar a `docs/recommendation-queue.md` una sección de consulta: las dos rutas, el
  cuerpo de respuesta, los cinco estados, la regla de paginación con sus límites y la relación
  entre estado —batch más reciente— e items —último batch completado—.
- [x] 8.2 Reescribir `../docs/employee-searching-for-position.md` y
  `../docs/employer-searching-for-employees.md` de la raíz `laburi.to/` con las rutas reales,
  el ownership desde el JWT, los estados y el orden con su desempate, reemplazando el
  `ArrangeRecommendations` que nunca existió por el orden que resuelve la base; dejar asentado
  que esa carpeta no tiene Git propio y sus cambios no se commitean acá.
- [x] 8.3 Actualizar `CLAUDE.md` y `AGENTS.md` del backend: las consultas de recomendaciones
  dejan de ser trabajo pendiente de LAB-35 y pasan a ser endpoints implementados.
- [x] 8.4 Ejecutar `gofmt -l .` sin salida, `go vet ./...` y `go test ./...`.
- [x] 8.5 Comprobar explícitamente cada criterio de aceptación del ticket contra un test o una
  observación concreta, y dejarlo asentado en el commit de implementación.
