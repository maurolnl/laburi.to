## Purpose

Definir el contrato protegido para crear, consultar y completar por etapas el perfil de un empleado asociado a su cuenta de usuario.

## Requirements

### Requirement: Creación asociada al usuario autenticado
La API SHALL crear el perfil base del empleado asociado al usuario autenticado solamente cuando su rol de cuenta persistido sea `employee`, MUST derivar identificador y rol de cuenta del JWT/contexto y MUST impedir que el cliente los reemplace. El campo multipart `role` SHALL conservar su significado actual de especialidad profesional del employee.

#### Scenario: Creación válida sin archivo
- **WHEN** un usuario autenticado con rol `employee` envía datos base válidos como `multipart/form-data` sin archivos de certificación
- **THEN** la API crea un empleado vinculado a ese usuario y responde `201`

#### Scenario: Creación sin autenticación
- **WHEN** una petición sin autenticación intenta crear un empleado
- **THEN** la API rechaza la petición y no crea ningún perfil

#### Scenario: Employer intenta crear employee
- **WHEN** un usuario autenticado con rol `employer` intenta crear un perfil de employee
- **THEN** la API responde `403` y no crea ningún perfil

### Requirement: Certificación PDF opcional
La API SHALL recibir en `POST /employees` y `PUT /employees/{employeeID}` el campo multipart `certifications` como un arreglo JSON de ítems `{ "name", "document"?, "document_id"? }`. Cada ítem MUST tener `name` no vacío tras recortar espacios, y los nombres MUST ser únicos dentro del perfil sin distinguir mayúsculas. `document` SHALL nombrar la clave de un archivo del mismo multipart, y `document_id` SHALL identificar un certificado ya cargado del mismo empleado; un ítem MUST NOT traer ambos. Cada archivo MUST ser PDF de hasta 5 MB, y SHALL persistirse vinculado al empleado y al nombre de su certificación. La API MUST rechazar con `400` y cuerpo `{"error": "..."}`, sin persistir cambios ni conservar archivos subidos, cuando no se cumpla alguna de estas reglas, cuando un archivo del multipart no esté referenciado por ningún ítem, cuando una clave o un `document_id` se referencie más de una vez, cuando `document_id` no pertenezca al empleado o no esté cargado, o cuando llegue el formato anterior (`certifications[]` o `certifications_file`).

#### Scenario: Creación válida con PDF
- **WHEN** un usuario autenticado envía datos base válidos y una certificación cuyo `document` apunta a un PDF permitido
- **THEN** la API almacena el archivo, crea el empleado y persiste la metadata asociada a esa certificación antes de responder `201`

#### Scenario: Dos certificaciones con su PDF cada una
- **WHEN** un empleado envía `certifications` con «Scrum Master» y «AWS Cloud Practitioner», cada una con `document` apuntando a un PDF válido distinto
- **THEN** la API almacena ambos archivos, cada uno asociado a su certificación, y responde con éxito

#### Scenario: Certificación sin PDF
- **WHEN** un ítem de `certifications` no trae `document` ni `document_id`
- **THEN** la API persiste la certificación sin documento

#### Scenario: Archivo inválido
- **WHEN** un archivo referenciado no es PDF o excede 5 MB
- **THEN** la API responde `400` con `{"error": "..."}` y no persiste cambios ni conserva archivos

#### Scenario: Archivo sin certificación asociada
- **WHEN** el multipart incluye un archivo cuya clave ningún ítem referencia
- **THEN** la API responde `400` con `{"error": "..."}` y no persiste cambios

#### Scenario: Nombre vacío o repetido
- **WHEN** un ítem trae `name` vacío o dos ítems comparten nombre sin distinguir mayúsculas
- **THEN** la API responde `400` con `{"error": "..."}`

#### Scenario: Documento ajeno
- **WHEN** un ítem trae un `document_id` que no es un certificado cargado del mismo empleado
- **THEN** la API responde `400` con `{"error": "..."}` y no persiste cambios

#### Scenario: Falla posterior a la carga
- **WHEN** los archivos se cargan correctamente pero falla la persistencia del empleado o de su metadata
- **THEN** la API informa el fallo y elimina de forma compensatoria los objetos cargados

### Requirement: Consulta protegida con correo relacionado
La API SHALL devolver el empleado del usuario solicitado junto con el correo vigente de su cuenta y MUST impedir que un usuario consulte el perfil de otro.

#### Scenario: Consulta del perfil propio
- **WHEN** un usuario autenticado consulta `/users/{userID}/employee` usando su propio identificador
- **THEN** la API responde `200` con el empleado y el correo obtenido de la cuenta relacionada

#### Scenario: Consulta de otro usuario
- **WHEN** un usuario autenticado consulta el perfil correspondiente a otro `userID`
- **THEN** la API responde `403` sin exponer datos del empleado

### Requirement: Actualización por sección del perfil
La API SHALL ofrecer actualizaciones independientes para datos base, locación, recursos técnicos, disponibilidad y educación, y MUST comprobar la propiedad del empleado antes de modificar cualquier sección.

#### Scenario: Actualización de una sección propia
- **WHEN** el propietario envía un payload válido al endpoint de actualización de una sección
- **THEN** la API actualiza únicamente esa sección y responde `200`

#### Scenario: Actualización de un empleado ajeno
- **WHEN** un usuario intenta actualizar cualquier sección de un empleado que no le pertenece
- **THEN** la API rechaza la operación sin modificar el perfil

#### Scenario: Actualización base con PDF
- **WHEN** el propietario actualiza los datos base e incluye un PDF válido para una certificación
- **THEN** la API actualiza los datos base y la metadata del archivo asociado a esa certificación de forma atómica

### Requirement: Validación según obligatoriedad
La API MUST validar todos los campos obligatorios aun cuando contengan su valor cero y SHALL permitir la ausencia de campos definidos explícitamente como opcionales. En `POST /employees` y `PUT /employees/{employeeID}` los errores de validación MUST responderse como JSON `{"error": "..."}`.

#### Scenario: Campo obligatorio omitido
- **WHEN** una petición omite un campo obligatorio o envía un valor fuera de su dominio permitido
- **THEN** la API responde con error de validación y no persiste cambios

#### Scenario: Campo opcional ausente
- **WHEN** una petición válida no incluye un campo opcional
- **THEN** la API procesa la operación sin exigir ese campo

### Requirement: Reemplazo y baja de certificados sin huérfanos
En `PUT /employees/{employeeID}` el arreglo `certifications` SHALL reemplazar el conjunto de certificaciones. Un certificado asociado que la petición no conserva mediante `document_id` —porque su certificación recibe un PDF nuevo, queda sin PDF o se elimina— MUST quedar con estado `deleted` en la misma transacción que la actualización, y su objeto MUST borrarse del almacenamiento después de confirmarla. Un certificado conservado por `document_id` SHALL quedar asociado al `name` del ítem que lo referencia. Los certificados sin asociar que ningún ítem referencia MUST NOT modificarse. En ningún momento una certificación MUST tener más de un certificado activo.

#### Scenario: Reemplazo del PDF de una certificación
- **WHEN** el empleado envía un `document` nuevo para una certificación que ya tenía PDF
- **THEN** el certificado anterior queda `deleted`, su objeto se borra del almacenamiento, deja de poder descargarse y el perfil muestra el nuevo

#### Scenario: Conservar el PDF
- **WHEN** el empleado envía la certificación con el `document_id` de su PDF actual
- **THEN** el certificado sigue activo y asociado a esa certificación

#### Scenario: Eliminar una certificación con PDF
- **WHEN** el empleado omite en `certifications` una certificación que tenía PDF
- **THEN** su certificado queda `deleted` y su objeto se borra del almacenamiento

#### Scenario: Certificado viejo sin asociar
- **WHEN** el empleado tiene un certificado sin asociar y ningún ítem lo referencia
- **THEN** la actualización no lo modifica y sigue listado como sin asociar

### Requirement: Migración de asociación idempotente
El esquema SHALL registrar para cada certificado el nombre de la certificación a la que pertenece, nulo para los certificados existentes antes del cambio. La migración MUST aplicarse con el comando de migración del deploy (`./out migrate`) y MUST poder ejecutarse más de una vez sin error ni cambios adicionales.

#### Scenario: Deploy sobre una base existente
- **WHEN** la migración corre sobre una base con certificados cargados
- **THEN** los certificados existentes quedan sin certificación asociada y siguen descargables

#### Scenario: Ejecución repetida
- **WHEN** la migración se ejecuta sobre una base que ya la tiene aplicada
- **THEN** termina sin error y sin modificar el esquema
