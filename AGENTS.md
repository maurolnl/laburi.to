# Laburi.to Backend

## Stack y comandos

API Go 1.25 basada en `net/http`, PostgreSQL/libpq, sqlc, validator v10, JWT,
Argon2id y AWS SDK v2 para S3.

```bash
go run ./cmd                 # desarrollo; carga .env si existe
make build                   # bin/laburito
make run                     # build + ejecución
go test ./...                # suite completa
go test ./internal/auth      # paquete concreto
go test ./internal/auth -run TestName
go vet ./...
gofmt -w ruta.go
sqlc generate                # después de cambiar schema/queries
```

Ejecutar comandos desde esta carpeta. No editar ni versionar `.env`; tomar
`.env.example` como referencia. La aplicación requiere, según la feature, `DB_URL`,
`SECRET_KEY`, `SPA_URL`, `AWS_S3_BUCKET` y credenciales/región AWS.

## Arquitectura

- `cmd/`: configuración, composición de dependencias, middleware global y servidor.
- `internal/<feature>/handler.go`: registro de rutas y adaptación HTTP.
- handlers CRUD: parseo, validación, status y respuesta.
- `service.go`: orquestación y lógica de negocio.
- `store.go`: interfaces consumidas por servicios.
- `repo.go`: persistencia y transacciones.
- `internal/database/`: salida generada por sqlc; no editar manualmente.
- `sql/queries/`: consultas fuente de sqlc.
- `sql/schema/`: migraciones PostgreSQL ordenadas e inmutables una vez aplicadas.
- `internal/auth`, `files`, `uploader`: JWT/hashing, PDFs y S3.

Mantener el flujo `route -> middleware -> handler -> service -> store/repository`.
Las interfaces deben vivir del lado consumidor y ser pequeñas. Preferir cambios
locales a crear capas nuevas.

## Contrato HTTP actual

Rutas públicas: `GET /healthz`, `POST /auth/register`, `POST /auth/login`.
Rutas autenticadas: `GET /auth/me`, `GET /timezones`, creación/consulta de employee.
Las actualizaciones de `/employees/{employeeID}` y de sus secciones además validan
propiedad mediante `AuthenticatedEmployeeMiddleWare`.

- Usar patterns de Go `net/http` con método y ruta en `RegisterRoutes`.
- Extraer el usuario autenticado del contexto; nunca autorizar solo por path/body.
- Decodificar según content type y validar DTOs con el validator compartido.
- Respuestas JSON usan `internal.RespondWithJSON`; errores JSON tienen
  `{"error":"..."}` cuando se use `RespondWithError`.
- Conservar códigos actuales: POST de employee/secciones devuelve `201`, PUT `200`,
  login `202`, registro `200`.
- No filtrar errores internos, DSN, tokens o detalles AWS en respuestas nuevas.

Si cambia un contrato, actualizar tipos/mappers del frontend y la documentación de
la raíz. Los diagramas de `../docs/` contienen diseño futuro: empleadores, puestos,
índices, colas y recomendaciones aún no existen en este código.

## Persistencia y transacciones

- Cambios de DB comienzan en una migración nueva y en `sql/queries/`; ejecutar
  `sqlc generate` y revisar los archivos generados.
- Usar consultas tipadas de `internal/database`; SQL directo solo cuando sqlc no sea
  adecuado y exista una razón concreta, como la validación de timezone actual.
- Toda operación multi-tabla debe ser atómica con `BeginTx`, `WithTx`, rollback
  diferido y commit al final.
- Propagar `context.Context` desde la petición a DB, S3 y servicios.
- Respetar nullability al mapear `sql.Null*` y punteros.

## Autenticación y archivos

- Contraseñas: Argon2id. Nunca almacenar texto plano ni volver a bcrypt por asumir lo
  indicado en un documento desactualizado.
- Access token JWT corto y refresh token aleatorio persistido. No registrar grants.
- Certificaciones: solo PDF, límite actual de 5 MB y metadata/checksum persistidos.
- Si S3 termina y DB falla, conservar la limpieza compensatoria de archivos huérfanos
  con timeout; no ocultar fallos de cleanup.
- No aceptar bucket ni object key arbitrarios enviados por el cliente.

## Go y pruebas

- Ejecutar `gofmt` en cada archivo Go modificado.
- Errores sentinela con `errors.Is` cuando se necesite clasificación; envolver con
  contexto útil sin duplicar logs en cada capa.
- Evitar globals mutables, `panic`, goroutines sin lifecycle y `context.Background()`
  salvo tareas compensatorias deliberadamente desacopladas.
- Tests junto al paquete como `*_test.go`; preferir table-driven tests para validación
  y lógica pura, y stores falsos para servicios.
- Para cambios de auth, ownership, transacciones o uploads, agregar cobertura de los
  caminos de error relevantes.

## Finalización

Ejecutar `gofmt`, el test del paquete afectado y luego `go test ./...`; añadir
`go vet ./...` para cambios amplios. Revisar generación sqlc cuando aplique. No
ejecutar migraciones sobre una DB compartida ni hacer commit, push o deploy sin pedido
explícito.

## OpenSpec

Este repositorio tiene una raíz OpenSpec propia en `openspec/`, configurada con el
schema `spec-driven`. Los artefactos se escriben en español, conservando headings
estructurales y palabras normativas SHALL/MUST en inglés.

Para tickets iniciados mediante `/start-ticket`, crear aquí la rama indicada por
`gitBranchName` de Linear cuando el backend esté afectado. Ejecutar
`/opsx-propose` desde esta carpeta y detenerse para revisión antes de editar código.
Tras aprobación explícita, commitear primero los artefactos del spec y después
ejecutar `/opsx-apply`. Al finalizar, validar, commitear y abrir un PR contra la rama
base real del repositorio (`main` o `master`). No archivar el cambio sin solicitud.
