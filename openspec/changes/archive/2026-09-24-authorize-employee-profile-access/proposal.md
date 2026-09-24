## Why

LAB-35 dejó al empleador viendo una lista de candidatos recomendados y al empleado una lista
de puestos, pero nadie puede abrir lo que esa lista señala: no existe ninguna ruta que
devuelva el perfil completo de un empleado por su identificador de empleado, y la única que se
le parece —`GET /users/{userID}/employee`— solo sirve al dueño y se direcciona por usuario.

Los certificados agravan el problema. Hoy el perfil devuelve el `object_key` crudo de los
documentos de educación, es decir la coordenada de un objeto privado de S3 viajando al
navegador como si fuera un dato de negocio. Sin un mecanismo de entrega propio, exponer el
perfil a un tercero significaría exponerle también esas coordenadas.

## What Changes

- Ruta nueva `GET /employees/{employeeID}`, autenticada por JWT, que devuelve el perfil
  completo del empleado. Dos actores la pueden invocar:
  - el propio empleado, cuando el perfil es suyo;
  - un empleador, solo mientras exista una recomendación vigente que vincule a ese empleado con
    alguno de sus puestos activos.
- El empleador recibe el mismo perfil **sin el correo electrónico**: una recomendación habilita
  a evaluar un candidato, no a contactarlo por fuera de la plataforma.
- El cuerpo de esta ruta nunca contiene `bucket` ni `object_key`. Cada certificado viaja
  identificado por un id, y el acceso al contenido se pide aparte.
- Dos rutas nuevas de entrega, con la misma autorización que el perfil, que devuelven una URL
  S3 prefirmada de corta duración y nada más:
  - `GET /employees/{employeeID}/files/{fileID}/download-url` para los certificados de
    `employee_files`;
  - `GET /employees/{employeeID}/education-documents/{educationID}/download-url` para los
    documentos de título, que hoy viven como `object_key` en `employee_education`.
- La URL prefirmada corresponde a un único objeto, el pedido, y caduca sola. El backend no
  guarda ni revoca URLs ya emitidas: la revocación es lógica y opera sobre la emisión de URLs
  nuevas. Un puesto eliminado o un batch reemplazado que deja de vincular al par cortan el
  acceso en el siguiente pedido.
- Toda falla de autorización del perfil —rol sin relación, empleado ajeno, empleado
  inexistente— responde `403` con un único mensaje. El `404` queda reservado para un archivo
  que no pertenece al empleado ya autorizado, donde no revela nada que el solicitante no
  pudiera ver.
- La persistencia de recomendaciones gana la resolución del vínculo vigente entre un empleado y
  los puestos activos de un empleador, en las dos direcciones: el batch vigente del empleado y
  el batch vigente de cada puesto.

Sin cambios en el frontend. `GET /users/{userID}/employee` conserva su contrato actual intacto,
incluido el `object_key` que devuelve en `education[].certification`: corregirlo rompe al
cliente que hoy lo consume y por lo tanto es un cambio de contrato que toca ambos repositorios.
Queda declarado como deuda en «Impact».

## Capabilities

### New Capabilities
- `employee-profile-access`: el contrato HTTP de lectura del perfil completo de un empleado por
  identificador de empleado y de entrega de sus certificados —rutas, quién puede invocarlas,
  qué datos ve cada actor, cómo se entrega un archivo privado, cuánto dura ese permiso y qué
  código responde cada situación—.

### Modified Capabilities
- `recommendation-persistence`: la capa gana la resolución del vínculo vigente entre un empleado
  y los puestos activos de un empleador, que es la condición que autoriza a un tercero a ver un
  perfil ajeno.

`employee-profile-onboarding` no cambia: el contrato de `GET /users/{userID}/employee` y el de
las cinco etapas de carga quedan exactamente como están.

## Impact

- **Código nuevo**: `internal/employee/get_employee_profile.go`, `download_url.go`,
  `authorization.go` y sus tests; el presignado en `internal/uploader` sobre
  `s3.NewPresignClient`.
- **Código existente**: `internal/employee/handler.go` (tres rutas nuevas), `store.go` (puertos
  de perfil por identificador, de archivo y de acceso), `repo.go`, `models.go`,
  `internal/recommendation/repo.go` y `store.go`, `cmd/api.go` (cableado del presigner y del
  puerto de acceso).
- **Base de datos**: ninguna migración. Tres consultas nuevas en `sql/queries/employees.sql`
  —perfil por `employees.id`, archivo de `employee_files` por par empleado/archivo y documento
  de `employee_education` por par empleado/educación— y una en `sql/queries/recommendations.sql`
  —existencia de vínculo vigente—, más `sqlc generate`.
- **Dependencias**: ninguna nueva. `internal/uploader` pasa a usar el presignador que
  `aws-sdk-go-v2/service/s3` ya trae. `internal/employee` sigue sin importar
  `internal/recommendation`: el acceso entra por un puerto, como ya entra el disparador de
  regeneración.
- **Documentación**: `docs/employer-searching-for-employees.md` de la raíz gana el paso de
  apertura del perfil y de descarga de certificados; `docs/use-cases.md` refleja la nueva
  precondición de acceso.
- **Deuda declarada, fuera de alcance**: `GET /users/{userID}/employee` sigue devolviendo el
  `object_key` de los documentos de educación. Corregirlo exige cambiar el mapper del frontend
  en la misma tarea y no es backend puro.
- **Fuera de alcance**: el algoritmo de indicadores, las pantallas del frontend, cualquier
  registro de auditoría de descargas, la caducidad configurable por entorno y la corrección del
  contrato viejo del perfil por usuario.
