// Package jobposition expone el contrato HTTP protegido con el que un empleador
// autenticado crea, lista, consulta, edita y elimina lógicamente sus puestos de trabajo.
// La identidad del empleador se deriva siempre del JWT: el identificador recibido por
// path solo sirve para detectar accesos a recursos ajenos. Eliminar es soft delete y no
// existe reapertura. Tras cada alta o edición se notifica JobPositionEventPublisher, el
// puerto que la épica de recomendaciones implementará sin acoplar este paquete a
// ninguna tecnología de cola.
package jobposition
