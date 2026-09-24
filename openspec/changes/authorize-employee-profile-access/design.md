## Context

`internal/employee` ya posee el perfil y sus archivos, pero solo sabe leerlo por `user_id`
(`GetEmployee`, detrás de `GET /users/{userID}/employee`) y su única regla de autorización es
la propiedad, resuelta en `AuthenticatedEmployeeMiddleWare`. `internal/recommendation` sabe qué
pares empleado/puesto están vigentes, pero no conoce el perfil. Ninguno de los dos importa al
otro: `internal/employee` recibe el disparador de regeneración por el puerto
`EmployeeEventPublisher`, y ese es el único punto de contacto.

`internal/uploader` sube y borra objetos de S3, y no sabe firmar una URL de lectura. Los
`object_key` que produce hoy salen del backend por dos caminos: `employee_files`, que el perfil
expone solo como `original_filename`, y `employee_education.certification`, que el perfil
expone crudo.

El motivo del cambio está en `proposal.md - Why`; los requisitos, en
`specs/employee-profile-access/spec.md` y `specs/recommendation-persistence/spec.md`.

## Goals / Non-Goals

**Goals:**

- Un perfil legible por un tercero sin que ese tercero reciba jamás la ubicación de un objeto
  privado.
- Una autorización con una sola definición, compartida por el perfil y por las dos rutas de
  entrega: si divergen, la entrega se convierte en la puerta de atrás del perfil.
- Revocación que funcione sin inventario: sin registro de URLs emitidas ni proceso que las
  persiga.

**Non-Goals:**

- Cambiar el contrato de `GET /users/{userID}/employee`, que rompe al frontend y por lo tanto
  no es backend puro.
- Auditar descargas, limitar su frecuencia o contarlas.
- Hacer configurable la caducidad por entorno.
- Unificar los dos espacios de identificadores de archivo en una migración.

## Decisions

### Los tres endpoints viven en `internal/employee`

Alternativa considerada: ponerlos en `internal/recommendation`, que es quien conoce la regla de
acceso, como ya se hizo con las consultas de LAB-35.

Se descartó porque el sujeto de estas rutas es el perfil y no la recomendación. El paquete
tendría que aprender a leer las cinco etapas del perfil, los archivos y los documentos de
educación —es decir, duplicar `internal/employee`— para devolver algo que no es una
recomendación. La recomendación acá es una precondición de acceso, no el dato devuelto.

`internal/employee` recibe entonces la regla por un puerto, `RecommendationAccess`, con una
única operación que responde por sí o por no. El paquete sigue sin importar
`internal/recommendation`; `cmd/api.go` cablea el repositorio de recomendaciones como
implementación, igual que ya cablea el `Trigger`.

### La autorización vive en el servicio, no en un middleware

`AuthenticatedEmployeeMiddleWare` resuelve propiedad y responde `403` a cualquiera que no sea
el dueño. Es exactamente lo que las rutas nuevas no pueden hacer: el empleador autorizado no es
el dueño. Envolverlas con ese middleware las cerraría al único actor que el ticket quiere
habilitar.

La decisión replica la de `internal/recommendation`: una función de autorización en el servicio
que recibe el `auth.Principal`, ramifica por rol y devuelve un sentinela. Las tres rutas la
invocan primero y ninguna duplica la regla.

El middleware existente queda intacto para las rutas de escritura por etapas, que siguen siendo
estrictamente del dueño. «Conservar el patrón existente» se cumple por la regla —identidad del
JWT, path solo para detectar acceso ajeno—, no por el middleware.

### Toda falla de autorización del perfil es `403`, incluido el empleado inexistente

Alternativa considerada: `404` para el empleado inexistente y `403` para el ajeno, que es lo que
hace `recommendation-query-api`.

Se descartó acá porque el actor cambia el riesgo. En LAB-35 el sujeto del path es siempre
propio, así que el `404` solo lo ve quien ya conoce sus identificadores. Acá el empleador
recorre identificadores ajenos por definición, y distinguir `404` de `403` le permitiría
enumerar qué empleados existen en la plataforma sin ninguna relación con ellos.

El precio es una asimetría con LAB-35 que queda documentada en el spec en vez de disimulada.

### El `404` se reserva para el archivo, después de autorizar el perfil

Una vez autorizado el perfil, distinguir «ese archivo no existe» de «ese archivo es de otro
empleado» no revela nada nuevo: quien pregunta ya puede ver la lista completa de archivos de
ese perfil, y la respuesta correcta es la misma para ambos. Unificarlos en `404` evita que el
código de estado le diga a un tercero que el identificador existe en otra parte.

### Dos rutas de entrega y no una

Los certificados del paso 1 son filas de `employee_files` con identificador propio. Los
documentos de título son un `object_key` guardado en la columna `employee_education.certification`,
sin fila de archivo y sin identificador propio. Son dos espacios de identificadores distintos.

Alternativas consideradas:

- Una sola ruta con identificadores prefijados (`f-12`, `ed-7`): mete un esquema de codificación
  en la URL para ahorrar una ruta.
- Migrar los documentos de educación a `employee_files`: unifica de verdad, pero exige un
  backfill de filas que no tienen bucket, tamaño ni content type, valores que la migración no
  puede inventar sin mentir.

Se eligieron dos rutas, cada una direccionando su propia tabla por su propia clave. Es la única
opción en la que el identificador del path significa exactamente lo que dice. La unificación
queda disponible como cambio posterior, con su propio backfill.

### El vínculo se resuelve en SQL y en las dos direcciones

La consulta responde un booleano y no una lista: el borde solo necesita autorizar, y devolver el
conjunto lo obligaría a recorrerlo para llegar a la misma respuesta.

Las dos direcciones se resuelven como dos `EXISTS` unidos por `OR`, cada uno anclado al último
batch `completed` de su sujeto. Un único `EXISTS` con los dos batches en un `IN` exigiría
correlacionar una subconsulta de `FROM` con el `job_position_id` de la fila externa, que en
Postgres requiere `LATERAL`; dos `EXISTS` independientes evitan esa complicación y se leen como
las dos reglas que son.

La vigencia sale de la misma definición que usa LAB-35 —último batch `completed`, puestos con
`deleted_at IS NULL`—, así que el acceso no puede habilitar un perfil que el listado de
candidatos ya no muestra.

### El presignado entra por un puerto y la caducidad es constante

`internal/uploader` gana `PresignGetObject` sobre `s3.NewPresignClient`, que el SDK ya trae; no
hay dependencia nueva. `internal/employee` lo consume por una interfaz propia, `Presigner`, para
que los tests de servicio puedan sustituirlo por un doble que registra bucket, clave y plazo sin
tocar la red. Es la única forma de verificar «la URL corresponde al archivo pedido» sin firmar
de verdad.

La caducidad es una constante del paquete —cinco minutos— y no una variable de entorno. Una URL
prefirmada es una credencial: su plazo es una decisión de seguridad del producto, no de
despliegue, y una variable mal puesta en producción la convierte en un enlace permanente sin que
nadie lo note.

La URL se firma con `ResponseContentDisposition` fijado al nombre original del archivo, para que
el navegador lo descargue con su nombre y no con el UUID de la clave.

### La revocación es lógica

Una URL prefirmada no se puede invalidar sin rotar credenciales o borrar el objeto. El diseño lo
acepta en vez de simularlo: la autorización se evalúa en cada emisión, y perder el vínculo corta
las emisiones siguientes. Con cinco minutos de plazo, la ventana entre la pérdida del vínculo y
la caducidad de la última URL emitida es acotada y conocida.

## Risks / Trade-offs

- **Ventana residual tras la revocación**: una URL emitida un instante antes de eliminar el
  puesto sigue sirviendo hasta cinco minutos. Es inherente al mecanismo; el spec lo declara en
  vez de prometer lo contrario.
- **Asimetría de códigos con `recommendation-query-api`**: dos bordes de la misma épica
  responden distinto ante un sujeto inexistente. Justificada arriba y documentada en ambos
  specs.
- **Dos rutas de descarga**: el frontend tendrá que saber cuál usar según el origen del archivo.
  A cambio, ningún identificador del path necesita ser decodificado para saber qué nombra.
- **`GET /users/{userID}/employee` sigue filtrando el `object_key`** de los documentos de
  educación. Es deuda declarada en `proposal.md - Impact`: corregirla es un cambio de contrato
  que toca el frontend.

## Migration Plan

Ninguna migración de base. Las rutas son nuevas y aditivas; ningún contrato existente cambia,
así que no hay despliegue coordinado con el frontend ni orden de salida que respetar.

## Open Questions

Ninguna.
