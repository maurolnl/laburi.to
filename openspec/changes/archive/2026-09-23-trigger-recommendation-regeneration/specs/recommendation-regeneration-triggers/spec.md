## Purpose

Definir qué cambios de dominio solicitan regenerar las recomendaciones de un sujeto —un empleado o
un puesto de trabajo—, bajo qué condición un perfil de empleado pasa a ser elegible, en qué orden
ocurren la persistencia y la emisión, y qué operaciones explícitamente no solicitan nada.

## ADDED Requirements

### Requirement: Disparo tras el alta y la edición de un puesto activo
El sistema SHALL solicitar la regeneración de las recomendaciones de un puesto de trabajo después
de que un alta o una edición de ese puesto quede persistida. La solicitud MUST identificar al
puesto como sujeto y MUST emitirse exactamente una vez por operación exitosa.

#### Scenario: Alta de un puesto
- **WHEN** un empleador crea un puesto de trabajo correctamente
- **THEN** se solicita exactamente una regeneración cuyo sujeto es ese puesto

#### Scenario: Edición de un puesto
- **WHEN** un empleador edita un puesto activo correctamente
- **THEN** se solicita exactamente una regeneración cuyo sujeto es ese puesto

#### Scenario: Ediciones sucesivas de puestos distintos
- **WHEN** se editan dos puestos distintos
- **THEN** cada uno origina su propia solicitud y ninguna sustituye a la otra

### Requirement: Disparo tras completar o editar un perfil de empleado completo
El sistema SHALL solicitar la regeneración de las recomendaciones de un empleado después de que
una escritura de su perfil quede persistida y deje al perfil completo. Alcanza a la creación del
perfil base, a su edición y a la creación y edición de cada sección del perfil. La solicitud MUST
identificar al empleado como sujeto y MUST emitirse exactamente una vez por escritura exitosa que
cumpla la condición.

#### Scenario: Escritura que completa el perfil
- **WHEN** un empleado persiste la sección que le faltaba y su perfil queda completo
- **THEN** se solicita exactamente una regeneración cuyo sujeto es ese empleado

#### Scenario: Edición de una sección de un perfil ya completo
- **WHEN** un empleado edita cualquier sección de un perfil que ya estaba completo y la escritura
  se persiste
- **THEN** se solicita exactamente una regeneración cuyo sujeto es ese empleado

#### Scenario: Edición del registro base de un perfil ya completo
- **WHEN** un empleado edita sus datos base —posición, especialidad, experiencia, certificaciones o
  portafolio— sobre un perfil ya completo
- **THEN** se solicita exactamente una regeneración cuyo sujeto es ese empleado

### Requirement: Perfil incompleto sin trabajo generado
Un perfil de empleado SHALL considerarse completo solo cuando existan su registro base, su
locación, sus recursos técnicos, su disponibilidad y al menos un título de educación. Mientras
falte cualquiera de esos cinco elementos, ninguna escritura del perfil MUST solicitar regeneración
alguna.

#### Scenario: Creación del perfil base
- **WHEN** un usuario crea su perfil base de empleado y todavía no cargó ninguna sección
- **THEN** no se solicita ninguna regeneración

#### Scenario: Perfil con secciones faltantes
- **WHEN** un empleado persiste una sección y su perfil sigue sin alguno de los cinco elementos
- **THEN** no se solicita ninguna regeneración

#### Scenario: Sección presente pero sin datos opcionales
- **WHEN** un empleado registra sus recursos técnicos sin declarar sistema operativo ni software
  pago, y el resto de los elementos está presente
- **THEN** el perfil se considera completo y se solicita la regeneración, porque la sección existe

#### Scenario: Primera solicitud al completarse
- **WHEN** un empleado completa el último elemento que le faltaba
- **THEN** esa escritura, y no ninguna anterior, es la que origina la primera solicitud

### Requirement: Operaciones que no solicitan regeneración
El sistema MUST NOT solicitar regeneración a partir de una eliminación lógica de un puesto, de una
operación rechazada por validación, autorización o error de persistencia, ni de una lectura.
Tampoco existe reapertura de puestos, de modo que no hay ninguna operación que reactive un puesto
eliminado y pueda disparar.

#### Scenario: Eliminación lógica de un puesto
- **WHEN** un empleador elimina lógicamente un puesto
- **THEN** no se solicita ninguna regeneración para ese puesto

#### Scenario: Operación rechazada
- **WHEN** un alta o una edición es rechazada por validación, autorización o error de persistencia
- **THEN** no se solicita ninguna regeneración, porque no hubo cambio persistido

#### Scenario: Consulta de un recurso
- **WHEN** se consulta un puesto, un listado de puestos o un perfil de empleado
- **THEN** no se solicita ninguna regeneración

#### Scenario: Ausencia de reapertura
- **WHEN** se busca una operación que reactive un puesto eliminado y dispare su regeneración
- **THEN** el sistema no expone ninguna

### Requirement: Persistencia confirmada antes de la emisión
El sistema SHALL persistir el cambio de dominio antes de emitir la solicitud, y la respuesta de la
operación MUST NOT depender del cálculo de puntajes ni de la creación de recomendaciones. La
operación MUST resolverse sin esperar a que el trabajo solicitado cambie de estado.

#### Scenario: Orden de las operaciones
- **WHEN** una escritura de dominio dispara una regeneración
- **THEN** el cambio ya está persistido en el momento en que se emite la solicitud

#### Scenario: Respuesta independiente del cálculo
- **WHEN** una escritura de dominio dispara una regeneración
- **THEN** la respuesta se emite sin que se haya evaluado ningún indicador ni persistido ninguna
  recomendación

#### Scenario: Ausencia de espera
- **WHEN** una escritura de dominio dispara una regeneración
- **THEN** la operación no aguarda a que el trabajo pase a `processing`, `completed` ni `failed`

### Requirement: Fallo de emisión sin revertir lo confirmado
Cuando la emisión de la solicitud falla, el sistema MUST NOT revertir el cambio de dominio ya
confirmado ni convertir la operación en un error para el cliente. El fallo SHALL quedar
representado de forma durable por el batch del sujeto en estado `failed` y registrado como
diagnóstico, sin exponer la ubicación de la cola, credenciales ni datos del sujeto más allá de sus
identificadores.

#### Scenario: Alta persistida con emisión fallida
- **WHEN** un puesto se crea correctamente y la emisión de su solicitud falla
- **THEN** la respuesta conserva su código de éxito, el puesto permanece persistido y el batch de
  ese puesto queda en `failed`

#### Scenario: Perfil completado con emisión fallida
- **WHEN** una escritura deja el perfil de un empleado completo y la emisión de su solicitud falla
- **THEN** la respuesta conserva su código de éxito, la escritura permanece persistida y el batch de
  ese empleado queda en `failed`

#### Scenario: Diagnóstico del fallo
- **WHEN** una emisión falla
- **THEN** queda registrado un diagnóstico que identifica al sujeto por su identificador, sin
  filtrar la ubicación de la cola, credenciales ni datos del perfil o del puesto

#### Scenario: Sujeto no bloqueado tras el fallo
- **WHEN** ocurre una escritura posterior sobre un sujeto cuya emisión anterior falló
- **THEN** esa escritura vuelve a solicitar la regeneración, porque el sujeto ya no tiene trabajo en
  curso

### Requirement: Escrituras sucesivas sin reemplazo por un trabajo antiguo
Ante escrituras sucesivas sobre el mismo sujeto, el conjunto vigente resultante MUST corresponder
siempre al trabajo más reciente que haya sido ejecutado. Un trabajo abierto por una escritura
anterior MUST NOT reemplazar el conjunto dejado por uno posterior.

#### Scenario: Segunda escritura durante un trabajo en curso
- **WHEN** se persiste una segunda escritura del mismo sujeto mientras su trabajo anterior sigue en
  curso
- **THEN** no se abre un trabajo adicional y el trabajo en curso es el que produce el conjunto
  vigente

#### Scenario: Escritura posterior al cierre del trabajo
- **WHEN** se persiste una escritura sobre un sujeto cuyo trabajo anterior ya terminó
- **THEN** se solicita una regeneración nueva y su conjunto reemplaza al anterior

#### Scenario: Ausencia de trabajos concurrentes del mismo sujeto
- **WHEN** se observan los trabajos de un sujeto en cualquier instante
- **THEN** a lo sumo uno está en curso, de modo que no existen dos conjuntos compitiendo por
  reemplazar al vigente

### Requirement: Emisión inerte en un entorno que no emite
Cuando el transporte de recomendaciones está deshabilitado, las escrituras de dominio MUST seguir
resolviéndose con normalidad y MUST NOT abrir batches ni dejarlos en `failed`. Un entorno
deliberadamente sin emisión no SHALL acumular trabajo fallido.

#### Scenario: Escritura con el transporte deshabilitado
- **WHEN** se crea o edita un puesto, o se completa un perfil, con el transporte deshabilitado
- **THEN** la operación responde con su código de éxito y no queda ningún batch abierto ni en
  `failed` para ese sujeto

#### Scenario: Diagnóstico del arranque
- **WHEN** el sistema arranca con el transporte deshabilitado
- **THEN** ese estado es visible en el arranque, de modo que la ausencia de recomendaciones no deba
  descubrirse por ausencia de resultados

### Requirement: Disparadores desacoplados del transporte
Los bordes de escritura SHALL solicitar la regeneración a través de un contrato que reciba
únicamente la identidad del sujeto y devuelva el desenlace de la emisión. Ese contrato MUST NOT
mencionar ninguna tecnología de cola, ninguna estructura de mensaje ni ningún atributo del perfil o
del puesto, de modo que sustituir el transporte no obligue a modificar los bordes de escritura.

#### Scenario: Borde de escritura ajeno al proveedor
- **WHEN** un borde de escritura solicita una regeneración
- **THEN** solo indica el identificador del sujeto y observa si la emisión se resolvió o falló

#### Scenario: Sustitución del transporte
- **WHEN** se reemplaza el transporte por otro con las mismas garantías
- **THEN** los bordes de escritura no requieren cambios

#### Scenario: Ausencia de referencia nula
- **WHEN** un entorno no debe emitir solicitudes
- **THEN** los bordes de escritura reciben igualmente una implementación del contrato y no deben
  comprobar si les falta una
