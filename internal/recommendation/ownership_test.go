package recommendation

import (
	"context"
	"errors"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

// Los dueños esperados se leen directamente de la base en vez de devolverse desde las
// factorías: así el test contrasta la consulta contra el esquema real y no contra lo que la
// factoría creyó haber insertado.
func TestEmployeeOwner(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("empleado existente", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)

		var want int32
		if err := db.QueryRowContext(ctx, `SELECT user_id FROM employees WHERE id = $1`, employeeID).Scan(&want); err != nil {
			t.Fatalf("leer el dueño esperado: %v", err)
		}

		got, err := repo.EmployeeOwner(ctx, employeeID)
		if err != nil {
			t.Fatalf("resolver la propiedad: %v", err)
		}
		if got != want {
			t.Fatalf("se esperaba el usuario %d, se obtuvo %d", want, got)
		}
	})

	t.Run("empleado inexistente", func(t *testing.T) {
		if _, err := repo.EmployeeOwner(ctx, 0); !errors.Is(err, ErrSubjectNotFound) {
			t.Fatalf("se esperaba ErrSubjectNotFound, se obtuvo %v", err)
		}
	})
}

func TestJobPositionOwner(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("puesto activo", func(t *testing.T) {
		employerID := newTestEmployer(t, db)
		jobPositionID := newTestJobPosition(t, db, employerID)

		var wantUserID int32
		if err := db.QueryRowContext(ctx, `SELECT user_id FROM employers WHERE id = $1`, employerID).Scan(&wantUserID); err != nil {
			t.Fatalf("leer el dueño esperado: %v", err)
		}

		got, err := repo.JobPositionOwner(ctx, jobPositionID)
		if err != nil {
			t.Fatalf("resolver la propiedad: %v", err)
		}
		if got.EmployerID != employerID || got.UserID != wantUserID {
			t.Fatalf("se esperaba el empleador %d del usuario %d, se obtuvo %+v", employerID, wantUserID, got)
		}
	})

	// Un puesto eliminado lógicamente y uno que nunca existió deben ser indistinguibles para
	// quien consulta: los dos son ErrSubjectNotFound y el borde los traduce al mismo 404.
	t.Run("puesto eliminado lógicamente", func(t *testing.T) {
		employerID := newTestEmployer(t, db)
		jobPositionID := newTestJobPosition(t, db, employerID, withTestJobPositionDeleted())

		if _, err := repo.JobPositionOwner(ctx, jobPositionID); !errors.Is(err, ErrSubjectNotFound) {
			t.Fatalf("se esperaba ErrSubjectNotFound, se obtuvo %v", err)
		}
	})

	t.Run("puesto inexistente", func(t *testing.T) {
		if _, err := repo.JobPositionOwner(ctx, 0); !errors.Is(err, ErrSubjectNotFound) {
			t.Fatalf("se esperaba ErrSubjectNotFound, se obtuvo %v", err)
		}
	})
}
