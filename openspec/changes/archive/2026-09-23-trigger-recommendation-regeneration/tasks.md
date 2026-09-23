## 1. Adaptador de disparadores en `internal/recommendation`

- [x] 1.1 Crear `internal/recommendation/triggers.go` con el tipo `Trigger`, que envuelve un
  `JobPublisher`, y su constructor `NewTrigger(publisher JobPublisher) Trigger`; documentar por qué
  existe —satisfacer por tipado estructural los puertos de `employee` y `jobposition` sin que
  ningún paquete importe a otro— y por qué no puede vivir en esos paquetes; verificar con
  `go build ./...`.
- [x] 1.2 Implementar `EmployeeProfileCompleted(ctx, employeeID int32) error` y
  `JobPositionPublished(ctx, jobPositionID int32) error`, cada uno traduciendo el identificador al
  `Subject` correspondiente con `NewEmployeeSubject` / `NewJobPositionSubject` y delegando en
  `PublishJob`; verificar con un test que usa un `JobPublisher` falso y afirma el `Subject` exacto
  recibido en cada sentido.
- [x] 1.3 Cubrir con un test que el error del `JobPublisher` se propaga sin envolver información
  extra, para que el borde de escritura pueda registrarlo tal cual.
- [x] 1.4 Verificar que la dirección de dependencias se conserva:
  `go list -deps ./internal/recommendation | grep -E 'internal/(jobposition|employee)'` sin
  coincidencias.

## 2. Puerto angostado en `internal/jobposition`

- [x] 2.1 Cambiar en `internal/jobposition/events.go` la firma de `JobPositionEventPublisher` a
  `JobPositionPublished(ctx context.Context, jobPositionID int32) error`, ajustar
  `NoopEventPublisher` y el helper `publish` para recibir el identificador, y documentar por qué el
  puerto ya no transporta la representación completa; verificar con `go build ./...`.
- [x] 2.2 Ajustar `CreateJobPosition` y `UpdateJobPosition` en `internal/jobposition/service.go`
  para notificar con `position.ID` después de persistir; verificar con `go build ./...`.
- [x] 2.3 Actualizar el doble de `internal/jobposition/service_test.go` a la firma nueva y adaptar
  las aserciones para comparar identificadores; verificar con `go test ./internal/jobposition`.
- [x] 2.4 Sumar tests de los escenarios del requisito modificado que la suite todavía no cubre:
  la eliminación lógica no notifica, y el fallo del publicador deja el alta persistida con su
  código de éxito; verificar con `go test ./internal/jobposition`.
- [x] 2.5 Actualizar `internal/jobposition/doc.go` para describir la notificación por identificador.

## 3. Completitud del perfil en `internal/employee`

- [x] 3.1 Agregar a `sql/queries/employees.sql` la consulta `IsEmployeeProfileComplete :one`, que
  devuelve un booleano combinando la existencia del empleado con `EXISTS` sobre
  `employee_location`, `employee_profile_tech`, `employee_profile_availability` y
  `employee_education`; documentar en un comentario por qué la regla es «la fila existe» y no «la
  sección tiene datos».
- [x] 3.2 Ejecutar `sqlc generate` y revisar la salida en `internal/database`, sin editarla a mano;
  verificar con `go build ./...`.
- [x] 3.3 Agregar `IsProfileComplete(ctx context.Context, employeeID int32) (bool, error)` a
  `EmployeeStore` en `internal/employee/store.go` e implementarlo en
  `internal/employee/repo.go` sobre la consulta generada; verificar con `go build ./...`.
- [x] 3.4 Actualizar los dobles de `internal/employee/fakes_test.go` con el método nuevo, con un
  valor configurable por caso y registro de las invocaciones recibidas; verificar con
  `go test ./internal/employee`.
- [x] 3.5 Cubrir la consulta contra el esquema real con un test de integración sobre
  `testsupport.PostgresTx`: el perfil con las cinco etapas está completo, quitar cada etapa por
  vez lo deja incompleto, la etapa de recursos técnicos sin datos opcionales cuenta igual y un
  empleado inexistente devuelve `false` sin error; verificar con `go test ./internal/employee`.

## 4. Puerto de eventos en `internal/employee`

- [x] 4.1 Crear `internal/employee/events.go` con `EmployeeEventPublisher`
  (`EmployeeProfileCompleted(ctx, employeeID int32) error`), su `NoopEventPublisher` y el helper
  `publish` que registra el error y no lo propaga; documentar por qué el fallo no invalida la
  escritura ya persistida y por qué el puerto solo transporta el identificador; verificar con
  `go build ./...`.
- [x] 4.2 Inyectar el puerto en `employeeService` y en `NewService`/`BuildHandlers`
  (`internal/employee/handler.go`, `service.go`), manteniendo el orden
  `route -> handler -> service -> store`; verificar con `go build ./...`.
- [x] 4.3 Implementar el disparo en las diez escrituras de perfil —`CreateEmployee`,
  `UpdateEmployee`, `CreateLocation`, `UpdateLocation`, `CreateTech`, `UpdateTech`,
  `CreateAvailability`, `UpdateAvailability`, `CreateEducation`, `UpdateEducation`— consultando
  `IsProfileComplete` después de que el repositorio confirme y notificando solo cuando devuelva
  `true`; verificar con `go build ./...`.
- [x] 4.4 Cubrir con tests de servicio, usando productor falso, que cada una de esas escrituras
  notifica exactamente una vez cuando el perfil queda completo, con el identificador correcto;
  verificar con `go test ./internal/employee`.
- [x] 4.5 Cubrir con tests que ninguna de esas escrituras notifica cuando `IsProfileComplete`
  devuelve `false`, y que un fallo del repositorio no notifica ni consulta la completitud;
  verificar con `go test ./internal/employee`.
- [x] 4.6 Cubrir con un test que un fallo del productor deja la escritura persistida y la operación
  resuelta sin error para el llamador; verificar con `go test ./internal/employee`.
- [x] 4.7 Cubrir con un test que un fallo de `IsProfileComplete` no notifica y no convierte la
  escritura en un error, porque el cambio de dominio ya está confirmado; verificar con
  `go test ./internal/employee`.
- [x] 4.8 Actualizar el comentario de paquete de `internal/employee` para declarar la notificación
  tras cada escritura que deja el perfil completo.

## 5. Cableado en `cmd`

- [x] 5.1 En `cmd/api.go`, construir el `recommendation.Trigger` a partir de
  `NewQueueJobPublisher(recommendation.NewRepository(psqlDB), app.queueClient)` cuando
  `app.config.queueCfg.Enabled`, y a partir de `recommendation.NoopJobPublisher{}` cuando no;
  documentar por qué el interruptor decide, con el mismo criterio que `startWorker`; verificar con
  `go build ./...`.
- [x] 5.2 Inyectar ese trigger en `employee.BuildHandlers` y en `jobposition.BuildHandlers`,
  reemplazando `jobposition.NoopEventPublisher{}`; comprobar que `mountQueue` se ejecuta antes que
  `mountFeatureRoutes` para que el cliente ya exista, y ajustar el orden en `main.go` o `mount()` si
  no fuera así; verificar con `go build ./...` y `go vet ./...`.

## 6. Documentación y validación final

- [x] 6.1 Agregar a `docs/recommendation-queue.md` una sección de disparadores: qué operaciones
  emiten, la regla de perfil completo, qué operaciones no emiten y la estrategia ante fallo de
  publicación —batch `failed` como registro durable, sin outbox ni reintento—.
- [x] 6.2 Actualizar `CLAUDE.md` y `AGENTS.md` del backend: `internal/recommendation` ya tiene
  productor cableado y los disparadores dejaron de ser trabajo pendiente de LAB-34.
- [x] 6.3 Ejecutar `gofmt -l .` sin salida, `go vet ./...` y `go test ./...`.
- [x] 6.4 Comprobar explícitamente cada criterio de aceptación del ticket contra un test o una
  observación concreta, y dejarlo asentado en el commit de implementación.
