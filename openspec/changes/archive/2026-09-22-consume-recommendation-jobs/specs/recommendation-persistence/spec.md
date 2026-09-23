## ADDED Requirements

### Requirement: Reclamo condicional del batch
La persistencia SHALL ofrecer una operación que mueva un batch a `processing` únicamente
cuando su estado actual no sea terminal, e informe al llamador si el reclamo prosperó. La
operación MUST NOT modificar un batch `completed` ni uno `failed`, de modo que un
reprocesamiento no pueda reabrir un trabajo ya cerrado ni destruir el conjunto vigente que
dejó.

#### Scenario: Reclamo de un batch pendiente
- **WHEN** se reclama un batch en estado `pending`
- **THEN** la operación lo deja en `processing`, refresca su fecha de actualización y lo
  informa como reclamado

#### Scenario: Reclamo de un batch en curso
- **WHEN** se reclama un batch que ya está en `processing`
- **THEN** la operación lo informa como reclamado y el batch sigue en `processing`, de modo
  que un procesamiento interrumpido pueda retomarse sin dejar al sujeto bloqueado

#### Scenario: Reclamo de un batch completado
- **WHEN** se reclama un batch en estado `completed`
- **THEN** la operación no modifica ninguna fila, lo informa como no reclamable y el
  conjunto de recomendaciones de ese batch queda intacto

#### Scenario: Reclamo de un batch fallido
- **WHEN** se reclama un batch en estado `failed`
- **THEN** la operación no modifica ninguna fila y lo informa como no reclamable

#### Scenario: Reclamo de un batch inexistente
- **WHEN** se reclama un batch que no existe
- **THEN** la operación no modifica ninguna fila y lo informa de forma distinguible de un
  batch existente no reclamable

### Requirement: Lectura de un batch por identificador
La persistencia SHALL ofrecer una operación que recupere un batch por su identificador, con
su sujeto, su estado y sus marcas temporales, e informe de forma distinguible que no existe.

#### Scenario: Batch existente
- **WHEN** se lee un batch por su identificador
- **THEN** la operación devuelve su sujeto, su estado y sus marcas temporales

#### Scenario: Batch inexistente
- **WHEN** se lee un batch cuyo identificador no corresponde a ninguno
- **THEN** la operación lo informa como inexistente y no devuelve ningún batch

### Requirement: Resolución de candidatos activos para la generación
La persistencia SHALL ofrecer operaciones tipadas que resuelvan, para un sujeto, los pares
empleado–puesto a evaluar, expresados en la entrada normalizada que define el contrato de
scoring. Esas operaciones MUST excluir los puestos eliminados lógicamente, MUST conservar la
distinción entre un atributo del perfil ausente y uno con valor cero o vacío, y MUST informar
de forma distinguible que el sujeto ya no existe.

#### Scenario: Candidatos de un empleado
- **WHEN** se resuelven los candidatos de un empleado
- **THEN** la operación devuelve un par por cada puesto vigente, con los atributos
  comparables de ambos lados

#### Scenario: Candidatos de un puesto
- **WHEN** se resuelven los candidatos de un puesto vigente
- **THEN** la operación devuelve un par por cada empleado con perfil

#### Scenario: Exclusión de puestos eliminados
- **WHEN** existen puestos eliminados lógicamente
- **THEN** no aparecen en ningún conjunto de candidatos

#### Scenario: Sujeto eliminado
- **WHEN** se resuelven los candidatos de un puesto eliminado lógicamente o de un sujeto
  inexistente
- **THEN** la operación lo informa de forma distinguible de un universo vacío legítimo

#### Scenario: Atributos ausentes del perfil
- **WHEN** un empleado no completó los pasos de ubicación, disponibilidad o educación
- **THEN** esos atributos viajan marcados como ausentes y no como cero ni como vacío

#### Scenario: Sin acceso a los paquetes de dominio
- **WHEN** se resuelven candidatos
- **THEN** la operación lee directamente de la persistencia, sin que el paquete de
  recomendaciones dependa de los paquetes de empleados ni de puestos

## MODIFIED Requirements

### Requirement: Acceso tipado a las recomendaciones
La capa de persistencia SHALL ofrecer operaciones tipadas para crear un batch, leerlo por su
identificador, transicionar su estado, reclamarlo condicionalmente para su procesamiento,
resolver el batch vigente y el último batch completado de un sujeto, resolver los candidatos
a evaluar, reemplazar atómicamente el conjunto de recomendaciones y listarlo paginado en
ambos sentidos.

#### Scenario: Transición de estado
- **WHEN** se transiciona un batch a `processing`, `completed` o `failed`
- **THEN** la operación persiste el nuevo estado y refresca la fecha de actualización

#### Scenario: Transición de un batch inexistente
- **WHEN** se intenta transicionar un batch que no existe
- **THEN** la operación no modifica ninguna fila y lo informa

#### Scenario: Reclamo para procesamiento
- **WHEN** un consumidor necesita tomar el trabajo de un batch
- **THEN** dispone de una operación que lo reclama solo si todavía no es terminal, distinta
  de la transición incondicional

#### Scenario: Lectura por empleado
- **WHEN** se listan los puestos recomendados del conjunto vigente de un empleado
- **THEN** la operación devuelve las recomendaciones con los datos del puesto, su
  puntaje cuando existe y el estado vigente del sujeto

#### Scenario: Lectura por puesto
- **WHEN** se listan los empleados recomendados del conjunto vigente de un puesto
- **THEN** la operación devuelve las recomendaciones con los datos del empleado, su
  puntaje cuando existe y el estado vigente del sujeto
