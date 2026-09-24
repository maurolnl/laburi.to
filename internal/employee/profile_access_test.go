package employee

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

const profileTestSecret = "test-secret"

// Los tests de estas rutas arman el handler sobre el servicio real y no sobre el doble de
// servicio: lo que verifican —quién puede leer, qué datos ve y qué se firmó— vive justamente en
// el servicio, así que un doble no probaría nada.
func newProfileTestServer(t *testing.T) (*http.ServeMux, *fakeEmployeeStore, *fakeUploader, *fakeRecommendationAccess) {
	t.Helper()

	store := &fakeEmployeeStore{}
	upl := newFakeUploader()
	access := &fakeRecommendationAccess{}

	service := NewService(store, upl, access, &fakeEmployeePublisher{})
	handler := NewHandler(service, validator.New())

	authMiddleware := user.AuthenticatedUser(profileTestSecret)
	mux := http.NewServeMux()
	mux.Handle("GET /employees/{employeeID}", authMiddleware(http.HandlerFunc(handler.GetEmployeeProfile)))
	mux.Handle("GET /employees/{employeeID}/files/{fileID}/download-url", authMiddleware(http.HandlerFunc(handler.CertificateDownloadURL)))
	mux.Handle("GET /employees/{employeeID}/education-documents/{educationID}/download-url", authMiddleware(http.HandlerFunc(handler.EducationDocumentDownloadURL)))

	return mux, store, upl, access
}

func getWithToken(t *testing.T, mux *http.ServeMux, path, token string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	return recorder
}

func profileFixture() EmployeeProfile {
	documentID := int32(77)

	return EmployeeProfile{
		ID:                   42,
		UserID:               7,
		Email:                "empleado@laburi.to",
		Position:             "Backend Developer",
		Role:                 "Individual Contributor",
		YearsOfExperience:    "2_to_5y",
		Certifications:       []string{"AWS"},
		Timezone:             "America/Argentina/Buenos_Aires",
		Os:                   "linux",
		PaidSoftware:         []string{"JetBrains"},
		AvailableHoursPerDay: 8,
		InternetConnections:  []InternetConnection{{Type: "fiber", Speed: "more_50mb"}},
		Education: []ProfileEducationItem{
			{EducationType: "university", Title: "Ingeniería", Status: "completed", CertificationDocumentID: &documentID},
			{EducationType: "tertiary", Title: "Tecnicatura", Status: "in-progress"},
		},
		Files:     []ProfileFileItem{{ID: 11, Title: "certificado.pdf"}},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}

func TestGetEmployeeProfileAutorizacion(t *testing.T) {
	t.Run("el dueño lee su perfil", func(t *testing.T) {
		mux, store, _, _ := newProfileTestServer(t)
		store.ownerData = Employee{ID: 42, UserID: 7}
		store.profileData = profileFixture()

		recorder := getWithToken(t, mux, "/employees/42", makeEmployeeToken(t, profileTestSecret, 7))
		if recorder.Code != http.StatusOK {
			t.Fatalf("se esperaba 200, se obtuvo %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	// El empleador llega con una recomendación vigente, que es la única puerta que tiene.
	t.Run("el empleador con vínculo lee el perfil", func(t *testing.T) {
		mux, store, _, access := newProfileTestServer(t)
		store.profileData = profileFixture()
		access.allowed = true

		recorder := getWithToken(t, mux, "/employees/42", makeEmployerToken(t, profileTestSecret, 99))
		if recorder.Code != http.StatusOK {
			t.Fatalf("se esperaba 200, se obtuvo %d: %s", recorder.Code, recorder.Body.String())
		}

		if len(access.calls) != 1 {
			t.Fatalf("se esperaba una consulta de vínculo, se obtuvieron %d", len(access.calls))
		}
		// La autorización pregunta por el usuario del token y por el empleado del path: si
		// tomara la identidad del path, cualquiera podría hacerse pasar por otro empleador.
		if access.calls[0] != (accessCall{EmployeeID: 42, EmployerUserID: 99}) {
			t.Fatalf("se consultó el vínculo con %+v", access.calls[0])
		}
		// El perfil ajeno no se lee con la lectura de propiedad: esa es solo del dueño.
		if len(store.ownerCalls) != 0 {
			t.Fatalf("se resolvió propiedad para un empleador: %v", store.ownerCalls)
		}
	})

	t.Run("el empleador sin vínculo recibe 403", func(t *testing.T) {
		mux, store, _, access := newProfileTestServer(t)
		store.profileData = profileFixture()
		access.allowed = false

		recorder := getWithToken(t, mux, "/employees/42", makeEmployerToken(t, profileTestSecret, 99))
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("se esperaba 403, se obtuvo %d", recorder.Code)
		}
		if len(store.profileCalls) != 0 {
			t.Fatalf("se leyó el perfil de alguien sin autorización: %v", store.profileCalls)
		}
	})

	// El empleado ajeno y el inexistente comparten respuesta exacta —código y cuerpo—, que es
	// lo que impide recorrer identificadores para descubrir cuáles existen.
	t.Run("el empleado ajeno y el inexistente son indistinguibles", func(t *testing.T) {
		mux, store, _, _ := newProfileTestServer(t)
		store.ownerData = Employee{ID: 42, UserID: 7}

		ajeno := getWithToken(t, mux, "/employees/42", makeEmployeeToken(t, profileTestSecret, 8))

		store.ownerErr = ErrEmployeeProfileNotFound
		inexistente := getWithToken(t, mux, "/employees/42", makeEmployeeToken(t, profileTestSecret, 8))

		if ajeno.Code != http.StatusForbidden || inexistente.Code != http.StatusForbidden {
			t.Fatalf("se esperaba 403 en ambos, se obtuvo %d y %d", ajeno.Code, inexistente.Code)
		}
		if ajeno.Body.String() != inexistente.Body.String() {
			t.Fatalf("las respuestas difieren: %q vs %q", ajeno.Body.String(), inexistente.Body.String())
		}
	})

	t.Run("sin token", func(t *testing.T) {
		mux, store, _, _ := newProfileTestServer(t)

		recorder := getWithToken(t, mux, "/employees/42", "")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("se esperaba 401, se obtuvo %d", recorder.Code)
		}
		if len(store.ownerCalls)+len(store.profileCalls) != 0 {
			t.Fatal("se consultó la base sin token")
		}
	})

	t.Run("identificador inválido", func(t *testing.T) {
		mux, _, _, _ := newProfileTestServer(t)
		token := makeEmployeeToken(t, profileTestSecret, 7)

		for _, path := range []string{"/employees/0", "/employees/-1", "/employees/abc"} {
			recorder := getWithToken(t, mux, path, token)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("%s: se esperaba 400, se obtuvo %d", path, recorder.Code)
			}
		}
	})

	// Un rol que no es employee ni employer no llega siquiera al servicio: hoy ni siquiera se
	// puede emitir un token con ese rol. La regla se comprueba igual sobre el servicio, para que
	// siga siendo una negativa y no un permiso si alguna vez se admite un rol nuevo.
	t.Run("rol desconocido", func(t *testing.T) {
		store := &fakeEmployeeStore{ownerData: Employee{ID: 42, UserID: 7}}
		service := NewService(store, newFakeUploader(), &fakeRecommendationAccess{allowed: true}, &fakeEmployeePublisher{}).(*employeeService)

		_, err := service.GetEmployeeProfile(t.Context(), 42, auth.Principal{UserID: 7, Role: "admin"})
		if !errors.Is(err, ErrProfileAccessForbidden) {
			t.Fatalf("se esperaba ErrProfileAccessForbidden, se obtuvo %v", err)
		}
	})
}

func TestGetEmployeeProfileCuerpo(t *testing.T) {
	// La aserción es sobre el JSON crudo y no sobre el struct: lo que el criterio de aceptación
	// prohíbe es que la ubicación de un objeto salga del backend, y eso solo se ve en el cuerpo.
	t.Run("no viaja bucket ni clave de objeto", func(t *testing.T) {
		mux, store, _, _ := newProfileTestServer(t)
		store.ownerData = Employee{ID: 42, UserID: 7}
		store.profileData = profileFixture()

		recorder := getWithToken(t, mux, "/employees/42", makeEmployeeToken(t, profileTestSecret, 7))
		body := recorder.Body.String()

		for _, forbidden := range []string{"bucket", "object_key", "certification\":", ".pdf\",\"bucket"} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("el cuerpo contiene %q: %s", forbidden, body)
			}
		}
	})

	t.Run("cada certificado y cada documento viaja identificado", func(t *testing.T) {
		mux, store, _, _ := newProfileTestServer(t)
		store.ownerData = Employee{ID: 42, UserID: 7}
		store.profileData = profileFixture()

		recorder := getWithToken(t, mux, "/employees/42", makeEmployeeToken(t, profileTestSecret, 7))

		var response EmployeeProfileResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("deserializar la respuesta: %v", err)
		}

		if len(response.Files) != 1 || response.Files[0].ID != 11 || response.Files[0].Title != "certificado.pdf" {
			t.Fatalf("archivos inesperados: %+v", response.Files)
		}
		if len(response.Education) != 2 {
			t.Fatalf("educación inesperada: %+v", response.Education)
		}
		if response.Education[0].CertificationDocumentID == nil || *response.Education[0].CertificationDocumentID != 77 {
			t.Fatalf("el título con documento no trae su identificador: %+v", response.Education[0])
		}
		// Un título sin documento deja el identificador vacío en vez de inventar un cero, que
		// el cliente leería como un documento descargable.
		if response.Education[1].CertificationDocumentID != nil {
			t.Fatalf("el título sin documento trae identificador: %+v", response.Education[1])
		}
	})

	t.Run("el correo solo viaja para el dueño", func(t *testing.T) {
		mux, store, _, access := newProfileTestServer(t)
		store.ownerData = Employee{ID: 42, UserID: 7}
		store.profileData = profileFixture()

		propio := getWithToken(t, mux, "/employees/42", makeEmployeeToken(t, profileTestSecret, 7))
		if !strings.Contains(propio.Body.String(), "empleado@laburi.to") {
			t.Fatalf("el dueño no recibió su correo: %s", propio.Body.String())
		}

		access.allowed = true
		ajeno := getWithToken(t, mux, "/employees/42", makeEmployerToken(t, profileTestSecret, 99))
		if ajeno.Code != http.StatusOK {
			t.Fatalf("se esperaba 200, se obtuvo %d", ajeno.Code)
		}
		if strings.Contains(ajeno.Body.String(), "empleado@laburi.to") || strings.Contains(ajeno.Body.String(), `"email"`) {
			t.Fatalf("el empleador recibió el correo del empleado: %s", ajeno.Body.String())
		}
	})
}

func TestDownloadURL(t *testing.T) {
	const certificatePath = "/employees/42/files/11/download-url"
	const educationPath = "/employees/42/education-documents/77/download-url"

	storedCertificate := StoredFile{
		Bucket:      "laburito-bucket",
		ObjectKey:   "certifications/9f0b.pdf",
		Filename:    "certificado.pdf",
		ContentType: "application/pdf",
	}

	t.Run("el dueño obtiene la URL de su certificado", func(t *testing.T) {
		mux, store, upl, _ := newProfileTestServer(t)
		store.ownerData = Employee{ID: 42, UserID: 7}
		store.fileData = storedCertificate

		recorder := getWithToken(t, mux, certificatePath, makeEmployeeToken(t, profileTestSecret, 7))
		if recorder.Code != http.StatusOK {
			t.Fatalf("se esperaba 200, se obtuvo %d: %s", recorder.Code, recorder.Body.String())
		}

		var response DownloadURLResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("deserializar la respuesta: %v", err)
		}
		if response.URL == "" {
			t.Fatal("la respuesta no trae URL")
		}

		// La firma tiene que ser exactamente del archivo pedido: bucket, clave y nombre.
		signed := upl.lastPresign()
		if signed.Bucket != storedCertificate.Bucket || signed.Key != storedCertificate.ObjectKey {
			t.Fatalf("se firmó otro objeto: %+v", signed)
		}
		if signed.Filename != storedCertificate.Filename {
			t.Fatalf("se firmó con otro nombre: %+v", signed)
		}

		// El cuerpo entrega la URL y nada más: ni el bucket ni la clave pueden salir por acá.
		body := recorder.Body.String()
		if strings.Contains(body, storedCertificate.Bucket) || strings.Contains(body, "object_key") {
			t.Fatalf("el cuerpo revela la ubicación del objeto: %s", body)
		}

		if len(store.fileCalls) != 1 || store.fileCalls[0] != (storeFileCall{EmployeeID: 42, FileID: 11}) {
			t.Fatalf("se resolvió otro archivo: %+v", store.fileCalls)
		}
	})

	t.Run("el empleador autorizado obtiene la URL", func(t *testing.T) {
		mux, store, _, access := newProfileTestServer(t)
		store.fileData = storedCertificate
		access.allowed = true

		recorder := getWithToken(t, mux, certificatePath, makeEmployerToken(t, profileTestSecret, 99))
		if recorder.Code != http.StatusOK {
			t.Fatalf("se esperaba 200, se obtuvo %d: %s", recorder.Code, recorder.Body.String())
		}
	})

	// Quien no puede ver el perfil no obtiene ninguna URL, y la negativa llega antes de que el
	// archivo se resuelva: si no, el código de respuesta diría si el identificador existe.
	t.Run("el empleador sin vínculo recibe 403 sin firmar nada", func(t *testing.T) {
		mux, store, upl, access := newProfileTestServer(t)
		store.fileData = storedCertificate
		access.allowed = false

		recorder := getWithToken(t, mux, certificatePath, makeEmployerToken(t, profileTestSecret, 99))
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("se esperaba 403, se obtuvo %d", recorder.Code)
		}
		if len(store.fileCalls) != 0 {
			t.Fatalf("se resolvió el archivo sin autorización: %+v", store.fileCalls)
		}
		if len(upl.presignCalls) != 0 {
			t.Fatalf("se firmó una URL sin autorización: %+v", upl.presignCalls)
		}
	})

	t.Run("sin token", func(t *testing.T) {
		mux, _, upl, _ := newProfileTestServer(t)

		recorder := getWithToken(t, mux, certificatePath, "")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("se esperaba 401, se obtuvo %d", recorder.Code)
		}
		if len(upl.presignCalls) != 0 {
			t.Fatal("se firmó una URL sin token")
		}
	})

	// El archivo ajeno y el inexistente llegan como el mismo error desde la persistencia, que
	// filtra por el par empleado/archivo. Acá se comprueba que el borde no los separa.
	t.Run("archivo ajeno e inexistente son 404", func(t *testing.T) {
		mux, store, upl, _ := newProfileTestServer(t)
		store.ownerData = Employee{ID: 42, UserID: 7}
		store.fileErr = ErrFileNotFound
		store.educationDocumentErr = ErrFileNotFound

		for _, path := range []string{certificatePath, educationPath} {
			recorder := getWithToken(t, mux, path, makeEmployeeToken(t, profileTestSecret, 7))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("%s: se esperaba 404, se obtuvo %d", path, recorder.Code)
			}
		}
		if len(upl.presignCalls) != 0 {
			t.Fatalf("se firmó una URL de un archivo inexistente: %+v", upl.presignCalls)
		}
	})

	t.Run("identificador de archivo inválido", func(t *testing.T) {
		mux, store, _, _ := newProfileTestServer(t)
		store.ownerData = Employee{ID: 42, UserID: 7}
		token := makeEmployeeToken(t, profileTestSecret, 7)

		paths := []string{
			"/employees/42/files/0/download-url",
			"/employees/42/files/abc/download-url",
			"/employees/42/education-documents/0/download-url",
		}
		for _, path := range paths {
			recorder := getWithToken(t, mux, path, token)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("%s: se esperaba 400, se obtuvo %d", path, recorder.Code)
			}
		}
	})

	t.Run("el documento de un título se firma con su propia clave", func(t *testing.T) {
		mux, store, upl, _ := newProfileTestServer(t)
		store.ownerData = Employee{ID: 42, UserID: 7}
		store.educationDocumentData = StoredFile{
			ObjectKey:   "certifications/titulo.pdf",
			Filename:    "Ingeniería.pdf",
			ContentType: "application/pdf",
		}

		recorder := getWithToken(t, mux, educationPath, makeEmployeeToken(t, profileTestSecret, 7))
		if recorder.Code != http.StatusOK {
			t.Fatalf("se esperaba 200, se obtuvo %d: %s", recorder.Code, recorder.Body.String())
		}

		signed := upl.lastPresign()
		if signed.Key != "certifications/titulo.pdf" {
			t.Fatalf("se firmó otro objeto: %+v", signed)
		}
		// El bucket vacío es deliberado: el documento de un título no lo persiste, y el uploader
		// resuelve el suyo. Lo que no puede pasar es que el borde invente uno.
		if signed.Bucket != "" {
			t.Fatalf("el borde inventó un bucket: %+v", signed)
		}
		if len(store.educationDocumentCalls) != 1 || store.educationDocumentCalls[0] != (storeFileCall{EmployeeID: 42, FileID: 77}) {
			t.Fatalf("se resolvió otro documento: %+v", store.educationDocumentCalls)
		}
	})
}

func TestDownloadURLCaducidad(t *testing.T) {
	t.Run("la caducidad informada es la firmada y es breve", func(t *testing.T) {
		store := &fakeEmployeeStore{
			ownerData: Employee{ID: 42, UserID: 7},
			fileData:  StoredFile{Bucket: "b", ObjectKey: "k", Filename: "f.pdf"},
		}
		upl := newFakeUploader()
		frozen := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

		service := NewService(store, upl, &fakeRecommendationAccess{}, &fakeEmployeePublisher{}).(*employeeService)
		service.now = func() time.Time { return frozen }

		response, err := service.CertificateDownloadURL(t.Context(), 42, 11, auth.Principal{UserID: 7, Role: user.UserRoleEmployee})
		if err != nil {
			t.Fatalf("emitir la URL: %v", err)
		}

		if !response.ExpiresAt.Equal(frozen.Add(presignTTL)) {
			t.Fatalf("la caducidad informada es %v y se firmó con %v", response.ExpiresAt, presignTTL)
		}
		if got := upl.lastPresign().TTL; got != presignTTL {
			t.Fatalf("se firmó con un plazo distinto del informado: %v", got)
		}
		// Una URL prefirmada es una credencial: un plazo largo la convierte en un enlace
		// permanente, que es exactamente lo que la revocación lógica no puede compensar.
		if presignTTL > 15*time.Minute {
			t.Fatalf("el plazo de la URL dejó de ser breve: %v", presignTTL)
		}
	})

	// La revocación es lógica: no invalida la URL ya emitida, corta la emisión de las nuevas.
	t.Run("perder el vínculo corta la emisión siguiente", func(t *testing.T) {
		mux, store, _, access := newProfileTestServer(t)
		store.fileData = StoredFile{Bucket: "b", ObjectKey: "k", Filename: "f.pdf"}
		access.allowed = true

		token := makeEmployerToken(t, profileTestSecret, 99)
		primera := getWithToken(t, mux, "/employees/42/files/11/download-url", token)
		if primera.Code != http.StatusOK {
			t.Fatalf("se esperaba 200, se obtuvo %d", primera.Code)
		}

		access.allowed = false
		segunda := getWithToken(t, mux, "/employees/42/files/11/download-url", token)
		if segunda.Code != http.StatusForbidden {
			t.Fatalf("se esperaba 403 tras perder el vínculo, se obtuvo %d", segunda.Code)
		}
	})

	t.Run("un fallo de firma no filtra detalles de S3", func(t *testing.T) {
		mux, store, upl, _ := newProfileTestServer(t)
		store.ownerData = Employee{ID: 42, UserID: 7}
		store.fileData = StoredFile{Bucket: "laburito-bucket", ObjectKey: "certifications/9f0b.pdf"}
		upl.presignErr = errors.New("AccessDenied: arn:aws:s3:::laburito-bucket")

		recorder := getWithToken(t, mux, "/employees/42/files/11/download-url", makeEmployeeToken(t, profileTestSecret, 7))
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("se esperaba 500, se obtuvo %d", recorder.Code)
		}
		if strings.Contains(recorder.Body.String(), "laburito-bucket") || strings.Contains(recorder.Body.String(), "arn:aws") {
			t.Fatalf("el error filtró detalles de S3: %s", recorder.Body.String())
		}
	})
}
