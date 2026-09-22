## Purpose

Definir cómo se configura y se accede al transporte asíncrono que lleva las solicitudes de
recomendación entre el productor y el worker: qué valores son obligatorios y bajo qué
condición, cómo se validan, cómo falla el arranque sin revelar secretos, qué ocurre cuando
el transporte está deshabilitado, qué operaciones expone su puerto y qué garantías de
entrega y reintento deben asumir sus consumidores. La capacidad cubre la configuración y
el acceso, nunca qué se publica ni cómo se procesa.

## ADDED Requirements

### Requirement: Configuración explícita del transporte
La configuración del transporte de recomendaciones SHALL resolverse desde el entorno en un
único punto durante el arranque, y MUST quedar disponible como un valor pasado
explícitamente a quien la necesita. Ningún componente MUST leer el entorno por su cuenta,
mantener el cliente en estado global ni construirlo en tiempo de importación.

#### Scenario: Resolución única en el arranque
- **WHEN** el proceso arranca
- **THEN** la configuración del transporte se resuelve una sola vez y se entrega a los
  componentes que la necesitan

#### Scenario: Ausencia de estado global
- **WHEN** se inspecciona el código que provee el transporte
- **THEN** no existe variable global mutable, inicialización implícita ni lectura del
  entorno fuera de la resolución de arranque

#### Scenario: Validación sin depender del entorno del proceso
- **WHEN** se ejercita la resolución de la configuración con un conjunto de valores dado
- **THEN** es posible hacerlo sin modificar las variables de entorno del proceso que
  ejecuta la prueba

### Requirement: Valores obligatorios del transporte habilitado
Con el transporte habilitado, la configuración SHALL exigir la región, la ubicación de la
cola principal y la ubicación de la cola de mensajes fallidos. Cada uno de esos valores
MUST estar presente y no vacío; su ausencia MUST impedir el arranque.

#### Scenario: Falta la ubicación de la cola principal
- **WHEN** el transporte está habilitado y no se declara la ubicación de la cola principal
- **THEN** el arranque falla e identifica cuál es el valor faltante

#### Scenario: Falta la cola de mensajes fallidos
- **WHEN** el transporte está habilitado y no se declara la ubicación de la cola de
  mensajes fallidos
- **THEN** el arranque falla, de modo que ningún despliegue quede corriendo sin destino
  para los mensajes que agotan sus reintentos

#### Scenario: Valor declarado pero vacío
- **WHEN** un valor obligatorio está declarado con contenido vacío o solo espacios
- **THEN** se trata como ausente y el arranque falla

### Requirement: Parámetros de consumo con valor por defecto y rango validado
La configuración SHALL admitir el visibility timeout, el tiempo de espera de long polling,
la cantidad máxima de mensajes por recepción y el máximo de reintentos del cliente como
valores opcionales con un valor por defecto seguro. Cada valor declarado MUST validarse
contra el rango que el servicio de colas admite, y un valor fuera de rango o no numérico
MUST impedir el arranque en vez de corregirse en silencio.

#### Scenario: Valor omitido
- **WHEN** no se declara uno de estos parámetros
- **THEN** la configuración adopta su valor por defecto documentado y el arranque continúa

#### Scenario: Valor fuera de rango
- **WHEN** se declara un tiempo de long polling mayor al máximo admitido por el servicio
- **THEN** el arranque falla e identifica el parámetro, en vez de acotarlo silenciosamente

#### Scenario: Valor no numérico
- **WHEN** se declara un parámetro numérico con un texto que no es un número
- **THEN** el arranque falla e identifica el parámetro

#### Scenario: Long polling activo por defecto
- **WHEN** no se declara el tiempo de espera de recepción
- **THEN** el valor por defecto habilita long polling en lugar de recepción inmediata

### Requirement: Fallo temprano con mensajes seguros
Todo error de configuración del transporte SHALL producirse durante el arranque y antes de
atender la primera petición, y MUST ser clasificable por el llamador sin inspeccionar el
texto del error. El mensaje MUST nombrar la variable responsable y MUST NOT contener su
valor, credenciales, identificadores de cuenta ni la ubicación completa de ninguna cola.

#### Scenario: Fallo antes de atender tráfico
- **WHEN** la configuración del transporte es inválida
- **THEN** el proceso termina durante el arranque y no llega a atender ninguna petición
  HTTP

#### Scenario: Mensaje sin secretos
- **WHEN** se emite un error de configuración
- **THEN** el mensaje identifica la variable responsable y no incluye su valor ni ningún
  dato sensible

#### Scenario: Clasificación del error
- **WHEN** un llamador recibe un error de configuración
- **THEN** puede distinguir un valor faltante de un valor inválido sin comparar cadenas de
  texto

#### Scenario: Diagnóstico sin exponer la cola
- **WHEN** el arranque registra el estado del transporte
- **THEN** el registro indica si está habilitado y no revela la ubicación de la cola ni
  credenciales

### Requirement: Transporte deshabilitado como estado legítimo
El transporte SHALL poder quedar deshabilitado, y en ese estado la aplicación MUST arrancar
con normalidad y MUST conservar sin cambios todo su contrato HTTP. Con el transporte
deshabilitado, el acceso MUST seguir estando disponible como valor no nulo y toda operación
sobre él MUST fallar de forma explícita e identificable, nunca simular éxito ni perder un
mensaje en silencio.

#### Scenario: Arranque sin cola configurada
- **WHEN** el transporte no está habilitado y no se declara ninguna de sus variables
- **THEN** la aplicación arranca y responde su contrato HTTP actual sin cambios

#### Scenario: Operación sobre un transporte deshabilitado
- **WHEN** un consumidor intenta usar el transporte deshabilitado
- **THEN** la operación falla con un error identificable como "transporte no habilitado" y
  no reporta éxito

#### Scenario: Ausencia de valor nulo
- **WHEN** un consumidor recibe el transporte deshabilitado
- **THEN** recibe un valor utilizable y no un valor nulo que deba comprobar antes de cada
  llamada

### Requirement: Puerto de cola independiente del proveedor
El acceso al transporte SHALL exponerse como un contrato con las operaciones que el
productor y el consumidor necesitan: enviar un mensaje, recibir un lote de mensajes,
confirmar el procesamiento de un mensaje y extender su plazo de procesamiento. Ese contrato
MUST expresarse con tipos propios y MUST NOT obligar a sus consumidores a conocer el
proveedor de colas ni sus tipos.

#### Scenario: Consumidor ajeno al proveedor
- **WHEN** se inspeccionan las firmas del contrato de cola
- **THEN** ninguna expone tipos del proveedor de colas

#### Scenario: Sustitución del proveedor
- **WHEN** se reemplaza la implementación del contrato por otra sobre un proveedor distinto
- **THEN** el productor y el consumidor no requieren cambios

#### Scenario: Operaciones necesarias presentes
- **WHEN** un consumidor procesa un mensaje
- **THEN** el contrato le permite confirmarlo tras procesarlo y extender su plazo mientras
  lo procesa

#### Scenario: Identificador de confirmación transportado
- **WHEN** se recibe un mensaje
- **THEN** el mensaje incluye el identificador necesario para confirmarlo y extender su
  plazo, sin que el consumidor deba derivarlo

### Requirement: Sustitución del transporte en pruebas
El transporte SHALL poder sustituirse por un doble en memoria desde las pruebas de
cualquier paquete que lo consuma, sin conexión de red ni credenciales. El doble MUST
reproducir las garantías observables del transporte real —entrega al menos una vez,
invisibilidad temporal de un mensaje recibido, redelivery al vencer ese plazo y conteo de
recepciones— y MUST NOT formar parte del binario de producción.

#### Scenario: Prueba sin red ni credenciales
- **WHEN** se ejecuta la suite completa sin credenciales AWS y sin acceso a la red
- **THEN** todas las pruebas que usan el transporte pasan

#### Scenario: Doble usable desde otro paquete
- **WHEN** un paquete distinto al del transporte prueba un componente que lo consume
- **THEN** puede usar el doble sin duplicar su propia implementación

#### Scenario: Redelivery observable
- **WHEN** un mensaje se recibe del doble y no se confirma antes de vencer su plazo
- **THEN** vuelve a entregarse en una recepción posterior, con su conteo de recepciones
  incrementado

#### Scenario: Doble fuera del binario
- **WHEN** se inspeccionan las dependencias de la ruta de producción
- **THEN** ninguna alcanza al doble de pruebas

### Requirement: Garantías de entrega y reintento declaradas
El transporte SHALL documentar que la entrega es al menos una vez, que un mensaje no
confirmado se vuelve a entregar al vencer su plazo de procesamiento, y que tras agotar el
máximo de recepciones el mensaje termina en la cola de mensajes fallidos. Los consumidores
MUST diseñarse asumiendo esas garantías, y la política de reintento y su destino MUST estar
declarados en la configuración y la documentación aunque su aprovisionamiento ocurra fuera
de la aplicación.

#### Scenario: Procesamiento idempotente asumido
- **WHEN** se documenta el contrato para sus consumidores
- **THEN** declara explícitamente que el mismo mensaje puede entregarse más de una vez y
  que el procesamiento debe tolerarlo

#### Scenario: Destino de los mensajes agotados
- **WHEN** un mensaje agota el máximo de recepciones configurado en la cola
- **THEN** el destino documentado es la cola de mensajes fallidos declarada en la
  configuración

#### Scenario: Atributos de la cola documentados
- **WHEN** alguien necesita aprovisionar el transporte en un entorno nuevo
- **THEN** la documentación enumera los atributos que la cola y su cola de mensajes
  fallidos deben tener, sin incluir ningún secreto

### Requirement: Configuración documentada sin secretos
La documentación del transporte SHALL describir el arranque local y el desplegado, y MUST
enumerar todas sus variables de entorno con su obligatoriedad, su valor por defecto y su
rango válido. Ningún artefacto versionado MUST contener credenciales, identificadores de
cuenta ni ubicaciones reales de colas.

#### Scenario: Plantilla de entorno completa
- **WHEN** alguien configura el proyecto desde la plantilla de variables de entorno
- **THEN** encuentra todas las variables del transporte declaradas y sin valor asignado

#### Scenario: Entorno local sin AWS
- **WHEN** alguien quiere ejercitar el transporte en su máquina
- **THEN** la documentación describe cómo apuntarlo a un servicio de colas local en lugar
  del servicio real

#### Scenario: Ausencia de secretos versionados
- **WHEN** se revisan los archivos versionados por este cambio
- **THEN** ninguno contiene credenciales, identificadores de cuenta ni ubicaciones reales
  de colas
