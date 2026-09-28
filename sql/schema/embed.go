// Package schema embebe las migraciones goose en el binario, para que el deploy pueda
// aplicarlas sin depender del CLI de goose ni del árbol de fuentes en la imagen.
package schema

import "embed"

//go:embed *.sql
var Migrations embed.FS
