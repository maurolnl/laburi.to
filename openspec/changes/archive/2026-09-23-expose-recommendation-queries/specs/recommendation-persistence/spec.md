## ADDED Requirements

### Requirement: Tamaño total del conjunto vigente
La persistencia SHALL informar, junto con cada lectura paginada, el tamaño total del conjunto
vigente del sujeto una vez aplicada la exclusión de puestos eliminados. El total y el tramo
devuelto MUST resolverse contra el mismo batch completado, para que no puedan describir
conjuntos distintos. Un sujeto sin batch completado MUST informar total cero.

#### Scenario: Total sobre un conjunto paginado
- **WHEN** se lee un tramo del conjunto vigente de un sujeto
- **THEN** la lectura informa además cuántas recomendaciones tiene el conjunto completo

#### Scenario: Total sin puestos eliminados
- **WHEN** el conjunto vigente de un empleado contiene puestos activos y eliminados
- **THEN** el total cuenta únicamente los activos

#### Scenario: Sujeto sin batch completado
- **WHEN** el sujeto nunca tuvo un batch completado
- **THEN** el total es cero y el tramo devuelto está vacío

#### Scenario: Desplazamiento más allá del conjunto
- **WHEN** el desplazamiento pedido supera el tamaño del conjunto vigente
- **THEN** el tramo devuelto está vacío y el total conserva el tamaño real del conjunto

### Requirement: Resolución de la propiedad de un sujeto
La persistencia SHALL ofrecer operaciones tipadas que resuelvan a qué usuario pertenece un
empleado y a qué usuario y empleador pertenece un puesto activo. Ambas MUST distinguir el
sujeto inexistente del sujeto ajeno devolviendo un error propio cuando no existe. La
resolución de un puesto MUST excluir los puestos eliminados lógicamente, que a estos efectos
son inexistentes.

#### Scenario: Empleado existente
- **WHEN** se resuelve la propiedad de un empleado existente
- **THEN** la operación devuelve el usuario dueño de ese perfil

#### Scenario: Empleado inexistente
- **WHEN** se resuelve la propiedad de un empleado que no existe
- **THEN** la operación devuelve el error de sujeto inexistente

#### Scenario: Puesto activo
- **WHEN** se resuelve la propiedad de un puesto activo
- **THEN** la operación devuelve su empleador y el usuario dueño de ese empleador

#### Scenario: Puesto eliminado lógicamente
- **WHEN** se resuelve la propiedad de un puesto eliminado lógicamente
- **THEN** la operación devuelve el error de sujeto inexistente
