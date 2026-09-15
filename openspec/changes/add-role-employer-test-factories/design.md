## Context

Los paquetes `internal/user` e `internal/employer` ya tienen pruebas unitarias, pero construyen requests, respuestas, principales y entidades mediante literales repetidos. También duplican helpers de tokens y fakes con estado implícito. La migración `0005_add_user_roles_and_employers.sql` contiene el backfill, el trigger de rol inmutable, la unicidad del empleador y los triggers de exclusividad; actualmente las pruebas del repositorio solo verifican el mapeo de algunos SQLSTATE y no el contrato completo de esa migración.

LAB-23 es un cambio exclusivo de test. La suite debe ejecutarse sin `DB_URL`, Docker, AWS, red ni credenciales reales, por lo que no puede depender de PostgreSQL integrado o testcontainers.

## Goals / Non-Goals

**Goals:**

- Ofrecer builders legibles, deterministas y reutilizables dentro de cada paquete afectado.
- Evitar aliasing de slices y estado mutable compartido entre casos o subtests.
- Hacer que cada escenario declare únicamente los campos que difieren del caso válido.
- Relacionar explícitamente la cobertura con los requisitos existentes de roles, persistencia y API de empleadores.
- Mantener `go test ./...` autocontenido y repetible.

**Non-Goals:**

- Cambiar rutas, modelos de producción, migraciones, queries sqlc o reglas de dominio.
- Introducir una base en memoria que simule PostgreSQL o afirmar que tests de texto equivalen a una prueba de integración del motor.
- Crear un paquete público de fixtures consumido por producción.
- Cambiar pruebas frontend o documentación funcional.

## Decisions

### Usar factories package-local en archivos `_test.go`

Se crearán factories con opciones funcionales en `internal/user` e `internal/employer`. Cada factory devolverá los tipos reales del paquete y tendrá valores válidos por defecto; opciones explícitas permitirán cambiar rol, identidad, campos del perfil, modalidades y errores/resultados de los dobles. Los casos inválidos se construirán mediante overrides visibles, no mediante defaults inválidos separados.

Esta ubicación evita incluir soporte de test en el binario y evita ciclos de importación que surgirían con un paquete `internal/testfactory` que importase los mismos paquetes bajo prueba. Se descarta una factory global genérica porque perdería tipos y no podría construir de forma segura todos los modelos del dominio.

### Crear datos nuevos en cada invocación

Las factories no mantendrán contadores globales ni objetos singleton. Cada llamada asignará structs, slices y tiempos deterministas nuevos; cualquier slice recibida por override será clonada. Los tests comprobarán defaults, overrides e independencia mutando una instancia y verificando que otra no cambie.

Se eligen valores deterministas frente a Faker o UUID aleatorios porque simplifican fallos y no requieren dependencias. Cuando un test necesite unicidad, la declarará mediante un override.

### Reutilizar factories sin ocultar la intención del escenario

Los tests existentes de usuario, servicio de empleador, handler y repositorio migrarán los literales relevantes a factories, manteniendo tablas y assertions cerca de la conducta verificada. Los dobles seguirán registrando llamadas y contextos; las factories solo reducirán preparación de datos, no encapsularán la ejecución ni las assertions.

Se descartan helpers que ejecuten el sistema bajo prueba porque harían menos visibles ownership, rol y persistencia.

### Dividir cobertura entre capas

- `user`: registro de ambos roles, rechazo de roles inválidos, conservación del rol al guardar, login de una cuenta tratada como backfill con rol `employer`, y exposición del mismo rol en token/sesión.
- `employer` service/handler: rol permitido/prohibido, identidad derivada del principal, duplicados, conflicto con perfil opuesto, consulta propia y acceso a otro usuario sin llamada al servicio/store.
- `employer` repository: parámetros tipados, mapeo, ausencia, unicidad y clasificación del conflicto de exclusividad mediante errores PostgreSQL sintéticos.
- migración `0005`: test de contrato sobre el artefacto SQL para comprobar backfill a `employer`, dominio y `NOT NULL` del rol, trigger de inmutabilidad, `UNIQUE (user_id)`, ambos triggers de exclusividad y uso del mismo advisory lock transaccional.

La última prueba valida que el artefacto versionado conserva las cláusulas responsables de las garantías, pero no ejecuta PostgreSQL. Se prefiere esta limitación explícita a una dependencia opcional que pueda saltarse silenciosamente en CI o requerir infraestructura externa.

### No usar secretos ni servicios reales

Los JWT se firmarán con constantes locales de test y las contraseñas serán valores ficticios. Stores y queries serán dobles en memoria. Ninguna factory leerá entorno, `.env`, reloj global mutable, red, S3 o base de datos.

## Risks / Trade-offs

- [Una prueba contractual del SQL no detecta diferencias de semántica entre versiones de PostgreSQL] → Limitarla a invariantes estructurales críticas y conservar tests unitarios de traducción de errores; una futura suite de integración deberá ser un cambio separado con infraestructura explícita.
- [Las opciones funcionales pueden ocultar defaults importantes] → Mantener nombres de opciones específicos y agregar tests directos de defaults, overrides e independencia.
- [Refactorizar fixtures puede alterar accidentalmente el alcance de tests existentes] → Preservar assertions y ejecutar primero paquetes enfocados, luego la suite completa.
- [Un requisito existente puede fallar al agregar cobertura] → Detener el apply y volver a revisión si corregirlo exige cambiar código de producción o alcance material.

## Migration Plan

1. Incorporar y probar las factories package-local sin cambiar producción.
2. Migrar preparación repetida y completar escenarios por capa.
3. Agregar la verificación contractual de la migración sin ejecutar servicios.
4. Ejecutar `gofmt`, pruebas enfocadas y `go test ./...`.
5. El rollback consiste en retirar los helpers y casos nuevos; no existen cambios de datos o despliegue.
