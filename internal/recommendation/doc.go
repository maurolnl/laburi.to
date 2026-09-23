// Package recommendation persiste las recomendaciones entre empleados y puestos de
// trabajo, las ejecuciones que las generan, y emite las solicitudes de regeneración hacia
// el transporte asíncrono.
//
// Alcance acumulado: la persistencia (LAB-29), el productor de solicitudes (LAB-32), el
// worker que las consume (LAB-33) y el borde HTTP de consulta (LAB-35). El contrato de scoring
// (LAB-30) y el transporte (LAB-31) viven en sus propios paquetes; los disparadores del dominio
// (LAB-34) conectan los bordes de escritura con el productor.
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
// El paquete no importa internal/jobposition ni internal/employee, y no debe hacerlo:
// invertir esa dirección crearía un ciclo. Los disparadores de LAB-34 se conectan sin
// importar nada gracias a que los puertos de esos paquetes reciben solo el identificador del
// sujeto, así que Trigger los satisface por tipado estructural y cmd compone las tres piezas.
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
//
// # El borde de consulta
//
// GET /employees/{employeeID}/job-recommendations y
// GET /jobs/{jobPositionID}/employee-recommendations exponen esos dos conceptos juntos: status
// sale del batch vigente e items del conjunto vigente.
//
// El cliente distingue cinco estados. Los cuatro de BatchStatus más none, que significa que el
// sujeto nunca tuvo una generación solicitada. none no vive en BatchStatus porque ese tipo
// replica el check de la migración 0007 y la base no conoce ese valor.
//
// La autorización deriva del JWT y vive en el servicio, no en un middleware: el rol determina el
// sentido de la consulta —un employee solo consulta su perfil y un employer solo sus puestos—,
// así que hay que comprobarlo junto con la propiedad. El identificador del path sirve
// únicamente para detectar el acceso ajeno. La propiedad se resuelve con consultas propias del
// paquete, para no invertir la dirección de importación hacia employee ni jobposition.
//
// El orden, el desempate y la exclusión de puestos eliminados son comportamiento de la
// persistencia; el borde los expone tal cual y no reordena ni filtra.
package recommendation
