## 1. Vínculo vigente en la persistencia de recomendaciones

- [x] 1.1 Agregar a `sql/queries/recommendations.sql` la consulta
  `EmployerHasCurrentRecommendationForEmployee :one`, que devuelve un booleano y resuelve el
  vínculo como dos `EXISTS` unidos por `OR`: uno anclado al último batch `completed` del
  empleado y otro al último batch `completed` de cada puesto, ambos con
  `JOIN employers ON employers.id = job_positions.employer_id`, `employers.user_id = $2` y
  `job_positions.deleted_at IS NULL`. Documentar en un comentario por qué son dos `EXISTS` y no
  un `IN` correlacionado.
- [x] 1.2 Ejecutar `sqlc generate` y revisar la salida en `internal/database` sin editarla a
  mano; verificar con `go build ./...`.
- [x] 1.3 Declarar en `internal/recommendation/store.go` el puerto `ProfileAccess` con
  `EmployerHasCurrentRecommendation(ctx context.Context, employeeID, employerUserID int32) (bool, error)`,
  documentando que responde por sí o por no y que un empleado inexistente responde `false` sin
  error propio.
- [x] 1.4 Implementar el método en `internal/recommendation/repo.go` sobre la consulta generada
  y sumar la aserción de interfaz `var _ ProfileAccess = (*RecommendationRepository)(nil)`;
  verificar con `go build ./...`.
- [x] 1.5 Cubrir con un test de integración sobre `testsupport.PostgresDB` —el repositorio toma
  `*sql.DB`, igual que en `ownership_test.go`— los siete escenarios
  del requisito «Resolución del vínculo vigente entre un empleado y los puestos de un
  empleador»: vínculo por el batch del empleado, vínculo por el batch del puesto, puesto
  eliminado, conjunto reemplazado, batch sin completar, empleador ajeno y empleado inexistente;
  verificar con `go test ./internal/recommendation`.

## 2. Lectura del perfil y de los archivos por identificador de empleado

- [x] 2.1 Agregar a `sql/queries/employees.sql` la consulta `GetEmployeeProfileByID :one`,
  igual a `GetEmployee` salvo por `WHERE employees.id = $1` y por dos diferencias en los
  agregados: `files` incluye `'id', id` junto al nombre original, y `education` reemplaza
  `'certification', certification` por `'certification_document_id', id` más un booleano que
  indique si hay documento. La consulta MUST NOT proyectar `bucket` ni `object_key`.
- [x] 2.2 Agregar `GetEmployeeFileForEmployee :one`, que devuelve `bucket`, `object_key`,
  `original_filename` y `content_type` de una fila de `employee_files` filtrando por
  `id = $1 AND employee_id = $2 AND status = 'uploaded'`. El filtro por empleado va en la
  consulta y no en Go: un archivo ajeno debe ser indistinguible de uno inexistente desde la
  base.
- [x] 2.3 Agregar `GetEmployeeEducationDocumentForEmployee :one`, que devuelve la
  `certification` de una fila de `employee_education` filtrando por `id = $1 AND employee_id = $2`
  y descartando el documento vacío o nulo.
- [x] 2.4 Ejecutar `sqlc generate`; verificar con `go build ./...`.
- [x] 2.5 Declarar en `internal/employee/store.go` los puertos
  `GetEmployeeProfileByID(ctx, employeeID int32) (Employee, error)`,
  `GetEmployeeFile(ctx, employeeID, fileID int32) (StoredFile, error)` y
  `GetEmployeeEducationDocument(ctx, employeeID, educationID int32) (StoredFile, error)`, más el
  tipo `StoredFile{Bucket, ObjectKey, Filename, ContentType string}`. Documentar que los dos
  últimos devuelven `ErrFileNotFound` tanto para el archivo inexistente como para el ajeno.
- [x] 2.6 Implementar los tres métodos en `internal/employee/repo.go` reutilizando el mapeo de
  filas de `GetEmployee` en un helper compartido, y traducir `sql.ErrNoRows` a los sentinelas
  nuevos de `internal/employee/errors.go` (`ErrEmployeeProfileNotFound`, `ErrFileNotFound`);
  verificar con `go build ./...`.
- [x] 2.7 Agregar los tipos del perfil nuevo en `internal/employee/models.go`: `EmployeeProfile`,
  `ProfileFileItem` —con `ID`—, `ProfileEducationItem` —con `CertificationDocumentID *int32` en
  vez del `object_key`—, `EmployeeProfileResponse` —con `Email *string` omitido para el
  empleador— y `DownloadURLResponse{URL string; ExpiresAt time.Time}`. Son tipos aparte de
  `FileItem` y `EducationItem` y no una extensión de ellos: esos los usa el perfil viejo por
  usuario, y tocarlos cambiaría su contrato.
- [x] 2.8 Cubrir con tests de integración sobre `testsupport.PostgresDB` que
  `GetEmployeeProfileByID` no proyecta bucket ni clave, que devuelve el identificador de cada
  certificado y de cada documento de título, y que las dos lecturas de archivo devuelven
  `ErrFileNotFound` ante un archivo ajeno, uno inexistente, un certificado sin subir y un
  título sin documento; verificar con `go test ./internal/employee`.

## 3. Presignado en el uploader

- [x] 3.1 Agregar a `internal/uploader/service.go` el método
  `PresignGetObject(ctx context.Context, input PresignInput) (string, error)` en la interfaz
  `Service`, con `PresignInput{Bucket, Key, Filename string; TTL time.Duration}`.
- [x] 3.2 Implementarlo con `s3.NewPresignClient(s.s3Client)` y
  `PresignGetObject(ctx, &s3.GetObjectInput{Bucket, Key, ResponseContentDisposition}, s3.WithPresignExpires(input.TTL))`,
  construyendo el `Content-Disposition` como `attachment` con el nombre original citado.
  Documentar que el cliente de presignado se crea una vez y no por pedido.
- [x] 3.3 Descartado durante la implementación: no se declara un puerto `Presigner` propio.
  `internal/employee` ya depende de `uploader.Service` para subir y borrar, y su doble de test ya
  la implementa, así que un segundo puerto sobre la misma dependencia no aísla nada y obliga a un
  adaptador en `cmd`. Justificado en `design.md`.
- [x] 3.4 Extender `fakeUploader` en `internal/employee/fakes_test.go` para que registre bucket,
  clave, nombre y plazo de cada firma y devuelva una URL determinística, y agregar un doble de
  `RecommendationAccess` con respuesta configurable.

## 4. Autorización del acceso al perfil

- [x] 4.1 Declarar en `internal/employee/store.go` el puerto `RecommendationAccess` con
  `EmployerHasCurrentRecommendation(ctx, employeeID, employerUserID int32) (bool, error)`,
  documentando que es la condición que habilita a un tercero y que el paquete no importa
  `internal/recommendation`.
- [x] 4.2 Agregar a `internal/employee/errors.go` el sentinela `ErrProfileAccessForbidden`, con
  un comentario que explique por qué cubre el empleado ajeno, el empleador sin vínculo y el
  empleado inexistente bajo la misma respuesta.
- [x] 4.3 Implementar en `internal/employee/authorization.go` la función del servicio que
  recibe `auth.Principal` y `employeeID` y devuelve si el solicitante es el dueño: rol
  `employee` exige propiedad, resuelta con `GetEmployeeByID` y no con el perfil entero; rol
  `employer` consulta `RecommendationAccess`; cualquier otro rol y cualquier falta de relación
  devuelven `ErrProfileAccessForbidden`. El empleado inexistente MUST caer en el mismo sentinela
  y no en uno propio.
- [x] 4.4 Cubrir los seis escenarios de autorización —dueño, empleador con vínculo, empleador sin
  vínculo, empleado ajeno, empleado inexistente y rol desconocido— sobre el servicio real
  montado tras `user.AuthenticatedUser`. El rol desconocido se comprueba invocando el servicio
  directamente: `auth.MakeJWT` no emite tokens con roles fuera del enum, así que ese caso no
  tiene borde HTTP. Verificar con `go test ./internal/employee`.

## 5. Perfil completo por identificador de empleado

- [x] 5.1 Implementar en `internal/employee/get_employee_profile.go` el método de servicio que
  autoriza, lee el perfil y arma `EmployeeProfileResponse` omitiendo el correo cuando el
  solicitante no es el dueño.
- [x] 5.2 Implementar el handler `GetEmployeeProfile`, que valida el identificador del path como
  entero positivo (`400`), obtiene el `auth.Principal` del contexto (`401` si falta) y traduce
  `ErrProfileAccessForbidden` a `403` con un mensaje único.
- [x] 5.3 Registrar `GET /employees/{employeeID}` en `RegisterRoutes` detrás de
  `user.AuthenticatedUser` y **no** detrás de `AuthenticatedEmployeeMiddleWare`, con un
  comentario que explique por qué el middleware de propiedad no sirve acá.
- [x] 5.4 Cubrir con tests de handler los escenarios de los requisitos «Lectura del perfil
  completo por identificador de empleado», «Acceso del empleador condicionado a una
  recomendación vigente», «Una sola respuesta para toda falla de autorización del perfil», «El
  perfil no revela la ubicación de ningún archivo» y «El empleador no recibe el correo del
  empleado», incluida una aserción sobre el JSON crudo de que no contiene el nombre del bucket
  ni ninguna clave de objeto; verificar con `go test ./internal/employee`.

## 6. Rutas de entrega

- [x] 6.1 Definir en `internal/employee/config.go` la constante `presignTTL = 5 * time.Minute`,
  con un comentario que explique por qué no es configurable por entorno.
- [x] 6.2 Implementar en `internal/employee/download_url.go` los dos métodos de servicio
  —certificado y documento de título—, que autorizan con la misma función de la sección 4, leen
  el archivo por su par empleado/identificador, presignan y devuelven
  `DownloadURLResponse{URL, ExpiresAt}` con `ExpiresAt` calculado a partir del mismo instante
  usado para firmar.
- [x] 6.3 Implementar los dos handlers, que validan ambos identificadores del path (`400`),
  traducen `ErrProfileAccessForbidden` a `403` y `ErrFileNotFound` a `404`, y responden sin
  incluir bucket ni clave.
- [x] 6.4 Registrar `GET /employees/{employeeID}/files/{fileID}/download-url` y
  `GET /employees/{employeeID}/education-documents/{educationID}/download-url` detrás de
  `user.AuthenticatedUser`.
- [x] 6.5 Cubrir con tests de handler, usando el presigner falso, los escenarios de los
  requisitos «Entrega de certificados por URL prefirmada», «La URL emitida corresponde a un
  único archivo del empleado autorizado» y «La URL caduca y la revocación es lógica»: dueño,
  empleador autorizado, empleador sin vínculo, sin token, archivo ajeno, archivo inexistente,
  título sin documento, identificador inválido, plazo de caducidad efectivamente pasado al
  presigner y pérdida del vínculo entre dos pedidos; verificar con `go test ./internal/employee`.

## 7. Cableado y documentación

- [x] 7.1 En `cmd/api.go`, construir el repositorio de recomendaciones para el borde de perfil y
  pasarlo junto con el `uploaderService` a `employee.BuildHandlers`, con un comentario que
  explique que el acceso entra por un puerto y que `internal/employee` sigue sin importar
  `internal/recommendation`; verificar con `go build ./...`.
- [x] 7.2 Actualizar `docs/employer-searching-for-employees.md` de la raíz con el paso de
  apertura del perfil y de descarga de certificados, incluyendo la precondición de
  recomendación vigente y la caducidad de la URL.
- [x] 7.3 Actualizar `docs/use-cases.md` de la raíz con la nueva precondición de acceso al
  perfil ajeno y con las tres rutas nuevas, y sumar las rutas y sus reglas al contrato HTTP de
  `CLAUDE.md` y `AGENTS.md` del backend.

## 8. Validación

- [x] 8.1 Ejecutar `gofmt -l .` y `go vet ./...` sin hallazgos.
- [x] 8.2 Ejecutar `go test ./...` en verde.
- [x] 8.3 Comprobar explícitamente cada criterio de aceptación de LAB-36 contra el código y los
  tests: ningún bucket ni clave en ninguna respuesta, URL acotada al archivo pedido y con
  caducidad, revocación lógica por puesto eliminado y por conjunto reemplazado, `403` para el
  empleador sin relación, `404` que no filtra existencia sensible, y presigner falso cubriendo
  propiedad, caducidad y archivo ajeno.
