package recommendation

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

// pgErrorCode devuelve el SQLSTATE de un error de PostgreSQL, o "" si no lo es.
func pgErrorCode(err error) string {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code)
	}
	return ""
}

func insertBatchRaw(db *sql.DB, subjectType string, employeeID, jobPositionID sql.NullInt32, status string) error {
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO recommendation_batches (subject_type, employee_id, job_position_id, status)
		VALUES ($1, $2, $3, $4)
	`, subjectType, employeeID, jobPositionID, status)
	return err
}

func TestSujetoExcluyente(t *testing.T) {
	db := testsupport.PostgresDB(t)

	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	jobPositionID := newTestJobPosition(t, db, employerID)

	employee := sql.NullInt32{Int32: employeeID, Valid: true}
	jobPosition := sql.NullInt32{Int32: jobPositionID, Valid: true}
	none := sql.NullInt32{}

	t.Run("acepta un batch por empleado", func(t *testing.T) {
		if err := insertBatchRaw(db, "employee", employee, none, "pending"); err != nil {
			t.Fatalf("debería aceptarse: %v", err)
		}
	})

	t.Run("acepta un batch por puesto", func(t *testing.T) {
		if err := insertBatchRaw(db, "job_position", none, jobPosition, "pending"); err != nil {
			t.Fatalf("debería aceptarse: %v", err)
		}
	})

	invalid := []struct {
		name          string
		subjectType   string
		employeeID    sql.NullInt32
		jobPositionID sql.NullInt32
		status        string
		wantCode      string
	}{
		{"dos sujetos", "employee", employee, jobPosition, "pending", "23514"},
		{"ningún sujeto", "employee", none, none, "pending", "23514"},
		{"sujeto cruzado con el tipo", "employee", none, jobPosition, "pending", "23514"},
		{"tipo de sujeto desconocido", "company", employee, none, "pending", "23514"},
		{"estado desconocido", "employee", employee, none, "archived", "23514"},
	}

	for _, tc := range invalid {
		t.Run("rechaza "+tc.name, func(t *testing.T) {
			err := insertBatchRaw(db, tc.subjectType, tc.employeeID, tc.jobPositionID, tc.status)
			if err == nil {
				t.Fatal("la base debería rechazar la operación")
			}
			if got := pgErrorCode(err); got != tc.wantCode {
				t.Fatalf("SQLSTATE esperado %s, se obtuvo %s (%v)", tc.wantCode, got, err)
			}
		})
	}

	t.Run("rechaza un sujeto inexistente", func(t *testing.T) {
		err := insertBatchRaw(db, "employee", sql.NullInt32{Int32: 999999, Valid: true}, none, "pending")
		if got := pgErrorCode(err); got != "23503" {
			t.Fatalf("debería violar la clave foránea, se obtuvo %s (%v)", got, err)
		}
	})
}

func TestCicloDeEstadosDelBatch(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employeeID := newTestEmployee(t, db)

	t.Run("la creación deja el batch en pending", func(t *testing.T) {
		batch, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
		if err != nil {
			t.Fatalf("crear batch: %v", err)
		}
		if batch.Status != BatchPending {
			t.Fatalf("estado esperado pending, se obtuvo %s", batch.Status)
		}
		if batch.CreatedAt.IsZero() || batch.UpdatedAt.IsZero() {
			t.Fatal("la creación debería inicializar las marcas temporales")
		}
		if batch.EmployeeID == nil || *batch.EmployeeID != employeeID {
			t.Fatalf("el batch debería apuntar al empleado %d", employeeID)
		}
		if batch.JobPositionID != nil {
			t.Fatal("un batch por empleado no debería tener puesto")
		}
	})

	t.Run("acepta los cuatro estados", func(t *testing.T) {
		for _, status := range []BatchStatus{BatchProcessing, BatchCompleted, BatchFailed, BatchPending} {
			id := newTestBatch(t, db, NewJobPositionSubject(newTestJobPosition(t, db, newTestEmployer(t, db))))
			batch, err := repo.TransitionBatch(ctx, id, status)
			if err != nil {
				t.Fatalf("transicionar a %s: %v", status, err)
			}
			if batch.Status != status {
				t.Fatalf("estado esperado %s, se obtuvo %s", status, batch.Status)
			}
		}
	})

	t.Run("rechaza un estado desconocido", func(t *testing.T) {
		id := newTestBatch(t, db, NewEmployeeSubject(newTestEmployee(t, db)))
		if _, err := repo.TransitionBatch(ctx, id, BatchStatus("archived")); !errors.Is(err, ErrInvalidBatchStatus) {
			t.Fatalf("se esperaba ErrInvalidBatchStatus, se obtuvo %v", err)
		}
	})

	t.Run("una transición refresca updated_at", func(t *testing.T) {
		id := newTestBatch(t, db, NewEmployeeSubject(newTestEmployee(t, db)))

		var before time.Time
		if err := db.QueryRowContext(ctx, `SELECT updated_at FROM recommendation_batches WHERE id = $1`, id).Scan(&before); err != nil {
			t.Fatalf("leer updated_at: %v", err)
		}

		batch, err := repo.TransitionBatch(ctx, id, BatchProcessing)
		if err != nil {
			t.Fatalf("transicionar: %v", err)
		}
		if !batch.UpdatedAt.After(before) {
			t.Fatalf("updated_at debería avanzar: antes %s, después %s", before, batch.UpdatedAt)
		}
	})

	t.Run("transicionar un batch inexistente no modifica nada", func(t *testing.T) {
		if _, err := repo.TransitionBatch(ctx, 999999, BatchCompleted); !errors.Is(err, ErrBatchNotFound) {
			t.Fatalf("se esperaba ErrBatchNotFound, se obtuvo %v", err)
		}
	})
}

func TestUnicidadDelBatchEnCurso(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("rechaza un segundo batch en curso para el mismo sujeto", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		if _, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID)); err != nil {
			t.Fatalf("primer batch: %v", err)
		}
		if _, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID)); !errors.Is(err, ErrBatchAlreadyInFlight) {
			t.Fatalf("se esperaba ErrBatchAlreadyInFlight, se obtuvo %v", err)
		}
	})

	t.Run("rechaza también cuando el anterior está processing", func(t *testing.T) {
		employerID := newTestEmployer(t, db)
		jobPositionID := newTestJobPosition(t, db, employerID)

		first, err := repo.CreateBatch(ctx, NewJobPositionSubject(jobPositionID))
		if err != nil {
			t.Fatalf("primer batch: %v", err)
		}
		if _, err := repo.TransitionBatch(ctx, first.ID, BatchProcessing); err != nil {
			t.Fatalf("transicionar: %v", err)
		}
		if _, err := repo.CreateBatch(ctx, NewJobPositionSubject(jobPositionID)); !errors.Is(err, ErrBatchAlreadyInFlight) {
			t.Fatalf("se esperaba ErrBatchAlreadyInFlight, se obtuvo %v", err)
		}
	})

	for _, terminal := range []BatchStatus{BatchCompleted, BatchFailed} {
		t.Run("acepta uno nuevo cuando el anterior está "+string(terminal), func(t *testing.T) {
			employeeID := newTestEmployee(t, db)

			first, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
			if err != nil {
				t.Fatalf("primer batch: %v", err)
			}
			if _, err := repo.TransitionBatch(ctx, first.ID, terminal); err != nil {
				t.Fatalf("transicionar: %v", err)
			}
			if _, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID)); err != nil {
				t.Fatalf("debería aceptarse un batch nuevo: %v", err)
			}
		})
	}

	t.Run("acepta batches en curso simultáneos para sujetos distintos", func(t *testing.T) {
		if _, err := repo.CreateBatch(ctx, NewEmployeeSubject(newTestEmployee(t, db))); err != nil {
			t.Fatalf("primer sujeto: %v", err)
		}
		if _, err := repo.CreateBatch(ctx, NewEmployeeSubject(newTestEmployee(t, db))); err != nil {
			t.Fatalf("segundo sujeto: %v", err)
		}
	})
}

func TestRelacionManyToMany(t *testing.T) {
	db := testsupport.PostgresDB(t)
	ctx := context.Background()

	employerID := newTestEmployer(t, db)

	t.Run("un puesto admite varios empleados", func(t *testing.T) {
		jobPositionID := newTestJobPosition(t, db, employerID)
		batchID := newTestBatch(t, db, NewJobPositionSubject(jobPositionID))

		for i := 0; i < 3; i++ {
			newTestRecommendation(t, db, batchID, newTestEmployee(t, db), jobPositionID, nil)
		}

		if got := countRecommendations(t, db, batchID); got != 3 {
			t.Fatalf("se esperaban 3 recomendaciones, se obtuvieron %d", got)
		}
	})

	t.Run("un empleado admite varios puestos", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID))

		for i := 0; i < 3; i++ {
			newTestRecommendation(t, db, batchID, employeeID, newTestJobPosition(t, db, employerID), nil)
		}

		if got := countRecommendations(t, db, batchID); got != 3 {
			t.Fatalf("se esperaban 3 recomendaciones, se obtuvieron %d", got)
		}
	})

	t.Run("rechaza la misma dupla dos veces en el mismo batch", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		jobPositionID := newTestJobPosition(t, db, employerID)
		batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID))

		newTestRecommendation(t, db, batchID, employeeID, jobPositionID, nil)

		_, err := db.ExecContext(ctx, `
			INSERT INTO recommendations (batch_id, employee_id, job_position_id) VALUES ($1, $2, $3)
		`, batchID, employeeID, jobPositionID)
		if got := pgErrorCode(err); got != "23505" {
			t.Fatalf("debería violar la unicidad de la dupla, se obtuvo %s (%v)", got, err)
		}
	})

	t.Run("acepta la misma dupla en batches distintos", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		jobPositionID := newTestJobPosition(t, db, employerID)

		first := newTestBatch(t, db, NewEmployeeSubject(employeeID), withTestBatchStatus(BatchCompleted))
		second := newTestBatch(t, db, NewEmployeeSubject(employeeID))

		newTestRecommendation(t, db, first, employeeID, jobPositionID, nil)
		newTestRecommendation(t, db, second, employeeID, jobPositionID, nil)
	})

	t.Run("borrar el batch arrastra sus recomendaciones", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID))
		newTestRecommendation(t, db, batchID, employeeID, newTestJobPosition(t, db, employerID), nil)

		if _, err := db.ExecContext(ctx, `DELETE FROM recommendation_batches WHERE id = $1`, batchID); err != nil {
			t.Fatalf("borrar el batch: %v", err)
		}
		if got := countRecommendations(t, db, batchID); got != 0 {
			t.Fatalf("las recomendaciones deberían haberse borrado en cascada, quedaron %d", got)
		}
	})
}

func TestPuntajeOpcional(t *testing.T) {
	db := testsupport.PostgresDB(t)

	employerID := newTestEmployer(t, db)
	employeeID := newTestEmployee(t, db)
	batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID))

	cases := []struct {
		name  string
		score *float64
	}{
		{"sin puntaje", nil},
		{"puntaje cero", scorePtr(0)},
		{"puntaje fraccionario", scorePtr(0.8125)},
		{"puntaje porcentual máximo", scorePtr(100)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jobPositionID := newTestJobPosition(t, db, employerID)
			id := newTestRecommendation(t, db, batchID, employeeID, jobPositionID, tc.score)

			var stored sql.NullString
			if err := db.QueryRowContext(context.Background(), `SELECT score FROM recommendations WHERE id = $1`, id).Scan(&stored); err != nil {
				t.Fatalf("leer el puntaje: %v", err)
			}

			got, err := scoreFromDatabase(stored)
			if err != nil {
				t.Fatalf("convertir el puntaje: %v", err)
			}

			switch {
			case tc.score == nil && got != nil:
				t.Fatalf("la ausencia de puntaje debería leerse como ausencia, se obtuvo %v", *got)
			case tc.score != nil && got == nil:
				t.Fatalf("el puntaje %v debería leerse como presente", *tc.score)
			case tc.score != nil && *got != *tc.score:
				t.Fatalf("puntaje esperado %v, se obtuvo %v", *tc.score, *got)
			}
		})
	}
}

func countRecommendations(t *testing.T, db *sql.DB, batchID int32) int {
	t.Helper()

	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM recommendations WHERE batch_id = $1`, batchID).Scan(&count); err != nil {
		t.Fatalf("contar recomendaciones: %v", err)
	}

	return count
}
