package jobposition

import (
	"strings"
	"time"
)

// CreateJobPositionRequest es el cuerpo aceptado al publicar un puesto. La edición
// reemplaza exactamente los mismos campos con las mismas reglas de dominio, por lo que
// UpdateJobPositionRequest es un alias y no una copia: divergir una de las dos exigiría
// cambiar antes el spec de `job-position-api`.
type CreateJobPositionRequest struct {
	Position               string   `json:"position" validate:"required"`
	Role                   string   `json:"role" validate:"required"`
	RequiredExperience     string   `json:"required_experience" validate:"required,oneof=less_1y 1y 2_to_5y 5_to_10y more_10y"`
	RequiredEducationLevel string   `json:"required_education_level" validate:"required,oneof=university postgraduate high-school-orientation tertiary"`
	AvailableHoursPerDay   int16    `json:"available_hours_per_day" validate:"required,min=1,max=8"`
	Timezone               string   `json:"timezone" validate:"required,min=2,max=100"`
	TechnicalResources     []string `json:"technical_resources" validate:"dive,required"`
}

type UpdateJobPositionRequest = CreateJobPositionRequest

// Normalize recorta los espacios exteriores de los strings y garantiza que los recursos
// técnicos sean siempre una lista, incluso cuando el cliente omite el campo o envía null.
func (r *CreateJobPositionRequest) Normalize() {
	r.Position = strings.TrimSpace(r.Position)
	r.Role = strings.TrimSpace(r.Role)
	r.RequiredExperience = strings.TrimSpace(r.RequiredExperience)
	r.RequiredEducationLevel = strings.TrimSpace(r.RequiredEducationLevel)
	r.Timezone = strings.TrimSpace(r.Timezone)

	if r.TechnicalResources == nil {
		r.TechnicalResources = []string{}
	}
	for i := range r.TechnicalResources {
		r.TechnicalResources[i] = strings.TrimSpace(r.TechnicalResources[i])
	}
}

type JobPosition struct {
	ID                     int32     `json:"id"`
	EmployerID             int32     `json:"employer_id"`
	Position               string    `json:"position"`
	Role                   string    `json:"role"`
	RequiredExperience     string    `json:"required_experience"`
	RequiredEducationLevel string    `json:"required_education_level"`
	AvailableHoursPerDay   int16     `json:"available_hours_per_day"`
	Timezone               string    `json:"timezone"`
	TechnicalResources     []string  `json:"technical_resources"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// Normalize evita exponer null en technical_resources: el contrato siempre entrega una lista.
func (j *JobPosition) Normalize() {
	if j.TechnicalResources == nil {
		j.TechnicalResources = []string{}
	}
}
