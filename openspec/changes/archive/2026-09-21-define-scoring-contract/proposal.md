## Why

La épica LAB-17 prohíbe explícitamente definir o simular el algoritmo de indicadores,
pero tres tickets posteriores dependen de que exista su contrato: el productor (LAB-32)
decide qué encolar, el consumidor (LAB-33) debe puntuar cada par antes de completar el
batch y las consultas (LAB-35) ordenan por puntaje. Hoy `recommendation.Candidate` ya
tiene un `Score *float64` que nadie sabe quién rellena ni con qué reglas.

Sin ese contrato, LAB-33 no puede escribirse sin inventar un algoritmo temporal, que es
justamente lo que la épica prohíbe. Este cambio define la interfaz, sus tipos de entrada
y salida y su modo de fallo, para que el worker se implemente contra una dependencia
declarada y no disponible, en vez de contra puntajes ficticios.

## What Changes

- Nuevo paquete `internal/scoring`: solo contrato. No calcula nada, no toca la base de
  datos, no expone rutas HTTP y no publica mensajes.
- Entrada normalizada y propia del paquete: `EmployeeProfile` y `JobRequirements` con
  los atributos comparables del dominio (experiencia, nivel educativo, horas diarias,
  timezone, recursos técnicos). El paquete **no importa** `internal/employee` ni
  `internal/jobposition`: la traducción desde esos paquetes pertenece a quien arme la
  entrada, hoy nadie, mañana el worker de LAB-33.
- `Pair` es la unidad de puntuación: un `EmployeeProfile` y un `JobRequirements` con sus
  identificadores, que son los que `recommendation.Candidate` necesita para persistir.
- `Result` separa el resultado total de sus componentes: un `Total *float64` opcional y
  una lista de `Indicator` con nombre, peso y valor. Un algoritmo que arranque con
  indicadores parciales se incorpora agregando entradas a esa lista, sin tocar el
  transporte ni el esquema `recommendations.score`.
- `Result` distingue tres desenlaces de un par: puntuado, descartado por filtro duro
  (`Eligible = false` con su motivo) y sin puntaje disponible (`Total == nil`).
- Tres responsabilidades separadas en tres puertos distintos:
  - `HardFilter` decide elegibilidad y no produce puntaje;
  - `Scorer` puntúa pares elegibles;
  - la persistencia sigue siendo de `internal/recommendation` y no cambia.
- `Scorer` expone `Score` para un par y `ScoreAll` para un lote del mismo tipo, de modo
  que LAB-33 no quede atado a N llamadas y una implementación futura con costo fijo por
  invocación pueda amortizarlo sin cambiar el contrato.
- Implementación de producción `Unavailable`: cumple ambas interfaces y devuelve siempre
  `ErrScoringUnavailable`. Nunca genera un puntaje. Es la señal que LAB-33 traducirá a un
  batch `failed`.
- Fake determinista **solo para tests** (archivo `_test.go`, fuera del binario de
  producción): puntajes fijos y reproducibles para probar orden, empates y ausencia de
  puntaje sin depender de ningún algoritmo.
- No se cablea nada en `cmd/api.go`: hoy no existe consumidor. LAB-33 inyectará la
  implementación cuando monte el worker.

Sin cambios BREAKING: no se modifica ningún contrato HTTP, ninguna migración ni ninguna
interfaz existente.

## Capabilities

### New Capabilities

- `recommendation-scoring`: el contrato inyectable que puntúa un par empleado-puesto —
  su entrada normalizada, la separación entre filtro duro y cálculo, la forma del
  resultado con indicadores parciales, la distinción entre inelegible y sin puntaje, y
  el modo de fallo explícito de una implementación de producción que todavía no existe.

### Modified Capabilities

Ninguna. `recommendation-persistence` ya admite `score` ausente y sigue siendo la única
responsable de persistir; este cambio no altera ninguno de sus requirements.

## Impact

- Código nuevo: `internal/scoring/` (contrato, tipos, errores, implementación no
  disponible y fake de test).
- Código existente: sin modificaciones. `internal/recommendation`, `internal/employee`,
  `internal/jobposition` y `cmd/api.go` quedan intactos.
- Tickets habilitados: LAB-33 (worker) puede implementarse contra este puerto; LAB-35
  (lecturas) confirma que el puntaje ausente es un estado legítimo y no un error.
- Bloqueo que este cambio **no** levanta: sigue sin existir un ticket para el algoritmo
  productivo de indicadores. Mientras no exista, toda ejecución real terminará en un
  batch `failed`, y eso es el comportamiento deseado, no una regresión.
- Dependencias externas: ninguna nueva.
