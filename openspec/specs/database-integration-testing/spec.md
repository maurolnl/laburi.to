# database-integration-testing Specification

## Purpose
Definir la disponibilidad de una base PostgreSQL efímera dentro de la suite de tests,
de modo que el comportamiento real del esquema —atomicidad, constraints, índices únicos
parciales y orden— pueda verificarse sin depender de recursos externos ni de bases
compartidas.

## Requirements

### Requirement: Base efímera con el esquema vigente
La suite de tests SHALL disponer de una base PostgreSQL efímera con todas las
migraciones de `sql/schema` aplicadas en orden, y MUST crearla y destruirla dentro de la
propia ejecución de tests.

#### Scenario: Esquema disponible
- **WHEN** un test solicita la base de integración
- **THEN** recibe una conexión a una base con todas las migraciones aplicadas

#### Scenario: Migración incompleta
- **WHEN** alguna migración falla al aplicarse
- **THEN** la preparación falla con un mensaje que identifica la migración responsable

#### Scenario: Reutilización dentro de una ejecución
- **WHEN** varios tests solicitan la base de integración durante la misma ejecución
- **THEN** comparten la misma instancia en lugar de crear una por test

### Requirement: Aislamiento entre tests
La suite SHALL garantizar que ningún test observe datos escritos por otro, y MUST
permitir que un test verifique el efecto de un commit real cuando la atomicidad sea
precisamente lo que se está probando.

#### Scenario: Datos no compartidos
- **WHEN** dos tests escriben filas en las mismas tablas
- **THEN** ninguno observa las filas del otro

#### Scenario: Verificación de commits reales
- **WHEN** un test necesita comprobar que una transacción se confirmó
- **THEN** dispone de un mecanismo que permite el commit y limpia los datos al terminar

### Requirement: Degradación explícita sin entorno de contenedores
La suite MUST seguir siendo ejecutable en una máquina sin entorno de contenedores: los
tests que requieren la base efímera SHALL omitirse con un motivo explícito en lugar de
fallar, y el resto de la suite MUST ejecutarse sin cambios.

#### Scenario: Entorno sin contenedores
- **WHEN** se ejecuta la suite en una máquina sin entorno de contenedores disponible
- **THEN** los tests de integración se omiten indicando el motivo y la ejecución global
  no falla

#### Scenario: Tests sin dependencia de base
- **WHEN** se ejecuta la suite en esa misma máquina
- **THEN** los tests que no requieren base de datos se ejecutan y reportan normalmente

### Requirement: Ninguna base compartida
La suite MUST no ejecutar migraciones ni escribir datos sobre ninguna base de datos
preexistente del entorno de desarrollo o de producción.

#### Scenario: Configuración del entorno ignorada
- **WHEN** el entorno define una cadena de conexión a una base existente
- **THEN** los tests de integración la ignoran y usan exclusivamente la base efímera
