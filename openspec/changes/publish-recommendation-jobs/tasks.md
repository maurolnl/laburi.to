## 1. Alcance del paquete y contrato del mensaje

- [ ] 1.1 Ampliar `internal/recommendation/doc.go` para declarar que el paquete ahora también
  emite solicitudes de recomendación, por qué el productor vive acá y no en `jobposition`, y
  que sigue sin conocer `jobposition` ni `employee`; verificar con
  `go list -deps ./internal/recommendation | grep -E 'internal/(jobposition|employee)'` sin
  coincidencias.
- [ ] 1.2 Crear `internal/recommendation/events.go` con `JobRequestedVersion` y el struct
  `JobRequestedEvent` (`event_id`, `version`, `subject_type`, `subject_id`, `batch_id`,
  `emitted_at`), documentando por qué el mensaje lleva un único `subject_id` y por qué la
  versión es un entero; verificar con `go build ./...`.
- [ ] 1.3 Implementar en `events.go` la construcción del evento a partir de un `Batch` y su
  serialización a JSON, normalizando el instante a UTC; verificar con un test que compara el
  cuerpo completo contra un JSON literal esperado, con reloj e identificador fijos, para los
  dos tipos de sujeto.
- [ ] 1.4 Cubrir con un test que el cuerpo serializado no contiene ningún atributo del
  perfil ni del puesto: afirmar el conjunto exacto de claves del JSON, de modo que agregar un
  campo nuevo rompa el test en vez de filtrar datos en silencio.
- [ ] 1.5 Cubrir con un test que un cuerpo emitido se deserializa de vuelta al mismo evento y
  que la versión es legible sin depender de ningún otro campo, que es lo que permite a un
  consumidor rechazar un contrato desconocido.

## 2. Puerto de emisión

- [ ] 2.1 Crear `internal/recommendation/publisher.go` con la interfaz `JobPublisher`
  (`PublishJob(ctx, Subject) error`), documentando por qué recibe `Subject` y por qué no
  devuelve el batch; verificar con `go build ./...`.
- [ ] 2.2 Implementar `NoopJobPublisher` en el mismo archivo, con la aserción de
  compilación `var _ JobPublisher = NoopJobPublisher{}`, documentando por qué devuelve `nil`
  y no error a diferencia de `queue.Disabled` y `scoring.Unavailable`; verificar con un test
  que no abre batch ni publica, usando un store y un `queuetest.Fake` que fallan la prueba si
  se los invoca.

## 3. Productor sobre la cola

- [ ] 3.1 Implementar `QueueJobPublisher` en `internal/recommendation/publisher.go` con sus
  dependencias —`RecommendationStore` y `queue.Client`— y su constructor, más el reloj y el
  generador de identificadores de evento no exportados con `time.Now` y `uuid.NewString` por
  defecto; verificar con `go build ./...` y la aserción de compilación contra `JobPublisher`.
- [ ] 3.2 Implementar el camino feliz: validar el sujeto, abrir el batch `pending`, serializar
  el evento con el identificador de ese batch y publicarlo; verificar con un test por tipo de
  sujeto que afirma que el store recibió el sujeto correcto y que `queuetest.Fake` conserva
  exactamente un cuerpo, el esperado.
- [ ] 3.3 Implementar el orden batch-antes-de-mensaje de forma observable: un test con un
  store que falla al crear debe comprobar que el fake no recibió ningún envío y que el error
  se propaga.
- [ ] 3.4 Implementar el rechazo del sujeto inválido antes de tocar el store, devolviendo
  `ErrInvalidSubject`; verificar con un test que usa un `Subject` cero y comprueba que ni el
  store ni el fake fueron invocados.
- [ ] 3.5 Implementar la deduplicación: ante `ErrBatchAlreadyInFlight` devolver `nil` sin
  publicar, registrando una línea de diagnóstico sin datos del sujeto más allá de sus
  identificadores; verificar con un test que el fake no recibió nada y que el error no se
  propaga.

## 4. Fallo de publicación sin éxito falso

- [ ] 4.1 Implementar el tratamiento del fallo de envío: transicionar el batch recién abierto
  a `failed` y devolver el error envolviendo el del transporte con `%w`; verificar con un test
  que fuerza el error de envío en `queuetest.Fake` y afirma la transición y que
  `errors.Is` reconoce el error original.
- [ ] 4.2 Cubrir con un test que el transporte deshabilitado —`queue.Disabled`— produce el
  mismo desenlace: `errors.Is(err, queue.ErrQueueDisabled)` y batch en `failed`, sin que la
  emisión se reporte como exitosa.
- [ ] 4.3 Implementar el caso en que la transición a `failed` también falla: devolver la causa
  original de la publicación y registrar la segunda; verificar con un test cuyo store falla en
  `TransitionBatch` y que comprueba qué error llega al llamador.
- [ ] 4.4 Cubrir el reintento seguro con un test que, tras un fallo de publicación, emite una
  solicitud nueva para el mismo sujeto contra un store que ya no tiene trabajo en curso y
  comprueba que se abre un batch nuevo y se publica su mensaje.

## 5. Desacople del cálculo

- [ ] 5.1 Cubrir con un test que la emisión no consulta ningún puntaje ni persiste ninguna
  recomendación: el store doble debe fallar la prueba si se invoca `CompleteBatch`, y el
  paquete no debe importar `internal/scoring`, verificado con
  `go list -deps ./internal/recommendation | grep internal/scoring` sin coincidencias.
- [ ] 5.2 Cubrir con un test que la emisión se resuelve sin esperar ningún cambio de estado
  del batch: tras `PublishJob`, el batch sigue en `pending` y la llamada ya retornó.

## 6. Documentación y cierre

- [ ] 6.1 Actualizar `docs/recommendation-queue.md`: mover la publicación de "no implementado"
  a "implementado en LAB-32", documentar el contrato del mensaje con un ejemplo sin datos
  reales, y dejar explícito que los disparadores siguen pendientes de LAB-34; verificar
  leyendo el documento que ninguna URL de cola ni identificador de cuenta aparece.
- [ ] 6.2 Verificar el cambio completo con `gofmt -l .` sin salida, `go vet ./...` sin
  hallazgos y `go test ./...` en verde.
- [ ] 6.3 Comprobar explícitamente cada criterio de aceptación del ticket contra un test o un
  comando concreto, y dejar constancia del mapeo en el commit de implementación.
