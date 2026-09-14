## 1. Rol y registro

- [x] 1.1 Definir el tipo de dominio para `employee` y `employer`, incorporarlo a los modelos de usuario y verificar con tests table-driven que acepte solamente ambos valores.
- [x] 1.2 Extender `CreateUserReq` para exigir `role`, persistirlo desde `CreateUser` y regenerar sqlc; verificar que el código generado incluya el parámetro y que `sqlc generate` no deje diferencias inesperadas.
- [x] 1.3 Cubrir el handler y servicio de registro con rol employee, employer, ausente e inválido; verificar respuestas `200`/`400` y que los rechazos no invoquen persistencia.

## 2. Grants y sesión con rol

- [x] 2.1 Incorporar el rol validado a los claims del access token y devolver un principal tipado al validarlo; verificar con tests de `internal/auth` tokens válidos para ambos roles y rechazo de claims ausentes o inválidos.
- [x] 2.2 Propagar el rol persistido desde el repositorio al login, generar grants con ese rol y agregarlo a la respuesta; verificar con tests de servicio y handler el JSON y el código `202` existentes.
- [x] 2.3 Exponer el rol persistido en `GET /auth/me` conservando el casing actual de `ID` y `Email`; verificar con tests autenticados que la respuesta incluya `Role` y no tome identidad del cliente.
- [x] 2.4 Reemplazar el contexto de solo user id por el principal autenticado con id y rol en todos los middlewares consumidores; verificar que tokens incompletos fallen con `401` y que ownership continúe funcionando.

## 3. Autorización de perfiles

- [x] 3.1 Implementar la regla de negocio tipada que recibe `UserRole` y tipo de perfil objetivo, permite employee→employee y employer→employer y devuelve un error de autorización estable para ambos accesos cruzados; verificar sus cuatro casos unitariamente.
- [x] 3.2 Aplicar la regla al alta existente de employee antes de cargar archivos o persistir datos y mapear su error a `403`; verificar con tests de handler/servicio que un employer no invoque uploader ni store.
- [x] 3.3 Verificar la compatibilidad del backfill usando siempre el rol almacenado, incluido un usuario histórico con rol `employer` y perfil employee preexistente, sin agregar inferencias ni actualizaciones de rol.

## 4. Validación integral

- [x] 4.1 Ejecutar `gofmt` sobre archivos Go modificados, `sqlc generate`, tests enfocados de `internal/auth`, `internal/user` e `internal/employee`, y verificar que todos pasen.
- [x] 4.2 Ejecutar `go test ./...`, `go vet ./...` y `openspec validate add-role-to-registration-and-session --strict`; documentar cualquier fallo preexistente separado del cambio.
- [x] 4.3 Comprobar manualmente todos los criterios de LAB-19 y coordinar que frontend y `docs/use-cases.md` reflejen los payloads definitivos, incluido Argon2id en lugar de la referencia obsoleta a bcrypt, antes de archivar el cambio.
