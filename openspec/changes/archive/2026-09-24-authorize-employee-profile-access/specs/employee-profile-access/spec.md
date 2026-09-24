## Purpose

Definir el contrato HTTP con el que se lee el perfil completo de un empleado por su
identificador de empleado y se obtiene acceso temporal a sus certificados: qué rutas existen,
qué actor puede invocarlas y bajo qué vínculo, qué datos ve cada uno, cómo se entrega un
archivo privado sin nombrar su ubicación y qué código responde cada situación.

## ADDED Requirements

### Requirement: Lectura del perfil completo por identificador de empleado
El sistema SHALL exponer `GET /employees/{employeeID}`, que devuelve el perfil completo de un
empleado: datos base, locación, recursos técnicos, disponibilidad, educación y certificados.
La ruta MUST requerir un JWT válido y MUST responder `200` con un cuerpo JSON cuando el
solicitante está autorizado.

El identificador del path MUST ser un entero positivo y MUST NOT resolver identidad: la
identidad del solicitante sale siempre del JWT.

#### Scenario: Empleado consulta su propio perfil
- **WHEN** un empleado autenticado consulta el perfil cuyo identificador le pertenece
- **THEN** la respuesta es `200` con su perfil completo

#### Scenario: Sin token
- **WHEN** la petición llega sin JWT válido
- **THEN** la respuesta es `401` y no se revela si el empleado existe

#### Scenario: Identificador inválido
- **WHEN** el identificador del path no es un entero positivo
- **THEN** la respuesta es `400`

### Requirement: Acceso del empleador condicionado a una recomendación vigente
El sistema SHALL permitir que un usuario con rol `employer` lea el perfil de un empleado
solamente mientras exista una recomendación vigente que vincule a ese empleado con alguno de
sus puestos activos. El vínculo MUST considerarse vigente en cualquiera de las dos direcciones:
cuando el puesto aparece en el conjunto vigente del empleado, o cuando el empleado aparece en
el conjunto vigente del puesto.

Un puesto eliminado lógicamente MUST NOT sostener el vínculo. Un conjunto vigente reemplazado
por un batch posterior que ya no contiene el par MUST dejar de sostenerlo.

#### Scenario: Empleador con recomendación vigente
- **WHEN** un empleador autenticado consulta el perfil de un empleado recomendado para uno de
  sus puestos activos
- **THEN** la respuesta es `200` con el perfil del empleado

#### Scenario: Empleador sin vínculo
- **WHEN** un empleador autenticado consulta el perfil de un empleado que no está vinculado a
  ninguno de sus puestos
- **THEN** la respuesta es `403`

#### Scenario: Puesto eliminado
- **WHEN** el único puesto que sostenía el vínculo fue eliminado lógicamente
- **THEN** la respuesta pasa a ser `403`

#### Scenario: Recomendación reemplazada
- **WHEN** un batch posterior reemplaza el conjunto vigente y el empleado ya no figura en él
- **THEN** la respuesta pasa a ser `403`

#### Scenario: Empleador sin perfil de empleador
- **WHEN** un usuario con rol `employer` que todavía no creó su perfil consulta un perfil de
  empleado
- **THEN** la respuesta es `403`

### Requirement: Una sola respuesta para toda falla de autorización del perfil
El sistema MUST responder `403` con un único mensaje ante cualquier falla de autorización de
`GET /employees/{employeeID}`: empleado ajeno, empleador sin vínculo vigente y empleado
inexistente. Las tres situaciones MUST ser indistinguibles en la respuesta, de modo que nadie
pueda descubrir qué identificadores de empleado existen recorriéndolos.

#### Scenario: Empleado ajeno
- **WHEN** un empleado autenticado consulta el perfil de otro empleado
- **THEN** la respuesta es `403`

#### Scenario: Empleado inexistente
- **WHEN** se consulta un identificador de empleado que no existe
- **THEN** la respuesta es `403`, idéntica a la del empleado ajeno

### Requirement: El perfil no revela la ubicación de ningún archivo
El cuerpo de `GET /employees/{employeeID}` MUST NOT contener el bucket ni la clave de objeto de
ningún archivo, ni ninguna otra coordenada que permita alcanzarlo directamente. Cada
certificado y cada documento de título MUST viajar identificado por un identificador estable
que solo sirve para pedir su entrega por las rutas de descarga.

#### Scenario: Certificados identificados
- **WHEN** el empleado tiene certificados cargados
- **THEN** cada uno aparece en el perfil con su identificador y su nombre original, y sin
  bucket ni clave de objeto

#### Scenario: Documentos de título identificados
- **WHEN** un título de educación tiene un documento asociado
- **THEN** ese título aparece con el identificador con el que se pide la entrega del documento,
  y sin bucket ni clave de objeto

#### Scenario: Título sin documento
- **WHEN** un título de educación no tiene documento asociado
- **THEN** el identificador de entrega viaja vacío y el título se devuelve igual

### Requirement: El empleador no recibe el correo del empleado
El sistema MUST omitir el correo electrónico del empleado cuando quien lee el perfil es un
empleador. El correo MUST estar presente solamente cuando el perfil pertenece al propio
solicitante. Una recomendación habilita a evaluar un candidato dentro de la plataforma y no a
contactarlo por fuera de ella.

#### Scenario: Empleador lee un perfil
- **WHEN** un empleador autorizado consulta el perfil de un empleado recomendado
- **THEN** la respuesta es `200` y no contiene el correo del empleado

#### Scenario: Empleado lee su perfil
- **WHEN** el empleado consulta su propio perfil
- **THEN** la respuesta incluye el correo vigente de su cuenta

### Requirement: Entrega de certificados por URL prefirmada
El sistema SHALL exponer `GET /employees/{employeeID}/files/{fileID}/download-url` y
`GET /employees/{employeeID}/education-documents/{educationID}/download-url`, que devuelven una
URL prefirmada de corta duración para descargar un único archivo del empleado.

Ambas rutas MUST exigir exactamente la misma autorización que la lectura del perfil: quien no
puede ver el perfil no puede obtener ninguna URL de sus archivos. La respuesta MUST contener la
URL y el instante en que caduca, y MUST NOT contener el bucket ni la clave de objeto.

#### Scenario: Empleado descarga su certificado
- **WHEN** el empleado pide la URL de un certificado propio
- **THEN** la respuesta es `200` con una URL prefirmada y su instante de caducidad

#### Scenario: Empleador autorizado descarga un certificado
- **WHEN** un empleador con vínculo vigente pide la URL de un certificado de ese empleado
- **THEN** la respuesta es `200` con una URL prefirmada

#### Scenario: Empleador sin vínculo
- **WHEN** un empleador sin vínculo vigente pide la URL de un certificado
- **THEN** la respuesta es `403` y no se emite ninguna URL

#### Scenario: Sin token
- **WHEN** la petición llega sin JWT válido
- **THEN** la respuesta es `401` y no se emite ninguna URL

### Requirement: La URL emitida corresponde a un único archivo del empleado autorizado
El sistema MUST emitir la URL del archivo pedido y de ningún otro. Un archivo que no pertenece
al empleado del path MUST responder `404`, y un archivo inexistente MUST responder `404`
también, de modo que un solicitante ya autorizado para ese perfil no pueda distinguir entre
ambos. Un certificado cuyo contenido no llegó a subirse MUST tratarse como inexistente.

#### Scenario: Archivo de otro empleado
- **WHEN** el identificador de archivo existe pero pertenece a otro empleado
- **THEN** la respuesta es `404` y no se emite ninguna URL

#### Scenario: Archivo inexistente
- **WHEN** el identificador de archivo no existe
- **THEN** la respuesta es `404`

#### Scenario: Título sin documento
- **WHEN** el identificador de educación existe y pertenece al empleado, pero ese título no
  tiene documento asociado
- **THEN** la respuesta es `404`

#### Scenario: Identificador de archivo inválido
- **WHEN** el identificador de archivo del path no es un entero positivo
- **THEN** la respuesta es `400`

### Requirement: La URL caduca y la revocación es lógica
La URL emitida MUST caducar por sí sola en un plazo breve y fijo, sin que el backend conserve
ni invalide URLs ya entregadas. La pérdida del vínculo —por puesto eliminado o por conjunto
vigente reemplazado— MUST cortar la emisión de URLs nuevas, y MUST NOT pretender invalidar una
URL ya emitida que todavía no caducó.

#### Scenario: Caducidad informada
- **WHEN** se emite una URL
- **THEN** la respuesta informa el instante de caducidad, que es breve y el mismo para todas las
  emisiones

#### Scenario: Vínculo perdido
- **WHEN** el empleador pierde el vínculo vigente y vuelve a pedir la URL del mismo archivo
- **THEN** la respuesta es `403` y no se emite una URL nueva
