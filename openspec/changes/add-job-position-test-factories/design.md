## Context

Ver `proposal.md` — Why. El estado actual de `internal/jobposition` es:

- `repo_test.go` concentra los helpers `newTestDatabaseJobPosition`, `newTestJobPosition` y
  `newTestCreateJobPositionRequest`, sin parámetros ni overrides, junto al doble de
  `jobPositionQueries` y a las pruebas del repositorio.
- `service_test.go` define `fakeJobPositionStore`, `fakePublisher` y los principales
  `employerPrincipal`/`employeePrincipal`, y ya cubre rol incorrecto, perfil de empleador
  ausente, colección ajena, puesto ajeno, puesto eliminado y notificación del publicador.
- `handler_test.go` define `fakeJobPositionService`, el token de empleador y `validBody()`,
  y cubre autenticación, IDs de path inválidos, cuerpos inválidos, mapeo de errores y
  respuestas exitosas.

Las restricciones son las mismas de LAB-23: la suite debe correr sin `DB_URL`, Docker, AWS,
SQS, red ni credenciales. `cmd/api.go:75` arma el handler con `jobposition.NoopEventPublisher{}`,
así que tampoco hay una dependencia real de cola en producción todavía.

## Goals / Non-Goals

**Goals:**

- Unificar la construcción de datos de `internal/jobposition` en factories con opciones
  funcionales, con la misma forma que `internal/employer/factories_test.go`.
- Hacer que cada escenario declare solo los campos que difieren del caso válido.
- Cubrir el dominio completo de los enums y de las horas, en vez de un representante por enum.
- Cerrar los escenarios de `job-position-api` que hoy no tienen prueba directa: edición de un
  puesto eliminado a nivel HTTP e intento de reasignar el empleador.
- Dejar explícito que el productor de recomendaciones es un doble y que la suite no toca SQS.

**Non-Goals:**

- Cambiar producción: rutas, modelos, migraciones, queries sqlc o reglas de dominio.
- Introducir PostgreSQL embebido, testcontainers o cualquier prueba de integración real.
- Crear un paquete público de fixtures importable desde el binario.
- Reescribir las pruebas que ya cubren correctamente un requisito; solo migran su preparación
  de datos.

## Decisions

### Un único `factories_test.go` package-local

Las factories viven en `internal/jobposition/factories_test.go`, en el mismo paquete, siguiendo
el precedente de `internal/user` e `internal/employer`. Se descarta un paquete compartido
`internal/testfactory` porque los tipos son package-local y un paquete común generaría un
ciclo de importación con los paquetes bajo prueba, además de arrastrar soporte de test al
binario. Se descarta también reutilizar las factories de `internal/employer`: `Employer` y
`JobPosition` viven en paquetes distintos y el puesto solo necesita el `employerID`.

Los helpers actuales de `repo_test.go` se mueven a ese archivo y pasan a aceptar opciones, de
modo que las llamadas sin argumentos existentes siguen compilando sin cambios.

### Opciones funcionales sobre un struct de datos intermedio

Se replica la forma de `internal/employer`: un `testJobPositionData` con todos los campos, un
`defaultTestJobPositionData()` válido, opciones `withTestJobPosition*` y constructores que
proyectan ese struct a `CreateJobPositionRequest`, `JobPosition`, `database.JobPosition` y
`auth.Principal`.

Se elige esta forma frente a literales parcialmente rellenados porque permite describir un
escenario inválido como un override visible sobre un caso válido, y frente a una factory
genérica basada en `map[string]any` porque se perderían los tipos del dominio.

### Datos nuevos y slices clonadas en cada invocación

Cada llamada construye structs y slices nuevos, y toda slice recibida por override se clona
con `slices.Clone` antes de guardarse y antes de devolverse. Los timestamps siguen siendo
deterministas (`testJobPositionTime`), sin reloj global ni valores aleatorios: un fallo debe
ser reproducible sin depender del momento de ejecución.

Un test directo de las factories comprueba defaults, overrides, independencia entre instancias
y que una slice mutada por el llamador no afecte al objeto construido.

### Recorrer el dominio de los enums desde el borde HTTP

La aceptación de los cinco valores de `required_experience`, los cuatro de
`required_education_level` y los límites `1` y `8` de horas se verifica en `handler_test.go`,
porque el `validator` es quien impone esas reglas y solo actúa en `decodeRequest`. Las tablas
se generan a partir de las constantes de la factory para que agregar un valor al dominio
obligue a tocar un único lugar.

Se descarta verificarlo en el servicio: el servicio no valida el dominio, y hacerlo allí daría
una falsa sensación de cobertura.

### Recursos técnicos: nulo, vacío y múltiple en las tres capas

`models_test.go` ya cubre `null`, omitido, vacío y valores en `Normalize` y en el unmarshal.
Se completa con overrides de factory que atraviesen handler → servicio → repositorio: un alta
sin recursos llega al store con una slice vacía no nula, y un alta con varios recursos
conserva orden y contenido en los parámetros tipados de la query.

### Ciclo de vida: sin reapertura, sin reasignación de empleador

- `service_test.go` ya verifica que `get`, `update` y `delete` sobre un puesto eliminado
  devuelven `ErrJobPositionNotFound` sin escribir. Se agrega la contraparte HTTP: `PUT` sobre
  un puesto eliminado responde `404` y el publicador no recibe notificación.
- Se agrega un test de que un cuerpo con campos ajenos al contrato (por ejemplo `employer_id`
  o `id`) es rechazado como JSON con contenido no esperado o, en su defecto, no altera el
  `employerID` derivado del JWT. La atribución siempre proviene del principal.

### El productor de recomendaciones es siempre un doble

`fakePublisher` se mueve a las factories y registra las notificaciones recibidas. Se agrega un
test que ejerce `NoopEventPublisher` como implementación por defecto del puerto, documentando
que el binario actual no habla con ninguna cola. Ninguna prueba abre sockets ni lee entorno.

## Risks / Trade-offs

- [Mover helpers entre archivos puede alterar en silencio el alcance de un test existente] →
  Migrar solo la preparación de datos, conservar assertions literales y correr primero
  `go test ./internal/jobposition` antes de la suite completa.
- [Las opciones funcionales pueden esconder defaults relevantes] → Nombres de opción
  específicos por campo y un test dedicado a defaults, overrides e independencia.
- [Recorrer el dominio de los enums desde tablas generadas puede ocultar un valor eliminado del
  `validate:"oneof=..."`] → Declarar las constantes del dominio en la factory y afirmar la
  cantidad esperada de valores, de modo que quitar uno haga fallar el test.
- [Completar cobertura puede revelar un defecto real de producción] → Detener el apply y volver
  a revisión antes de tocar código de producto; LAB-28 es un cambio exclusivo de test.

## Migration Plan

1. Crear `factories_test.go` con las factories y su test directo, sin tocar el resto.
2. Migrar `repo_test.go`, `service_test.go` y `handler_test.go` a las factories.
3. Agregar la cobertura faltante de enums, horas, recursos técnicos, ciclo de vida y productor.
4. Ejecutar `gofmt`, `go vet ./...`, pruebas enfocadas y `go test ./...`.

No hay despliegue ni datos involucrados. El rollback consiste en revertir el commit de tests.
