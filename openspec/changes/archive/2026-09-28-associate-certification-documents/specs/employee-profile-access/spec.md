## MODIFIED Requirements

### Requirement: El perfil no revela la ubicación de ningún archivo
El cuerpo de `GET /employees/{employeeID}` MUST NOT contener el bucket ni la clave de objeto de
ningún archivo, ni ninguna otra coordenada que permita alcanzarlo directamente. Cada
certificado y cada documento de título MUST viajar identificado por un identificador estable
que solo sirve para pedir su entrega por las rutas de descarga.

`certifications` SHALL ser un arreglo de `{ "name", "document_id" }` en el orden en que el
empleado las declaró, con `document_id` igual al identificador del certificado activo asociado
a esa certificación o `null` cuando no tiene. `files` SHALL contener solo los certificados
activos sin certificación asociada, con `id` y `title`. `GET /users/{userID}/employee` MUST
devolver `certifications` y `files` con la misma forma. Ninguna de las dos listas MUST viajar
como `null`.

#### Scenario: Certificados identificados
- **WHEN** el empleado tiene certificaciones con PDF
- **THEN** cada certificación aparece con su `name` y el `document_id` de su PDF, y sin bucket
  ni clave de objeto

#### Scenario: Certificación sin PDF
- **WHEN** una certificación no tiene PDF asociado
- **THEN** aparece con `document_id: null`

#### Scenario: Certificado viejo sin asociar
- **WHEN** el empleado tiene un certificado cargado antes de este cambio
- **THEN** aparece en `files` con su `id` y su nombre original, y se descarga por
  `GET /employees/{employeeID}/files/{fileID}/download-url`

#### Scenario: Certificado dado de baja
- **WHEN** un certificado fue reemplazado o quitado
- **THEN** no aparece ni en `certifications` ni en `files`, y su descarga responde `404`

#### Scenario: Documentos de título identificados
- **WHEN** un título de educación tiene un documento asociado
- **THEN** ese título aparece con el identificador con el que se pide la entrega del documento,
  y sin bucket ni clave de objeto

#### Scenario: Título sin documento
- **WHEN** un título de educación no tiene documento asociado
- **THEN** el identificador de entrega viaja vacío y el título se devuelve igual
