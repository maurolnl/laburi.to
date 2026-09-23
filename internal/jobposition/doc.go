// Package jobposition expone el contrato HTTP protegido con el que un empleador
// autenticado crea, lista, consulta, edita y elimina lógicamente sus puestos de trabajo.
// La identidad del empleador se deriva siempre del JWT: el identificador recibido por
// path solo sirve para detectar accesos a recursos ajenos. Eliminar es soft delete y no
// existe reapertura. Tras cada alta o edición se notifica JobPositionEventPublisher con el
// identificador del puesto —y nada más—, que es el puerto por el que la épica de
// recomendaciones se entera del cambio sin acoplar este paquete a ninguna tecnología de
// cola. La eliminación lógica no notifica, y un fallo del publicador no invalida el cambio
// ya persistido.
package jobposition
