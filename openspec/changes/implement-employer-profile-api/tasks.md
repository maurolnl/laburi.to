## 1. Contratos y persistencia

- [ ] 1.1 Crear modelos, normalización, validaciones, errores sentinela e interfaces consumidoras en `internal/employer`; verificar con tests table-driven campos obligatorios, espacios, modalidades libres y conversión de modalidades omitidas a `[]string{}`.
- [ ] 1.2 Implementar el repositorio sobre `CreateEmployer` y `GetEmployerByUserID`, traduciendo ausencia, unicidad y exclusividad a errores de dominio; verificar con tests unitarios la clasificación de `sql.ErrNoRows` y SQLSTATE `23505`/`23514` sin abrir una transacción explícita.

## 2. Servicio y autorización

- [ ] 2.1 Implementar creación y consulta propagando `context.Context`, derivando el usuario del principal y exigiendo rol `employer`; verificar con stores falsos éxito, rol incorrecto, conflictos, ausencia y errores internos.

## 3. API HTTP

- [ ] 3.1 Implementar `POST /employers` con decodificación JSON, normalización, validator y helpers HTTP existentes para respuestas `201`, `400`, `401`, `403`, `409` y `500`; verificar mediante tests de handler éxito, formatos de error de JSON/validator, rol incorrecto, duplicado y error interno.
- [ ] 3.2 Implementar `GET /users/{userID}/employer` con validación del path, ownership, rol y respuesta `snake_case`; verificar mediante tests de handler éxito, identificador inválido, petición sin autenticación, acceso ajeno, rol incorrecto, ausencia y error interno, incluyendo `hiring_modalities: []` cuando no existen modalidades.
- [ ] 3.3 Registrar ambas rutas con `AuthenticatedUser` y componer repositorio, servicio y handler en `cmd`; verificar que los tests de rutas sin token respondan `401` y que `go test ./cmd ./internal/employer` compile la composición.

## 4. Documentación y verificación

- [ ] 4.1 Actualizar `../docs/create-employer.md` con `POST /employers`, `GET /users/{id}/employer`, nombres sin el typo `Employeer` y estados HTTP acordados; verificar manualmente que el diagrama coincide con la especificación.
- [ ] 4.2 Ejecutar `gofmt` sobre los archivos Go modificados, `go test ./internal/employer`, `go test ./...` y `go vet ./...`; verificar además cada escenario de `employer-profile-api` contra los tests y resultados obtenidos.
