## Context

- `CreateEmployee`/`UpdateEmployee` (`crud_employee.go`) leen un PDF con
  `files.GetPDF(r, "certifications_file")` y los nombres con `getCertificationsFromForm`
  (`certifications[]` repetido o un JSON `string[]`). Crear usa un CTE que inserta empleado y
  un archivo; actualizar agrega una fila más a `employee_files` sin tocar las anteriores.
- `employee_files` no tiene vínculo con un nombre. `GetEmployeeProfileByID` lista los archivos
  `uploaded`; `GetEmployee` (por usuario) los lista todos, solo con `title`.
- Educación ya resuelve el patrón: JSON con `document` = clave del archivo en el multipart
  (`getEducationDocumentsFromForm`).
- `employees.certifications TEXT[]` alimenta la consulta y el scoring de recomendaciones.

## Goals / Non-Goals

**Goals:** asociación 1:1 opcional entre certificación y PDF; N archivos por request; no
dejar PDFs reemplazados activos ni en la base ni en S3; perfiles viejos legibles.

**Non-Goals:** documentos de educación; asociar automáticamente PDFs viejos; validar
autenticidad; cambiar el contrato de recomendaciones.

## Contrato API (compartido con el frontend)

Request multipart (`POST /employees`, `PUT /employees/{employeeID}`):

```
certifications = '[{"name":"Scrum Master","document":"certification_document_0"},
                   {"name":"AWS Cloud Practitioner","document_id":12},
                   {"name":"ITIL"}]'
certification_document_0 = <PDF>
```

- `name`: obligatorio; se recorta; no vacío; único por perfil sin distinguir mayúsculas.
- `document`: clave de un archivo del mismo multipart; PDF ≤ 5 MB. Cada clave se referencia
  una sola vez.
- `document_id`: certificado `uploaded` del mismo empleado que se conserva y queda asociado a
  este `name` (sirve también para asociar uno viejo sin nombre). Excluyente con `document`.
  En `POST` nunca existe → `400`.
- Ninguno: sin PDF; si tenía uno, se da de baja.
- Campo ausente o `[]`: sin certificaciones. `null` se acepta como `[]`.
- Presencia de `certifications[]` o de cualquier archivo no referenciado (incluido
  `certifications_file`) → `400`.

Lectura (`GET /users/{userID}/employee`, `GET /employees/{employeeID}`):

```json
"certifications": [
  { "name": "Scrum Master", "document_id": 31 },
  { "name": "ITIL", "document_id": null }
],
"files": [ { "id": 7, "title": "certificado.pdf" } ]
```

Orden de `certifications` = orden de `employees.certifications`. `files` = certificados
`uploaded` con `certification_name IS NULL`. Nunca `null`: listas vacías.

## Decisions

- **Columna y no tabla.** `employee_files.certification_name TEXT NULL`; los nombres siguen en
  `employees.certifications`. `document_id` se resuelve con un `LEFT JOIN` por
  (`employee_id`, `certification_name`, `status = 'uploaded'`). Alternativa descartada: tabla
  `employee_certifications`, que obliga a sincronizar el array o reescribir recomendaciones.
- **Índice único parcial** `employee_files_active_certification_key` sobre
  (`employee_id`, `certification_name`) `WHERE status = 'uploaded' AND certification_name IS NOT NULL`:
  garantiza a nivel base un solo PDF activo por certificación. La unicidad sin mayúsculas la
  aplica la validación (el nombre se persiste tal como lo escribió el empleado, recortado).
- **Parseo en el borde.** Un `parseCertificationsForm(r)` puro devuelve los ítems y los
  archivos abiertos, o un error de validación; es lo que cubren los tests de multipart. Corre
  antes de subir nada a S3.
- **Orden de escritura (PUT)**: 1) subir los PDFs nuevos; 2) en una transacción: verificar que
  cada `document_id` sea del empleado y `uploaded` (si no, `400`), actualizar `employees`,
  marcar `deleted` los asociados no conservados, poner `certification_name = NULL` en los
  conservados, reasignar sus nombres, insertar los nuevos con su nombre; 3) commit; 4) borrar
  de S3 los objetos dados de baja (best-effort, con log). Si falla 2, se borran los recién
  subidos (patrón `cleanupOrphanFile`). Liberar y reasignar nombres en dos pasos evita
  conflictos transitorios del índice al renombrar o intercambiar.
- **Certificados viejos (`NULL`)** no se tocan en `PUT` salvo que un ítem los referencie con
  `document_id`.
- **POST** deja de usar el CTE de un archivo: una transacción crea el empleado e inserta N
  archivos.
- **Errores**: sentinela `ErrInvalidCertifications` (+ los de `files`) → `400` con
  `RespondWithError`; DTO → `RespondWithValidatorError`.
- **Límite de tamaño**: `files.GetPDF` ya valida 5 MB por archivo; `ParseMultipartForm`
  sigue en 5 MB de memoria (el resto va a disco).

## Risks / Trade-offs

- [Borrado en S3 falla tras el commit] → La fila ya está `deleted` y ninguna ruta la sirve; se
  loguea bucket/key para limpieza manual.
- [Cliente viejo manda el formato anterior] → `400` explícito en vez de borrar certificaciones
  en silencio; FE y BE se despliegan juntos.
- [Renombrar una certificación sin `document_id` pierde su PDF] → Es la semántica declarada;
  el FE siempre reenvía `document_id` al conservar.

## Migration Plan

`0008_employee_file_certification_name.sql` con `-- +goose Up`/`Down`,
`ADD COLUMN IF NOT EXISTS` y `CREATE UNIQUE INDEX IF NOT EXISTS`; corre en el
`preDeployCommand` `./out migrate`. Sin backfill. Rollback: el `Down` elimina índice y columna;
revertir FE y BE juntos.
