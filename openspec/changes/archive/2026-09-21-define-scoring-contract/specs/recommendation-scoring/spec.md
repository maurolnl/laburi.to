## Purpose

Definir el contrato inyectable con el que se puntúa la afinidad entre un empleado y un
puesto de trabajo: su entrada normalizada, la separación entre descarte por filtro duro
y cálculo de puntaje, la forma de un resultado que admite indicadores parciales, y el
modo de fallo explícito mientras el algoritmo de indicadores no exista. La capacidad
cubre el contrato y sus modos de fallo, nunca el algoritmo.

## ADDED Requirements

### Requirement: Entrada normalizada e independiente del origen
El contrato de scoring SHALL recibir los atributos comparables del empleado y del puesto
en una forma normalizada propia, y MUST ser evaluable sin acceder a la base de datos, a
la petición HTTP ni a los paquetes que modelan el perfil de empleado o el puesto. La
traducción desde esos modelos hacia la entrada normalizada MUST ser responsabilidad del
llamador y no del contrato.

#### Scenario: Puntuación sin infraestructura
- **WHEN** se evalúa un par empleado-puesto con la entrada ya normalizada
- **THEN** la evaluación se resuelve sin consultar la base de datos ni ningún servicio
  externo al contrato

#### Scenario: Atributos comparables presentes
- **WHEN** se arma la entrada de un par
- **THEN** incluye, para ambos lados, los atributos que el dominio declara comparables:
  experiencia, nivel educativo, horas diarias disponibles, timezone y recursos técnicos

#### Scenario: Identificación del par
- **WHEN** se evalúa un par
- **THEN** la entrada conserva los identificadores del empleado y del puesto, de modo que
  el resultado pueda persistirse como una recomendación sin correlación adicional

#### Scenario: Atributo ausente en el perfil
- **WHEN** un empleado o un puesto no tiene valor para alguno de los atributos
  comparables
- **THEN** la entrada representa esa ausencia de forma explícita y distinta de un valor
  vacío o cero

### Requirement: Separación entre filtro duro y cálculo de puntaje
El contrato SHALL exponer la decisión de elegibilidad y el cálculo de puntaje como dos
responsabilidades distintas. El filtro duro MUST decidir si un par puede recomendarse sin
producir ningún puntaje, y el cálculo MUST aplicarse sobre pares ya elegibles. Ninguna de
las dos MUST persistir resultados: la persistencia es responsabilidad de la capacidad
`recommendation-persistence`.

#### Scenario: Par descartado por filtro duro
- **WHEN** el filtro duro descarta un par
- **THEN** el resultado lo declara inelegible con un motivo identificable y sin puntaje

#### Scenario: Par elegible
- **WHEN** el filtro duro acepta un par
- **THEN** el par queda habilitado para el cálculo de puntaje

#### Scenario: Cálculo sin filtro
- **WHEN** se sustituye la implementación del filtro duro
- **THEN** la implementación del cálculo de puntaje no requiere cambios, y a la inversa

#### Scenario: Ausencia de escritura
- **WHEN** se evalúa cualquier par
- **THEN** el contrato no escribe recomendaciones, batches ni ningún otro registro

### Requirement: Resultado con indicadores parciales
El resultado de puntuar un par SHALL admitir un puntaje total opcional acompañado de la
lista de indicadores que lo componen, cada uno con su nombre y su aporte. Incorporar,
quitar o repesar indicadores MUST ser posible sin modificar la forma del resultado, el
transporte entre worker y persistencia ni el esquema en el que se guarda el puntaje.

#### Scenario: Resultado con indicadores
- **WHEN** una implementación calcula un puntaje a partir de varios indicadores
- **THEN** el resultado expone el total y el detalle de cada indicador que lo compone

#### Scenario: Implementación con un subconjunto de indicadores
- **WHEN** una implementación calcula solo algunos de los indicadores previstos
- **THEN** el resultado sigue siendo válido y expone únicamente los indicadores
  calculados

#### Scenario: Incorporación de un indicador nuevo
- **WHEN** una implementación futura agrega un indicador
- **THEN** ni la forma del resultado, ni el transporte, ni el esquema de persistencia del
  puntaje requieren cambios

#### Scenario: Persistencia del total
- **WHEN** un resultado con puntaje total se persiste como recomendación
- **THEN** se guarda el total y el detalle de indicadores no forma parte del esquema
  principal

### Requirement: Desenlaces distinguibles de la evaluación de un par
El contrato MUST distinguir tres desenlaces de la evaluación de un par: puntuado,
inelegible por filtro duro y elegible pero sin puntaje disponible. La ausencia de puntaje
MUST ser distinta de un puntaje igual a cero y MUST ser distinta de un fallo de la
evaluación.

#### Scenario: Par puntuado
- **WHEN** la evaluación produce un puntaje
- **THEN** el resultado es elegible y expone el puntaje, incluido cuando vale cero

#### Scenario: Par inelegible
- **WHEN** la evaluación descarta el par por filtro duro
- **THEN** el resultado es inelegible, no expone puntaje e indica el motivo

#### Scenario: Par elegible sin puntaje
- **WHEN** la evaluación acepta el par pero no puede producir un puntaje
- **THEN** el resultado es elegible y el puntaje queda ausente, sin que eso constituya un
  error

#### Scenario: Puntaje cero frente a puntaje ausente
- **WHEN** se comparan un resultado con puntaje cero y uno sin puntaje
- **THEN** ambos son distinguibles entre sí

### Requirement: Evaluación individual y por lote
El contrato SHALL permitir puntuar un único par y puntuar un lote de pares en una sola
invocación. La evaluación por lote MUST devolver exactamente un resultado por par de
entrada, en el mismo orden, y MUST producir para cada par el mismo resultado que la
evaluación individual de ese par con la misma implementación.

#### Scenario: Correspondencia uno a uno
- **WHEN** se evalúa un lote de pares
- **THEN** se obtiene un resultado por cada par, en el mismo orden en que se enviaron

#### Scenario: Equivalencia entre lote e individual
- **WHEN** se evalúa el mismo par de forma individual y dentro de un lote con la misma
  implementación
- **THEN** ambos resultados coinciden

#### Scenario: Lote vacío
- **WHEN** se evalúa un lote sin pares
- **THEN** se obtiene un conjunto vacío de resultados y ningún error

#### Scenario: Lote con pares inelegibles
- **WHEN** un lote mezcla pares elegibles e inelegibles
- **THEN** cada par conserva su propio desenlace y los inelegibles no invalidan el resto

### Requirement: Implementación de producción explícitamente no disponible
Mientras el algoritmo de indicadores no exista, la implementación de producción del
contrato MUST rechazar toda evaluación con un fallo identificable de dependencia no
disponible, y MUST NOT devolver puntajes, ni fijos, ni aleatorios, ni derivados de una
heurística provisoria. El fallo MUST ser distinguible por el llamador de cualquier otro
error, para que pueda traducirse a un batch fallido.

#### Scenario: Evaluación individual en producción
- **WHEN** se evalúa un par con la implementación de producción
- **THEN** la operación falla con el error de dependencia no disponible y no devuelve
  ningún puntaje

#### Scenario: Evaluación por lote en producción
- **WHEN** se evalúa un lote con al menos un par con la implementación de producción
- **THEN** la operación falla con el mismo error y no devuelve resultados parciales

#### Scenario: Lote vacío en producción
- **WHEN** se evalúa un lote sin pares con la implementación de producción
- **THEN** la operación devuelve un conjunto vacío sin error, porque no hay nada que
  puntuar y ninguna dependencia hace falta

#### Scenario: Filtro duro en producción
- **WHEN** se consulta la elegibilidad de un par con la implementación de producción
- **THEN** la operación falla con el mismo error en lugar de declarar el par elegible o
  inelegible

#### Scenario: Clasificación del fallo por el llamador
- **WHEN** un llamador recibe el fallo de la implementación de producción
- **THEN** puede clasificarlo como dependencia no disponible y distinguirlo de un error
  de entrada inválida o de infraestructura

### Requirement: Fake determinista limitado a los tests
El contrato SHALL contar con una implementación falsa de comportamiento determinista y
configurable para probar a sus consumidores. Esa implementación MUST NOT formar parte del
binario de producción ni ser alcanzable desde la composición de dependencias de la
aplicación.

#### Scenario: Reproducibilidad
- **WHEN** el fake evalúa dos veces el mismo par con la misma configuración
- **THEN** devuelve exactamente el mismo resultado

#### Scenario: Cobertura de desenlaces
- **WHEN** un test necesita un par puntuado, uno inelegible, uno sin puntaje o un fallo
- **THEN** el fake puede producir cualquiera de esos desenlaces bajo control del test

#### Scenario: Exclusión del binario de producción
- **WHEN** se compila la aplicación
- **THEN** el fake no queda incluido y la composición de dependencias no puede
  seleccionarlo

### Requirement: Consumidores desacoplados del algoritmo
Los consumidores del contrato — productor de trabajos, worker de generación y endpoints
de consulta — MUST depender únicamente de la interfaz y recibir su implementación por
inyección. Ningún consumidor MUST conocer los indicadores, sus pesos ni la forma en que
se combinan, y reemplazar la implementación MUST NOT alterar rutas, cuerpos, códigos de
estado ni el esquema de persistencia.

#### Scenario: Inyección de la implementación
- **WHEN** un consumidor necesita puntuar pares
- **THEN** recibe la implementación desde afuera y no la construye ni la selecciona por
  sí mismo

#### Scenario: Sustitución de la implementación
- **WHEN** se reemplaza la implementación de scoring por otra
- **THEN** no cambian las rutas HTTP, los cuerpos de respuesta, los códigos de estado ni
  el esquema de persistencia

#### Scenario: Desconocimiento de los indicadores
- **WHEN** se agrega, quita o repesa un indicador
- **THEN** ningún consumidor requiere cambios
