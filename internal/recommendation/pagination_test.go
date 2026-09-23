package recommendation

import (
	"errors"
	"net/url"
	"testing"
)

func TestParsePage(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    Page
		wantErr bool
	}{
		{name: "parámetros ausentes", query: "", want: Page{Limit: defaultPageLimit, Offset: 0}},
		{name: "ambos presentes", query: "limit=5&offset=10", want: Page{Limit: 5, Offset: 10}},
		{name: "límite en el máximo", query: "limit=100", want: Page{Limit: maxPageLimit, Offset: 0}},
		{name: "solo desplazamiento", query: "offset=3", want: Page{Limit: defaultPageLimit, Offset: 3}},
		{name: "límite sobre el máximo", query: "limit=101", wantErr: true},
		{name: "límite cero", query: "limit=0", wantErr: true},
		{name: "límite negativo", query: "limit=-1", wantErr: true},
		{name: "desplazamiento negativo", query: "offset=-1", wantErr: true},
		{name: "límite no numérico", query: "limit=abc", wantErr: true},
		{name: "desplazamiento no numérico", query: "offset=1.5", wantErr: true},
		{name: "límite presente y vacío", query: "limit=", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query, err := url.ParseQuery(test.query)
			if err != nil {
				t.Fatalf("armar la query: %v", err)
			}

			got, err := parsePage(query)
			if test.wantErr {
				if !errors.Is(err, ErrInvalidPagination) {
					t.Fatalf("se esperaba ErrInvalidPagination, se obtuvo %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("no se esperaba error: %v", err)
			}
			if got != test.want {
				t.Fatalf("se esperaba %+v, se obtuvo %+v", test.want, got)
			}
		})
	}
}

// Un valor fuera de rango tiene que ser un error y no un recorte: si 1000 se convirtiera en 100,
// el cliente recibiría una página completa creyendo que es el conjunto entero.
func TestParsePageNoRecortaElLimite(t *testing.T) {
	query, err := url.ParseQuery("limit=1000")
	if err != nil {
		t.Fatalf("armar la query: %v", err)
	}

	page, err := parsePage(query)
	if err == nil {
		t.Fatalf("un límite fuera de rango debería ser error, se obtuvo la página %+v", page)
	}
}
