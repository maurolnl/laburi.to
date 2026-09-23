## Purpose

Definir cómo el sistema consume las solicitudes de regeneración de recomendaciones: cuándo
existe el consumidor, cómo interpreta el mensaje, cómo reclama el trabajo sin rehacerlo ante
un redelivery, de dónde salen los candidatos, en qué momento reconoce el mensaje y qué hace
ante cada clase de fallo.

## ADDED Requirements

### Requirement: Consumidor desacoplado del ciclo de request
El sistema SHALL consumir las solicitudes de recomendación en una unidad de ejecución
propia, separada del camino de atención de peticiones HTTP. El consumo MUST NOT bloquear
ningún handler, y un fallo del consumidor MUST NOT afectar la disponibilidad de la API.

#### Scenario: Consumo fuera del camino de request
- **WHEN** el consumidor está procesando una solicitud
- **THEN** la API sigue atendiendo peticiones sin esperar ese procesamiento

#### Scenario: Fallo del consumidor
- **WHEN** el procesamiento de una solicitud falla
- **THEN** la API sigue operativa y el fallo no se propaga a ningún handler

### Requirement: Consumidor opcional y sin efecto cuando está apagado
El consumidor SHALL activarse mediante una opción de configuración propia, distinta de la
que habilita el transporte. Con la opción apagada, el sistema MUST comportarse exactamente
como sin este cambio: ninguna recepción, ningún batch transicionado y ningún mensaje
reconocido. Con la opción encendida pero el transporte deshabilitado, el consumidor MUST NOT
iniciar su ciclo y MUST registrar la razón, en lugar de fallar repetidamente contra un
transporte que no existe.

#### Scenario: Consumidor apagado
- **WHEN** la aplicación arranca con el consumidor deshabilitado
- **THEN** no recibe ningún mensaje, no toca ningún batch y el resto del sistema funciona
  igual que antes

#### Scenario: Consumidor encendido sin transporte
- **WHEN** la aplicación arranca con el consumidor habilitado y el transporte deshabilitado
- **THEN** el ciclo de consumo no se inicia y queda constancia del motivo

#### Scenario: Opciones independientes
- **WHEN** se habilita la emisión sin habilitar el consumo
- **THEN** la aplicación publica solicitudes y no consume ninguna

#### Scenario: Apagado ordenado
- **WHEN** la aplicación recibe la señal de terminar mientras el consumidor espera mensajes
- **THEN** el ciclo deja de recibir y no inicia ningún procesamiento nuevo

#### Scenario: Solicitud en curso durante el apagado
- **WHEN** la aplicación termina mientras una solicitud ya reclamada se está procesando
- **THEN** ese procesamiento se lleva a un estado persistido y no deja el batch bloqueando
  al sujeto

### Requirement: Interpretación del mensaje y contrato desconocido
El consumidor SHALL interpretar el cuerpo del mensaje según el contrato versionado que el
productor emite, y MUST rechazar todo mensaje cuya versión no sea la que implementa o cuyo
cuerpo no pueda interpretarse, en lugar de adivinar su contenido.

#### Scenario: Mensaje válido
- **WHEN** llega un mensaje con la versión implementada y un cuerpo completo
- **THEN** el consumidor obtiene de él el tipo de sujeto, el identificador del sujeto y el
  identificador del batch

#### Scenario: Versión desconocida
- **WHEN** llega un mensaje cuya versión no es la implementada
- **THEN** el consumidor no lo procesa, no toca ningún batch y deja constancia del rechazo

#### Scenario: Cuerpo ilegible
- **WHEN** llega un mensaje cuyo cuerpo no corresponde al contrato
- **THEN** el consumidor no lo procesa, no toca ningún batch y deja constancia del rechazo

### Requirement: Reclamo idempotente del trabajo
El consumidor SHALL reclamar el batch que el mensaje identifica antes de ejecutar trabajo, y
ese reclamo MUST prosperar únicamente si el batch todavía no llegó a un estado terminal. Un
redelivery o un mensaje duplicado MUST NOT rehacer el trabajo ya cerrado ni alterar el
conjunto vigente que ese trabajo dejó.

#### Scenario: Primer procesamiento
- **WHEN** el consumidor reclama un batch en estado `pending`
- **THEN** el reclamo prospera y el batch queda en `processing`

#### Scenario: Redelivery de un batch ya completado
- **WHEN** vuelve a entregarse el mensaje de un batch ya `completed`
- **THEN** el reclamo no prospera, el conjunto vigente queda intacto y el mensaje se
  reconoce

#### Scenario: Redelivery de un batch ya fallido
- **WHEN** vuelve a entregarse el mensaje de un batch ya `failed`
- **THEN** el reclamo no prospera, no se genera ningún conjunto y el mensaje se reconoce

#### Scenario: Batch inexistente
- **WHEN** el mensaje identifica un batch que ya no existe
- **THEN** el consumidor no crea ninguno, no genera recomendaciones y reconoce el mensaje

#### Scenario: Dos entregas del mismo mensaje
- **WHEN** el mismo mensaje se procesa dos veces de punta a punta
- **THEN** el sujeto queda con un único conjunto vigente coherente y sin recomendaciones
  duplicadas

#### Scenario: Reclamo de un batch interrumpido
- **WHEN** vuelve a entregarse el mensaje de un batch que quedó en `processing` porque su
  procesamiento anterior se interrumpió
- **THEN** el reclamo prospera y el sujeto no queda bloqueado por ese batch

### Requirement: Universo de candidatos activos en ambos sentidos
El consumidor SHALL resolver, para el sujeto de la solicitud, el universo de pares
empleado–puesto a evaluar. Para un sujeto empleado, el universo MUST ser el de los puestos
vigentes; para un sujeto puesto, el de los empleados con perfil. Un puesto eliminado
lógicamente MUST NOT aparecer en ningún universo, ni como candidato ni como sujeto.

#### Scenario: Universo de un empleado
- **WHEN** se procesa la solicitud de un empleado
- **THEN** el universo contiene un par por cada puesto vigente y ninguno por los eliminados

#### Scenario: Universo de un puesto
- **WHEN** se procesa la solicitud de un puesto vigente
- **THEN** el universo contiene un par por cada empleado con perfil

#### Scenario: Puesto eliminado como candidato
- **WHEN** un puesto se elimina lógicamente y después se procesa la solicitud de un empleado
- **THEN** ese puesto no se recomienda

#### Scenario: Puesto eliminado como sujeto
- **WHEN** se procesa la solicitud de un puesto que fue eliminado lógicamente después de
  emitirse
- **THEN** no se genera ninguna recomendación para él y su batch queda cerrado sin bloquear
  futuras solicitudes

#### Scenario: Sujeto inexistente
- **WHEN** se procesa la solicitud de un sujeto que ya no existe
- **THEN** no se genera ninguna recomendación y el batch queda cerrado

### Requirement: Entrada de scoring fiel a la ausencia de datos del perfil
El consumidor SHALL traducir el perfil del empleado a la entrada normalizada del contrato de
scoring conservando la distinción entre un dato ausente y un dato con valor cero o vacío. Un
empleado que todavía no completó un paso de su perfil MUST seguir siendo candidato: decidir
si su perfil alcanza es un filtro duro, y los filtros duros no están definidos.

#### Scenario: Perfil completo
- **WHEN** se traduce un empleado con todos los pasos de su perfil completos
- **THEN** la entrada de scoring lleva todos sus atributos comparables con valor

#### Scenario: Pasos del perfil sin completar
- **WHEN** se traduce un empleado sin ubicación, sin disponibilidad o sin educación
- **THEN** esos atributos quedan marcados como ausentes y no como cero ni como vacío

#### Scenario: Recursos informados como ninguno
- **WHEN** un empleado declaró explícitamente no tener recursos técnicos
- **THEN** la entrada distingue ese caso del de no haberlos informado

#### Scenario: Nivel educativo más alto
- **WHEN** un empleado registró varios niveles educativos
- **THEN** la entrada lleva el de mayor nivel según el orden que define el contrato de
  scoring

#### Scenario: Empleado con perfil incompleto como candidato
- **WHEN** se resuelve el universo de un puesto
- **THEN** los empleados con perfil incompleto también forman parte de él

### Requirement: Resultado vacío como desenlace legítimo
Cuando el universo de candidatos del sujeto es vacío, el consumidor SHALL completar el batch
con un conjunto vacío y MUST NOT consultar el contrato de scoring ni tratar la situación como
un fallo.

#### Scenario: Empleado sin puestos vigentes
- **WHEN** se procesa la solicitud de un empleado y no hay ningún puesto vigente
- **THEN** el batch queda `completed` con un conjunto vacío

#### Scenario: Puesto sin empleados
- **WHEN** se procesa la solicitud de un puesto y no hay ningún empleado con perfil
- **THEN** el batch queda `completed` con un conjunto vacío

#### Scenario: Sin consulta al scoring
- **WHEN** el universo de candidatos es vacío
- **THEN** no se invoca ninguna operación del contrato de scoring

#### Scenario: Vacío distinguible del fallo
- **WHEN** un batch se completa con un conjunto vacío
- **THEN** su estado es `completed` y no `failed`

### Requirement: Batch fallido explícito sin scoring productivo
Cuando hay candidatos que evaluar y la implementación de scoring inyectada se declara no
disponible, el consumidor SHALL dejar el batch en `failed` y MUST NOT persistir ninguna
recomendación. El consumidor MUST NOT inventar puntajes, ni fijos, ni aleatorios, ni
derivados de una heurística provisoria, y MUST NOT completar el batch con pares sin puntaje.
La transición MUST producirse sin haber abierto `processing`, de modo que un fallo posterior
no deje al sujeto bloqueado por un trabajo que no podía prosperar.

#### Scenario: Scoring no disponible con candidatos
- **WHEN** se procesa una solicitud con candidatos y el scoring se declara no disponible
- **THEN** el batch queda `failed` y no se persiste ninguna recomendación

#### Scenario: Sin apertura de processing
- **WHEN** el scoring se declara no disponible antes de evaluar
- **THEN** el batch pasa directamente de `pending` a `failed`

#### Scenario: Ausencia de puntajes inventados
- **WHEN** el scoring no está disponible
- **THEN** ninguna recomendación se persiste, con o sin puntaje

#### Scenario: Sujeto disponible para una solicitud nueva
- **WHEN** un batch termina en `failed` por scoring no disponible
- **THEN** el sujeto puede recibir una solicitud nueva sin intervención manual

### Requirement: Reemplazo atómico del conjunto vigente
El consumidor SHALL completar el batch y persistir su conjunto de recomendaciones en una
única transacción, de modo que el conjunto vigente del sujeto pase del anterior al nuevo sin
estados intermedios observables. Ante cualquier fallo, el conjunto vigente anterior MUST
permanecer intacto.

#### Scenario: Reemplazo exitoso
- **WHEN** se completa un batch con un conjunto nuevo
- **THEN** el sujeto pasa a tener exactamente ese conjunto y ninguna recomendación del
  anterior

#### Scenario: Fallo al persistir
- **WHEN** la persistencia del conjunto nuevo falla a mitad de camino
- **THEN** el sujeto conserva íntegro su conjunto vigente anterior y el batch no queda
  `completed`

#### Scenario: Fallo posterior a un conjunto ya vigente
- **WHEN** una solicitud nueva de un sujeto con conjunto vigente termina en `failed`
- **THEN** el conjunto vigente anterior sigue siendo el que se consulta

#### Scenario: Sin conjunto parcial
- **WHEN** se observa el conjunto de un sujeto en cualquier momento del procesamiento
- **THEN** nunca se observa una mezcla del conjunto anterior con el nuevo

### Requirement: Reconocimiento posterior a la persistencia
El consumidor SHALL reconocer un mensaje únicamente después de que el desenlace de su batch
quedó persistido. MUST NOT reconocer un mensaje cuyo trabajo no llegó a un estado persistido,
y un fallo al reconocer un mensaje ya procesado MUST NOT alterar el estado persistido.

#### Scenario: Reconocimiento tras completar
- **WHEN** un batch queda `completed` con su conjunto persistido
- **THEN** recién entonces el mensaje se reconoce

#### Scenario: Reconocimiento tras fallar
- **WHEN** un batch queda `failed` de forma persistida
- **THEN** recién entonces el mensaje se reconoce

#### Scenario: Persistencia fallida
- **WHEN** el desenlace del batch no pudo persistirse
- **THEN** el mensaje no se reconoce y vuelve a entregarse

#### Scenario: Fallo al reconocer
- **WHEN** el batch ya quedó persistido y el reconocimiento del mensaje falla
- **THEN** el estado persistido no cambia y el redelivery se resuelve sin rehacer el trabajo

### Requirement: Errores recuperables frente a errores terminales
El consumidor SHALL clasificar todo fallo en recuperable o terminal. Un fallo recuperable
—infraestructura: base de datos, red o transporte— MUST devolver el mensaje a la cola sin
reconocerlo, de modo que al agotar los reintentos que la cola tiene configurados termine en
la cola de mensajes fallidos. Un fallo terminal —contrato desconocido, cuerpo ilegible,
entrada inválida o scoring no disponible— MUST reconocerse tras dejar el batch en un estado
terminal, porque reintentarlo no puede cambiar el desenlace.

#### Scenario: Fallo de infraestructura
- **WHEN** la base de datos no responde durante el procesamiento
- **THEN** el mensaje no se reconoce y vuelve a entregarse

#### Scenario: Reintentos agotados
- **WHEN** un mensaje vuelve a entregarse hasta agotar los reintentos configurados en la
  cola
- **THEN** el transporte lo deriva a la cola de mensajes fallidos

#### Scenario: Fallo terminal por contrato
- **WHEN** un mensaje tiene una versión desconocida o un cuerpo ilegible
- **THEN** se reconoce sin volver a intentarse

#### Scenario: Fallo terminal por entrada inválida
- **WHEN** el contrato de scoring rechaza un par por entrada inválida
- **THEN** el batch queda `failed` y el mensaje se reconoce

#### Scenario: Un mensaje envenenado no frena la cola
- **WHEN** un mensaje produce un fallo terminal
- **THEN** los mensajes siguientes se procesan igual

### Requirement: Consumidor ajeno al algoritmo y al proveedor de colas
El consumidor MUST depender únicamente de los puertos de scoring, de persistencia y de
transporte, y recibir sus implementaciones por inyección. MUST NOT conocer los indicadores,
sus pesos ni cómo se combinan, y MUST NOT mencionar ningún tipo del SDK del proveedor de
colas.

#### Scenario: Sustitución de la implementación de scoring
- **WHEN** se reemplaza la implementación de scoring por otra
- **THEN** el consumidor no requiere cambios

#### Scenario: Sustitución del transporte
- **WHEN** se reemplaza el transporte por otro con las mismas garantías
- **THEN** el consumidor no requiere cambios

#### Scenario: Prueba sin infraestructura
- **WHEN** se ejercita el consumidor en pruebas
- **THEN** puede hacerse con dobles de los tres puertos, sin cola real ni credenciales
