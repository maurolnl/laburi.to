## MODIFIED Requirements

### Requirement: Creación asociada al usuario autenticado
La API SHALL crear el perfil base del empleado asociado al usuario autenticado solamente cuando su rol de cuenta persistido sea `employee`, MUST derivar identificador y rol de cuenta del JWT/contexto y MUST impedir que el cliente los reemplace. El campo multipart `role` SHALL conservar su significado actual de especialidad profesional del employee.

#### Scenario: Creación válida sin archivo
- **WHEN** un usuario autenticado con rol `employee` envía datos base válidos como `multipart/form-data` sin `certifications_file`
- **THEN** la API crea un empleado vinculado a ese usuario y responde `201`

#### Scenario: Creación sin autenticación
- **WHEN** una petición sin autenticación intenta crear un empleado
- **THEN** la API rechaza la petición y no crea ningún perfil

#### Scenario: Employer intenta crear employee
- **WHEN** un usuario autenticado con rol `employer` intenta crear un perfil de employee
- **THEN** la API responde `403` y no crea ningún perfil
