## Why

LAB-25 dejó `internal/jobposition` con pruebas de modelos, repositorio, servicio y handler, pero
los datos de prueba se construyen con literales sin overrides definidos en `repo_test.go`, no
existe una factory equivalente a las de `internal/user` e `internal/employer` (LAB-23), y varias
garantías del dominio solo se ejercitan con un único valor de cada enum. LAB-28 necesita fixtures
reutilizables y una suite que cubra explícitamente enums, recursos técnicos, autorización,
ownership, ciclo de vida y notificación, sin SQS ni servicios externos.

## What Changes

- Incorporar factories package-local con opciones funcionales en `internal/jobposition` para
  `CreateJobPositionRequest`, `JobPosition`, `database.JobPosition` y el principal autenticado,
  con defaults válidos, timestamps deterministas y clonación de slices.
- Permitir overrides explícitos de cada valor de `required_experience` y
  `required_education_level`, de las horas disponibles y de recursos técnicos nulos, vacíos o
  múltiples, incluida la construcción intencional de casos inválidos.
- Mover los helpers actuales de `repo_test.go` a un archivo de factories y reutilizarlos en
  `models_test.go`, `repo_test.go`, `service_test.go` y `handler_test.go` sin ocultar assertions.
- Completar la cobertura de validación: aceptación de los cinco valores de experiencia, los
  cuatro niveles educativos y los límites válidos de horas (1 y 8), junto a los rechazos ya
  cubiertos.
- Completar la cobertura de ciclo de vida: edición de un puesto eliminado responde `404` sin
  escribir ni notificar, y un cuerpo que intente reasignar el empleador no altera la atribución
  derivada del JWT.
- Formalizar el doble del productor: un publicador fake registra las notificaciones y un test
  documenta que el binario se arma con `NoopEventPublisher`, de modo que la suite nunca
  necesita SQS, red ni credenciales.
- Ejecutar pruebas enfocadas, `gofmt`, `go vet ./...` y `go test ./...` como validación final.

## Capabilities

### New Capabilities

Ninguna. El cambio agrega infraestructura y cobertura de pruebas sobre requisitos existentes.

### Modified Capabilities

Ninguna. Las garantías ya están definidas en `job-position-api` y `job-position-persistence`; no
cambia ninguna ruta, payload ni regla de dominio.

## Impact

- Archivos `*_test.go` de `internal/jobposition`, más un nuevo `factories_test.go`.
- Sin cambios en código de producción, rutas, migraciones, queries sqlc ni frontend.
- Sin nuevas dependencias, credenciales, conexiones de red, base de datos, AWS ni SQS.
