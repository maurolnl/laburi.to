// Package recommendation persiste las recomendaciones entre empleados y puestos de
// trabajo, las ejecuciones que las generan, y emite las solicitudes de regeneración hacia
// el transporte asíncrono.
//
// Alcance acumulado: la persistencia (LAB-29), el productor de solicitudes (LAB-32) y el
// worker que las consume (LAB-33). El contrato de scoring (LAB-30) y el transporte (LAB-31)
// viven en sus propios paquetes; los disparadores del dominio (LAB-34) y los endpoints HTTP
// (LAB-35) son cambios posteriores que consumen este paquete.
//
// # Por qué el productor vive acá
//
// Emitir una solicitud necesita las dos mitades a la vez: RecommendationStore para abrir el
// batch y queue.Client para publicar el mensaje. La primera se define en este paquete, así
// que ubicar el productor acá no obliga a exportar nada nuevo y deja la dirección de
// importación en un solo sentido, recommendation → queue.
//
// No vive en internal/jobposition, que es donde LAB-31 lo anticipaba, porque atiende los dos
// sujetos: un empleado no tiene por qué pasar por el paquete de puestos.
//
// El paquete no importa internal/jobposition ni internal/employee, y no debe hacerlo. Los
// disparadores de LAB-34 se conectan con adaptadores que viven del lado del dominio o en
// cmd; invertir esa dirección crearía un ciclo.
//
// Sí importa internal/scoring desde LAB-33, porque CandidateSource devuelve la entrada
// normalizada ya armada. No es un ciclo: scoring no importa ningún paquete de dominio. Lo que
// sí obliga es a que los tests de scoring que necesiten este paquete vivan en el paquete
// externo scoring_test.
//
// # El orden del worker no es arbitrario
//
// El worker resuelve los candidatos con el batch todavía en pending, y solo después decide
// entre completar vacío, fallar por falta de scoring o reclamar y puntuar. Los dos desenlaces
// que no prosperan no deben abrir processing: abrirlo dejaría al sujeto bloqueado por el
// índice único parcial ante cualquier fallo posterior.
//
// El reclamo del batch es condicional y solo prospera desde pending o processing. Incluir
// processing es deliberado: un procesamiento interrumpido tiene que poder retomarse. Que dos
// entregas se solapen es inocuo porque CompleteBatch reemplaza el conjunto entero y el último
// en commitear gana.
//
// # Dos conceptos que no hay que confundir
//
//   - El batch vigente es el más reciente del sujeto, cualquiera sea su estado. Determina
//     si el sujeto se muestra procesando, completado, vacío o con error.
//   - El conjunto vigente son las recomendaciones del último batch completado, que puede
//     no ser el más reciente: un batch fallido posterior no destruye el conjunto anterior.
package recommendation
