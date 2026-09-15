## Context

LAB-18 ya incorporó la tabla `employers`, su unicidad por usuario, la exclusividad con `employees` y las consultas sqlc `CreateEmployer` y `GetEmployerByUserID`. LAB-19 incorporó el rol al principal autenticado y la autorización reutilizable por tipo de perfil. Falta adaptar estas piezas al flujo HTTP del backend y clasificar los errores de persistencia sin exponer detalles de PostgreSQL.

El patrón vigente separa handlers, servicio, interfaz store y repositorio por feature. La creación de un empleador afecta una sola tabla mediante una única sentencia; la atomicidad y los locks necesarios ya están dentro de PostgreSQL.

## Goals / Non-Goals

**Goals:**

- Mantener el flujo `route -> middleware -> handler -> service -> store/repository` y las interfaces del lado consumidor.
- Hacer explícitas la autorización por rol, la propiedad de la consulta y la clasificación de conflictos y ausencia.
- Entregar un contrato JSON estable que pueda consumir LAB-22.
- Probar handlers y servicio sin depender de una base compartida.

**Non-Goals:**

- Agregar actualización o eliminación de perfiles de empleador.
- Cambiar migraciones o consultas sqlc existentes.
- Implementar el formulario frontend, factories generales o integración real con PostgreSQL, cubiertos por LAB-22 y LAB-23.
- Crear puestos de trabajo o redirigir navegación.

## Decisions

### Crear un paquete de dominio `internal/employer`

El paquete contendrá modelos HTTP/dominio, errores sentinela, una interfaz store pequeña, servicio, repositorio y handler. `cmd` construirá el repositorio con `*sql.DB`, compartirá el validator existente y registrará ambas rutas con `user.AuthenticatedUser`. Se elige este diseño por consistencia con `internal/employee`; agregar la lógica a `user` mezclaría identidad de cuenta con perfiles de dominio.

### Usar JSON y responder `201` sin cuerpo al crear

El perfil no contiene archivos, por lo que el handler decodificará JSON en vez de multipart. La respuesta exitosa seguirá el patrón de creación de employee con `201` sin cuerpo. Se descarta conservar el `200` del diagrama actual porque ese documento describe una ruta futura desactualizada y el contrato vigente usa `201` para perfiles.

### Normalizar strings antes de validarlos

El borde HTTP eliminará espacios exteriores de `name`, `industry`, `location` y cada modalidad antes de ejecutar la validación compartida. Los tres campos escalares serán obligatorios; la lista de modalidades podrá estar ausente o vacía, pero cada elemento presente deberá conservar contenido. Una lista omitida o `nil` se convertirá en `[]string{}` antes de llamar al repositorio para respetar `NOT NULL` y mantener `[]` en las respuestas JSON. No se impondrán enums, longitudes arbitrarias ni deduplicación porque la persistencia define estos valores como texto libre.

Se descarta validar únicamente con `required`, ya que validator acepta strings formados por espacios. También se descarta normalizar dentro del repositorio porque esa capa debe recibir datos de dominio ya válidos.

### Autorizar en servicio y proteger ownership en el handler

La creación y la consulta exigirán rol `employer` mediante `user.AuthorizeProfileRole`. El POST tomará siempre `principal.UserID`; el request no incluirá `user_id`. En GET, el handler comprobará primero que el path pertenece al principal y el servicio comprobará el rol antes de consultar el store. Esto mantiene la propiedad visible en la adaptación HTTP y la política de rol reutilizable en la lógica de negocio.

Se descarta confiar solo en el path o incluir identidad en el body porque permitiría seleccionar otra cuenta. Ambos rechazos usan `403`, por lo que no revelan existencia del perfil.

### Traducir errores de PostgreSQL en el repositorio

El repositorio mapeará `sql.ErrNoRows` a un error de perfil inexistente. Durante `CreateEmployer`, los SQLSTATE `23505` de unicidad y `23514` de la exclusividad instalada por LAB-18 se traducirán a errores sentinela de conflicto; el handler responderá `409`. El resto se envolverá y llegará como error interno genérico.

Se descarta comparar textos de error porque son frágiles y podrían filtrar implementación. También se descarta devolver siempre `500`, ya que impediría distinguir los duplicados exigidos por el ticket.

### No abrir una transacción explícita para la creación

`CreateEmployer` ejecuta una sola sentencia. PostgreSQL garantiza su atomicidad y el trigger toma un advisory lock transaccional dentro de esa sentencia. Un `BeginTx` adicional no agrega una frontera útil. Si una operación futura modifica varias tablas, deberá aplicar `BeginTx`, `WithTx`, rollback diferido y commit final.

### Mantener la documentación externa alineada

Durante apply se actualizará `../docs/create-employer.md` para reemplazar `POST api/employer` por `POST /employers`, corregir la terminología `Employer`, reflejar `201` y documentar `GET /users/{id}/employer`. Como `docs/` no pertenece al Git del backend ni tiene historial propio, el cambio se reportará por separado y no se incluirá en commits del backend.

### Reutilizar los helpers HTTP sin estandarización transversal

Los errores de autenticación, parsing, autorización, conflicto, ausencia e internos usarán `RespondWithError`, que produce `{"error":"..."}`. Los errores del validator conservarán `PrintValidatorError` y su formato actual. Se descarta modificar ese helper compartido dentro de LAB-20 porque alteraría contratos de registro y employee fuera del alcance del ticket.

## Risks / Trade-offs

- [El SQLSTATE `23514` puede representar otros checks futuros en la misma inserción] → Limitar el mapeo a la operación `CreateEmployer`, documentarlo con tests unitarios y revisar la clasificación si la tabla incorpora nuevos checks.
- [Normalizar espacios modifica literalmente el payload] → Documentar el comportamiento y verificar en tests el valor enviado al store.
- [Los tests unitarios no ejercitan triggers reales] → Cubrir aquí la traducción de errores y dejar la validación integrada de PostgreSQL en LAB-23.
- [La documentación externa queda fuera del commit backend] → Incluirla en la verificación y reportarla explícitamente antes de publicar.

## Migration Plan

1. Incorporar el paquete de empleadores y registrar las rutas en la composición existente.
2. Ejecutar pruebas enfocadas, suite completa y `go vet ./...`; no ejecutar migraciones porque el esquema requerido ya existe.
3. Actualizar y revisar el diagrama externo junto con el contrato implementado.
4. Para rollback, retirar el registro de rutas y el paquete; no hay datos ni esquema nuevos que revertir.
