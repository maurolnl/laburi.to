// Package recommendation persiste las recomendaciones entre empleados y puestos de
// trabajo, y las ejecuciones que las generan.
//
// Alcance de LAB-29: solo persistencia. El contrato de scoring (LAB-30), la cola y el
// worker (LAB-31 a LAB-33), los disparadores (LAB-34) y los endpoints HTTP (LAB-35) son
// cambios posteriores que consumen este paquete.
//
// Dos conceptos que no hay que confundir:
//
//   - El batch vigente es el más reciente del sujeto, cualquiera sea su estado. Determina
//     si el sujeto se muestra procesando, completado, vacío o con error.
//   - El conjunto vigente son las recomendaciones del último batch completado, que puede
//     no ser el más reciente: un batch fallido posterior no destruye el conjunto anterior.
package recommendation
