# user-role-employer-persistence Specification

## Purpose

Definir la integridad persistente del rol de cada cuenta y del perfil de empleador, incluida la exclusividad respecto del perfil de empleado.

## Requirements

### Requirement: Rol de usuario obligatorio e inmutable
La persistencia SHALL exigir que cada usuario tenga exactamente uno de los roles `employee` o `employer`, MUST migrar las cuentas existentes a `employer` y MUST rechazar cualquier cambio posterior del rol almacenado.

#### Scenario: Migración de una cuenta existente
- **WHEN** la migración se aplica sobre un usuario creado previamente
- **THEN** el usuario conserva su identidad y queda almacenado con rol `employer`

#### Scenario: Alta con rol inválido o ausente
- **WHEN** se intenta persistir un usuario sin rol o con un valor distinto de `employee` y `employer`
- **THEN** la base de datos rechaza el alta

#### Scenario: Cambio posterior del rol
- **WHEN** se intenta modificar el rol de un usuario existente
- **THEN** la base de datos rechaza la actualización y conserva el rol original

### Requirement: Perfil persistente de empleador
La persistencia SHALL almacenar un único perfil de empleador por usuario con nombre, industria y ubicación obligatorios, una lista de modalidades de contratación expresadas como strings libres y campos de timestamp inicializados al crear el perfil.

#### Scenario: Creación de empleador válida
- **WHEN** se crea un perfil con un usuario existente y todos los campos obligatorios
- **THEN** la base de datos persiste el perfil y sus modalidades sin restringirlas a un enum cerrado

#### Scenario: Perfil duplicado para el mismo usuario
- **WHEN** se intenta crear un segundo perfil de empleador para un usuario
- **THEN** la base de datos rechaza el duplicado

#### Scenario: Usuario relacionado inexistente
- **WHEN** se intenta crear un perfil de empleador para un usuario inexistente
- **THEN** la base de datos rechaza la relación

### Requirement: Exclusividad entre perfiles de dominio
La persistencia MUST impedir que un mismo usuario tenga simultáneamente un perfil de empleado y un perfil de empleador, incluso ante intentos concurrentes de creación.

#### Scenario: Empleador para usuario con perfil de empleado
- **WHEN** se intenta crear un perfil de empleador para un usuario que ya tiene perfil de empleado
- **THEN** la base de datos rechaza la operación sin alterar el perfil existente

#### Scenario: Empleado para usuario con perfil de empleador
- **WHEN** se intenta crear un perfil de empleado para un usuario que ya tiene perfil de empleador
- **THEN** la base de datos rechaza la operación sin alterar el perfil existente

#### Scenario: Creaciones concurrentes de perfiles opuestos
- **WHEN** dos transacciones intentan crear al mismo tiempo perfiles de empleado y empleador para el mismo usuario
- **THEN** como máximo una creación finaliza y el usuario conserva un único tipo de perfil

### Requirement: Acceso tipado al perfil de empleador
La capa de persistencia SHALL ofrecer operaciones sqlc tipadas para crear un perfil de empleador y obtenerlo mediante el identificador del usuario.

#### Scenario: Consulta por usuario con perfil
- **WHEN** se consulta por el identificador de un usuario que tiene perfil de empleador
- **THEN** la operación devuelve el perfil completo, incluidas sus modalidades y timestamps

#### Scenario: Consulta por usuario sin perfil
- **WHEN** se consulta por el identificador de un usuario que no tiene perfil de empleador
- **THEN** la operación informa que no existe una fila correspondiente
