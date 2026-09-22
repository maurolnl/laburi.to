// Package recommendation persiste las recomendaciones entre empleados y puestos de
// trabajo, las ejecuciones que las generan, y emite las solicitudes de regeneración hacia
// el transporte asíncrono.
//
// Alcance acumulado: la persistencia (LAB-29) y el productor de solicitudes (LAB-32). El
// contrato de scoring (LAB-30) y el transporte (LAB-31) viven en sus propios paquetes; el
// worker que consume las solicitudes (LAB-33), los disparadores del dominio (LAB-34) y los
// endpoints HTTP (LAB-35) son cambios posteriores que consumen este paquete.
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
// # Dos conceptos que no hay que confundir
//
//   - El batch vigente es el más reciente del sujeto, cualquiera sea su estado. Determina
//     si el sujeto se muestra procesando, completado, vacío o con error.
//   - El conjunto vigente son las recomendaciones del último batch completado, que puede
//     no ser el más reciente: un batch fallido posterior no destruye el conjunto anterior.
package recommendation
