package employee

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

// Estos tests ejercen la consulta de completitud contra el esquema real. Los tests de
// servicio usan un store falso y afirman qué se hace con la respuesta; acá se afirma la
// respuesta misma, que es donde vive la regla de las cinco etapas.

// completenessStep inserta una de las cuatro secciones que acompañan al registro base.
type completenessStep struct {
	name   string
	insert func(*testing.T, *sql.Tx, int32)
}

func completenessSteps() []completenessStep {
	return []completenessStep{
		{"location", insertTestLocation},
		{"tech", insertTestTech},
		{"availability", insertTestAvailability},
		{"education", insertTestEducation},
	}
}

func TestIsEmployeeProfileCompleteRequiresEveryStep(t *testing.T) {
	steps := completenessSteps()

	t.Run("perfil con las cinco etapas", func(t *testing.T) {
		tx := testsupport.PostgresTx(t)
		employeeID := insertTestEmployee(t, tx)
		for _, step := range steps {
			step.insert(t, tx, employeeID)
		}

		if !isProfileComplete(t, tx, employeeID) {
			t.Fatal("un perfil con sus cinco etapas debería estar completo")
		}
	})

	// Quitar una etapa por vez comprueba que ninguna es prescindible: un olvido en la consulta
	// haría pasar el caso completo y fallar exactamente el subtest de la etapa olvidada.
	for _, missing := range steps {
		t.Run("sin "+missing.name, func(t *testing.T) {
			tx := testsupport.PostgresTx(t)
			employeeID := insertTestEmployee(t, tx)
			for _, step := range steps {
				if step.name == missing.name {
					continue
				}
				step.insert(t, tx, employeeID)
			}

			if isProfileComplete(t, tx, employeeID) {
				t.Fatalf("un perfil sin %s no debería estar completo", missing.name)
			}
		})
	}
}

// TestIsEmployeeProfileCompleteIgnoresOptionalTechFields fija la decisión de diseño: la etapa
// de recursos técnicos cuenta por existir, no por tener datos. Su validación admite os y
// paid_software vacíos, así que exigir contenido dejaría esos perfiles fuera para siempre.
func TestIsEmployeeProfileCompleteIgnoresOptionalTechFields(t *testing.T) {
	tx := testsupport.PostgresTx(t)
	employeeID := insertTestEmployee(t, tx)
	insertTestLocation(t, tx, employeeID)
	insertTestAvailability(t, tx, employeeID)
	insertTestEducation(t, tx, employeeID)

	mustExec(t, tx, `
		INSERT INTO employee_profile_tech (employee_id, os, paid_software, created_at, updated_at)
		VALUES ($1, NULL, '{}', now(), now())
	`, employeeID)

	if !isProfileComplete(t, tx, employeeID) {
		t.Fatal("una etapa de recursos técnicos sin datos opcionales debería contar igual")
	}
}

// TestIsEmployeeProfileCompleteForUnknownEmployee documenta que un empleado inexistente
// responde false y no error: quien pregunta quiere saber si corresponde disparar.
func TestIsEmployeeProfileCompleteForUnknownEmployee(t *testing.T) {
	tx := testsupport.PostgresTx(t)

	if isProfileComplete(t, tx, 999_999) {
		t.Fatal("un empleado inexistente no debería estar completo")
	}
}

func isProfileComplete(t *testing.T, tx *sql.Tx, employeeID int32) bool {
	t.Helper()

	complete, err := database.New(tx).IsEmployeeProfileComplete(context.Background(), employeeID)
	if err != nil {
		t.Fatalf("IsEmployeeProfileComplete(%d): %v", employeeID, err)
	}

	return complete
}

func insertTestEmployee(t *testing.T, tx *sql.Tx) int32 {
	t.Helper()

	var userID int32
	email := fmt.Sprintf("completeness-%d@laburi.to", time.Now().UnixNano())
	if err := tx.QueryRowContext(context.Background(), `
		INSERT INTO users (email, hashed_password, role, created_at, updated_at)
		VALUES ($1, 'hash', 'employee', now(), now())
		RETURNING id
	`, email).Scan(&userID); err != nil {
		t.Fatalf("crear usuario: %v", err)
	}

	var employeeID int32
	if err := tx.QueryRowContext(context.Background(), `
		INSERT INTO employees (user_id, position, role, years_of_experience, created_at, updated_at)
		VALUES ($1, 'Backend Engineer', 'Go developer', '2_to_5y', now(), now())
		RETURNING id
	`, userID).Scan(&employeeID); err != nil {
		t.Fatalf("crear empleado: %v", err)
	}

	return employeeID
}

func insertTestLocation(t *testing.T, tx *sql.Tx, employeeID int32) {
	t.Helper()
	mustExec(t, tx, `
		INSERT INTO employee_location (employee_id, timezone, created_at, updated_at)
		VALUES ($1, 'America/Argentina/Buenos_Aires', now(), now())
	`, employeeID)
}

func insertTestTech(t *testing.T, tx *sql.Tx, employeeID int32) {
	t.Helper()
	mustExec(t, tx, `
		INSERT INTO employee_profile_tech (employee_id, os, paid_software, created_at, updated_at)
		VALUES ($1, 'macOS', '{"JetBrains"}', now(), now())
	`, employeeID)
}

func insertTestAvailability(t *testing.T, tx *sql.Tx, employeeID int32) {
	t.Helper()
	mustExec(t, tx, `
		INSERT INTO employee_profile_availability (employee_id, available_hours_per_day, created_at, updated_at)
		VALUES ($1, 6, now(), now())
	`, employeeID)
}

func insertTestEducation(t *testing.T, tx *sql.Tx, employeeID int32) {
	t.Helper()
	mustExec(t, tx, `
		INSERT INTO employee_education (employee_id, education_type, title, status, created_at, updated_at)
		VALUES ($1, 'university', 'Ingeniería en Sistemas', 'completed', now(), now())
	`, employeeID)
}

func mustExec(t *testing.T, tx *sql.Tx, query string, args ...any) {
	t.Helper()

	if _, err := tx.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("ejecutar %q: %v", query, err)
	}
}
