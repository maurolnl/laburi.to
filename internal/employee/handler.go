// Package employee expone el contrato HTTP protegido con el que un usuario autenticado crea y
// completa por etapas su perfil de empleado: registro base, locación, recursos técnicos,
// disponibilidad y educación, más los certificados PDF asociados.
//
// Después de cada escritura ya persistida se consulta si el perfil quedó completo —las cinco
// etapas presentes— y solo entonces se notifica EmployeeEventPublisher, el puerto por el que la
// épica de recomendaciones se entera del cambio sin acoplar este paquete a ninguna tecnología
// de cola. Un perfil incompleto no genera trabajo, y un fallo de la notificación no invalida la
// escritura ya confirmada.
package employee

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

type EmployeeHandler struct {
	service  EmployeeService
	validate *validator.Validate
}

func NewHandler(service EmployeeService, validate *validator.Validate) *EmployeeHandler {
	return &EmployeeHandler{
		service:  service,
		validate: validate,
	}
}

func BuildHandlers(store EmployeeStore, validate *validator.Validate, uploader uploader.Service, access RecommendationAccess, publisher EmployeeEventPublisher) *EmployeeHandler {
	employeeService := NewService(store, uploader, access, publisher)
	employeeHandler := NewHandler(employeeService, validate)

	return employeeHandler
}

func RegisterRoutes(mux *http.ServeMux, h *EmployeeHandler, repo *EmployeeRepository, secretKey string) {
	authMiddleware := user.AuthenticatedUser(secretKey)
	employeeMiddlware := AuthenticatedEmployeeMiddleWare(AuthMiddlewareCfg{
		SecretKey:   secretKey,
		GetEmployee: repo.GetEmployeeByID,
	})

	mux.Handle("POST /employees", authMiddleware(http.HandlerFunc(h.CreateEmployee)))
	mux.Handle("PUT /employees/{employeeID}", employeeMiddlware(http.HandlerFunc(h.UpdateEmployee)))

	mux.Handle("POST /employees/{employeeID}/location", employeeMiddlware(http.HandlerFunc(h.CreateLocation)))
	mux.Handle("PUT /employees/{employeeID}/location", employeeMiddlware(http.HandlerFunc(h.UpdateLocation)))
	mux.Handle("POST /employees/{employeeID}/tech", employeeMiddlware(http.HandlerFunc(h.CreateTech)))
	mux.Handle("PUT /employees/{employeeID}/tech", employeeMiddlware(http.HandlerFunc(h.UpdateTech)))
	mux.Handle("POST /employees/{employeeID}/availability", employeeMiddlware(http.HandlerFunc(h.CreateAvailability)))
	mux.Handle("PUT /employees/{employeeID}/availability", employeeMiddlware(http.HandlerFunc(h.UpdateAvailability)))
	mux.Handle("POST /employees/{employeeID}/education", employeeMiddlware(http.HandlerFunc(h.CreateEducation)))
	mux.Handle("PUT /employees/{employeeID}/education", employeeMiddlware(http.HandlerFunc(h.UpdateEducation)))
	mux.Handle("GET /users/{userID}/employee", authMiddleware(http.HandlerFunc(h.GetEmployee)))

	// Las tres rutas de lectura por identificador de empleado van detrás de authMiddleware y no
	// de employeeMiddlware. El middleware de propiedad responde 403 a todo el que no sea el
	// dueño, que es exactamente el actor que estas rutas habilitan: un empleador con
	// recomendación vigente. La autorización vive en el servicio, que es el único lugar donde
	// se puede resolver junto con esa condición.
	mux.Handle("GET /employees/{employeeID}", authMiddleware(http.HandlerFunc(h.GetEmployeeProfile)))
	mux.Handle("GET /employees/{employeeID}/files/{fileID}/download-url", authMiddleware(http.HandlerFunc(h.CertificateDownloadURL)))
	mux.Handle("GET /employees/{employeeID}/education-documents/{educationID}/download-url", authMiddleware(http.HandlerFunc(h.EducationDocumentDownloadURL)))
}
