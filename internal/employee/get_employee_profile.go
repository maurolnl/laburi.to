package employee

import (
	"context"
	"errors"
	"net/http"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

// GetEmployeeProfile expone el perfil completo de un empleado a su dueño y a un empleador con
// recomendación vigente. El identificador del path es el del empleado y no el del usuario,
// porque quien llega acá desde una recomendación conoce lo primero y no lo segundo.
func (h *EmployeeHandler) GetEmployeeProfile(w http.ResponseWriter, r *http.Request) {
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

	profile, err := h.service.GetEmployeeProfile(r.Context(), employeeID, principal)
	if err != nil {
		respondWithProfileError(w, err, ErrInternalErrorReadingProfile)
		return
	}

	internal.RespondWithJSON(w, http.StatusOK, profile)
}

func (s *employeeService) GetEmployeeProfile(ctx context.Context, employeeID int32, principal auth.Principal) (EmployeeProfileResponse, error) {
	viewer, err := s.authorizeProfileAccess(ctx, employeeID, principal)
	if err != nil {
		return EmployeeProfileResponse{}, err
	}

	profile, err := s.repo.GetEmployeeProfileByID(ctx, employeeID)
	// La autorización ya resolvió que el empleado existe; que desaparezca entre ambas lecturas
	// es una carrera, y la respuesta correcta sigue siendo la misma que la del perfil ajeno.
	if errors.Is(err, ErrEmployeeProfileNotFound) {
		return EmployeeProfileResponse{}, ErrProfileAccessForbidden
	}
	if err != nil {
		return EmployeeProfileResponse{}, err
	}

	return profileResponse(profile, viewer), nil
}

// profileResponse arma el cuerpo. Las tres listas nunca viajan nulas: un arreglo vacío y null
// son cosas distintas para el cliente, y el perfil a medio completar es el caso más común.
//
// El correo solo se incluye para el dueño. La omisión se hace acá, en el único lugar por el que
// pasan los dos actores, y no en la consulta: la base devuelve el perfil entero y quién puede
// ver qué es una decisión del borde.
func profileResponse(profile EmployeeProfile, viewer profileViewer) EmployeeProfileResponse {
	response := EmployeeProfileResponse{
		ID:                   profile.ID,
		UserID:               profile.UserID,
		Position:             profile.Position,
		Role:                 profile.Role,
		YearsOfExperience:    profile.YearsOfExperience,
		Certifications:       profile.Certifications,
		PortfolioURL:         profile.PortfolioURL,
		Timezone:             profile.Timezone,
		Os:                   profile.Os,
		PaidSoftware:         profile.PaidSoftware,
		AvailableHoursPerDay: profile.AvailableHoursPerDay,
		CompatibleProjects:   profile.CompatibleProjects,
		IncompatibleProjects: profile.IncompatibleProjects,
		InternetConnections:  profile.InternetConnections,
		Education:            profile.Education,
		Files:                profile.Files,
		CreatedAt:            profile.CreatedAt,
		UpdatedAt:            profile.UpdatedAt,
	}

	if response.Certifications == nil {
		response.Certifications = []string{}
	}
	if response.PaidSoftware == nil {
		response.PaidSoftware = []string{}
	}
	if response.InternetConnections == nil {
		response.InternetConnections = []InternetConnection{}
	}
	if response.Education == nil {
		response.Education = []ProfileEducationItem{}
	}
	if response.Files == nil {
		response.Files = []ProfileFileItem{}
	}

	if viewer.isOwner {
		email := profile.Email
		response.Email = &email
	}

	return response
}

// positivePathID exige un entero positivo. El cero y los negativos se rechazan en el borde en
// vez de llegar a la base como identificadores que nunca van a existir.
func positivePathID(w http.ResponseWriter, r *http.Request, key string, invalidErr error) (int32, bool) {
	id, err := internal.GetPathValueAsInt(r, key)
	if err != nil || id <= 0 {
		internal.RespondWithError(w, http.StatusBadRequest, invalidErr.Error())
		return 0, false
	}

	return id, true
}

// respondWithProfileError traduce los sentinelas del dominio a códigos HTTP. El 403 lleva un
// mensaje único: el perfil ajeno, el empleador sin vínculo y el empleado inexistente deben ser
// indistinguibles para quien pregunta.
func respondWithProfileError(w http.ResponseWriter, err, internalErr error) {
	switch {
	case errors.Is(err, ErrProfileAccessForbidden):
		internal.RespondWithError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, ErrFileNotFound):
		internal.RespondWithError(w, http.StatusNotFound, ErrFileNotFound.Error())
	default:
		internal.RespondWithError(w, http.StatusInternalServerError, internalErr.Error())
	}
}
