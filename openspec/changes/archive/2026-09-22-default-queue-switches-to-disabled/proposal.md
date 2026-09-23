## Why

Toda la infraestructura de recomendaciones es andamiaje: el algoritmo de indicadores no
existe, la épica prohíbe simularlo y ninguna ruta HTTP publica nada todavía. La aplicación
tiene que funcionar sin nada de esto, y hoy hay un camino en el que no lo hace.

`RECOMMENDATIONS_QUEUE_ENABLED` y `RECOMMENDATIONS_WORKER_ENABLED` son interruptores de
encendido. Un valor que `strconv.ParseBool` no entiende —`si`, `yes`, `on`, un espacio raro
pegado al copiar— **aborta el arranque del proceso entero**. Verificado ejecutando el
binario: con `RECOMMENDATIONS_WORKER_ENABLED=si` la API no levanta y `/healthz` no responde.

Es desproporcionado. Una funcionalidad que hoy no hace nada puede impedir que la aplicación
arranque, y el fallo no viene de la cola sino de cómo se escribió su interruptor.

La intención original sigue siendo válida: un typo no debe apagar la cola **en silencio**.
Lo que cambia es cómo se cumple. Apagado con advertencia explícita en el arranque conserva
la visibilidad y no derriba el proceso.

## What Changes

- Un interruptor de habilitación ilegible pasa a significar **deshabilitado**, igual que
  ausente o vacío, en lugar de abortar el arranque.
- Esa degradación nunca es silenciosa: la resolución de la configuración devuelve una
  advertencia que nombra la variable, y el arranque la registra junto al estado del
  transporte.
- El resto de la validación **no cambia**. Con la cola habilitada, una variable obligatoria
  ausente o un valor numérico fuera de rango siguen abortando el arranque: son parámetros de
  algo que sí está encendido, y adivinarlos haría desaparecer mensajes.
- Las advertencias siguen la misma regla que los errores: nombran la variable responsable y
  nunca incluyen su valor.

Sin cambios en el contrato HTTP, sin migraciones y sin efecto alguno cuando las variables
están bien escritas.

## Capabilities

### Modified Capabilities
- `recommendation-queue-configuration`: el fallo temprano deja de aplicarse a los
  interruptores de encendido y se acota a la configuración de un transporte ya habilitado.
  El estado deshabilitado gana una segunda puerta de entrada —el interruptor ilegible— con
  la obligación de advertirlo.

## Impact

- **Código**: `internal/queue/config.go` —interpretación de los interruptores y advertencias
  en `Config` y `WorkerConfig`— y `cmd/api.go`, que las registra en el arranque.
- **Tests**: los que hoy fijan el aborto por interruptor ilegible pasan a fijar el apagado
  con advertencia. El que comprueba que los errores no filtran valores conserva sus casos
  numéricos y suma la advertencia.
- **Base de datos**: ninguna migración.
- **Documentación**: `docs/recommendation-queue.md`, tabla de variables y notas.
- **Fuera de alcance**: la validación de los parámetros del transporte habilitado, que sigue
  abortando, y cualquier otra variable de entorno de la aplicación.
