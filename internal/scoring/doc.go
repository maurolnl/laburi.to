// Package scoring define el contrato inyectable con el que se puntúa la afinidad entre
// un empleado y un puesto de trabajo. Define el contrato; no el algoritmo.
//
// La épica LAB-17 prohíbe explícitamente definir o simular los indicadores mientras no
// exista el algoritmo productivo. Por eso este paquete no calcula nada: declara la
// entrada normalizada, separa el filtro duro del cálculo, fija la forma del resultado y
// ofrece una única implementación de producción, Unavailable, que falla con
// ErrScoringUnavailable en vez de inventar puntajes.
//
// El paquete no importa internal/employee ni internal/jobposition: sus tipos de entrada
// son propios y contienen solo los atributos comparables. Traducir los modelos de
// dominio a esa entrada es responsabilidad de quien lee de la base de datos, que será el
// worker de LAB-33. Esa misma tarea inyectará la implementación; hoy no hay ningún
// consumidor y cmd/api.go no referencia este paquete.
//
// Tres desenlaces conviven en Result y no hay que confundirlos:
//
//   - Puntuado: el par es elegible y Total tiene valor, incluido cuando ese valor es cero.
//   - Inelegible: un filtro duro descartó el par; no hay puntaje y Reason dice por qué.
//   - Sin puntaje: el par es elegible pero Total es nil. No es un error.
//
// Un fallo de la implementación es una cuarta cosa, distinta de las tres anteriores: el
// par no llegó a evaluarse.
//
// Availability agrega la quinta y última: saber de antemano que la evaluación no va a poder
// ocurrir, sin haber pasado ningún par. Unavailable la implementa; el resto no está obligado.
// Sirve a quien necesita decidir antes de comprometer estado, que es el caso del worker de
// LAB-33: descubrir la indisponibilidad recién al puntuar lo obligaría a marcar el batch como
// en ejecución para después fallarlo.
package scoring
