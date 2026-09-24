package employee

import (
	"context"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

// CertificateDownloadURL entrega el certificado cargado en el paso base del perfil, que es una
// fila de employee_files con identificador propio.
func (h *EmployeeHandler) CertificateDownloadURL(w http.ResponseWriter, r *http.Request) {
	h.downloadURL(w, r, "fileID", h.service.CertificateDownloadURL)
}

// EducationDocumentDownloadURL entrega el documento de un título, que no es una fila de
// archivos sino una clave guardada en la propia fila de educación. Son dos rutas y no una
// porque son dos espacios de identificadores distintos: unificarlos exigiría codificar el
// origen dentro del identificador del path.
func (h *EmployeeHandler) EducationDocumentDownloadURL(w http.ResponseWriter, r *http.Request) {
	h.downloadURL(w, r, "educationID", h.service.EducationDocumentDownloadURL)
}

// downloadURL es el borde común de las dos entregas: mismo orden de validaciones, misma
// traducción de errores y misma forma de respuesta. Lo único que cambia es qué archivo se
// resuelve, que llega como función.
func (h *EmployeeHandler) downloadURL(
	w http.ResponseWriter,
	r *http.Request,
	fileIDKey string,
	resolve func(ctx context.Context, employeeID, fileID int32, principal auth.Principal) (DownloadURLResponse, error),
) {
	defer r.Body.Close()

	principal, ok := user.PrincipalFromContext(r.Context())
	if !ok {
		internal.RespondWithError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	employeeID, ok := positivePathID(w, r, "employeeID", ErrEmployeeNotFound)
	if !ok {
		return
	}

	fileID, ok := positivePathID(w, r, fileIDKey, ErrFileNotFound)
	if !ok {
		return
	}

	response, err := resolve(r.Context(), employeeID, fileID, principal)
	if err != nil {
		respondWithProfileError(w, err, ErrInternalErrorSigningDownload)
		return
	}

	internal.RespondWithJSON(w, http.StatusOK, response)
}

func (s *employeeService) CertificateDownloadURL(ctx context.Context, employeeID, fileID int32, principal auth.Principal) (DownloadURLResponse, error) {
	return s.downloadURL(ctx, employeeID, principal, func(ctx context.Context) (StoredFile, error) {
		return s.repo.GetEmployeeFile(ctx, employeeID, fileID)
	})
}

func (s *employeeService) EducationDocumentDownloadURL(ctx context.Context, employeeID, educationID int32, principal auth.Principal) (DownloadURLResponse, error) {
	return s.downloadURL(ctx, employeeID, principal, func(ctx context.Context) (StoredFile, error) {
		return s.repo.GetEmployeeEducationDocument(ctx, employeeID, educationID)
	})
}

// downloadURL autoriza primero y recién después resuelve el archivo. El orden importa: quien no
// puede ver el perfil no debe poder distinguir, por el código de respuesta, si un identificador
// de archivo existe.
//
// La autorización es exactamente la misma función que usa el perfil. Si divergieran, la entrega
// se convertiría en la puerta de atrás del perfil.
func (s *employeeService) downloadURL(
	ctx context.Context,
	employeeID int32,
	principal auth.Principal,
	resolveFile func(ctx context.Context) (StoredFile, error),
) (DownloadURLResponse, error) {
	if _, err := s.authorizeProfileAccess(ctx, employeeID, principal); err != nil {
		return DownloadURLResponse{}, err
	}

	file, err := resolveFile(ctx)
	if err != nil {
		return DownloadURLResponse{}, err
	}

	// El instante de caducidad se calcula a partir del mismo reloj con el que se firma, y no de
	// una segunda lectura del tiempo: el cliente tiene que poder confiar en que lo que informa
	// la respuesta es lo que efectivamente se firmó.
	signedAt := s.now()

	url, err := s.uploader.PresignGetObject(ctx, uploader.PresignInput{
		Bucket:      file.Bucket,
		Key:         file.ObjectKey,
		Filename:    file.Filename,
		ContentType: file.ContentType,
		TTL:         presignTTL,
	})
	if err != nil {
		return DownloadURLResponse{}, err
	}

	return DownloadURLResponse{URL: url, ExpiresAt: signedAt.Add(presignTTL).UTC()}, nil
}
