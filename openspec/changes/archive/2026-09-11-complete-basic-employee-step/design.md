## Context

El backend ya contiene rutas separadas para las cinco secciones, asociación de `employees.user_id`, consulta con `JOIN users` y soporte inicial de carga a S3. Sin embargo, la cobertura actual se limita al parseo de certificaciones y no demuestra el contrato completo de LAB-11. La creación y la actualización base usan multipart; locación, recursos y disponibilidad usan JSON; educación usa multipart por sus documentos.

El frontend queda fuera de este cambio. Su payload actual usa `certifications_files`, mientras el contrato backend y la documentación funcional definen el campo singular `certifications_file`; LAB-14 resolverá esa alineación.

## Goals / Non-Goals

**Goals:**

- Cerrar los caminos correctos y de error de creación con y sin PDF.
- Mantener la identidad derivada del JWT y la autorización por propiedad.
- Consolidar las rutas por sección existentes como contrato de actualización.
- Ajustar validaciones sin volver obligatorios los campos realmente opcionales.
- Cubrir el comportamiento con pruebas enfocadas en handlers, servicio y repositorio según corresponda.

**Non-Goals:**

- Modificar el frontend o aceptar temporalmente nombres alternativos del campo multipart.
- Cambiar el esquema PostgreSQL, soportar múltiples certificaciones PDF o reemplazar archivos existentes.
- Rediseñar S3, autenticación o el asistente completo de empleado.

## Decisions

### Mantener un PDF singular para el paso base

Se conservará `certifications_file` como única clave y se rechazará contenido no PDF o superior a 5 MB mediante la abstracción de archivos existente. Esto mantiene un contrato único y delega la corrección del payload frontend a LAB-14. Aceptar ambas claves ocultaría una incompatibilidad y crearía compatibilidad legacy sin necesidad demostrada.

### Derivar identidad y autorización del JWT

La creación seguirá obteniendo `userID` del contexto autenticado. Las actualizaciones seguirán pasando por el middleware de propiedad y la consulta comparará el usuario autenticado con el path. Aceptar IDs desde el body reduciría la seguridad y duplicaría fuentes de identidad.

### Preservar las rutas y transacciones existentes

Se mantendrán `PUT /employees/{employeeID}` y los subrecursos `/location`, `/tech`, `/availability` y `/education`. Las operaciones que abarcan varias tablas usarán una transacción; si S3 termina antes de un fallo de DB, el servicio ejecutará la eliminación compensatoria con timeout.

### Mantener el correo como dato de la relación

La respuesta no copiará ni almacenará el correo en `employees`: la consulta seguirá obteniéndolo desde `users`. Así refleja cambios futuros en la cuenta sin duplicar datos.

### Auditar validación campo por campo

Se retirará `omitempty` solo cuando permita saltar reglas que también aplican al valor cero. Se mantendrá para campos realmente opcionales, como URL o documento, evitando convertir una limpieza de tags en un cambio accidental de contrato.

## Risks / Trade-offs

- [LAB-14 no está implementado] → Documentar y probar la clave singular para que el ajuste frontend tenga un contrato inequívoco.
- [Las pruebas con S3 real serían lentas o frágiles] → Usar el servicio uploader detrás de su interfaz y cubrir éxito, rechazo y cleanup con dobles controlados.
- [Cambiar validaciones puede rechazar payloads antes tolerados] → Limitar el ajuste a reglas expresadas en la spec y agregar casos para cero, ausencia y valores inválidos.
- [El código actual ya cubre parte del ticket] → Priorizar pruebas de caracterización y modificar producto solo donde el comportamiento observado no cumpla la spec.

## Migration Plan

No requiere migración de datos. El despliegue conserva rutas y esquema actuales; ante rollback se revierte el cambio de código y pruebas sin transformar registros existentes.
