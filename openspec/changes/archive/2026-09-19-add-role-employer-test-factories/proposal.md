## Why

Las pruebas de roles y empleadores repiten datos y helpers en varios archivos, mientras que parte de las garantías persistentes de LAB-18 solo está cubierta indirectamente. LAB-23 necesita fixtures aislados y una suite enfocada que permita verificar estas reglas sin credenciales, datos compartidos ni servicios externos.

## What Changes

- Incorporar factories de test reutilizables para usuarios `employee` y `employer`, principales autenticados y perfiles de empleador, con valores válidos por defecto y overrides explícitos.
- Garantizar que cada invocación produzca datos independientes, incluidas slices y timestamps, y permitir construir casos inválidos de forma intencional.
- Reutilizar las factories en pruebas de `user` y `employer`, eliminando helpers duplicados donde corresponda.
- Completar la cobertura de servicios y handlers para rol persistido/backfill lógico, autorización por rol, ownership, duplicados, conflictos de perfiles y acceso cruzado.
- Completar la cobertura de repositorio y del contrato de la migración para rol inmutable, unicidad y exclusividad entre perfiles, sin conectarse a PostgreSQL ni a otros servicios.
- Ejecutar pruebas enfocadas y `go test ./...` como validación final.

## Capabilities

### New Capabilities

Ninguna. El cambio agrega infraestructura y cobertura de pruebas para requisitos existentes, sin modificar comportamiento observable.

### Modified Capabilities

Ninguna. Las garantías funcionales ya están definidas en `role-aware-authentication`, `user-role-employer-persistence` y `employer-profile-api`.

## Impact

- Archivos de test y helpers `*_test.go` en `internal/user`, `internal/employer` y el área de persistencia/migraciones.
- Sin cambios en rutas, payloads, esquema aplicado, código de producción ni frontend.
- Sin nuevas credenciales, conexiones de red, base compartida, AWS ni otros servicios externos.
