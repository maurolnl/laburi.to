## Context

`queue.LoadConfig` y `queue.LoadWorkerConfig` resuelven su configuración con `lookupBool`,
que hoy trata tres situaciones distintas de dos maneras:

- ausente o vacío → deshabilitado, sin error;
- ilegible → `ErrInvalidQueueConfig`, que en `cmd/main.go` termina el proceso.

La segunda es la que sobra. El resto de la validación —variables obligatorias, rangos
numéricos— sí tiene que abortar, y este cambio no la toca.

## Goals / Non-Goals

**Goals**

- Que ningún valor escrito en un interruptor de recomendaciones pueda impedir que la
  aplicación arranque.
- Que la degradación sea visible en el arranque y atribuible a una variable concreta.
- Conservar intacta la validación del transporte habilitado.

**Non-Goals**

- Relajar los valores obligatorios o los rangos numéricos.
- Cambiar el comportamiento cuando las variables están bien escritas.
- Tocar variables de entorno ajenas a recomendaciones.

## Decisions

### El interruptor ilegible se equipara a ausente, no a "true"

Las dos degradaciones posibles son apagar o encender. Apagar es la única segura: encender por
error un transporte cuya configuración nadie revisó llevaría a fallos en runtime contra una
cola que quizá no existe. Apagado, lo peor que pasa es que no se generen recomendaciones,
que es exactamente el estado actual del producto.

Equipararlo a ausente también deja una regla de una sola línea, fácil de sostener: **si no
puedo entender si me prendés, quedo apagado y lo aviso.**

### La advertencia viaja en el resultado, no en un log del paquete

`internal/queue` no registra nada hoy, y no debería empezar: su configuración se resuelve con
un `Lookup` inyectable justamente para poder ejercitarla sin tocar el entorno ni el log del
proceso de tests. Un `log.Printf` dentro de `LoadConfig` ensuciaría toda la suite.

Las advertencias se devuelven en `Config.Warnings` y `WorkerConfig.Warnings`, y las registra
`cmd`, que es quien ya imprime el estado del transporte en el arranque. El paquete sigue
siendo puro y la advertencia sigue siendo observable en un test sin capturar salida.

Son un slice y no un único string porque `LoadConfig` y `LoadWorkerConfig` resuelven cada uno
su interruptor, y nada impide que los dos estén mal escritos a la vez.

### Las advertencias no llevan el valor

Misma regla que los errores, por la misma razón: los valores vienen del entorno y un secreto
pegado en la variable equivocada no debe terminar en un log de arranque. El test que ya fija
esa regla para los errores se extiende a las advertencias.

### Qué sigue abortando

| Situación | Antes | Ahora |
| --- | --- | --- |
| Interruptor ausente o vacío | deshabilitado | deshabilitado |
| Interruptor ilegible | **aborta** | deshabilitado + advertencia |
| Cola habilitada, variable obligatoria ausente | aborta | aborta |
| Cola habilitada, valor numérico ilegible o fuera de rango | aborta | aborta |

La línea divisoria es si el valor describe **si** una funcionalidad participa o **cómo**
participa. Lo primero admite un default seguro; lo segundo no: un `AWS_SQS_MAX_MESSAGES`
adivinado o una región equivocada hacen desaparecer mensajes de una cola que sí está
encendida.

## Risks / Trade-offs

- **Un typo en el interruptor deja de ser evidente por el arranque fallido.** Se compensa
  con la advertencia, que nombra la variable, y con la línea de estado que ya existía. Es el
  intercambio deliberado del cambio: visibilidad en vez de indisponibilidad.
- **Dos formas de llegar al mismo estado apagado.** Distinguirlas importa para el
  diagnóstico, y por eso la advertencia existe; el estado resultante es idéntico a propósito,
  porque comportarse distinto según *por qué* está apagado sería peor.

## Migration Plan

Ninguna. Un despliegue con las variables bien escritas se comporta exactamente igual, y uno
con un interruptor mal escrito pasa de no arrancar a arrancar con la funcionalidad apagada.
