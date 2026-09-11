## 1. Contrato y validación del perfil base

- [x] 1.1 Auditar los DTO de `internal/employee`, retirar `omitempty` solo de campos cuya regla deba evaluar el valor cero y verificar con pruebas tabulares los campos obligatorios, opcionales y valores inválidos.
- [x] 1.2 Ajustar cualquier desvío detectado en el handler de creación para derivar el usuario del contexto y aceptar multipart sin archivo; verificar con pruebas HTTP los casos autenticado válido, no autenticado y payload inválido.

## 2. Certificación PDF

- [x] 2.1 Completar el contrato singular `certifications_file` para creación y actualización base; verificar con pruebas multipart un PDF válido, tipo inválido, archivo mayor a 5 MB y petición sin archivo.
- [x] 2.2 Cubrir la orquestación de upload y persistencia con dobles de `uploader` y `EmployeeStore`; verificar metadata correcta y cleanup compensatorio cuando la persistencia falla después de cargar el objeto.
- [x] 2.3 Verificar que la creación o actualización con archivo mantiene atómicos los cambios de DB relacionados y corregir cualquier desvío encontrado mediante pruebas del repositorio o de su frontera transaccional.

## 3. Consulta y actualizaciones por sección

- [x] 3.1 Cubrir `GET /users/{userID}/employee`; verificar respuesta con correo de la cuenta relacionada, rechazo `403` para otro usuario y ausencia de datos ajenos.
- [x] 3.2 Revisar el registro de rutas y handlers `PUT` de datos base, locación, recursos técnicos, disponibilidad y educación; verificar con pruebas HTTP que cada endpoint modifica solo su sección y exige propiedad.
- [x] 3.3 Corregir los desvíos observados en consultas o repositorios sin modificar el esquema y regenerar sqlc solo si cambia una query fuente; verificar que el código generado quede sincronizado.

## 4. Validación final

- [x] 4.1 Ejecutar `gofmt` sobre los archivos Go modificados y verificar `go test ./internal/employee`.
- [x] 4.2 Ejecutar `go test ./...` y `go vet ./...`; documentar cualquier fallo preexistente separado del cambio.
