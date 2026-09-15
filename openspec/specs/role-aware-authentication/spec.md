## Purpose

Definir el contrato de autenticación que registra, transporta y expone el rol inmutable de cada usuario, y que lo convierte en autoridad para las operaciones de dominio protegidas.

## Requirements

### Requirement: Registro con rol obligatorio
La API SHALL exigir que `POST /auth/register` reciba `email`, `password` y `role`, y MUST aceptar exclusivamente los roles `employee` y `employer` antes de persistir la cuenta.

#### Scenario: Registro de employee
- **WHEN** un usuario envía credenciales válidas con `role` igual a `employee`
- **THEN** la API crea la cuenta con ese rol y responde `200` sin cuerpo

#### Scenario: Registro de employer
- **WHEN** un usuario envía credenciales válidas con `role` igual a `employer`
- **THEN** la API crea la cuenta con ese rol y responde `200` sin cuerpo

#### Scenario: Rol ausente
- **WHEN** un usuario intenta registrarse sin enviar `role`
- **THEN** la API responde `400` y no crea la cuenta

#### Scenario: Rol inválido
- **WHEN** un usuario intenta registrarse con un rol distinto de `employee` o `employer`
- **THEN** la API responde `400` y no crea la cuenta

### Requirement: Rol inmutable después del registro
El sistema MUST conservar durante toda la vida de la cuenta el rol elegido al registrarla o el rol `employer` asignado por el backfill, y MUST rechazar cualquier intento posterior de reemplazarlo.

#### Scenario: Cuenta creada después de incorporar roles
- **WHEN** una cuenta nueva completa el registro con un rol válido
- **THEN** las sesiones posteriores exponen exactamente el rol persistido al crearla

#### Scenario: Cuenta existente migrada
- **WHEN** inicia sesión una cuenta alcanzada por el backfill de LAB-18
- **THEN** la sesión expone `employer` aunque exista un perfil histórico de employee

#### Scenario: Intento posterior de cambio
- **WHEN** cualquier operación intenta reemplazar el rol de una cuenta existente
- **THEN** el sistema rechaza la modificación y conserva el rol original

### Requirement: Sesión autenticada con rol
La API SHALL incluir el rol persistido en la respuesta de `POST /auth/login`, en `GET /auth/me` y en el access token firmado, y MUST reconstruir la identidad autenticada usando el identificador y el rol validados del token.

#### Scenario: Login exitoso
- **WHEN** un usuario con credenciales válidas inicia sesión
- **THEN** la API responde `202` con id, email, rol, access token y refresh token, y el access token firmado contiene su identificador y rol persistidos

#### Scenario: Consulta de sesión actual
- **WHEN** una petición presenta un access token válido en `GET /auth/me`
- **THEN** la API responde `200` con el identificador, email y rol persistido de la cuenta autenticada

#### Scenario: Token sin rol válido
- **WHEN** una petición protegida presenta un token sin rol o con un rol fuera del dominio permitido
- **THEN** la API responde `401` y no construye una identidad autenticada

### Requirement: Autorización independiente del body
Las operaciones protegidas SHALL derivar el identificador y el rol de cuenta del access token validado y MUST impedir que campos del body, path o query reemplacen esos datos del principal autenticado. Los campos de dominio denominados `role`, como la especialidad profesional de un employee, SHALL conservar su significado y no se usarán como rol de cuenta.

#### Scenario: Identidad de cuenta contradictoria enviada por el cliente
- **WHEN** una petición autenticada incluye datos que intentan seleccionar otro usuario o rol de cuenta
- **THEN** la autorización utiliza solamente el identificador y rol de cuenta del contexto autenticado

#### Scenario: Especialidad profesional del employee
- **WHEN** un employee autorizado envía el campo multipart `role` de su perfil profesional
- **THEN** la API conserva ese valor como dato del perfil y no lo interpreta como rol de cuenta

#### Scenario: Operación sin principal autenticado
- **WHEN** una operación protegida no dispone de un principal con identificador y rol válidos
- **THEN** la API rechaza la operación sin persistir cambios
