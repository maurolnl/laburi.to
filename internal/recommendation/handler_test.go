package recommendation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

const testSecret = "test-secret"

type fakeQueryService struct {
	jobResponse      JobRecommendationsResponse
	jobErr           error
	employeeResponse EmployeeRecommendationsResponse
	employeeErr      error

	lastSubjectID int32
	lastPage      Page
	lastPrincipal auth.Principal
	calls         int
}

var _ QueryService = (*fakeQueryService)(nil)

func (f *fakeQueryService) JobRecommendations(_ context.Context, employeeID int32, page Page, principal auth.Principal) (JobRecommendationsResponse, error) {
	f.calls++
	f.lastSubjectID = employeeID
	f.lastPage = page
	f.lastPrincipal = principal
	return f.jobResponse, f.jobErr
}

func (f *fakeQueryService) EmployeeRecommendations(_ context.Context, jobPositionID int32, page Page, principal auth.Principal) (EmployeeRecommendationsResponse, error) {
	f.calls++
	f.lastSubjectID = jobPositionID
	f.lastPage = page
	f.lastPrincipal = principal
	return f.employeeResponse, f.employeeErr
}

func newQueryMux(service QueryService) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterQueryRoutes(mux, NewQueryHandler(service), testSecret)
	return mux
}

func tokenFor(t *testing.T, userID int32, role auth.UserRole) string {
	t.Helper()

	token, err := auth.MakeJWT(userID, role, testSecret, time.Hour)
	if err != nil {
		t.Fatalf("firmar el token: %v", err)
	}

	return token
}

func doQuery(t *testing.T, mux *http.ServeMux, target, token string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, target, strings.NewReader(""))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	return recorder
}

func jobRecommendationsPath() string {
	return fmt.Sprintf("/employees/%d/job-recommendations", queryEmployeeID)
}

func employeeRecommendationsPath() string {
	return fmt.Sprintf("/jobs/%d/employee-recommendations", queryJobPositionID)
}

func TestQueryHandlerRespuestaExitosa(t *testing.T) {
	published := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	score := 0.75
	fake := &fakeQueryService{jobResponse: JobRecommendationsResponse{
		Status: StatusCompleted,
		Items: []JobRecommendation{{
			RecommendationID:   1,
			JobPositionID:      queryJobPositionID,
			EmployerID:         queryEmployerID,
			Position:           "Backend Developer",
			Score:              &score,
			PublishedAt:        published,
			UpdatedAt:          published,
			TechnicalResources: []string{},
		}},
		Page: PageInfo{Limit: defaultPageLimit, Offset: 0, Total: 1},
	}}

	recorder := doQuery(t, newQueryMux(fake), jobRecommendationsPath(), tokenFor(t, queryOwnerUserID, user.UserRoleEmployee))
	if recorder.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d: %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Status string              `json:"status"`
		Items  []map[string]any    `json:"items"`
		Page   map[string]*float64 `json:"page"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decodificar la respuesta: %v", err)
	}
	if body.Status != string(StatusCompleted) {
		t.Fatalf("estado inesperado: %q", body.Status)
	}
	if len(body.Items) != 1 {
		t.Fatalf("se esperaba 1 item, se obtuvieron %d", len(body.Items))
	}
	for _, key := range []string{"limit", "offset", "total"} {
		if _, ok := body.Page[key]; !ok {
			t.Fatalf("la paginación debería informar %q: %s", key, recorder.Body.String())
		}
	}

	// El principal llega derivado del token y no del path.
	if fake.lastPrincipal.UserID != queryOwnerUserID || fake.lastPrincipal.Role != user.UserRoleEmployee {
		t.Fatalf("principal inesperado: %+v", fake.lastPrincipal)
	}
	if fake.lastSubjectID != queryEmployeeID {
		t.Fatalf("el servicio recibió el sujeto %d", fake.lastSubjectID)
	}
}

// Un conjunto vacío debe serializar como [] y no como null: son cosas distintas para el
// cliente, y el vacío es el caso más común de un usuario nuevo.
func TestQueryHandlerItemsVacioSerializaComoArreglo(t *testing.T) {
	fake := &fakeQueryService{jobResponse: JobRecommendationsResponse{
		Status: StatusNone,
		Items:  []JobRecommendation{},
		Page:   PageInfo{Limit: defaultPageLimit},
	}}

	recorder := doQuery(t, newQueryMux(fake), jobRecommendationsPath(), tokenFor(t, queryOwnerUserID, user.UserRoleEmployee))
	if recorder.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Fatalf("items debería serializar como arreglo vacío: %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"items":null`) {
		t.Fatalf("items nunca debería ser null: %s", recorder.Body.String())
	}
}

func TestQueryHandlerPaginacion(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantCode int
		wantPage Page
	}{
		{name: "sin parámetros", query: "", wantCode: http.StatusOK, wantPage: Page{Limit: defaultPageLimit, Offset: 0}},
		{name: "límite y desplazamiento", query: "?limit=5&offset=10", wantCode: http.StatusOK, wantPage: Page{Limit: 5, Offset: 10}},
		{name: "límite sobre el máximo", query: "?limit=101", wantCode: http.StatusBadRequest},
		{name: "desplazamiento negativo", query: "?offset=-1", wantCode: http.StatusBadRequest},
		{name: "límite no numérico", query: "?limit=abc", wantCode: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeQueryService{jobResponse: JobRecommendationsResponse{Status: StatusCompleted, Items: []JobRecommendation{}}}
			recorder := doQuery(t, newQueryMux(fake), jobRecommendationsPath()+test.query, tokenFor(t, queryOwnerUserID, user.UserRoleEmployee))

			if recorder.Code != test.wantCode {
				t.Fatalf("se esperaba %d, se obtuvo %d: %s", test.wantCode, recorder.Code, recorder.Body.String())
			}
			if test.wantCode != http.StatusOK {
				if fake.calls != 0 {
					t.Fatalf("una paginación inválida no debería llegar al servicio")
				}
				assertErrorBody(t, recorder)
				return
			}
			if fake.lastPage != test.wantPage {
				t.Fatalf("se esperaba la página %+v, se obtuvo %+v", test.wantPage, fake.lastPage)
			}
		})
	}
}

func TestQueryHandlerCodigosDeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{name: "rol incorrecto", err: user.ErrProfileRoleForbidden, wantCode: http.StatusForbidden},
		{name: "sujeto ajeno", err: ErrRecommendationsForbidden, wantCode: http.StatusForbidden},
		{name: "sujeto inexistente", err: ErrSubjectNotFound, wantCode: http.StatusNotFound},
		{name: "fallo inesperado", err: errors.New("boom"), wantCode: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeQueryService{jobErr: test.err}
			recorder := doQuery(t, newQueryMux(fake), jobRecommendationsPath(), tokenFor(t, queryOwnerUserID, user.UserRoleEmployee))

			if recorder.Code != test.wantCode {
				t.Fatalf("se esperaba %d, se obtuvo %d", test.wantCode, recorder.Code)
			}
			assertErrorBody(t, recorder)

			// El 403 no debe revelar si el sujeto existe ni a quién pertenece.
			if test.wantCode == http.StatusForbidden && !strings.Contains(recorder.Body.String(), `"error":"forbidden"`) {
				t.Fatalf("el 403 debería ser genérico: %s", recorder.Body.String())
			}
		})
	}
}

func TestQueryHandlerSinToken(t *testing.T) {
	fake := &fakeQueryService{}
	recorder := doQuery(t, newQueryMux(fake), jobRecommendationsPath(), "")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("se esperaba 401, se obtuvo %d", recorder.Code)
	}
	if fake.calls != 0 {
		t.Fatalf("una petición sin token no debería llegar al servicio")
	}
}

func TestQueryHandlerIdentificadorInvalido(t *testing.T) {
	for _, target := range []string{"/employees/abc/job-recommendations", "/employees/0/job-recommendations", "/jobs/-1/employee-recommendations"} {
		t.Run(target, func(t *testing.T) {
			fake := &fakeQueryService{}
			recorder := doQuery(t, newQueryMux(fake), target, tokenFor(t, queryOwnerUserID, user.UserRoleEmployee))

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("se esperaba 400, se obtuvo %d", recorder.Code)
			}
			if fake.calls != 0 {
				t.Fatalf("un identificador inválido no debería llegar al servicio")
			}
			assertErrorBody(t, recorder)
		})
	}
}

func TestQueryHandlerSentidoDelPuesto(t *testing.T) {
	fake := &fakeQueryService{employeeResponse: EmployeeRecommendationsResponse{
		Status: StatusCompleted,
		Items:  []EmployeeRecommendation{{EmployeeID: queryEmployeeID, Certifications: []string{}}},
		Page:   PageInfo{Limit: defaultPageLimit, Total: 1},
	}}

	recorder := doQuery(t, newQueryMux(fake), employeeRecommendationsPath(), tokenFor(t, queryOwnerUserID, user.UserRoleEmployer))
	if recorder.Code != http.StatusOK {
		t.Fatalf("se esperaba 200, se obtuvo %d: %s", recorder.Code, recorder.Body.String())
	}
	if fake.lastSubjectID != queryJobPositionID {
		t.Fatalf("el servicio recibió el sujeto %d", fake.lastSubjectID)
	}
	if fake.lastPrincipal.Role != user.UserRoleEmployer {
		t.Fatalf("principal inesperado: %+v", fake.lastPrincipal)
	}
}

// Recorrer páginas sucesivas sobre el servicio real y un store doble: el borde no reordena ni
// pierde items entre una página y la siguiente.
func TestQueryHandlerRecorridoDePaginas(t *testing.T) {
	all := []JobRecommendation{
		{JobPositionID: 1}, {JobPositionID: 2}, {JobPositionID: 3}, {JobPositionID: 4}, {JobPositionID: 5},
	}
	store := &pagingStore{items: all}
	mux := newQueryMux(NewQueryService(store, ownedByPrincipal()))
	token := tokenFor(t, queryOwnerUserID, user.UserRoleEmployee)

	seen := make([]int32, 0, len(all))
	for offset := 0; offset < len(all); offset += 2 {
		recorder := doQuery(t, mux, fmt.Sprintf("%s?limit=2&offset=%d", jobRecommendationsPath(), offset), token)
		if recorder.Code != http.StatusOK {
			t.Fatalf("se esperaba 200, se obtuvo %d: %s", recorder.Code, recorder.Body.String())
		}

		var body JobRecommendationsResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decodificar la respuesta: %v", err)
		}
		if body.Page.Total != int32(len(all)) {
			t.Fatalf("el total debería ser %d, fue %d", len(all), body.Page.Total)
		}
		for _, item := range body.Items {
			seen = append(seen, item.JobPositionID)
		}
	}

	if len(seen) != len(all) {
		t.Fatalf("se esperaban %d items en total, se recorrieron %d", len(all), len(seen))
	}
	for i, item := range all {
		if seen[i] != item.JobPositionID {
			t.Fatalf("en la posición %d se esperaba el puesto %d, se obtuvo %d", i, item.JobPositionID, seen[i])
		}
	}
}

// pagingStore corta el conjunto como lo haría la base, para ejercer el recorrido completo sin
// depender de PostgreSQL.
type pagingStore struct {
	queryStore
	items []JobRecommendation
}

func (s *pagingStore) JobRecommendationsForEmployee(_ context.Context, _ int32, page Page) (JobRecommendations, error) {
	start := int(page.Offset)
	if start > len(s.items) {
		start = len(s.items)
	}
	end := start + int(page.Limit)
	if end > len(s.items) {
		end = len(s.items)
	}

	return JobRecommendations{
		Status: BatchCompleted,
		Items:  s.items[start:end],
		Total:  int32(len(s.items)),
	}, nil
}

func assertErrorBody(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("el error debería ser JSON: %v (%s)", err, recorder.Body.String())
	}
	if _, ok := body["error"]; !ok {
		t.Fatalf("el error debería tener el campo error: %s", recorder.Body.String())
	}
}
