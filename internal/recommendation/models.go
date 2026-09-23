package recommendation

import (
	"time"
)

// SubjectType distingue el sentido de la recomendación: un batch por empleado calcula
// puestos sugeridos, y uno por puesto calcula candidatos sugeridos.
type SubjectType string

const (
	SubjectEmployee    SubjectType = "employee"
	SubjectJobPosition SubjectType = "job_position"
)

func (s SubjectType) Valid() bool {
	return s == SubjectEmployee || s == SubjectJobPosition
}

// BatchStatus es el ciclo de vida de una ejecución. Los valores replican exactamente el
// check recommendation_batches_status_check de la migración 0007.
type BatchStatus string

const (
	BatchPending    BatchStatus = "pending"
	BatchProcessing BatchStatus = "processing"
	BatchCompleted  BatchStatus = "completed"
	BatchFailed     BatchStatus = "failed"
)

func (s BatchStatus) Valid() bool {
	switch s {
	case BatchPending, BatchProcessing, BatchCompleted, BatchFailed:
		return true
	default:
		return false
	}
}

// Batch es una ejecución de generación de recomendaciones para un sujeto. EmployeeID y
// JobPositionID son excluyentes: exactamente uno está presente, y cuál de los dos lo
// determina SubjectType.
type Batch struct {
	ID            int32       `json:"id"`
	SubjectType   SubjectType `json:"subject_type"`
	EmployeeID    *int32      `json:"employee_id,omitempty"`
	JobPositionID *int32      `json:"job_position_id,omitempty"`
	Status        BatchStatus `json:"status"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// NewEmployeeSubject y NewJobPositionSubject construyen el sujeto de un batch sin dejar
// que el llamador arme una combinación inválida.
type Subject struct {
	Type          SubjectType
	EmployeeID    *int32
	JobPositionID *int32
}

func NewEmployeeSubject(employeeID int32) Subject {
	return Subject{Type: SubjectEmployee, EmployeeID: &employeeID}
}

func NewJobPositionSubject(jobPositionID int32) Subject {
	return Subject{Type: SubjectJobPosition, JobPositionID: &jobPositionID}
}

func (s Subject) Valid() bool {
	switch s.Type {
	case SubjectEmployee:
		return s.EmployeeID != nil && s.JobPositionID == nil
	case SubjectJobPosition:
		return s.JobPositionID != nil && s.EmployeeID == nil
	default:
		return false
	}
}

// Candidate es una recomendación a persistir. Score es un puntero porque la ausencia de
// puntaje y el puntaje cero son estados distintos: LAB-30 todavía no definió los
// indicadores y la épica prohíbe inventar un algoritmo temporal.
type Candidate struct {
	EmployeeID    int32
	JobPositionID int32
	Score         *float64
}

// JobRecommendation es un puesto recomendado a un empleado.
type JobRecommendation struct {
	RecommendationID       int32     `json:"recommendation_id"`
	JobPositionID          int32     `json:"job_position_id"`
	EmployerID             int32     `json:"employer_id"`
	Position               string    `json:"position"`
	Role                   string    `json:"role"`
	RequiredExperience     string    `json:"required_experience"`
	RequiredEducationLevel string    `json:"required_education_level"`
	AvailableHoursPerDay   int16     `json:"available_hours_per_day"`
	Timezone               string    `json:"timezone"`
	TechnicalResources     []string  `json:"technical_resources"`
	Score                  *float64  `json:"score"`
	PublishedAt            time.Time `json:"published_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// EmployeeRecommendation es un empleado recomendado para un puesto.
type EmployeeRecommendation struct {
	RecommendationID  int32     `json:"recommendation_id"`
	EmployeeID        int32     `json:"employee_id"`
	UserID            int32     `json:"user_id"`
	Position          string    `json:"position"`
	Role              string    `json:"role"`
	YearsOfExperience string    `json:"years_of_experience"`
	Certifications    []string  `json:"certifications"`
	PortfolioURL      *string   `json:"portfolio_url"`
	Score             *float64  `json:"score"`
	CreatedAt         time.Time `json:"created_at"`
	ProfileUpdatedAt  time.Time `json:"profile_updated_at"`
}

// Page acota una lectura del conjunto vigente. El contrato HTTP expone límite y
// desplazamiento y no un cursor opaco: el conjunto vigente es inmutable entre batches, así que
// no hay inserciones intercaladas que corran la ventana, que es el problema que un cursor
// resuelve.
type Page struct {
	Limit  int32
	Offset int32
}

// PageInfo son los metadatos de paginación que viajan en la respuesta. Total es el tamaño del
// conjunto vigente ya filtrado, para que el cliente sepa cuándo dejar de pedir páginas sin
// tener que descubrirlo con una respuesta vacía.
type PageInfo struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
	Total  int32 `json:"total"`
}

// QueryStatus es el estado que el borde HTTP le informa al cliente. Replica los cuatro estados
// de BatchStatus y agrega StatusNone.
//
// StatusNone no puede vivir en BatchStatus: ese tipo replica exactamente el check
// recommendation_batches_status_check de la migración 0007, y agregarle un valor que la base no
// conoce lo desalinearía del esquema. «Nunca se solicitó una generación» tampoco es un estado
// de batch, justamente porque no hay batch.
type QueryStatus string

const (
	StatusNone       QueryStatus = "none"
	StatusPending    QueryStatus = "pending"
	StatusProcessing QueryStatus = "processing"
	StatusCompleted  QueryStatus = "completed"
	StatusFailed     QueryStatus = "failed"
)

// queryStatusFrom traduce el estado del batch vigente al estado de transporte. Un estado que la
// base aceptó pero este código no conoce se informa como StatusNone en vez de propagarse tal
// cual: el cliente distingue cinco valores y no debe recibir un sexto.
func queryStatusFrom(status BatchStatus) QueryStatus {
	switch status {
	case BatchPending:
		return StatusPending
	case BatchProcessing:
		return StatusProcessing
	case BatchCompleted:
		return StatusCompleted
	case BatchFailed:
		return StatusFailed
	default:
		return StatusNone
	}
}

// JobRecommendationsResponse y EmployeeRecommendationsResponse son la forma exacta del cuerpo
// que devuelven las dos consultas. Items nunca es nil: un arreglo vacío y null son cosas
// distintas para el cliente, y el conjunto vacío es el caso más común de un usuario nuevo.
type JobRecommendationsResponse struct {
	Status QueryStatus         `json:"status"`
	Items  []JobRecommendation `json:"items"`
	Page   PageInfo            `json:"page"`
}

type EmployeeRecommendationsResponse struct {
	Status QueryStatus              `json:"status"`
	Items  []EmployeeRecommendation `json:"items"`
	Page   PageInfo                 `json:"page"`
}

// id devuelve el identificador del sujeto sin importar su tipo. Sirve para diagnóstico: un
// sujeto inválido no tiene identificador y devuelve cero.
func (s Subject) id() int32 {
	switch {
	case s.EmployeeID != nil:
		return *s.EmployeeID
	case s.JobPositionID != nil:
		return *s.JobPositionID
	default:
		return 0
	}
}
