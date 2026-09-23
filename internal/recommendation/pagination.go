package recommendation

import (
	"net/url"
	"strconv"
)

// defaultPageLimit y maxPageLimit acotan el tramo que una consulta puede pedir. El máximo
// existe para que un cliente no pueda materializar el conjunto vigente entero en una respuesta.
const (
	defaultPageLimit int32 = 20
	maxPageLimit     int32 = 100
)

// parsePage lee el límite y el desplazamiento de la query string.
//
// Un valor fuera de rango es error y no un recorte silencioso: recortar limit=1000 a 100 le
// haría creer al cliente que la página que recibió es todo lo que hay, y la única forma de
// notar la diferencia sería comparar contra el total.
func parsePage(query url.Values) (Page, error) {
	limit, err := parsePageValue(query, "limit", defaultPageLimit)
	if err != nil {
		return Page{}, err
	}
	if limit <= 0 || limit > maxPageLimit {
		return Page{}, ErrInvalidPagination
	}

	offset, err := parsePageValue(query, "offset", 0)
	if err != nil {
		return Page{}, err
	}
	if offset < 0 {
		return Page{}, ErrInvalidPagination
	}

	return Page{Limit: limit, Offset: offset}, nil
}

// parsePageValue distingue el parámetro ausente del presente y vacío: el primero toma el valor
// por defecto y el segundo es un valor inválido, porque el cliente escribió algo que no es un
// entero.
func parsePageValue(query url.Values, key string, fallback int32) (int32, error) {
	if !query.Has(key) {
		return fallback, nil
	}

	value, err := strconv.ParseInt(query.Get(key), 10, 32)
	if err != nil {
		return 0, ErrInvalidPagination
	}

	return int32(value), nil
}
