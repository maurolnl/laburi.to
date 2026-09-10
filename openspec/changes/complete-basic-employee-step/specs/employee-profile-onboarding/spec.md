## Purpose

Definir el contrato protegido para crear, consultar y completar por etapas el perfil de un empleado asociado a su cuenta de usuario.

## ADDED Requirements

### Requirement: Creación asociada al usuario autenticado
La API SHALL crear el perfil base del empleado asociado al usuario autenticado y MUST ignorar cualquier intento del cliente de elegir otra identidad.

#### Scenario: Creación válida sin archivo
- **WHEN** un usuario autenticado envía datos base válidos como `multipart/form-data` sin `certifications_file`
- **THEN** la API crea un empleado vinculado a ese usuario y responde `201`

#### Scenario: Creación sin autenticación
- **WHEN** una petición sin autenticación intenta crear un empleado
- **THEN** la API rechaza la petición y no crea ningún perfil

### Requirement: Certificación PDF opcional
La API SHALL aceptar como máximo un archivo opcional en el campo multipart `certifications_file`, MUST aceptar solamente PDF de hasta 5 MB y SHALL persistir su metadata vinculada al empleado.

#### Scenario: Creación válida con PDF
- **WHEN** un usuario autenticado envía datos base válidos y un PDF permitido en `certifications_file`
- **THEN** la API almacena el archivo, crea el empleado y persiste la metadata asociada antes de responder `201`

#### Scenario: Archivo inválido
- **WHEN** `certifications_file` no es PDF o excede 5 MB
- **THEN** la API responde `400` y no crea el empleado ni conserva el archivo

#### Scenario: Falla posterior a la carga
- **WHEN** el archivo se carga correctamente pero falla la persistencia del empleado o de su metadata
- **THEN** la API informa el fallo y elimina de forma compensatoria el objeto cargado

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
- **WHEN** el propietario actualiza los datos base mediante `multipart/form-data` e incluye un PDF válido en `certifications_file`
- **THEN** la API actualiza los datos base y agrega la metadata del archivo de forma atómica

### Requirement: Validación según obligatoriedad
La API MUST validar todos los campos obligatorios aun cuando contengan su valor cero y SHALL permitir la ausencia de campos definidos explícitamente como opcionales.

#### Scenario: Campo obligatorio omitido
- **WHEN** una petición omite un campo obligatorio o envía un valor fuera de su dominio permitido
- **THEN** la API responde con error de validación y no persiste cambios

#### Scenario: Campo opcional ausente
- **WHEN** una petición válida no incluye un campo opcional
- **THEN** la API procesa la operación sin exigir ese campo
