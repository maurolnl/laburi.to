## Why

Desde LAB-32 la mitad emisora está completa: `QueueJobPublisher` abre un batch `pending` y
publica en la cola el evento que lo identifica. Nadie lo recibe. Los mensajes se acumulan
hasta vencer su retención y los batches quedan `pending` para siempre, bloqueando por el
índice único parcial toda solicitud futura del mismo sujeto.

Este cambio aporta la mitad consumidora: el worker que recibe el mensaje, resuelve el
universo de candidatos activos, invoca el contrato de scoring y reemplaza el conjunto
vigente de recomendaciones en una única transacción, reconociendo el mensaje solo después
de haber persistido el desenlace.

El algoritmo de indicadores sigue sin existir, y la épica LAB-17 prohíbe inventarlo. La
única implementación de producción del contrato es `scoring.Unavailable`, que falla. Este
cambio no lo cambia: hace que esa ausencia termine en un batch `failed` explícito y no en
recomendaciones inventadas ni en un batch que nunca cierra.

## What Changes

- Nuevo worker de consumo, que recibe lotes de la cola, procesa cada mensaje y reconoce
  únicamente los que ya dejaron el batch en un estado terminal persistido.
- El worker arranca como goroutine del mismo binario, detrás de un flag propio, y no como
  proceso aparte. Con el flag apagado la API se comporta exactamente como hoy.
- Nuevo puerto de resolución de candidatos: dado un sujeto, devuelve los pares
  empleado–puesto a evaluar. Un puesto eliminado lógicamente nunca entra al universo, ni
  como sujeto ni como candidato.
- Reclamo condicional del batch en la persistencia: la transición a `processing` solo
  prospera desde un estado no terminal, de modo que un redelivery o un mensaje duplicado se
  reconozcan sin rehacer trabajo ya cerrado.
- Comprobación previa de disponibilidad del scoring: cuando hay pares que evaluar y la
  implementación inyectada se declara no disponible, el batch pasa de `pending` a `failed`
  sin abrir `processing`, y el mensaje se reconoce porque reintentarlo no puede prosperar.
- Clasificación explícita de errores en recuperables —infraestructura: base de datos, red,
  cola— y terminales —contrato de mensaje desconocido, cuerpo ilegible, scoring no
  disponible, sujeto inexistente—. Los recuperables no se reconocen y vuelven a la cola
  hasta agotar el `maxReceiveCount` que la cola tiene configurado en AWS y terminar en la
  DLQ; los terminales se reconocen tras dejar el batch cerrado.
- Un sujeto sin candidatos completa el batch con lista vacía, que es un desenlace legítimo y
  no un fallo.

Sin cambios en el contrato HTTP: ninguna ruta, cuerpo ni código de estado se modifica, y el
frontend no se toca. Los endpoints de consulta pertenecen a LAB-35.

## Capabilities

### New Capabilities
- `recommendation-job-consumption`: consumo de las solicitudes de regeneración: el ciclo de
  vida del worker, la interpretación del mensaje, el reclamo idempotente del batch, la
  resolución del universo de candidatos activos, la invocación del scoring, el reemplazo
  atómico del conjunto vigente, el momento del reconocimiento del mensaje y la clasificación
  de errores recuperables frente a terminales.

### Modified Capabilities
- `recommendation-persistence`: gana el reclamo condicional del batch y la lectura de un
  batch por identificador. Hoy `TransitionBatch` es incondicional: aplicada desde el worker,
  un redelivery reabriría un batch ya `completed` o `failed` y destruiría el conjunto
  vigente que ese batch dejó.
- `recommendation-scoring`: gana una comprobación previa y opcional de disponibilidad. Hoy
  la única forma de descubrir que no hay algoritmo es intentar puntuar, lo que obliga a
  abrir `processing` para un batch que no puede prosperar.

## Impact

- **Código nuevo**: worker, puerto de resolución de candidatos y su implementación sobre
  `internal/database` en `internal/recommendation`; nuevas consultas sqlc; comprobación de
  disponibilidad en `internal/scoring`; sus tests.
- **Código existente**: `cmd/main.go` y `cmd/api.go` montan el worker detrás del flag y le
  inyectan `scoring.Unavailable{}`. `internal/recommendation/doc.go` amplía su alcance.
  Ningún handler HTTP cambia.
- **Dependencias**: ninguna nueva.
- **Base de datos**: ninguna migración. El esquema de LAB-29 alcanza; se agregan consultas
  sobre tablas existentes.
- **Documentación**: `docs/recommendation-queue.md` gana el estado del consumidor, el flag
  del worker y la tabla de clasificación de errores.
- **Fuera de alcance**: los disparadores de alta y edición que hoy siguen cableados a
  `jobposition.NoopEventPublisher` (LAB-34), los endpoints HTTP de consulta (LAB-35), el
  algoritmo de indicadores y sus filtros duros, que la épica prohíbe definir, y el
  aprovisionamiento de la cola y su DLQ en AWS, que es infraestructura y no aplicación.
