package scoring

// EmployeeProfile es el lado empleado de la entrada normalizada: solo los atributos que
// el dominio declara comparables contra un puesto. No es el perfil completo y no debe
// crecer con datos que el scoring no compara.
//
// Los punteros y el slice nil expresan ausencia, que es distinta del valor cero: el
// perfil de empleado se completa en cinco pasos y un empleado puede no haber llegado
// todavía al paso de ubicación, disponibilidad o educación. Cero horas disponibles y
// horas desconocidas son estados distintos, y un algoritmo futuro puede querer tratarlos
// diferente.
type EmployeeProfile struct {
	EmployeeID int32

	// Experience siempre está presente: es parte del primer paso del perfil.
	Experience ExperienceLevel

	// HighestEducation es el nivel educativo más alto alcanzado, o nil si el empleado no
	// completó el paso de educación.
	HighestEducation *EducationLevel

	// AvailableHoursPerDay es nil mientras el empleado no complete el paso de
	// disponibilidad.
	AvailableHoursPerDay *int16

	// Timezone es nil mientras el empleado no complete el paso de ubicación.
	Timezone *string

	// TechnicalResources es nil cuando el empleado no informó sus recursos, y un slice
	// vacío cuando informó explícitamente que no tiene ninguno. Qué cuenta como recurso
	// del empleado lo decide quien arma la entrada.
	TechnicalResources []string
}

// JobRequirements es el lado puesto de la entrada normalizada. A diferencia del empleado,
// un puesto se publica completo: todos sus atributos son obligatorios en el contrato HTTP
// que lo crea, así que ninguno admite ausencia.
type JobRequirements struct {
	JobPositionID          int32
	RequiredExperience     ExperienceLevel
	RequiredEducationLevel EducationLevel
	AvailableHoursPerDay   int16
	Timezone               string

	// TechnicalResources nil y vacío son equivalentes aquí: el puesto siempre normaliza
	// la lista antes de persistirse.
	TechnicalResources []string
}

// Pair es la unidad de puntuación: un empleado y un puesto concretos. Conserva ambos
// identificadores, que son exactamente los que recommendation.Candidate necesita para
// persistir el resultado sin ninguna correlación adicional.
type Pair struct {
	Employee EmployeeProfile
	Job      JobRequirements
}

// IDs devuelve los identificadores del par en el mismo orden en que los espera
// recommendation.Candidate.
func (p Pair) IDs() (employeeID, jobPositionID int32) {
	return p.Employee.EmployeeID, p.Job.JobPositionID
}

// Indicator es un componente del puntaje: qué mide, cuánto pesa y qué valor tomó. Los
// indicadores viven solo en memoria. A la persistencia viaja únicamente Result.Total, de
// modo que agregar, quitar o repesar un indicador no toca el esquema ni el transporte.
type Indicator struct {
	Name   string
	Weight float64
	Value  float64
}

// Contribution es el aporte del indicador al total.
func (i Indicator) Contribution() float64 {
	return i.Weight * i.Value
}

// Eligibility es la decisión de un filtro duro. Reason solo es significativo cuando el
// par es inelegible; para un par elegible siempre queda vacío.
//
// Reason es texto libre y no una enumeración a propósito: los filtros duros del producto
// todavía no están definidos, y enumerarlos ahora sería inventar reglas que la épica
// prohíbe.
type Eligibility struct {
	Eligible bool
	Reason   string
}

// Accepted construye la decisión de un filtro que deja pasar el par.
func Accepted() Eligibility {
	return Eligibility{Eligible: true}
}

// Discarded construye la decisión de un filtro que descarta el par, con su motivo.
func Discarded(reason string) Eligibility {
	return Eligibility{Eligible: false, Reason: reason}
}

// Result es el desenlace de evaluar un par. Tres estados conviven en este único tipo
// porque el llamador recorre una lista homogénea y decide par por par:
//
//   - Eligible true con Total no nil: puntuado, incluso si el puntaje es cero.
//   - Eligible false: descartado por filtro duro; Total es nil y Reason dice por qué.
//   - Eligible true con Total nil: elegible pero sin puntaje disponible. No es un error.
//
// Construir un Result a mano permite combinaciones incoherentes. Usar Scored, Unscored y
// Rejected.
type Result struct {
	EmployeeID    int32
	JobPositionID int32
	Eligible      bool
	Reason        string
	Total         *float64
	Indicators    []Indicator
}

// Scored construye el resultado de un par puntuado. Los indicadores son opcionales: una
// implementación que calcule solo un subconjunto de los indicadores previstos produce un
// resultado igualmente válido.
func Scored(pair Pair, total float64, indicators ...Indicator) Result {
	employeeID, jobPositionID := pair.IDs()
	return Result{
		EmployeeID:    employeeID,
		JobPositionID: jobPositionID,
		Eligible:      true,
		Total:         &total,
		Indicators:    indicators,
	}
}

// Unscored construye el resultado de un par elegible para el que no hay puntaje. Es un
// desenlace legítimo y distinto de un puntaje cero.
func Unscored(pair Pair) Result {
	employeeID, jobPositionID := pair.IDs()
	return Result{
		EmployeeID:    employeeID,
		JobPositionID: jobPositionID,
		Eligible:      true,
	}
}

// Rejected construye el resultado de un par descartado por filtro duro. Nunca lleva
// puntaje ni indicadores.
func Rejected(pair Pair, reason string) Result {
	employeeID, jobPositionID := pair.IDs()
	return Result{
		EmployeeID:    employeeID,
		JobPositionID: jobPositionID,
		Eligible:      false,
		Reason:        reason,
	}
}

// HasScore distingue el puntaje ausente del puntaje cero sin que el llamador manipule el
// puntero.
func (r Result) HasScore() bool {
	return r.Total != nil
}
