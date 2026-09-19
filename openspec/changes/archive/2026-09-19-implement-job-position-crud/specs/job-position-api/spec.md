## Purpose

Definir el contrato HTTP protegido con el que un empleador autenticado crea, lista,
consulta, edita y elimina lógicamente sus puestos de trabajo, incluyendo la
autorización por rol, la derivación de identidad desde el JWT, el dominio válido de
cada campo y la notificación del proceso de recomendaciones.

## ADDED Requirements

### Requirement: Acceso restringido a empleadores autenticados
La API SHALL exigir un access token válido en todas las operaciones sobre puestos de
trabajo y MUST permitirlas solamente cuando el rol del principal sea `employer`. Una
petición sin token o con token inválido SHALL responder `401`; un principal con rol
`employee` SHALL responder `403` sin revelar si el puesto existe.

#### Scenario: Petición sin credenciales
- **WHEN** se invoca cualquier operación sobre puestos sin un header `Authorization` válido
- **THEN** la API responde `401` y no ejecuta la operación

#### Scenario: Principal con rol employee
- **WHEN** un usuario autenticado con rol `employee` invoca cualquier operación sobre puestos
- **THEN** la API responde `403` sin indicar si el puesto o el empleador existen

#### Scenario: Principal sin perfil de empleador
- **WHEN** un usuario autenticado con rol `employer` que todavía no creó su perfil de
  empleador invoca cualquier operación sobre puestos
- **THEN** la API responde `403` y no crea ni modifica ningún puesto

### Requirement: Ownership derivado del JWT
La API MUST resolver el empleador efectivo a partir del identificador de usuario del
principal autenticado y MUST ignorar cualquier identificador enviado por el cliente
como fuente de identidad. El `employerID` recibido por path SHALL usarse únicamente
para detectar un acceso a recursos ajenos, que MUST responder `403`. Toda operación
sobre un puesto concreto MUST verificar que el puesto pertenece al empleador del
principal antes de leerlo, modificarlo o eliminarlo.

#### Scenario: Colección de otro empleador
- **WHEN** un empleador autenticado invoca la colección de puestos usando el
  identificador de un empleador distinto del propio
- **THEN** la API responde `403` y no expone ni crea ningún puesto

#### Scenario: Puesto de otro empleador
- **WHEN** un empleador autenticado consulta, edita o elimina un puesto existente que
  pertenece a otro empleador
- **THEN** la API responde `403` y el puesto permanece sin cambios

#### Scenario: Creación atribuida al principal
- **WHEN** un empleador autenticado crea un puesto sobre su propia colección
- **THEN** el puesto queda asociado al empleador derivado del JWT, con independencia de
  cualquier identificador presente en el cuerpo de la petición

### Requirement: Alta de un puesto publicado
La API SHALL exponer `POST /employers/{employerID}/jobs` para crear un puesto que
queda publicado de inmediato, y SHALL responder `201` con la representación completa
del puesto creado. La API MUST NOT exponer estados de borrador ni transiciones de
publicación.

#### Scenario: Alta válida
- **WHEN** un empleador autenticado crea un puesto con todos los campos obligatorios válidos
- **THEN** la API responde `201` con el puesto creado, su identificador y sus timestamps

#### Scenario: Puesto publicado tras el alta
- **WHEN** un empleador consulta o lista un puesto inmediatamente después de crearlo
- **THEN** el puesto aparece disponible sin requerir ninguna acción de publicación adicional

### Requirement: Dominio de los campos del puesto
La API MUST exigir `position`, `role`, `required_experience`,
`required_education_level`, `available_hours_per_day` y `timezone` en el alta y en la
edición, y SHALL aceptar `technical_resources` como campo opcional. La API MUST
restringir `required_experience` a `less_1y`, `1y`, `2_to_5y`, `5_to_10y` y
`more_10y`; `required_education_level` a `university`, `postgraduate`,
`high-school-orientation` y `tertiary`; y `available_hours_per_day` al rango entero de
1 a 8 inclusive. La API MUST validar `timezone` contra los identificadores de zona
horaria reconocidos por la base de datos. La API SHALL normalizar los espacios
exteriores de los strings y SHALL representar `technical_resources` como lista vacía
cuando se omite o se envía vacía, nunca como `null`.

#### Scenario: Campo obligatorio ausente
- **WHEN** se envía un alta o una edición omitiendo cualquiera de los campos obligatorios
- **THEN** la API responde `400` sin persistir ni modificar el puesto

#### Scenario: Enum fuera del dominio
- **WHEN** se envía un valor de experiencia requerida o de nivel educativo pretendido
  fuera de los valores admitidos
- **THEN** la API responde `400` sin persistir ni modificar el puesto

#### Scenario: Horas fuera del rango
- **WHEN** se envían horas disponibles por día menores a 1 o mayores a 8
- **THEN** la API responde `400` sin persistir ni modificar el puesto

#### Scenario: Timezone desconocida
- **WHEN** se envía un identificador de zona horaria que la base de datos no reconoce
- **THEN** la API responde `400` sin persistir ni modificar el puesto

#### Scenario: Recursos técnicos omitidos
- **WHEN** se crea un puesto sin enviar recursos técnicos
- **THEN** la API responde `201` y la representación del puesto expone una lista vacía

#### Scenario: Recursos técnicos múltiples
- **WHEN** se crea un puesto con varios recursos técnicos
- **THEN** la API responde `201` y los conserva en la representación del puesto

### Requirement: Listado y consulta de puestos activos
La API SHALL exponer `GET /employers/{employerID}/jobs` con los puestos activos del
empleador autenticado y `GET /jobs/{jobPositionID}` con un puesto concreto. El listado
MUST excluir los puestos eliminados lógicamente y SHALL responder `200` con una lista
vacía cuando el empleador no tiene puestos activos. La consulta individual de un puesto
inexistente o eliminado MUST responder `404`. Los nombres JSON de la representación
SHALL usar `snake_case`.

#### Scenario: Listado sin puestos
- **WHEN** un empleador autenticado sin puestos activos lista su colección
- **THEN** la API responde `200` con una lista vacía, nunca con `null`

#### Scenario: Listado excluye eliminados
- **WHEN** un empleador lista su colección después de eliminar uno de sus puestos
- **THEN** el puesto eliminado no aparece en la respuesta

#### Scenario: Consulta de puesto eliminado
- **WHEN** se consulta un puesto que fue eliminado lógicamente
- **THEN** la API responde `404`

#### Scenario: Consulta de puesto inexistente
- **WHEN** se consulta un identificador de puesto que no existe
- **THEN** la API responde `404`

### Requirement: Edición de un puesto activo
La API SHALL exponer `PUT /jobs/{jobPositionID}` para reemplazar los campos editables
de un puesto activo propio y SHALL responder `200` con la representación actualizada.
La edición MUST aplicar las mismas reglas de dominio que el alta, MUST actualizar la
marca de actualización del puesto y MUST NOT permitir cambiar el empleador propietario.
Editar un puesto eliminado o inexistente MUST responder `404`.

#### Scenario: Edición válida
- **WHEN** un empleador edita un puesto activo propio con datos válidos
- **THEN** la API responde `200` con los valores nuevos y la marca de actualización renovada

#### Scenario: Edición de puesto eliminado
- **WHEN** un empleador intenta editar un puesto propio ya eliminado lógicamente
- **THEN** la API responde `404` y no se modifica ninguna fila

#### Scenario: Intento de reasignar el empleador
- **WHEN** el cuerpo de la edición incluye un identificador de empleador distinto del propietario
- **THEN** la API ignora ese valor y conserva el empleador original del puesto

### Requirement: Eliminación lógica con exclusión inmediata
La API SHALL exponer `DELETE /jobs/{jobPositionID}` para eliminar lógicamente un puesto
activo propio y SHALL responder `204` sin cuerpo. El puesto eliminado MUST quedar
excluido de inmediato de los listados, de las consultas individuales y de todo consumo
posterior de recomendaciones. La API MUST NOT ofrecer ninguna operación de reapertura o
restauración. Eliminar un puesto ya eliminado o inexistente MUST responder `404`.

#### Scenario: Eliminación válida
- **WHEN** un empleador elimina un puesto activo propio
- **THEN** la API responde `204` y el puesto deja de estar disponible en listados y consultas

#### Scenario: Eliminación repetida
- **WHEN** un empleador elimina dos veces el mismo puesto
- **THEN** la segunda petición responde `404`

#### Scenario: Ausencia de reapertura
- **WHEN** se busca una operación que reactive un puesto eliminado
- **THEN** la API no expone ninguna ruta ni campo que lo permita

### Requirement: Notificación del proceso de recomendaciones
La API MUST notificar a un publicador de eventos de puesto después de crear y después
de editar un puesto, con la representación del puesto resultante. El contrato SHALL
permanecer independiente de cualquier tecnología de cola concreta, de modo que la
infraestructura de recomendaciones pueda sustituir la implementación sin alterar rutas,
cuerpos ni códigos de estado. Un fallo del publicador MUST NOT invalidar el alta ni la
edición ya persistidas.

#### Scenario: Notificación tras el alta
- **WHEN** un empleador crea un puesto correctamente
- **THEN** el publicador recibe exactamente una notificación con el puesto creado

#### Scenario: Notificación tras la edición
- **WHEN** un empleador edita un puesto correctamente
- **THEN** el publicador recibe exactamente una notificación con el puesto actualizado

#### Scenario: Alta rechazada no notifica
- **WHEN** un alta es rechazada por validación, autorización o error de persistencia
- **THEN** el publicador no recibe ninguna notificación

#### Scenario: Fallo del publicador
- **WHEN** el publicador falla al notificar un puesto recién creado
- **THEN** la API responde `201` y el puesto permanece persistido

### Requirement: Formato uniforme de errores
La API SHALL responder todos los errores de estos endpoints con el contrato JSON
`{"error":"..."}`, incluidos los errores de validación de campos, que en el resto de
los endpoints vigentes se emiten como texto plano. Los mensajes MUST NOT filtrar
detalles internos de PostgreSQL ni distinguir entre recurso ajeno y recurso
inexistente cuando eso permita inferir datos de otro empleador. La API MUST responder
`500` con un mensaje genérico ante fallos internos no clasificados.

#### Scenario: Error de validación en JSON
- **WHEN** una petición es rechazada por validación de campos
- **THEN** la API responde `400` con un cuerpo JSON `{"error":"..."}` legible por el cliente

#### Scenario: Cuerpo JSON malformado
- **WHEN** el cuerpo de la petición no es JSON válido o contiene contenido adicional tras el objeto
- **THEN** la API responde `400` con el contrato JSON de error

#### Scenario: Fallo interno
- **WHEN** la persistencia falla por una causa no clasificada
- **THEN** la API responde `500` con un mensaje genérico que no expone detalles del motor de base de datos
