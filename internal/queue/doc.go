// Package queue configura el transporte asíncrono que lleva las solicitudes de
// recomendación entre el productor y el worker, y expone el puerto con el que ambos lo
// usan. Configura y da acceso; no publica ni consume.
//
// Alcance de LAB-31: la configuración validada, el puerto Client, la implementación sobre
// Amazon SQS y la implementación deshabilitada. Quién envía cada mensaje (LAB-32), quién
// lo procesa y cómo completa el batch (LAB-33) y los disparadores del dominio (LAB-34) son
// cambios posteriores que consumen este paquete. Por eso Send recibe un cuerpo ya
// serializado: qué contiene lo decide el productor, no el transporte.
//
// LAB-33 suma WorkerConfig y LoadWorkerConfig. Son un tipo y una función aparte de Config y
// LoadConfig a propósito: consumir no es transporte, es qué hace una instancia con él, y
// meterlo en LoadConfig rompería la garantía de que con la cola apagada no se lee ninguna
// otra variable. ShouldConsume resuelve la decisión de arranque, que tiene tres desenlaces y
// no dos: apagado a propósito, encendido sin transporte —un despliegue mal armado, visible en
// el log— y encendido.
//
// El paquete no importa internal/recommendation, internal/jobposition ni internal/employee:
// no conoce el dominio que viaja en los mensajes.
//
// # Entrega al menos una vez
//
// SQS garantiza entrega al menos una vez, no exactamente una vez. Un mensaje recibido y no
// confirmado con Delete vuelve a entregarse al vencer su visibility timeout, y tras agotar
// el maxReceiveCount de la cola termina en la DLQ. Todo consumidor debe ser idempotente.
// Esa es la razón de que internal/recommendation garantice en la base un único batch
// pending o processing por sujeto: un redelivery no debe generar trabajo paralelo.
//
// # La cola deshabilitada es un estado legítimo
//
// Mientras no exista productor ni worker, obligar a toda ejecución local a tener una cola
// aprovisionada sería costo sin beneficio. Con RECOMMENDATIONS_QUEUE_ENABLED apagado,
// LoadConfig no lee ninguna otra variable y New devuelve Disabled, que falla con
// ErrQueueDisabled en vez de simular éxito. Con la cola habilitada, en cambio, toda
// variable obligatoria faltante o fuera de rango aborta el arranque.
//
// Deshabilitada no es lo mismo que no-op: jobposition.NoopEventPublisher devuelve nil
// porque perder un evento no debe hacer fallar un alta ya persistida, y esa decisión
// pertenece al borde HTTP. Dentro del transporte, devolver éxito sin haber enviado nada
// sería mentir.
//
// # Configuración
//
// docs/recommendation-queue.md tiene la tabla completa de variables, el arranque local
// contra un SQS local y los atributos que la cola y su DLQ deben tener en AWS. El paquete
// nunca registra la URL de la cola, la región ni credenciales.
package queue
