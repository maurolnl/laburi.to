## ADDED Requirements

### Requirement: Resolución del vínculo vigente entre un empleado y los puestos de un empleador
La persistencia SHALL ofrecer una operación que responda si existe una recomendación vigente
entre un empleado y alguno de los puestos activos de un empleador dado, identificado por el
usuario dueño de ese empleador. La operación MUST responder por sí o por no, sin devolver el
conjunto ni ningún dato del empleado o del puesto: es la condición de autorización de un borde
ajeno a esta capa y no una consulta de negocio.

El vínculo MUST considerarse vigente en cualquiera de las dos direcciones: cuando el par
aparece en el conjunto vigente del empleado —su último batch completado— o cuando aparece en el
conjunto vigente de alguno de esos puestos. Los puestos eliminados lógicamente MUST quedar
excluidos con independencia de que la recomendación se haya generado antes de esa eliminación.
Un batch no completado MUST NOT sostener el vínculo.

#### Scenario: Vínculo por el conjunto vigente del empleado
- **WHEN** el último batch completado del empleado contiene un puesto activo del empleador
- **THEN** la operación responde que el vínculo existe

#### Scenario: Vínculo por el conjunto vigente del puesto
- **WHEN** el último batch completado de un puesto activo del empleador contiene al empleado
- **THEN** la operación responde que el vínculo existe

#### Scenario: Puesto eliminado lógicamente
- **WHEN** el único puesto que vinculaba al par fue eliminado lógicamente
- **THEN** la operación responde que el vínculo no existe

#### Scenario: Conjunto vigente reemplazado
- **WHEN** un batch completado posterior reemplaza al anterior y ya no contiene el par
- **THEN** la operación responde que el vínculo no existe

#### Scenario: Batch sin completar
- **WHEN** el único batch que contiene el par está en `pending`, `processing` o `failed`
- **THEN** la operación responde que el vínculo no existe

#### Scenario: Empleador ajeno
- **WHEN** el par existe pero el puesto pertenece a otro empleador
- **THEN** la operación responde que el vínculo no existe

#### Scenario: Empleado inexistente
- **WHEN** el empleado consultado no existe
- **THEN** la operación responde que el vínculo no existe, sin error propio que lo distinga de
  un empleado sin recomendaciones
