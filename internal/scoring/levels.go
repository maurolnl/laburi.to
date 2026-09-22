package scoring

// ExperienceLevel es el nivel de experiencia de un empleado o el exigido por un puesto.
// Los valores replican exactamente los que hoy aceptan employee.YearsOfExperience y el
// oneof de jobposition.CreateJobPositionRequest.RequiredExperience, de modo que traducir
// desde esos paquetes sea una conversión directa. levels_test.go fija esos valores para
// que una divergencia rompa la suite en vez de producir pares que nunca matchean.
type ExperienceLevel string

const (
	ExperienceLess1Y  ExperienceLevel = "less_1y"
	Experience1Y      ExperienceLevel = "1y"
	Experience2To5Y   ExperienceLevel = "2_to_5y"
	Experience5To10Y  ExperienceLevel = "5_to_10y"
	ExperienceMore10Y ExperienceLevel = "more_10y"
)

// experienceRanks ordena los niveles de menor a mayor experiencia. El orden es parte del
// contrato: un comparador necesita saber que 5_to_10y supera a 2_to_5y, y ningún paquete
// de dominio expone hoy esa relación.
var experienceRanks = map[ExperienceLevel]int{
	ExperienceLess1Y:  0,
	Experience1Y:      1,
	Experience2To5Y:   2,
	Experience5To10Y:  3,
	ExperienceMore10Y: 4,
}

func (e ExperienceLevel) Valid() bool {
	_, ok := experienceRanks[e]
	return ok
}

// Rank devuelve la posición del nivel en el orden del dominio. El segundo valor es false
// para un nivel desconocido, que no tiene posición y no debe compararse.
func (e ExperienceLevel) Rank() (int, bool) {
	rank, ok := experienceRanks[e]
	return rank, ok
}

// AtLeast informa si el nivel alcanza o supera a other. Devuelve false cuando alguno de
// los dos es desconocido: comparar contra un valor inválido nunca satisface un requisito.
func (e ExperienceLevel) AtLeast(other ExperienceLevel) bool {
	own, ok := e.Rank()
	if !ok {
		return false
	}
	target, ok := other.Rank()
	if !ok {
		return false
	}
	return own >= target
}

// EducationLevel es el nivel educativo alcanzado por un empleado o exigido por un puesto.
// Comparte valores con el oneof de employee.EmployeeEducationTitles.EducationType y con
// el de jobposition.CreateJobPositionRequest.RequiredEducationLevel.
type EducationLevel string

const (
	EducationHighSchoolOrientation EducationLevel = "high-school-orientation"
	EducationTertiary              EducationLevel = "tertiary"
	EducationUniversity            EducationLevel = "university"
	EducationPostgraduate          EducationLevel = "postgraduate"
)

// educationRanks ordena los niveles de menor a mayor. Postgrado supera a universitario,
// que supera a terciario, que supera a la orientación secundaria.
var educationRanks = map[EducationLevel]int{
	EducationHighSchoolOrientation: 0,
	EducationTertiary:              1,
	EducationUniversity:            2,
	EducationPostgraduate:          3,
}

func (e EducationLevel) Valid() bool {
	_, ok := educationRanks[e]
	return ok
}

func (e EducationLevel) Rank() (int, bool) {
	rank, ok := educationRanks[e]
	return rank, ok
}

func (e EducationLevel) AtLeast(other EducationLevel) bool {
	own, ok := e.Rank()
	if !ok {
		return false
	}
	target, ok := other.Rank()
	if !ok {
		return false
	}
	return own >= target
}
