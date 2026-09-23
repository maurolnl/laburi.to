## MODIFIED Requirements

### Requirement: Notificación del proceso de recomendaciones
La API MUST notificar a un publicador de eventos de puesto después de crear y después
de editar un puesto, con el identificador del puesto resultante y ningún otro dato del
puesto. El contrato SHALL permanecer independiente de cualquier tecnología de cola
concreta, de modo que la infraestructura de recomendaciones pueda sustituir la
implementación sin alterar rutas, cuerpos ni códigos de estado. Un fallo del publicador
MUST NOT invalidar el alta ni la edición ya persistidas.

#### Scenario: Notificación tras el alta
- **WHEN** un empleador crea un puesto correctamente
- **THEN** el publicador recibe exactamente una notificación con el identificador del puesto creado

#### Scenario: Notificación tras la edición
- **WHEN** un empleador edita un puesto correctamente
- **THEN** el publicador recibe exactamente una notificación con el identificador del puesto
  actualizado

#### Scenario: Notificación sin atributos del puesto
- **WHEN** se inspecciona una notificación emitida
- **THEN** transporta únicamente el identificador del puesto, sin su descripción, sus requisitos ni
  ningún otro atributo

#### Scenario: Alta rechazada no notifica
- **WHEN** un alta es rechazada por validación, autorización o error de persistencia
- **THEN** el publicador no recibe ninguna notificación

#### Scenario: Eliminación lógica no notifica
- **WHEN** un empleador elimina lógicamente un puesto
- **THEN** el publicador no recibe ninguna notificación

#### Scenario: Fallo del publicador
- **WHEN** el publicador falla al notificar un puesto recién creado
- **THEN** la API responde `201` y el puesto permanece persistido
