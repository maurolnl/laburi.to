## Purpose

Definir la integridad persistente del puesto de trabajo publicado por un empleador:
su relación con el perfil que lo publica, el dominio válido de cada campo, la
exclusión inmediata de los puestos eliminados lógicamente y el acceso tipado a las
operaciones de escritura y lectura.

## ADDED Requirements

### Requirement: Puesto perteneciente a un empleador
La persistencia SHALL almacenar cada puesto de trabajo asociado a exactamente un
perfil de empleador existente, y MUST eliminar los puestos asociados cuando ese
perfil deja de existir.

#### Scenario: Alta con empleador existente
- **WHEN** se crea un puesto indicando el identificador de un empleador existente
- **THEN** la base de datos persiste el puesto vinculado a ese empleador

#### Scenario: Alta con empleador inexistente
- **WHEN** se intenta crear un puesto para un identificador de empleador que no existe
- **THEN** la base de datos rechaza la relación

#### Scenario: Baja del empleador
- **WHEN** se elimina un perfil de empleador que tiene puestos asociados
- **THEN** la base de datos elimina también esos puestos

### Requirement: Campos obligatorios del puesto
La persistencia SHALL exigir posición, rol, experiencia requerida, nivel educativo
pretendido, horas disponibles por día y timezone en todo puesto almacenado.

#### Scenario: Alta sin un campo obligatorio
- **WHEN** se intenta crear un puesto omitiendo cualquiera de los campos obligatorios
- **THEN** la base de datos rechaza el alta

#### Scenario: Alta completa
- **WHEN** se crea un puesto con todos los campos obligatorios válidos
- **THEN** la base de datos persiste el puesto e inicializa sus timestamps de creación
  y actualización

### Requirement: Dominio compartido con el perfil de empleado
La persistencia MUST restringir la experiencia requerida a los valores `less_1y`,
`1y`, `2_to_5y`, `5_to_10y` y `more_10y`, y el nivel educativo pretendido a
`university`, `postgraduate`, `high-school-orientation` y `tertiary`, de modo que
el puesto y el perfil de empleado sean comparables sin conversión intermedia.

#### Scenario: Experiencia fuera del dominio
- **WHEN** se intenta persistir un puesto con una experiencia requerida distinta de
  los valores admitidos
- **THEN** la base de datos rechaza la operación

#### Scenario: Nivel educativo fuera del dominio
- **WHEN** se intenta persistir un puesto con un nivel educativo pretendido distinto
  de los valores admitidos
- **THEN** la base de datos rechaza la operación

#### Scenario: Valores alineados con un perfil de empleado
- **WHEN** se persiste un puesto cuya experiencia y nivel educativo coinciden con los
  de un perfil de empleado existente
- **THEN** ambos valores quedan almacenados con la misma representación textual

### Requirement: Horas disponibles acotadas
La persistencia MUST restringir las horas disponibles por día de un puesto al rango
entero de 1 a 8 inclusive.

#### Scenario: Horas dentro del rango
- **WHEN** se persiste un puesto con horas disponibles entre 1 y 8
- **THEN** la base de datos acepta el valor

#### Scenario: Horas fuera del rango
- **WHEN** se intenta persistir un puesto con horas disponibles menores a 1 o mayores
  a 8
- **THEN** la base de datos rechaza la operación

### Requirement: Recursos técnicos opcionales
La persistencia SHALL almacenar los recursos técnicos del puesto como una lista de
strings libres que MUST existir siempre, y que por defecto SHALL estar vacía.

#### Scenario: Alta sin recursos técnicos
- **WHEN** se crea un puesto sin indicar recursos técnicos
- **THEN** la base de datos persiste una lista vacía en lugar de un valor nulo

#### Scenario: Alta con recursos técnicos
- **WHEN** se crea un puesto indicando una lista de recursos técnicos
- **THEN** la base de datos persiste los valores sin restringirlos a un enum cerrado

### Requirement: Publicación inmediata sin estados intermedios
La persistencia MUST tratar todo puesto almacenado y no eliminado como publicado, sin
representar borradores, pausas ni reaperturas.

#### Scenario: Puesto recién creado
- **WHEN** se crea un puesto
- **THEN** queda inmediatamente disponible en las consultas de puestos activos

#### Scenario: Ausencia de estado de publicación
- **WHEN** se inspecciona la estructura del puesto almacenado
- **THEN** no existe ningún atributo que distinga un borrador de un puesto publicado

### Requirement: Eliminación lógica excluyente
La persistencia SHALL registrar la eliminación de un puesto mediante una marca
temporal de borrado, MUST conservar la fila y MUST excluir el puesto de toda consulta
de puestos activos desde el momento de la eliminación. La eliminación MUST ser
definitiva: no existe reapertura.

#### Scenario: Eliminación de un puesto activo
- **WHEN** se elimina lógicamente un puesto existente
- **THEN** la fila conserva sus datos y queda marcada con la fecha de eliminación

#### Scenario: Listado posterior a la eliminación
- **WHEN** se listan los puestos activos de un empleador que tiene un puesto eliminado
- **THEN** el resultado excluye ese puesto

#### Scenario: Consulta directa de un puesto eliminado
- **WHEN** se consulta un puesto ya eliminado mediante su identificador
- **THEN** la operación de puestos activos informa que no existe una fila
  correspondiente

#### Scenario: Segunda eliminación del mismo puesto
- **WHEN** se intenta eliminar lógicamente un puesto que ya fue eliminado
- **THEN** la operación no altera la fecha de eliminación original

### Requirement: Acceso tipado a los puestos
La capa de persistencia SHALL ofrecer operaciones tipadas para crear un puesto,
obtener un puesto activo por su identificador, listar los puestos activos de un
empleador, actualizar un puesto activo y eliminarlo lógicamente.

#### Scenario: Listado por empleador
- **WHEN** se listan los puestos activos de un empleador
- **THEN** la operación devuelve únicamente los puestos no eliminados de ese empleador
  con todos sus campos y timestamps

#### Scenario: Actualización de un puesto activo
- **WHEN** se actualiza un puesto no eliminado
- **THEN** la operación persiste los nuevos valores y refresca la fecha de
  actualización

#### Scenario: Actualización de un puesto eliminado
- **WHEN** se intenta actualizar un puesto ya eliminado
- **THEN** la operación no modifica ninguna fila

### Requirement: Soporte de consulta eficiente
La persistencia SHALL disponer de índices que permitan listar los puestos activos de
un empleador y filtrar puestos activos por experiencia requerida, nivel educativo
pretendido y timezone sin recorrer la tabla completa.

#### Scenario: Listado de puestos activos de un empleador
- **WHEN** se consultan los puestos activos de un empleador determinado
- **THEN** la consulta se resuelve mediante un índice que excluye los puestos
  eliminados

#### Scenario: Búsqueda de candidatos potenciales
- **WHEN** se filtran puestos activos por experiencia requerida, nivel educativo
  pretendido o timezone
- **THEN** la consulta se apoya en índices dedicados a esos atributos
