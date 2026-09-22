## Why

La cola de recomendaciones está configurada y expuesta como puerto desde LAB-31, pero nadie
publica en ella: `internal/queue` tiene un cliente construido en el arranque y sin ningún
consumidor. Sin productor, el worker de LAB-33 no tiene qué procesar y la épica LAB-17 no
puede avanzar. Este cambio aporta la mitad emisora: abrir el batch en la base y publicar el
evento que lo identifica.

## What Changes

- Nuevo puerto de publicación de solicitudes de recomendación, con una única operación que
  recibe el sujeto —empleado o puesto— y no menciona ninguna tecnología de cola.
- Nuevo evento versionado como contrato de mensaje: identificador de evento, versión, tipo
  de sujeto, identificador del sujeto, identificador del batch e instante de emisión. Nada
  más: ni datos personales, ni credenciales, ni el perfil o el puesto completos.
- Implementación del puerto que, para cada solicitud, abre un batch `pending` en la
  persistencia y recién después publica el evento con el identificador de ese batch.
- Tratamiento explícito de los dos desenlaces que no son éxito:
  - el sujeto ya tiene un batch `pending` o `processing`: no se publica nada y la operación
    se considera resuelta, porque el trabajo en curso ya va a generar el conjunto;
  - la publicación falla: el batch recién abierto pasa a `failed` y el error se propaga, de
    modo que ningún llamador reciba éxito sin que exista mensaje.
- Implementación inerte del puerto, para que un llamador nunca reciba `nil` ni tenga que
  comprobarlo, en línea con `scoring.Unavailable` y `queue.Disabled`.
- Documentación del contrato del mensaje en `docs/recommendation-queue.md`.

Sin cambios en el contrato HTTP: ninguna ruta, cuerpo ni código de estado se modifica, y el
frontend no se toca.

## Capabilities

### New Capabilities
- `recommendation-job-publishing`: emisión de solicitudes de regeneración de recomendaciones
  hacia el transporte asíncrono: el contrato del mensaje, la apertura del batch previa a la
  publicación, la deduplicación frente a un trabajo ya en curso y el tratamiento del fallo
  de publicación.

### Modified Capabilities
<!-- Ninguna. La apertura y la transición del batch usan RecommendationStore tal como
     LAB-29 lo dejó, y el envío usa queue.Client tal como LAB-31 lo dejó: ningún
     requisito de recommendation-persistence ni de recommendation-queue-configuration
     cambia. -->

## Impact

- **Código nuevo**: productor en `internal/recommendation` —evento, puerto, implementación
  sobre `queue.Client` + `RecommendationStore`, implementación inerte— y sus tests.
- **Código existente**: ninguno. `cmd/api.go` sigue cableando
  `jobposition.NoopEventPublisher{}`; conectar los disparadores de alta y edición pertenece
  a LAB-34.
- **Dependencias**: ninguna nueva. `github.com/google/uuid` ya está en `go.mod`.
- **Base de datos**: ninguna migración. Se usan `CreateBatch` y `TransitionBatch` existentes.
- **Documentación**: `docs/recommendation-queue.md` gana el contrato del mensaje y el estado
  del productor.
- **Fuera de alcance**: consumir mensajes y completar batches (LAB-33), disparar la
  regeneración desde employee y puestos (LAB-34), los endpoints HTTP de consulta (LAB-35) y
  el algoritmo de indicadores, que la épica prohíbe definir.
