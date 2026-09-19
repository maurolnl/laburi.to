## 1. Factories de test

- [x] 1.1 Crear factories package-local en `internal/user` para requests, usuarios y resultados de login `employee`/`employer`, con defaults válidos y opciones explícitas; verificar con tests que los defaults, roles y overrides produzcan el valor esperado.
- [x] 1.2 Crear factories package-local en `internal/employer` para requests, perfiles, principales y modelos persistidos, con clonación de slices y timestamps deterministas; verificar con tests que dos invocaciones no compartan estado y que puedan construirse escenarios inválidos controlados.

## 2. Cobertura de roles y sesión

- [x] 2.1 Refactorizar los tests relevantes de `internal/user` para consumir las factories sin ocultar assertions; verificar registro de ambos roles, rechazo de rol ausente/inválido y conservación del rol entregado a persistencia con `go test ./internal/user`.
- [x] 2.2 Completar el escenario de backfill lógico usando un resultado persistido con rol `employer`; verificar que login, access token y sesión actual exponen ese mismo rol y no dependen de datos compartidos con `go test ./internal/user`.

## 3. Cobertura de perfiles de empleador

- [x] 3.1 Reutilizar las factories en tests de servicio y completar creación/consulta para principal `employer`, rol `employee`, duplicado, perfil opuesto, ausencia y error interno; verificar identidad derivada del principal, propagación del contexto y ausencia de llamadas no autorizadas con `go test ./internal/employer`.
- [x] 3.2 Reutilizar las factories en tests de handler y completar ownership y acceso cruzado; verificar `401`, `403`, `409` y `404`, que otro `userID` no invoque el servicio y que ningún dato ajeno o error interno se exponga con `go test ./internal/employer`.

## 4. Cobertura de persistencia

- [x] 4.1 Completar tests del repositorio con factories para mapeo y parámetros de creación/consulta, modalidades independientes, duplicado `23505`, conflicto de exclusividad `23514`, ausencia y errores internos; verificar con `go test ./internal/employer`.
- [x] 4.2 Agregar un test autocontenido del contrato de `0005_add_user_roles_and_employers.sql`; verificar backfill a `employer`, dominio y `NOT NULL` del rol, trigger inmutable, usuario único por empleador, triggers en ambas direcciones y advisory lock compartido sin abrir conexiones ni leer entorno.

## 5. Validación final

- [x] 5.1 Ejecutar `gofmt` sobre archivos Go modificados, pruebas enfocadas, `go test ./...` y `go vet ./...`; comprobar explícitamente que factories válidas/invalidables, backfill lógico, rol inmutable, exclusividad, duplicados, ownership y acceso cruzado quedan cubiertos sin credenciales ni servicios externos.
