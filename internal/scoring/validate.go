package scoring

import "fmt"

// Validate comprueba que el par tenga identificadores y niveles utilizables. Es el
// chequeo que separa una entrada mal armada de una dependencia no disponible: todo error
// que devuelve envuelve ErrInvalidPair.
func (p Pair) Validate() error {
	if p.Employee.EmployeeID <= 0 {
		return fmt.Errorf("%w: employee ID must be positive", ErrInvalidPair)
	}
	if p.Job.JobPositionID <= 0 {
		return fmt.Errorf("%w: job position ID must be positive", ErrInvalidPair)
	}
	if !p.Employee.Experience.Valid() {
		return fmt.Errorf("%w: %w: %q", ErrInvalidPair, ErrInvalidExperienceLevel, p.Employee.Experience)
	}
	if !p.Job.RequiredExperience.Valid() {
		return fmt.Errorf("%w: %w: %q", ErrInvalidPair, ErrInvalidExperienceLevel, p.Job.RequiredExperience)
	}
	if !p.Job.RequiredEducationLevel.Valid() {
		return fmt.Errorf("%w: %w: %q", ErrInvalidPair, ErrInvalidEducationLevel, p.Job.RequiredEducationLevel)
	}
	// La educación del empleado es opcional, pero si está presente debe ser un nivel
	// conocido: un valor desconocido no es lo mismo que no haber informado educación.
	if p.Employee.HighestEducation != nil && !p.Employee.HighestEducation.Valid() {
		return fmt.Errorf("%w: %w: %q", ErrInvalidPair, ErrInvalidEducationLevel, *p.Employee.HighestEducation)
	}
	return nil
}
