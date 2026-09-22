## 1. Dependencia y alcance del paquete

- [ ] 1.1 Agregar `github.com/aws/aws-sdk-go-v2/service/sqs` con `go get` y verificar que
  queda como dependencia directa en `go.mod` y que `go build ./...` compila.
- [ ] 1.2 Crear `internal/queue/doc.go` declarando el alcance —configuración y acceso, no
  publicación ni consumo—, la garantía de entrega al menos una vez, quién lo consumirá
  (LAB-32 y LAB-33) y que la cola deshabilitada es un estado legítimo; verificar con
  `go list -deps ./internal/queue | grep -E 'internal/(recommendation|jobposition|employee)'`
  sin coincidencias.

## 2. Errores y configuración validada

- [ ] 2.1 Crear `internal/queue/errors.go` con `ErrQueueDisabled`, `ErrMissingQueueConfig` y
  `ErrInvalidQueueConfig`; verificar con un test que `errors.Is` clasifica los tres por
  separado y que un error de valor faltante no satisface `ErrInvalidQueueConfig` ni a la
  inversa.
- [ ] 2.2 Crear `internal/queue/config.go` con el tipo `Lookup func(string) (string, bool)`,
  el struct `Config` (habilitación, región, URL de cola, URL de DLQ, visibility timeout,
  wait time, máximo de mensajes, máximo de reintentos, endpoint opcional) y `LoadConfig`;
  verificar que compila y que `LoadConfig` no referencia `os.Getenv` con
  `grep -n "os.Getenv\|os.LookupEnv" internal/queue/` sin coincidencias.
- [ ] 2.3 Implementar en `LoadConfig` el cortocircuito de cola deshabilitada: sin
  `RECOMMENDATIONS_QUEUE_ENABLED` en `true` devuelve `Config{Enabled: false}` sin error y
  sin leer ninguna otra variable; verificar con un test que usa un lookup que falla la
  prueba si se le consulta cualquier otra clave.
- [ ] 2.4 Implementar la validación de los tres valores obligatorios con la cola habilitada
  —región, URL de cola y URL de DLQ—, tratando el valor vacío o solo espacios como ausente;
  verificar con un test table-driven que cubre, para cada variable, los casos ausente,
  vacía, solo espacios y presente, y que el error envuelve `ErrMissingQueueConfig`.
- [ ] 2.5 Implementar los cuatro parámetros numéricos con su default y su rango según la
  tabla D8 del diseño; verificar con un test table-driven que cubre omitido (toma el
  default), límite inferior, límite superior, fuera de rango por ambos extremos y texto no
  numérico, y que los dos últimos envuelven `ErrInvalidQueueConfig`.
- [ ] 2.6 Agregar el endpoint opcional `AWS_SQS_ENDPOINT_URL`; verificar con un test que
  ausente deja el campo vacío sin error y que presente lo conserva.
- [ ] 2.7 Agregar un test que recorre todos los errores que `LoadConfig` puede producir y
  comprueba que el mensaje contiene el nombre de la variable y **no** contiene su valor,
  usando valores centinela reconocibles como secretos.

## 3. Puerto de cola

- [ ] 3.1 Crear `internal/queue/client.go` con el tipo `Message` (identificador, cuerpo,
  receipt handle, conteo de recepciones) y la interfaz `Client` con `Send`, `Receive`,
  `Delete` y `ExtendVisibility`; verificar con
  `go doc ./internal/queue Client` que ninguna firma expone tipos de `aws-sdk-go-v2`.
- [ ] 3.2 Documentar en el doc comment de `Client` que la entrega es al menos una vez, que
  un mensaje no confirmado se vuelve a entregar, que `ReceiveCount` es aproximado y que el
  procesamiento debe ser idempotente; verificar que el texto está presente con
  `go doc ./internal/queue Client`.
- [ ] 3.3 Crear `internal/queue/disabled.go` con `Disabled` implementando `Client` y
  devolviendo siempre `ErrQueueDisabled`; verificar con la aserción de compilación
  `var _ Client = Disabled{}` y un test que cubre las cuatro operaciones y comprueba que
  ninguna reporta éxito.

## 4. Implementación sobre SQS

- [ ] 4.1 Crear `internal/queue/sqs.go` con la implementación real: construye la
  configuración del SDK con `config.LoadDefaultConfig`, región explícita,
  `config.WithRetryMaxAttempts` y el endpoint opcional cuando está declarado; verificar con
  un test que construir un cliente con una `Config` válida y endpoint local no devuelve
  error y no realiza ninguna llamada de red.
- [ ] 4.2 Implementar `Send`, `Receive`, `Delete` y `ExtendVisibility` mapeando desde y
  hacia los tipos del paquete, con `Receive` pidiendo el atributo
  `ApproximateReceiveCount` y usando el wait time y el máximo de mensajes de la `Config`;
  verificar con un test que la traducción de un `types.Message` de SQS a `queue.Message`
  conserva identificador, cuerpo, receipt handle y conteo, y que un conteo ausente o no
  numérico no rompe la traducción.
- [ ] 4.3 Agregar `queue.New(ctx, Config) (Client, error)` que devuelve `Disabled{}` cuando
  la configuración está deshabilitada y la implementación SQS cuando no; verificar con un
  test que cubre ambas ramas y con `grep -rn "log.Fatal\|os.Exit\|panic(" internal/queue/`
  sin coincidencias.

## 5. Doble de test importable

- [ ] 5.1 Crear `internal/queue/queuetest/fake.go` con un doble en memoria que implementa
  `queue.Client`, con reloj inyectable y visibility timeout configurable; verificar con la
  aserción de compilación `var _ queue.Client = (*Fake)(nil)`.
- [ ] 5.2 Implementar las garantías observables: un mensaje recibido queda invisible hasta
  vencer su visibility timeout, `Delete` lo elimina definitivamente, `ExtendVisibility`
  corre el vencimiento y cada entrega incrementa `ReceiveCount`; verificar con tests que
  ejercitan enviar-recibir-borrar, redelivery tras avanzar el reloj, no-redelivery tras
  `Delete` y extensión de visibilidad.
- [ ] 5.3 Agregar al doble la posibilidad de forzar error en cada operación y de inspeccionar
  los mensajes enviados, que es lo que LAB-32 y LAB-33 necesitarán para sus tests;
  verificar con un test de cada capacidad.
- [ ] 5.4 Verificar que el doble no entra al binario con
  `go list -deps ./cmd | grep queuetest` sin coincidencias.

## 6. Composición en cmd sin globals

- [ ] 6.1 Extender `cmd/config.go` con el campo de configuración de cola dentro de
  `appConfig` y cargarlo en `cmd/main.go` con `queue.LoadConfig(os.LookupEnv)`, abortando
  con `logErrorAndFail` ante error; verificar manualmente que
  `RECOMMENDATIONS_QUEUE_ENABLED=true go run ./cmd` sin las variables obligatorias termina
  antes de escuchar en el puerto y con un mensaje que nombra la variable faltante.
- [ ] 6.2 Construir el cliente en `cmd/api.go` con `queue.New` y guardarlo como campo de
  `application`; verificar con `grep -rn "^var \|func init(" cmd/*.go internal/queue/` que
  no se introdujo ninguna global ni `init()`.
- [ ] 6.3 Registrar en el arranque una única línea indicando si el transporte de
  recomendaciones está habilitado, sin URL, región ni credenciales; verificar con
  `go run ./cmd` en ambos estados y comprobando que la salida no contiene la URL.
- [ ] 6.4 Verificar que `cmd` no referencia `jobposition.NoopEventPublisher` de otra forma
  que la actual: el cableado del productor pertenece a LAB-32, con
  `git diff -- cmd/api.go` revisado explícitamente.

## 7. Documentación sin secretos

- [ ] 7.1 Agregar a `.env.example` las variables nuevas bajo una sección propia, todas sin
  valor asignado; verificar con
  `grep -nE "AWS_SQS|RECOMMENDATIONS_QUEUE" .env.example` que aparecen las ocho y que
  ninguna tiene contenido a la derecha del `=`.
- [ ] 7.2 Crear `docs/recommendation-queue.md` con la tabla de variables (obligatoriedad,
  default, rango), el arranque local contra un SQS local vía `AWS_SQS_ENDPOINT_URL`, el
  arranque desplegado, los atributos que deben tener la cola y su DLQ —redrive policy,
  `maxReceiveCount`, visibility timeout y long polling— y la consecuencia para el
  consumidor de que la entrega sea al menos una vez; verificar que el archivo no contiene
  ninguna URL de cola real, identificador de cuenta ni credencial.
- [ ] 7.3 Actualizar la sección de arquitectura de `CLAUDE.md` y `AGENTS.md` del backend
  para listar `internal/queue` como transversal y dejar registrada la divergencia con
  `internal/uploader` en el manejo de `AWS_REGION` y de los fallos de configuración;
  verificar con `diff <(sed -n '/## Arquitectura/,/^## /p' CLAUDE.md) <(sed -n '/## Arquitectura/,/^## /p' AGENTS.md)`
  sin diferencias.
- [ ] 7.4 Verificar que no se modificó `.env` con `git status --short` y que `.env` no
  aparece en el diff del cambio.

## 8. Validación y cierre

- [ ] 8.1 Ejecutar `gofmt -l internal/queue cmd` sin salida y `go vet ./...` sin hallazgos.
- [ ] 8.2 Ejecutar `go test ./internal/queue/...` y luego `go test ./...`, ambos en verde,
  sin credenciales AWS en el entorno.
- [ ] 8.3 Recorrer los cinco criterios de aceptación de LAB-31 y dejar constancia de con qué
  test o comprobación se satisface cada uno.
- [ ] 8.4 Verificar que no se agregó observabilidad fuera de los logs mínimos: sin métricas,
  sin tracing y sin dependencias nuevas más allá de `service/sqs`, comprobando el diff de
  `go.mod`.
