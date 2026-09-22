# recommendation-job-publishing Specification

## Purpose
Definir cómo el sistema solicita la regeneración de recomendaciones de un sujeto —un
empleado o un puesto de trabajo— sin calcularlas: qué viaja en el mensaje, en qué orden se
abre la ejecución que lo respalda, cuándo no corresponde emitir nada y qué ocurre cuando la
emisión falla.

## Requirements

### Requirement: Emisión desacoplada del transporte
El sistema SHALL exponer la emisión de solicitudes de recomendación como un puerto cuya
única operación recibe el sujeto de la solicitud y devuelve el desenlace de la emisión. Ese
contrato MUST NOT mencionar ningún proveedor de colas, ninguna estructura del mensaje ni
ningún tipo del SDK correspondiente, de modo que sustituir el transporte no obligue a
modificar a sus llamadores.

#### Scenario: Llamador ajeno al proveedor
- **WHEN** un llamador emite una solicitud de recomendación
- **THEN** solo indica el sujeto y observa si la emisión se resolvió o falló, sin conocer
  la cola, su ubicación ni el formato del mensaje

#### Scenario: Sustitución del transporte
- **WHEN** se reemplaza el transporte por otro con las mismas garantías
- **THEN** los llamadores no requieren cambios

#### Scenario: Sujeto en ambos sentidos
- **WHEN** se emite una solicitud para un empleado y otra para un puesto de trabajo
- **THEN** ambas usan la misma operación y se distinguen únicamente por el sujeto indicado

#### Scenario: Implementación inerte disponible
- **WHEN** un entorno no debe emitir solicitudes
- **THEN** existe una implementación del puerto que no emite nada y se resuelve sin error,
  de modo que ningún llamador reciba una referencia nula ni deba comprobarla

### Requirement: Contrato del mensaje versionado
El mensaje emitido SHALL contener exactamente el identificador del evento, la versión del
contrato, el tipo de sujeto, el identificador del sujeto, el identificador del batch que
respalda la solicitud y el instante de emisión. La versión MUST viajar en todo mensaje para
que un consumidor pueda rechazar un contrato que no entiende en lugar de interpretarlo mal.

#### Scenario: Mensaje de un empleado
- **WHEN** se emite una solicitud para un empleado
- **THEN** el cuerpo del mensaje declara el tipo de sujeto `employee`, el identificador de
  ese empleado, el identificador del batch abierto, un identificador de evento, la versión
  del contrato y el instante de emisión

#### Scenario: Mensaje de un puesto de trabajo
- **WHEN** se emite una solicitud para un puesto de trabajo
- **THEN** el cuerpo del mensaje declara el tipo de sujeto `job_position` y el identificador
  de ese puesto, con los mismos campos restantes y el mismo formato

#### Scenario: Instante en escala universal
- **WHEN** se serializa el instante de emisión
- **THEN** se expresa en UTC y en un formato que no dependa de la zona horaria del proceso
  que emitió el mensaje

#### Scenario: Consumidor ante una versión desconocida
- **WHEN** un consumidor recibe un mensaje cuya versión no es la que implementa
- **THEN** puede detectarlo leyendo únicamente el campo de versión, sin inferirlo de la
  presencia o ausencia de otros campos

### Requirement: Mensaje sin datos sensibles ni perfil completo
El cuerpo del mensaje MUST NOT contener datos personales, credenciales, tokens de
autenticación ni el perfil del empleado o la descripción del puesto. El consumidor SHALL
resolver desde la base de datos todo dato que necesite, a partir de los identificadores que
el mensaje transporta.

#### Scenario: Ausencia de datos personales
- **WHEN** se inspecciona el cuerpo de un mensaje emitido
- **THEN** no aparece ningún nombre, correo electrónico, teléfono, certificación, URL de
  portafolio ni ningún otro atributo del perfil o del puesto

#### Scenario: Ausencia de credenciales
- **WHEN** se emite una solicitud originada en una operación autenticada
- **THEN** el mensaje no transporta el token de autenticación, la identidad del usuario ni
  ningún otro dato de la sesión que la originó

#### Scenario: Suficiencia de los identificadores
- **WHEN** el consumidor recibe el mensaje
- **THEN** dispone del tipo de sujeto, su identificador y el del batch, que es todo lo que
  necesita para leer el estado vigente desde la base de datos

### Requirement: Batch abierto antes de publicar
El sistema SHALL abrir el batch `pending` del sujeto antes de publicar el mensaje, y MUST
publicar el identificador de ese batch ya persistido. Ningún mensaje puede referirse a un
batch inexistente.

#### Scenario: Orden de las operaciones
- **WHEN** se emite una solicitud para un sujeto sin trabajo en curso
- **THEN** primero queda persistido un batch `pending` para ese sujeto y solo después se
  publica el mensaje que lo referencia

#### Scenario: Fallo al abrir el batch
- **WHEN** la apertura del batch falla
- **THEN** no se publica ningún mensaje y la emisión se reporta como fallida

#### Scenario: Sujeto inválido
- **WHEN** se emite una solicitud cuyo sujeto no identifica exactamente a un empleado o a un
  puesto
- **THEN** la emisión se reporta como fallida sin abrir ningún batch ni publicar ningún
  mensaje

### Requirement: Deduplicación frente a un trabajo ya en curso
Cuando el sujeto ya tiene un batch `pending` o `processing`, el sistema MUST NOT abrir un
batch nuevo ni publicar un mensaje, y SHALL resolver la emisión sin error: el trabajo en
curso ya va a producir el conjunto vigente del sujeto.

#### Scenario: Segunda solicitud durante un trabajo en curso
- **WHEN** se emite una solicitud para un sujeto que ya tiene un batch `pending` o
  `processing`
- **THEN** no se publica ningún mensaje adicional, no se crea ningún batch adicional y la
  emisión se resuelve sin error

#### Scenario: Solicitud posterior al cierre del trabajo
- **WHEN** se emite una solicitud para un sujeto cuyo último batch está `completed` o
  `failed`
- **THEN** se abre un batch nuevo y se publica su mensaje

#### Scenario: Sujetos distintos
- **WHEN** un sujeto tiene un batch en curso y se emite una solicitud para otro sujeto
- **THEN** el trabajo en curso del primero no impide la emisión del segundo

#### Scenario: Visibilidad de la deducción
- **WHEN** una solicitud se resuelve sin publicar por haber trabajo en curso
- **THEN** queda registrado un diagnóstico que lo indica, sin exponer la ubicación de la
  cola ni datos del sujeto más allá de sus identificadores

### Requirement: Fallo de publicación sin éxito falso
Cuando el mensaje no se puede publicar, el sistema MUST reportar la emisión como fallida y
MUST NOT dejar el batch recién abierto en un estado en curso: ese batch SHALL pasar a
`failed`, de modo que el sujeto no quede bloqueado para solicitudes posteriores. Un
transporte deshabilitado es un fallo de publicación, no un éxito.

#### Scenario: Error del transporte
- **WHEN** la publicación del mensaje falla
- **THEN** el batch recién abierto queda en estado `failed` y la emisión se reporta como
  fallida

#### Scenario: Transporte deshabilitado
- **WHEN** se emite una solicitud con el transporte deshabilitado
- **THEN** la emisión se reporta como fallida y el batch abierto queda en `failed`, sin
  simular que el mensaje se envió

#### Scenario: Sujeto no bloqueado tras el fallo
- **WHEN** se emite una solicitud nueva para un sujeto cuya emisión anterior falló al
  publicar
- **THEN** la nueva solicitud abre un batch y publica su mensaje, porque el sujeto ya no
  tiene trabajo en curso

#### Scenario: Fallo al marcar el batch
- **WHEN** la publicación falla y además falla el intento de dejar el batch en `failed`
- **THEN** la emisión se reporta como fallida informando la causa original de la publicación

### Requirement: Identificadores aptos para deduplicación e idempotencia
Cada emisión SHALL llevar un identificador de evento distinto del de cualquier otra emisión,
y el identificador del batch MUST identificar el trabajo que el consumidor debe ejecutar. Dos
entregas del mismo mensaje MUST referirse al mismo batch, para que un consumidor idempotente
no duplique trabajo.

#### Scenario: Identificador de evento único
- **WHEN** se emiten dos solicitudes distintas, incluso para el mismo sujeto
- **THEN** cada mensaje lleva un identificador de evento diferente

#### Scenario: Reentrega del mismo mensaje
- **WHEN** el transporte entrega dos veces el mismo mensaje
- **THEN** ambas entregas declaran el mismo identificador de evento y el mismo identificador
  de batch

#### Scenario: Trabajo identificado por el batch
- **WHEN** el consumidor recibe un mensaje
- **THEN** el identificador del batch le alcanza para decidir si ese trabajo ya fue
  ejecutado, sin comparar el resto del cuerpo

### Requirement: Emisión desacoplada del cálculo
La emisión SHALL limitarse a abrir el batch y publicar el mensaje. MUST NOT calcular
puntajes, leer el perfil del empleado ni la descripción del puesto, ni esperar a que el
consumidor procese el trabajo.

#### Scenario: Ausencia de cálculo
- **WHEN** se emite una solicitud
- **THEN** no se evalúa ningún indicador ni se persiste ninguna recomendación

#### Scenario: Resolución inmediata
- **WHEN** se emite una solicitud
- **THEN** la operación se resuelve en cuanto el mensaje queda publicado, sin esperar a que
  el batch cambie de estado

#### Scenario: Origen en una operación de escritura
- **WHEN** una operación de alta o edición emite una solicitud tras haber persistido su
  propio cambio
- **THEN** su respuesta no depende de que las recomendaciones estén generadas
