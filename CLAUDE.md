# Laburi.to Backend

Espejo de `AGENTS.md` (mismo contenido). Mantener ambos alineados cuando se cambie una
regla.

## Stack y comandos

API Go 1.25 sobre `net/http` (routing con patterns método+ruta), PostgreSQL/libpq,
sqlc, validator v10, JWT, Argon2id y AWS SDK v2 para S3.

```bash
go run ./cmd                    # desarrollo; carga .env si existe
make build                      # bin/laburito
make run                        # build + ejecución
make build-railway              # binario de deploy (out)
go test ./...                   # suite completa
go test ./internal/employer     # paquete concreto
go test ./internal/auth -run TestName
go vet ./...
gofmt -w ruta.go
sqlc generate                   # después de cambiar schema/queries
```

Ejecutar comandos desde esta carpeta. No editar ni versionar `.env`; `.env.example` es
la referencia. Según la feature se requieren `DB_URL`, `SECRET_KEY`, `SPA_URL`,
`AWS_S3_BUCKET` y credenciales/región AWS.

## Arquitectura

- `cmd/`: `config.go`, `api.go` (composición de dependencias y montaje de rutas),
  `main.go`, y `middleware/` global (Logger, CORS).
- `internal/<feature>/handler.go`: `RegisterRoutes` + adaptación HTTP.
- `service.go`: orquestación y lógica de negocio.
- `store.go`: interfaces consumidas por servicios (del lado consumidor, pequeñas).
- `repo.go`: persistencia y transacciones.
- `internal/database/`: salida generada por sqlc; **no editar a mano**.
- `sql/queries/`: consultas fuente de sqlc.
- `sql/schema/`: migraciones PostgreSQL ordenadas e inmutables una vez aplicadas
  (`0001_employees` … `0005_add_user_roles_and_employers`).
- Features: `user` (auth + roles), `employee`, `employer`, `timezone`.
- Transversales: `internal/auth` (JWT, Argon2id, `UserRole`), `internal/files` (PDFs),
  `internal/uploader` (S3), `internal/*.go` (helpers de respuesta y validación).

Mantener el flujo `route -> middleware -> handler -> service -> store/repository`.
Preferir cambios locales a crear capas nuevas.

## Contrato HTTP actual

Públicas:

- `GET /healthz`
- `POST /auth/register` → `200`
- `POST /auth/login` → `202` con `{id, email, role, token, refreshToken}`

Autenticadas con `AuthenticatedUser` (JWT):

- `GET /auth/me` → `200`; serializa el struct `User` **sin json tags** → claves
  `ID`, `Email`, `Role`.
- `GET /timezones` → `200`
- `POST /employees` → `201`
- `GET /users/{userID}/employee` → `200`
- `POST /employers` → `201`
- `GET /users/{userID}/employer` → `200`
- `POST /employers/{employerID}/jobs` → `201` con el puesto publicado
- `GET /employers/{employerID}/jobs` → `200` con los puestos activos del empleador
- `GET /jobs/{jobPositionID}` → `200`
- `PUT /jobs/{jobPositionID}` → `200`
- `DELETE /jobs/{jobPositionID}` → `204` sin cuerpo (soft delete, sin reapertura)

Autenticadas con `AuthenticatedEmployeeMiddleWare` (JWT + propiedad del recurso):

- `PUT /employees/{employeeID}` → `200`
- `POST|PUT /employees/{employeeID}/location`, `/tech`, `/availability`, `/education`
  → `201` en POST, `200` en PUT.

Reglas:

- Registrar rutas con patterns de `net/http` (`"POST /employers"`) en `RegisterRoutes`.
- Extraer el usuario autenticado del contexto (`user.PrincipalFromContext`); nunca
  autorizar solo por path o body.
- El rol del JWT es inmutable y autoriza el tipo de perfil vía
  `user.AuthorizeProfileRole`: rol distinto al perfil → `403`.
- Decodificar según content type y validar DTOs con el validator compartido.
- Conservar los códigos actuales listados arriba.
- No filtrar errores internos, DSN, tokens ni detalles AWS en respuestas nuevas.

### Formato de errores (inconsistente, tenerlo presente)

- `internal.RespondWithError` → JSON `{"error":"..."}`.
- `internal.PrintValidatorError` → **texto plano** con `http.Error`.
- Algunos handlers (`POST /auth/register`) usan `http.Error` directo → texto plano.

- `internal.RespondWithValidatorError` → JSON `{"error":"..."}` para errores de
  validación. Hoy lo usa solo `internal/jobposition`.

El FE solo parsea `{error}` (fallback `{messages}`). En código nuevo usar
`RespondWithError` y `RespondWithValidatorError`: los endpoints de `jobposition`
responden **todos** sus errores en JSON, incluidos los de validación. Migrar los
endpoints viejos es un cambio de contrato: coordinar con el frontend en la misma tarea.

Si cambia un contrato, actualizar tipos/mappers del frontend y la documentación de la
raíz. Los diagramas de `../docs/` contienen diseño futuro: índices, colas y
recomendaciones aún no existen en este código (empleadores desde la migración 0005;
puestos de trabajo desde la 0006 y su CRUD en `internal/jobposition`).

## Persistencia y transacciones

- Todo cambio de DB empieza en una migración nueva y en `sql/queries/`; luego
  `sqlc generate` y revisar lo generado.
- Usar consultas tipadas de `internal/database`; SQL directo solo con razón concreta,
  como la validación de timezone actual.
- Toda operación multi-tabla debe ser atómica: `BeginTx`, `WithTx`, rollback diferido y
  commit al final.
- Propagar `context.Context` desde la petición hacia DB, S3 y servicios.
- Respetar nullability al mapear `sql.Null*` y punteros.

## Autenticación y archivos

- Contraseñas con Argon2id. Nunca texto plano ni volver a bcrypt por seguir un
  documento desactualizado.
- Access token JWT corto (incluye `Role`) y refresh token aleatorio persistido. No
  registrar grants.
- Certificaciones: solo PDF, límite actual de 5 MB, metadata y checksum persistidos.
- Si S3 termina y DB falla, conservar la limpieza compensatoria de archivos huérfanos
  con timeout; no ocultar fallos de cleanup.
- No aceptar bucket ni object key arbitrarios enviados por el cliente.
- CORS permite `http://localhost:5173` y `SPA_URL`; `*` habilita cualquier origen.

## Go y pruebas

- `gofmt` en cada archivo Go modificado.
- Errores sentinela con `errors.Is` cuando haga falta clasificar; envolver con contexto
  útil sin duplicar logs en cada capa.
- Evitar globals mutables, `panic`, goroutines sin lifecycle y `context.Background()`
  salvo tareas compensatorias deliberadamente desacopladas.
- Tests junto al paquete como `*_test.go`; table-driven para validación y lógica pura,
  stores falsos para servicios.
- Hay factories de test compartidas por paquete (`factories_test.go` en `user` y
  `employer`): reutilizarlas antes de escribir setup nuevo.
- Para cambios de auth, roles, ownership, transacciones o uploads, cubrir los caminos
  de error relevantes.

## Finalización

`gofmt`, test del paquete afectado y luego `go test ./...`; sumar `go vet ./...` en
cambios amplios. Revisar la generación de sqlc cuando aplique. No ejecutar migraciones
sobre una DB compartida ni hacer commit, push o deploy sin pedido explícito.

## OpenSpec

Raíz OpenSpec propia en `openspec/`, schema `spec-driven`. Artefactos en español,
headings estructurales y SHALL/MUST en inglés.

Los comandos `/opsx-*` viven en `.opencode/commands/` y **no están disponibles como
slash commands en Claude Code**: usar el CLI `openspec` desde esta carpeta o pedir al
usuario que los corra en opencode.

Flujo: crear aquí la rama con el `gitBranchName` de Linear cuando el backend esté
afectado, generar la propuesta y detenerse para revisión antes de editar código. Tras
aprobación explícita, commitear primero los artefactos del spec y después implementar.
Al finalizar, validar la implementación contra todos los criterios de aceptación,
archivar y validar el cambio OpenSpec, y recién entonces pushear o abrir PR contra la
rama base real (`main` o `master`), incluyendo el archivo OpenSpec en los commits.
