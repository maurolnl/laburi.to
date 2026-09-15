# Employer Profile API Specification

## Purpose

Definir el contrato HTTP protegido para crear y consultar el perfil de empleador asociado a una cuenta con rol `employer`.

## Requirements

### Requirement: Creación asociada al principal autenticado
La API SHALL exponer `POST /employers` para crear un perfil vinculado al identificador del principal autenticado, MUST ignorar cualquier intento del cliente de elegir otro usuario y MUST permitir la operación solamente cuando el rol autenticado sea `employer`.

#### Scenario: Creación válida
- **WHEN** un usuario autenticado con rol `employer` envía un perfil válido en JSON
- **THEN** la API crea el perfil para ese usuario y responde `201` sin cuerpo

#### Scenario: Creación sin autenticación
- **WHEN** una petición sin un principal autenticado válido intenta crear un perfil
- **THEN** la API responde `401` y no persiste cambios

#### Scenario: Employee intenta crear un empleador
- **WHEN** un usuario autenticado con rol `employee` intenta crear un perfil de empleador
- **THEN** la API responde `403` y no persiste cambios

### Requirement: Validación de los datos del empleador
La API MUST exigir `name`, `industry` y `location` como strings con contenido distinto de espacios, SHALL normalizar espacios exteriores y SHALL aceptar `hiring_modalities` como una lista vacía o como strings libres no vacíos, sin restringirlos a un enum.

#### Scenario: Datos válidos con modalidades libres
- **WHEN** el cliente envía nombre, industria y ubicación presentables junto con modalidades libres no vacías
- **THEN** la API normaliza los espacios exteriores y persiste los valores y modalidades recibidos

#### Scenario: Lista de modalidades ausente o vacía
- **WHEN** el cliente envía los campos obligatorios válidos y omite las modalidades o envía una lista vacía
- **THEN** la API crea el perfil persistiendo y exponiendo una lista vacía, nunca `null`

#### Scenario: Campo obligatorio vacío
- **WHEN** `name`, `industry` o `location` está ausente, vacío o contiene solamente espacios
- **THEN** la API responde `400` y no persiste el perfil

#### Scenario: Modalidad vacía
- **WHEN** cualquier elemento de `hiring_modalities` está vacío o contiene solamente espacios
- **THEN** la API responde `400` y no persiste el perfil

#### Scenario: JSON inválido
- **WHEN** el cuerpo no contiene un documento JSON válido para el contrato
- **THEN** la API responde `400` y no persiste el perfil

### Requirement: Conflictos de creación explícitos
La API MUST impedir más de un perfil de empleador por usuario y MUST conservar la exclusividad respecto de perfiles de empleado, exponiendo ambos rechazos como conflictos sin filtrar detalles internos de PostgreSQL.

#### Scenario: Perfil de empleador duplicado
- **WHEN** un usuario que ya tiene un perfil de empleador intenta crear otro
- **THEN** la API responde `409` y conserva el perfil existente sin cambios

#### Scenario: Usuario con perfil de empleado
- **WHEN** un usuario intenta crear un perfil de empleador pero ya tiene un perfil de empleado
- **THEN** la API responde `409` y conserva el perfil existente sin cambios

### Requirement: Consulta protegida del perfil propio
La API SHALL exponer `GET /users/{userID}/employer`, MUST permitir la consulta solamente al usuario autenticado con ese identificador y rol `employer`, y SHALL devolver el perfil completo con nombres JSON en `snake_case`.

#### Scenario: Consulta del perfil propio
- **WHEN** un usuario autenticado con rol `employer` consulta la ruta usando su propio identificador y tiene un perfil
- **THEN** la API responde `200` con `id`, `user_id`, `name`, `industry`, `location`, `hiring_modalities`, `created_at` y `updated_at`

#### Scenario: Consulta de otro usuario
- **WHEN** un usuario autenticado consulta la ruta con un `userID` distinto del propio
- **THEN** la API responde `403` sin exponer datos del perfil solicitado

#### Scenario: Consulta sin autenticación
- **WHEN** una petición sin un principal autenticado válido intenta consultar un perfil
- **THEN** la API responde `401` sin consultar ni exponer un perfil

#### Scenario: Rol incorrecto en la consulta
- **WHEN** un usuario autenticado con rol `employee` consulta su ruta de empleador
- **THEN** la API responde `403` sin consultar ni exponer un perfil

#### Scenario: Perfil inexistente
- **WHEN** un usuario autorizado consulta su identificador pero no tiene un perfil de empleador
- **THEN** la API responde `404`

#### Scenario: Identificador inválido
- **WHEN** `userID` no representa un identificador numérico válido
- **THEN** la API responde `400`

### Requirement: Errores HTTP seguros y consistentes
La API SHALL usar el contrato JSON `{"error":"..."}` existente para errores de autenticación, autorización, parsing, conflicto, ausencia e internos; los errores de validación de campos SHALL conservar el formato de texto plano compartido por los endpoints actuales. La API MUST responder `500` con un mensaje genérico ante fallos internos no clasificados.

#### Scenario: Falla interna al crear o consultar
- **WHEN** la persistencia falla por una causa distinta de validación, autorización, conflicto o ausencia
- **THEN** la API responde `500` sin exponer SQL, credenciales ni detalles internos
