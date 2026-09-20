# recommendation-persistence Specification

## Purpose
Definir la integridad persistente de las recomendaciones entre empleados y puestos de
trabajo: la ejecución que las genera y su ciclo de estados, el sujeto excluyente de esa
ejecución, la relación many-to-many resultante, la resolución de qué estado y qué
conjunto están vigentes, el reemplazo atómico del conjunto, la exclusión de los puestos
eliminados y el acceso tipado a todas esas operaciones.

## Requirements

### Requirement: Batch perteneciente a un único sujeto
La persistencia SHALL almacenar cada batch de recomendaciones asociado a exactamente un
sujeto, que MUST ser un empleado o un puesto de trabajo existente, y nunca ambos ni
ninguno. La persistencia MUST eliminar los batches asociados cuando ese sujeto deja de
existir.

#### Scenario: Batch por empleado
- **WHEN** se crea un batch indicando el identificador de un empleado existente y
  ningún puesto
- **THEN** la base de datos persiste el batch vinculado a ese empleado

#### Scenario: Batch por puesto
- **WHEN** se crea un batch indicando el identificador de un puesto existente y ningún
  empleado
- **THEN** la base de datos persiste el batch vinculado a ese puesto

#### Scenario: Batch con dos sujetos
- **WHEN** se intenta crear un batch que indica a la vez un empleado y un puesto
- **THEN** la base de datos rechaza la operación

#### Scenario: Batch sin sujeto
- **WHEN** se intenta crear un batch que no indica ni empleado ni puesto
- **THEN** la base de datos rechaza la operación

#### Scenario: Sujeto inexistente
- **WHEN** se intenta crear un batch para un identificador de empleado o de puesto que
  no existe
- **THEN** la base de datos rechaza la relación

#### Scenario: Baja del sujeto
- **WHEN** se elimina físicamente el empleado o el puesto sujeto de un batch
- **THEN** la base de datos elimina también ese batch y sus recomendaciones

### Requirement: Ciclo de estados del batch
La persistencia SHALL restringir el estado de todo batch al conjunto `pending`,
`processing`, `completed` y `failed`, y MUST registrar el instante de creación y de
última actualización de cada batch.

#### Scenario: Estado válido
- **WHEN** se persiste un batch con uno de los cuatro estados definidos
- **THEN** la base de datos acepta el valor

#### Scenario: Estado desconocido
- **WHEN** se intenta persistir un batch con un estado fuera de ese conjunto
- **THEN** la base de datos rechaza la operación

#### Scenario: Creación
- **WHEN** se crea un batch sin indicar estado
- **THEN** la base de datos lo persiste como `pending` con sus marcas temporales
  inicializadas

### Requirement: Un único batch en curso por sujeto
La persistencia MUST impedir que coexistan dos batches en estado `pending` o
`processing` para el mismo sujeto, de modo que el reprocesamiento sea idempotente sin
depender de una comprobación previa del llamador.

#### Scenario: Segundo batch para un sujeto sin trabajo en curso
- **WHEN** se crea un batch para un sujeto cuyo único batch previo está `completed` o
  `failed`
- **THEN** la base de datos acepta el nuevo batch

#### Scenario: Segundo batch para un sujeto con trabajo en curso
- **WHEN** se intenta crear un batch para un sujeto que ya tiene un batch `pending` o
  `processing`
- **THEN** la base de datos rechaza la operación

#### Scenario: Sujetos distintos en paralelo
- **WHEN** se crean batches en curso para dos sujetos diferentes
- **THEN** la base de datos acepta ambos

### Requirement: Relación many-to-many entre empleados y puestos
La persistencia SHALL almacenar cada recomendación como el vínculo entre un empleado y
un puesto de trabajo dentro de un batch, MUST permitir que un puesto se relacione con
múltiples empleados y que un empleado se relacione con múltiples puestos, y MUST
rechazar la misma dupla repetida dentro de un mismo batch.

#### Scenario: Varios empleados para un puesto
- **WHEN** un batch por puesto persiste recomendaciones hacia varios empleados distintos
- **THEN** la base de datos acepta todas las filas

#### Scenario: Varios puestos para un empleado
- **WHEN** un batch por empleado persiste recomendaciones hacia varios puestos distintos
- **THEN** la base de datos acepta todas las filas

#### Scenario: Dupla repetida en el mismo batch
- **WHEN** se intenta persistir dos veces la misma pareja empleado-puesto dentro de un
  mismo batch
- **THEN** la base de datos rechaza la segunda fila

#### Scenario: Misma dupla en batches distintos
- **WHEN** la misma pareja empleado-puesto aparece en dos batches diferentes
- **THEN** la base de datos acepta ambas filas

#### Scenario: Baja del batch
- **WHEN** se elimina un batch que tiene recomendaciones
- **THEN** la base de datos elimina también esas recomendaciones

### Requirement: Puntaje opcional
La persistencia SHALL almacenar el puntaje de cada recomendación como un valor numérico
que MUST admitir ausencia de valor mientras el algoritmo de indicadores no esté
definido, y MUST distinguir la ausencia de puntaje de un puntaje igual a cero.

#### Scenario: Recomendación sin puntaje
- **WHEN** se persiste una recomendación sin puntaje
- **THEN** la base de datos acepta la fila y conserva el puntaje como ausente

#### Scenario: Recomendación con puntaje
- **WHEN** se persiste una recomendación con un puntaje numérico
- **THEN** la base de datos conserva el valor sin pérdida de precisión al leerlo

#### Scenario: Puntaje cero
- **WHEN** se persiste una recomendación con puntaje cero
- **THEN** la lectura la distingue de una recomendación sin puntaje

### Requirement: Estado vigente y conjunto vigente
La persistencia SHALL resolver el estado vigente de un sujeto como el del batch más
reciente, cualquiera sea ese estado, y SHALL resolver el conjunto vigente de
recomendaciones como el del batch `completed` más reciente. Ambas resoluciones MUST ser
operaciones tipadas y no responsabilidad del llamador.

#### Scenario: Último batch completado
- **WHEN** el batch más reciente de un sujeto está `completed`
- **THEN** el estado vigente y el conjunto vigente corresponden a ese mismo batch

#### Scenario: Procesamiento en curso sobre un conjunto previo
- **WHEN** el batch más reciente de un sujeto está `pending` o `processing` y existe un
  batch `completed` anterior
- **THEN** el estado vigente es el del batch en curso y el conjunto vigente sigue siendo
  el del batch `completed` anterior

#### Scenario: Fallo posterior a un conjunto previo
- **WHEN** el batch más reciente de un sujeto está `failed` y existe un batch
  `completed` anterior
- **THEN** el estado vigente es `failed` y el conjunto vigente sigue siendo el del batch
  `completed` anterior

#### Scenario: Sujeto sin ningún batch completado
- **WHEN** un sujeto nunca tuvo un batch `completed`
- **THEN** el conjunto vigente está vacío y el estado vigente es el del batch más
  reciente

#### Scenario: Sujeto sin ningún batch
- **WHEN** un sujeto no tiene ningún batch
- **THEN** la resolución informa que no existe estado vigente

### Requirement: Resultado vacío distinguible del fallo
La persistencia MUST permitir que un batch `completed` no tenga ninguna recomendación
asociada, y MUST hacer que ese caso sea distinguible de un batch `failed` por su estado
y no por la cantidad de filas.

#### Scenario: Batch completado sin candidatos
- **WHEN** un batch se completa sin ninguna recomendación
- **THEN** la base de datos conserva el batch con estado `completed` y conjunto vacío

#### Scenario: Batch fallido
- **WHEN** un batch termina en `failed`
- **THEN** su estado lo distingue de un batch `completed` sin recomendaciones

### Requirement: Reemplazo atómico del conjunto
La persistencia SHALL ofrecer una operación única y transaccional que complete un batch,
persista su conjunto de recomendaciones y descarte los batches anteriores del mismo
sujeto. La operación MUST no dejar conjuntos parciales observables, y ante cualquier
fallo MUST conservar intactos el conjunto vigente anterior y el estado previo del batch.

#### Scenario: Reemplazo exitoso
- **WHEN** se completa un batch con un conjunto de recomendaciones
- **THEN** el conjunto vigente del sujeto pasa a ser el nuevo y los batches anteriores
  de ese sujeto dejan de existir junto con sus recomendaciones

#### Scenario: Fallo durante el reemplazo
- **WHEN** la operación de reemplazo falla en cualquiera de sus pasos
- **THEN** el batch conserva su estado previo, no se persiste ninguna recomendación
  nueva y el conjunto vigente anterior permanece disponible

#### Scenario: Ausencia de estados intermedios
- **WHEN** se observa el conjunto vigente de un sujeto mientras se ejecuta un reemplazo
- **THEN** se obtiene el conjunto anterior completo o el nuevo completo, nunca una
  mezcla de ambos

#### Scenario: Reemplazo con conjunto vacío
- **WHEN** se completa un batch con un conjunto vacío
- **THEN** el conjunto vigente del sujeto queda vacío y los batches anteriores dejan de
  existir

#### Scenario: Retención acotada
- **WHEN** un sujeto acumula ejecuciones sucesivas
- **THEN** la persistencia conserva a lo sumo el último batch `completed` y el último
  batch no completado

### Requirement: Exclusión de puestos eliminados
La persistencia MUST excluir de toda lectura de recomendaciones los puestos de trabajo
marcados como eliminados, con independencia de que la recomendación se haya generado
antes de esa eliminación.

#### Scenario: Puesto eliminado después de recomendado
- **WHEN** se elimina lógicamente un puesto que forma parte del conjunto vigente de un
  empleado
- **THEN** las lecturas posteriores de recomendaciones de ese empleado excluyen el
  puesto

#### Scenario: Batch por puesto eliminado
- **WHEN** se consultan los empleados recomendados para un puesto eliminado
  lógicamente
- **THEN** la operación no devuelve recomendaciones

#### Scenario: Puestos activos
- **WHEN** el conjunto vigente contiene puestos activos y eliminados
- **THEN** la lectura devuelve únicamente los activos

### Requirement: Orden y paginación de las recomendaciones
La persistencia SHALL ofrecer lecturas paginadas del conjunto vigente en ambos sentidos.
Los puestos recomendados a un empleado MUST ordenarse por puntaje descendente con
desempate por fecha de publicación del puesto más reciente. Los empleados recomendados
para un puesto MUST ordenarse por puntaje descendente con desempate por actualización de
perfil más reciente. Las recomendaciones sin puntaje MUST ubicarse al final.

#### Scenario: Orden por puntaje
- **WHEN** se lee el conjunto vigente de un sujeto cuyas recomendaciones tienen puntajes
  distintos
- **THEN** las recomendaciones se devuelven de mayor a menor puntaje

#### Scenario: Desempate de puestos
- **WHEN** dos puestos recomendados a un empleado tienen el mismo puntaje
- **THEN** se devuelve primero el puesto publicado más recientemente

#### Scenario: Desempate de empleados
- **WHEN** dos empleados recomendados para un puesto tienen el mismo puntaje
- **THEN** se devuelve primero el empleado cuyo perfil se actualizó más recientemente

#### Scenario: Recomendaciones sin puntaje
- **WHEN** el conjunto vigente mezcla recomendaciones con y sin puntaje
- **THEN** las que no tienen puntaje se devuelven después de todas las que sí lo tienen

#### Scenario: Paginación
- **WHEN** se lee el conjunto vigente indicando un límite y un desplazamiento
- **THEN** la operación devuelve únicamente ese tramo respetando el orden definido

#### Scenario: Soporte por índice
- **WHEN** se lee el conjunto vigente de un sujeto
- **THEN** la consulta se apoya en un índice sobre el batch y el puntaje en lugar de
  recorrer la tabla completa

### Requirement: Acceso tipado a las recomendaciones
La capa de persistencia SHALL ofrecer operaciones tipadas para crear un batch,
transicionar su estado, resolver el batch vigente y el último batch completado de un
sujeto, reemplazar atómicamente el conjunto de recomendaciones y listarlo paginado en
ambos sentidos.

#### Scenario: Transición de estado
- **WHEN** se transiciona un batch a `processing`, `completed` o `failed`
- **THEN** la operación persiste el nuevo estado y refresca la fecha de actualización

#### Scenario: Transición de un batch inexistente
- **WHEN** se intenta transicionar un batch que no existe
- **THEN** la operación no modifica ninguna fila y lo informa

#### Scenario: Lectura por empleado
- **WHEN** se listan los puestos recomendados del conjunto vigente de un empleado
- **THEN** la operación devuelve las recomendaciones con los datos del puesto, su
  puntaje cuando existe y el estado vigente del sujeto

#### Scenario: Lectura por puesto
- **WHEN** se listan los empleados recomendados del conjunto vigente de un puesto
- **THEN** la operación devuelve las recomendaciones con los datos del empleado, su
  puntaje cuando existe y el estado vigente del sujeto
