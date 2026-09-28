package employee

import (
	"context"
	"errors"
	"mime"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/uploader"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

func (h *EmployeeHandler) CreateEmployee(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	principal, ok := user.PrincipalFromContext(r.Context())
	if !ok {
		internal.RespondWithError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	employeeRequest, certifications, ok := h.parseEmployeeForm(w, r)
	if !ok {
		return
	}
	defer closeCertificationUploads(certifications)

	err := h.service.CreateEmployee(r.Context(), employeeRequest, principal, certifications)
	if err != nil {
		switch {
		case errors.Is(err, user.ErrProfileRoleForbidden):
			internal.RespondWithError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrInvalidCertifications):
			internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		default:
			internal.RespondWithError(w, http.StatusInternalServerError, ErrInternalErrorCreatingEmployee.Error())
		}
		return
	}

	internal.RespondWithNoBody(w, http.StatusCreated)
}

func (h *EmployeeHandler) UpdateEmployee(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	employeeID, err := internal.GetPathValueAsInt(r, "employeeID")
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrEmployeeNotFound.Error())
		return
	}

	employeeRequest, certifications, ok := h.parseEmployeeForm(w, r)
	if !ok {
		return
	}
	defer closeCertificationUploads(certifications)

	err = h.service.UpdateEmployee(r.Context(), employeeID, employeeRequest, certifications)
	if err != nil {
		if errors.Is(err, ErrInvalidCertifications) {
			internal.RespondWithError(w, http.StatusBadRequest, err.Error())
			return
		}
		internal.RespondWithError(w, http.StatusInternalServerError, ErrInternalErrorUpdatingEmployee.Error())
		return
	}

	internal.RespondWithNoBody(w, http.StatusOK)
}

// parseEmployeeForm es el borde común de alta y actualización del paso base. Responde el 400
// que corresponda y devuelve ok en false si la petición no es válida; todos los errores viajan
// como JSON {"error": "..."}.
func (h *EmployeeHandler) parseEmployeeForm(w http.ResponseWriter, r *http.Request) (CreateEmployeeRequest, []CertificationEntry, bool) {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		internal.RespondWithError(w, http.StatusBadRequest, ErrInvalidMultiPartForm.Error())
		return CreateEmployeeRequest{}, nil, false
	}

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, ErrInvalidMultiPartForm.Error())
		return CreateEmployeeRequest{}, nil, false
	}

	certifications, err := parseCertificationsForm(r)
	if err != nil {
		internal.RespondWithError(w, http.StatusBadRequest, err.Error())
		return CreateEmployeeRequest{}, nil, false
	}

	employeeRequest := CreateEmployeeRequest{
		BaseEmployeeRequest: BaseEmployeeRequest{
			Position:          r.FormValue("position"),
			Role:              r.FormValue("role"),
			YearsOfExperience: YearsOfExperience(r.FormValue("years_of_experience")),
			Certifications:    certificationNames(certifications),
			PortfolioURL:      r.FormValue("portfolio_url"),
		},
	}

	if err := h.validate.Struct(employeeRequest); err != nil {
		closeCertificationUploads(certifications)
		internal.RespondWithValidatorError(w, err)
		return CreateEmployeeRequest{}, nil, false
	}

	return employeeRequest, certifications, true
}

func (s *employeeService) CreateEmployee(ctx context.Context, employeeReq CreateEmployeeRequest, principal auth.Principal, certifications []CertificationEntry) error {
	if err := user.AuthorizeProfileRole(principal.Role, user.UserRoleEmployee); err != nil {
		return err
	}

	// Un empleado que todavía no existe no tiene certificados que conservar.
	for _, certification := range certifications {
		if certification.KeepFileID != nil {
			return invalidCertifications("certification document not found")
		}
	}

	files, err := s.uploadCertifications(ctx, certifications)
	if err != nil {
		return err
	}

	employeeID, err := s.repo.CreateEmployee(ctx, employeeReq, principal.UserID, files)
	if err != nil {
		s.cleanupUploadedCertifications(files)
		return err
	}

	// Un perfil recién creado nunca puede estar completo, pero la comprobación se hace igual:
	// dejar la regla en nueve lugares con una excepción en el décimo es lo que después se
	// olvida de actualizar.
	s.profileChanged(ctx, employeeID)

	return nil
}

func (s *employeeService) UpdateEmployee(ctx context.Context, employeeID int32, employeeReq CreateEmployeeRequest, certifications []CertificationEntry) error {
	kept := []KeptCertificationFile{}
	for _, certification := range certifications {
		if certification.KeepFileID != nil {
			kept = append(kept, KeptCertificationFile{FileID: *certification.KeepFileID, Name: certification.Name})
		}
	}

	files, err := s.uploadCertifications(ctx, certifications)
	if err != nil {
		return err
	}

	removed, err := s.repo.UpdateEmployee(ctx, employeeID, employeeReq, kept, files)
	if err != nil {
		s.cleanupUploadedCertifications(files)
		return err
	}

	// La baja ya está confirmada en la base, así que ninguna ruta vuelve a entregar estos
	// archivos aunque el borrado falle: el fallo se registra para limpieza manual.
	for _, file := range removed {
		go s.cleanupOrphanFile(file.Bucket, file.ObjectKey)
	}

	s.profileChanged(ctx, employeeID)

	return nil
}

// uploadCertifications sube los PDFs nuevos y devuelve su metadata, cada una con el nombre de
// su certificación. Si una subida falla, borra las anteriores del mismo request.
func (s *employeeService) uploadCertifications(ctx context.Context, certifications []CertificationEntry) ([]EmployeeFileMetadata, error) {
	files := []EmployeeFileMetadata{}
	for _, certification := range certifications {
		if certification.Upload == nil {
			continue
		}

		out, err := s.uploader.Upload(ctx, uploader.UploadInput{
			File:        certification.Upload.File,
			Filename:    certification.Upload.Filename,
			ContentType: certification.Upload.ContentType,
		})
		if err != nil {
			s.cleanupUploadedCertifications(files)
			return nil, err
		}

		files = append(files, EmployeeFileMetadata{
			Type:              certificationFileType,
			Bucket:            aws.ToString(out.Bucket),
			ObjectKey:         aws.ToString(out.Key),
			OriginalFilename:  certification.Upload.Filename,
			ContentType:       certification.Upload.ContentType,
			SizeBytes:         certification.Upload.Size,
			ChecksumSHA256:    aws.ToString(out.ChecksumSHA256),
			Status:            employeeFileStatusUploaded,
			CertificationName: certification.Name,
		})
	}

	return files, nil
}

func (s *employeeService) cleanupUploadedCertifications(files []EmployeeFileMetadata) {
	for _, file := range files {
		go s.cleanupOrphanFile(file.Bucket, file.ObjectKey)
	}
}
