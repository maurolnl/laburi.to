## 1. Interpretación de los interruptores

- [x] 1.1 Reemplazar `lookupBool` por una interpretación que trate el valor ilegible como
  deshabilitado y señale la degradación, documentando por qué apagar es la única degradación
  segura; verificar con `go build ./...`.
- [x] 1.2 Agregar `Warnings []string` a `Config` y a `WorkerConfig`, documentando por qué la
  advertencia viaja en el resultado y no en un `log.Printf` del paquete.
- [x] 1.3 Cubrir con un test que cada interruptor ilegible produce configuración
  deshabilitada, sin error y con exactamente una advertencia que nombra la variable.
- [x] 1.4 Cubrir con un test que un interruptor ausente, vacío o falso no produce ninguna
  advertencia.
- [x] 1.5 Cubrir con un test que la advertencia nunca incluye el valor de la variable,
  extendiendo el test que ya fija esa regla para los errores.

## 2. Validación que sigue abortando

- [x] 2.1 Comprobar con tests que, con la cola habilitada, una variable obligatoria ausente
  sigue devolviendo `ErrMissingQueueConfig`.
- [x] 2.2 Comprobar con tests que un valor numérico ilegible o fuera de rango sigue
  devolviendo `ErrInvalidQueueConfig`.
- [x] 2.3 Comprobar que un interruptor ilegible sigue sin hacer que `LoadConfig` lea ninguna
  otra variable.

## 3. Arranque y documentación

- [x] 3.1 Registrar las advertencias en `cmd/api.go` junto al estado del transporte, sin
  incluir ningún valor; verificar ejecutando el binario con un interruptor mal escrito y
  comprobando que arranca, responde `/healthz` y deja la advertencia en el log.
- [x] 3.2 Actualizar `docs/recommendation-queue.md`: tabla de variables y nota explicando qué
  degrada a apagado y qué sigue abortando el arranque.
- [x] 3.3 Verificar el cambio completo con `gofmt -l .` sin salida, `go vet ./...` sin
  hallazgos y `go test ./...` en verde.
