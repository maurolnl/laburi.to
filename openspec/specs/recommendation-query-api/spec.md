# recommendation-query-api Specification

## Purpose
Definir el contrato HTTP con el que un empleado consulta los puestos que se le recomiendan y
un empleador consulta los candidatos que se recomiendan para uno de sus puestos: qué rutas
existen, quién puede invocarlas, qué estado del proceso informan, cómo se paginan, en qué
orden se devuelven los resultados y qué código responde cada situación.

## Requirements

### Requirement: Consulta de puestos recomendados a un empleado
El sistema SHALL exponer `GET /employees/{employeeID}/job-recommendations`, que devuelve el
estado vigente del proceso de recomendaciones del empleado y el tramo pedido de su conjunto
vigente de puestos. La ruta MUST requerir un JWT válido y MUST responder `200` con un cuerpo
JSON que contenga el estado, la lista de items y los metadatos de paginación.

#### Scenario: Empleado con conjunto vigente
- **WHEN** un empleado autenticado consulta sus recomendaciones y su último batch completado
  tiene puestos activos
- **THEN** la respuesta es `200` con estado `completed` y los puestos del conjunto vigente

#### Scenario: Sin token
- **WHEN** la petición llega sin JWT válido
- **THEN** la respuesta es `401` y no se revela si el empleado existe

#### Scenario: Identificador inválido
- **WHEN** el identificador del path no es un entero positivo
- **THEN** la respuesta es `400`

### Requirement: Consulta de empleados recomendados para un puesto
El sistema SHALL exponer `GET /jobs/{jobPositionID}/employee-recommendations`, que devuelve el
estado vigente del proceso de recomendaciones del puesto y el tramo pedido de su conjunto
vigente de candidatos. La ruta MUST requerir un JWT válido y MUST responder `200` con la misma
forma de cuerpo que la consulta del empleado.

#### Scenario: Puesto con candidatos
- **WHEN** un empleador autenticado consulta un puesto propio cuyo último batch completado
  tiene candidatos
- **THEN** la respuesta es `200` con estado `completed` y los candidatos del conjunto vigente

#### Scenario: Puesto inexistente
- **WHEN** el puesto no existe
- **THEN** la respuesta es `404`

#### Scenario: Puesto eliminado lógicamente
- **WHEN** el puesto fue eliminado lógicamente
- **THEN** la respuesta es `404` y no se devuelve ningún candidato

### Requirement: Cuerpo de respuesta común a ambas consultas
Ambas consultas SHALL responder con un objeto JSON que contenga exactamente tres campos de
primer nivel: `status`, `items` y `page`. `items` MUST ser siempre un arreglo, incluso vacío, y
nunca `null`. `page` MUST contener `limit`, `offset` y `total`.

#### Scenario: Conjunto vacío
- **WHEN** el conjunto vigente no tiene ningún item
- **THEN** `items` es un arreglo vacío y no `null`

#### Scenario: Metadatos de paginación
- **WHEN** se responde cualquiera de las dos consultas
- **THEN** `page` informa el límite aplicado, el desplazamiento aplicado y el tamaño total del
  conjunto vigente ya filtrado

### Requirement: Estado del proceso informado al cliente
El campo `status` SHALL tomar uno de cinco valores: `none`, `pending`, `processing`,
`completed` y `failed`. `pending` y `processing` MUST significar que hay una generación en
curso; `completed` MUST significar que el conjunto vigente es el resultado definitivo de la
última generación, tenga items o no; `failed` MUST distinguir una generación fallida de un
resultado vacío; `none` MUST significar que el sujeto nunca tuvo una generación solicitada. El
estado MUST derivar del batch más reciente del sujeto y los items del último batch completado,
que pueden no ser el mismo batch.

#### Scenario: Generación en curso sobre un conjunto previo
- **WHEN** el batch más reciente del sujeto está `pending` o `processing` y existe un batch
  completado anterior
- **THEN** `status` informa la generación en curso y `items` devuelve el conjunto vigente
  anterior

#### Scenario: Completado sin resultados
- **WHEN** la última generación terminó sin ninguna recomendación
- **THEN** `status` es `completed` y `items` es un arreglo vacío

#### Scenario: Generación fallida
- **WHEN** el batch más reciente del sujeto está `failed`
- **THEN** `status` es `failed`, lo que el cliente distingue de un resultado vacío

#### Scenario: Sujeto sin ninguna generación
- **WHEN** el sujeto nunca tuvo un batch
- **THEN** `status` es `none`, `items` es un arreglo vacío y `total` es cero

### Requirement: Ownership derivado del JWT
El sistema SHALL derivar la identidad del solicitante exclusivamente del JWT y MUST rechazar
con `403` toda consulta sobre un sujeto ajeno. Un principal con rol `employee` MUST poder
consultar únicamente su propio perfil, y uno con rol `employer` únicamente los puestos de su
propio perfil de empleador. El rol del JWT MUST autorizar el sentido de la consulta: un
`employer` no consulta recomendaciones de empleado y un `employee` no consulta candidatos de un
puesto. El identificador del path MUST usarse solo para detectar el acceso ajeno y nunca para
resolver identidad.

#### Scenario: Empleado consultando un perfil ajeno
- **WHEN** un empleado autenticado pide las recomendaciones de otro empleado
- **THEN** la respuesta es `403`

#### Scenario: Empleador consultando un puesto ajeno
- **WHEN** un empleador autenticado pide los candidatos de un puesto de otro empleador
- **THEN** la respuesta es `403`

#### Scenario: Rol incorrecto para el sentido de la consulta
- **WHEN** un principal con rol `employer` pide recomendaciones de empleado, o uno con rol
  `employee` pide candidatos de un puesto
- **THEN** la respuesta es `403`

#### Scenario: Empleador sin perfil de empleador creado
- **WHEN** un principal con rol `employer` que todavía no creó su perfil de empleador consulta
  los candidatos de un puesto
- **THEN** la respuesta es `403`

#### Scenario: Empleado inexistente
- **WHEN** el empleado del path no existe
- **THEN** la respuesta es `404`

### Requirement: Validación de límite y desplazamiento
El sistema SHALL aceptar los parámetros de query `limit` y `offset` y MUST validarlos en el
borde. `limit` MUST ser un entero mayor que cero y menor o igual a un máximo declarado;
`offset` MUST ser un entero no negativo. Un valor ausente MUST tomar el valor por defecto
—`limit` su tamaño de página por defecto y `offset` cero—. Un valor no numérico, negativo, cero
en `limit` o superior al máximo MUST responder `400`, sin recortarse en silencio.

#### Scenario: Parámetros ausentes
- **WHEN** la petición no indica `limit` ni `offset`
- **THEN** se aplican los valores por defecto y la respuesta los informa en `page`

#### Scenario: Límite sobre el máximo
- **WHEN** `limit` supera el máximo declarado
- **THEN** la respuesta es `400` y no se devuelve una página recortada

#### Scenario: Desplazamiento negativo
- **WHEN** `offset` es negativo
- **THEN** la respuesta es `400`

#### Scenario: Valor no numérico
- **WHEN** `limit` u `offset` no son enteros
- **THEN** la respuesta es `400`

#### Scenario: Desplazamiento más allá del conjunto
- **WHEN** `offset` supera el tamaño del conjunto vigente
- **THEN** la respuesta es `200` con `items` vacío y `total` con el tamaño real del conjunto

### Requirement: Orden expuesto y puestos eliminados
La respuesta SHALL conservar el orden que define la persistencia y MUST no reordenar ni
filtrar en el borde. Los puestos recomendados a un empleado MUST devolverse por puntaje
descendente con desempate por publicación más reciente; los empleados recomendados para un
puesto, por puntaje descendente con desempate por actualización de perfil más reciente. Las
recomendaciones sin puntaje MUST ubicarse al final. Los puestos eliminados lógicamente MUST
quedar excluidos de la lista y del `total`, con independencia de cuándo se generó la
recomendación.

#### Scenario: Orden entre páginas
- **WHEN** se recorren páginas sucesivas del mismo conjunto vigente
- **THEN** ningún item se repite ni se omite y el orden global se conserva

#### Scenario: Desempate de puestos
- **WHEN** dos puestos recomendados tienen el mismo puntaje
- **THEN** se devuelve primero el publicado más recientemente

#### Scenario: Desempate de empleados
- **WHEN** dos candidatos recomendados tienen el mismo puntaje
- **THEN** se devuelve primero el de perfil actualizado más recientemente

#### Scenario: Puesto eliminado dentro del conjunto vigente
- **WHEN** un puesto del conjunto vigente de un empleado fue eliminado lógicamente
- **THEN** no aparece en `items` y no se cuenta en `total`

### Requirement: Errores en formato JSON
Todas las respuestas de error de estas rutas SHALL usar el formato JSON `{"error": "..."}`,
coherente con el resto de los endpoints de recomendaciones y puestos, y MUST no filtrar
información sobre sujetos ajenos en el texto del error.

#### Scenario: Error de validación
- **WHEN** la petición trae un parámetro de paginación inválido
- **THEN** la respuesta es `400` con un cuerpo JSON que contiene el campo `error`

#### Scenario: Acceso ajeno
- **WHEN** la respuesta es `403`
- **THEN** el mensaje no revela si el sujeto consultado existe ni a quién pertenece
