## 1. Factories de test

- [ ] 1.1 Crear `internal/jobposition/factories_test.go` con `testJobPositionData`, defaults
  válidos, opciones funcionales por campo y constructores de `CreateJobPositionRequest`,
  `JobPosition`, `database.JobPosition` y `auth.Principal`; verificar con un test directo que
  los defaults son los esperados y que cada override se refleja en el objeto construido.
- [ ] 1.2 Garantizar datos independientes: clonar toda slice recibida por override y devuelta,
  y mantener timestamps deterministas; verificar con un test que mutar una instancia o la
  slice original del llamador no altera otra instancia construida.
- [ ] 1.3 Mover a las factories los dobles y helpers compartidos (`fakeJobPositionStore`,
  `fakePublisher`, principales de empleador y empleado, token de prueba) y declarar las
  constantes del dominio de experiencia, nivel educativo y límites de horas; verificar con
  `go build ./...` y `go test ./internal/jobposition` que el paquete compila y pasa.

## 2. Reutilización en las pruebas existentes

- [ ] 2.1 Migrar `repo_test.go` a las factories, reemplazando los literales de parámetros y
  filas por construcciones con overrides, sin cambiar assertions; verificar con
  `go test ./internal/jobposition -run Repository`.
- [ ] 2.2 Migrar `service_test.go` y `handler_test.go` a las factories, incluido el cuerpo JSON
  válido derivado de la request de factory; verificar con
  `go test ./internal/jobposition -run 'Service|Handler|Routes'`.

## 3. Cobertura de validación y dominio

- [ ] 3.1 Recorrer desde el borde HTTP los cinco valores de `required_experience` y los cuatro
  de `required_education_level` aceptados, más los rechazos ya existentes; verificar que cada
  valor del dominio responde `201`/`200` y que un valor fuera del dominio responde `400` con
  el contrato JSON `{"error":...}`.
- [ ] 3.2 Cubrir los límites válidos de horas disponibles (`1` y `8`) junto a los inválidos
  (`0` y `9`); verificar el status esperado en alta y edición, y que el servicio no se alcanza
  cuando la validación falla.
- [ ] 3.3 Cubrir recursos técnicos nulos, omitidos, vacíos y múltiples atravesando handler,
  servicio y repositorio con overrides de factory; verificar que el store recibe siempre una
  slice no nula y que el orden y contenido de varios recursos se conservan en los parámetros
  tipados de la query.

## 4. Cobertura de autorización, ownership y ciclo de vida

- [ ] 4.1 Confirmar y, donde falte, completar los escenarios de usuario sin perfil de
  empleador, colección ajena y puesto ajeno en las cinco operaciones; verificar que el error
  es el sentinela correcto, que el handler responde `403` con mensaje opaco `forbidden` y que
  no se ejecuta ninguna escritura.
- [ ] 4.2 Cubrir la ausencia de reapertura a nivel HTTP: `PUT` y `DELETE` sobre un puesto
  eliminado responden `404` sin escribir ni notificar; verificar el status, el contrato de
  error y que el publicador fake no registró notificaciones.
- [ ] 4.3 Cubrir que la atribución del puesto se deriva siempre del JWT y no del cuerpo:
  verificar que un cuerpo que intenta fijar `employer_id` o `id` no cambia el empleador
  recibido por el servicio ni los parámetros enviados al store.

## 5. Publicación y ausencia de dependencias externas

- [ ] 5.1 Verificar publicación inmediata tras alta y edición con el publicador fake, ausencia
  de notificación tras eliminación o rechazo, y que un fallo del publicador no altera la
  respuesta ni el resultado persistido.
- [ ] 5.2 Verificar que el puerto `JobPositionEventPublisher` se satisface con dobles y con
  `NoopEventPublisher`, y que ninguna prueba del paquete requiere SQS, red, credenciales,
  `DB_URL` ni variables de entorno.

## 6. Validación final

- [ ] 6.1 Ejecutar `gofmt -l` sobre los archivos modificados, `go vet ./...` y `go test ./...`;
  comprobar explícitamente cada criterio de aceptación de LAB-28: overrides para cada enum y
  para recursos técnicos nulos o múltiples, cobertura de usuario sin perfil de empleador,
  puesto ajeno y puesto eliminado, publicación inmediata sin reapertura, productor sustituido
  por un fake y suite verde.
