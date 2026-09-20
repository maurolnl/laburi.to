package recommendation

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

func fullPage() Page { return Page{Limit: 100, Offset: 0} }

func countBatches(t *testing.T, db *sql.DB, column string, subjectID int32) int {
	t.Helper()

	var count int
	query := `SELECT count(*) FROM recommendation_batches WHERE ` + column + ` = $1`
	if err := db.QueryRowContext(context.Background(), query, subjectID).Scan(&count); err != nil {
		t.Fatalf("contar batches: %v", err)
	}

	return count
}

func TestCompleteBatchReemplazaElConjunto(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	oldJob := newTestJobPosition(t, db, employerID, withTestJobPositionPosition("Puesto viejo"))
	newJob := newTestJobPosition(t, db, employerID, withTestJobPositionPosition("Puesto nuevo"))

	first, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
	if err != nil {
		t.Fatalf("crear el primer batch: %v", err)
	}
	if _, err := repo.CompleteBatch(ctx, first.ID, []Candidate{
		{EmployeeID: employeeID, JobPositionID: oldJob, Score: scorePtr(0.5)},
	}); err != nil {
		t.Fatalf("completar el primer batch: %v", err)
	}

	second, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
	if err != nil {
		t.Fatalf("crear el segundo batch: %v", err)
	}
	completed, err := repo.CompleteBatch(ctx, second.ID, []Candidate{
		{EmployeeID: employeeID, JobPositionID: newJob, Score: scorePtr(0.9)},
	})
	if err != nil {
		t.Fatalf("completar el segundo batch: %v", err)
	}
	if completed.Status != BatchCompleted {
		t.Fatalf("el batch debería quedar completed, se obtuvo %s", completed.Status)
	}

	result, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
	if err != nil {
		t.Fatalf("leer las recomendaciones: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].JobPositionID != newJob {
		t.Fatalf("el conjunto vigente debería ser el nuevo, se obtuvo %+v", result.Items)
	}

	// El batch anterior y sus recomendaciones dejan de existir: no hay historial.
	if got := countBatches(t, db, "employee_id", employeeID); got != 1 {
		t.Fatalf("debería quedar un único batch, quedaron %d", got)
	}
	if got := countRecommendations(t, db, first.ID); got != 0 {
		t.Fatalf("las recomendaciones del batch anterior deberían haberse borrado, quedaron %d", got)
	}
}

func TestCompleteBatchConFalloConservaElConjuntoAnterior(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	goodJob := newTestJobPosition(t, db, employerID)
	otherJob := newTestJobPosition(t, db, employerID)

	first, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
	if err != nil {
		t.Fatalf("crear el primer batch: %v", err)
	}
	if _, err := repo.CompleteBatch(ctx, first.ID, []Candidate{
		{EmployeeID: employeeID, JobPositionID: goodJob, Score: scorePtr(0.5)},
	}); err != nil {
		t.Fatalf("completar el primer batch: %v", err)
	}

	second, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
	if err != nil {
		t.Fatalf("crear el segundo batch: %v", err)
	}

	// La segunda recomendación apunta a un empleado inexistente: la clave foránea aborta
	// la transacción después de que la primera ya fue insertada.
	_, err = repo.CompleteBatch(ctx, second.ID, []Candidate{
		{EmployeeID: employeeID, JobPositionID: otherJob, Score: scorePtr(0.9)},
		{EmployeeID: 999999, JobPositionID: otherJob, Score: scorePtr(0.8)},
	})
	if !errors.Is(err, ErrSubjectNotFound) {
		t.Fatalf("se esperaba ErrSubjectNotFound, se obtuvo %v", err)
	}

	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM recommendation_batches WHERE id = $1`, second.ID).Scan(&status); err != nil {
		t.Fatalf("leer el estado del batch: %v", err)
	}
	if status != string(BatchPending) {
		t.Fatalf("el batch fallido debería conservar su estado previo pending, se obtuvo %s", status)
	}
	if got := countRecommendations(t, db, second.ID); got != 0 {
		t.Fatalf("no debería quedar ninguna recomendación parcial, quedaron %d", got)
	}

	result, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
	if err != nil {
		t.Fatalf("leer las recomendaciones: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].JobPositionID != goodJob {
		t.Fatalf("el conjunto vigente anterior debería permanecer intacto, se obtuvo %+v", result.Items)
	}
}

func TestRetencionAcotada(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)

	for i := 0; i < 3; i++ {
		batch, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
		if err != nil {
			t.Fatalf("crear el batch %d: %v", i, err)
		}
		if _, err := repo.CompleteBatch(ctx, batch.ID, []Candidate{
			{EmployeeID: employeeID, JobPositionID: newTestJobPosition(t, db, employerID)},
		}); err != nil {
			t.Fatalf("completar el batch %d: %v", i, err)
		}
	}

	if got := countBatches(t, db, "employee_id", employeeID); got != 1 {
		t.Fatalf("en estado estable debería quedar un único batch, quedaron %d", got)
	}

	// Con trabajo en curso sobre un conjunto ya completado, el sujeto tiene dos batches.
	if _, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID)); err != nil {
		t.Fatalf("crear el batch en curso: %v", err)
	}
	if got := countBatches(t, db, "employee_id", employeeID); got != 2 {
		t.Fatalf("deberían convivir el último completado y el que está en curso, hay %d", got)
	}
}

func TestEstadoVigenteYConjuntoVigente(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employerID := newTestEmployer(t, db)

	t.Run("sujeto sin ningún batch", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		if _, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage()); !errors.Is(err, ErrNoCurrentBatch) {
			t.Fatalf("se esperaba ErrNoCurrentBatch, se obtuvo %v", err)
		}
	})

	t.Run("sujeto sin ningún batch completado", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		if _, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID)); err != nil {
			t.Fatalf("crear batch: %v", err)
		}

		result, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
		if err != nil {
			t.Fatalf("leer: %v", err)
		}
		if result.Status != BatchPending {
			t.Fatalf("estado esperado pending, se obtuvo %s", result.Status)
		}
		if len(result.Items) != 0 {
			t.Fatalf("el conjunto vigente debería estar vacío, se obtuvieron %d items", len(result.Items))
		}
	})

	t.Run("último batch completado", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		batch, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
		if err != nil {
			t.Fatalf("crear batch: %v", err)
		}
		if _, err := repo.CompleteBatch(ctx, batch.ID, []Candidate{
			{EmployeeID: employeeID, JobPositionID: newTestJobPosition(t, db, employerID)},
		}); err != nil {
			t.Fatalf("completar: %v", err)
		}

		result, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
		if err != nil {
			t.Fatalf("leer: %v", err)
		}
		if result.Status != BatchCompleted || len(result.Items) != 1 {
			t.Fatalf("se esperaba completed con 1 item, se obtuvo %s con %d", result.Status, len(result.Items))
		}
	})

	// Los dos escenarios siguientes son el motivo de separar estado vigente de conjunto
	// vigente: el sujeto muestra que algo pasó sin perder lo que ya tenía.
	for _, tc := range []struct {
		name   string
		status BatchStatus
	}{
		{"procesamiento en curso sobre un conjunto previo", BatchProcessing},
		{"fallo posterior a un conjunto previo", BatchFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			employeeID := newTestEmployee(t, db)
			jobPositionID := newTestJobPosition(t, db, employerID)

			completedBatch, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
			if err != nil {
				t.Fatalf("crear el primer batch: %v", err)
			}
			if _, err := repo.CompleteBatch(ctx, completedBatch.ID, []Candidate{
				{EmployeeID: employeeID, JobPositionID: jobPositionID, Score: scorePtr(0.7)},
			}); err != nil {
				t.Fatalf("completar: %v", err)
			}

			next, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
			if err != nil {
				t.Fatalf("crear el segundo batch: %v", err)
			}
			if _, err := repo.TransitionBatch(ctx, next.ID, tc.status); err != nil {
				t.Fatalf("transicionar: %v", err)
			}

			result, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
			if err != nil {
				t.Fatalf("leer: %v", err)
			}
			if result.Status != tc.status {
				t.Fatalf("el estado vigente debería ser %s, se obtuvo %s", tc.status, result.Status)
			}
			if len(result.Items) != 1 || result.Items[0].JobPositionID != jobPositionID {
				t.Fatalf("el conjunto vigente anterior debería seguir disponible, se obtuvo %+v", result.Items)
			}
		})
	}
}

func TestResultadoVacioSeDistingueDelFallo(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("batch completado sin candidatos", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		batch, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
		if err != nil {
			t.Fatalf("crear batch: %v", err)
		}
		if _, err := repo.CompleteBatch(ctx, batch.ID, nil); err != nil {
			t.Fatalf("completar con conjunto vacío: %v", err)
		}

		result, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
		if err != nil {
			t.Fatalf("leer: %v", err)
		}
		if result.Status != BatchCompleted {
			t.Fatalf("estado esperado completed, se obtuvo %s", result.Status)
		}
		if len(result.Items) != 0 {
			t.Fatalf("el conjunto debería estar vacío, se obtuvieron %d items", len(result.Items))
		}
	})

	t.Run("batch fallido", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		batch, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
		if err != nil {
			t.Fatalf("crear batch: %v", err)
		}
		if _, err := repo.TransitionBatch(ctx, batch.ID, BatchFailed); err != nil {
			t.Fatalf("transicionar: %v", err)
		}

		result, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
		if err != nil {
			t.Fatalf("leer: %v", err)
		}
		// Ambos casos tienen cero items: lo que los distingue es el estado.
		if result.Status != BatchFailed {
			t.Fatalf("estado esperado failed, se obtuvo %s", result.Status)
		}
		if len(result.Items) != 0 {
			t.Fatalf("un batch fallido no debería aportar items, se obtuvieron %d", len(result.Items))
		}
	})
}

func TestExclusionDePuestosEliminados(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employerID := newTestEmployer(t, db)

	t.Run("puesto eliminado después de recomendado", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		activeJob := newTestJobPosition(t, db, employerID, withTestJobPositionPosition("Activo"))
		doomedJob := newTestJobPosition(t, db, employerID, withTestJobPositionPosition("A eliminar"))

		batch, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
		if err != nil {
			t.Fatalf("crear batch: %v", err)
		}
		if _, err := repo.CompleteBatch(ctx, batch.ID, []Candidate{
			{EmployeeID: employeeID, JobPositionID: activeJob, Score: scorePtr(0.9)},
			{EmployeeID: employeeID, JobPositionID: doomedJob, Score: scorePtr(0.8)},
		}); err != nil {
			t.Fatalf("completar: %v", err)
		}

		if _, err := db.ExecContext(ctx, `UPDATE job_positions SET deleted_at = now() WHERE id = $1`, doomedJob); err != nil {
			t.Fatalf("eliminar el puesto: %v", err)
		}

		result, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
		if err != nil {
			t.Fatalf("leer: %v", err)
		}
		if len(result.Items) != 1 || result.Items[0].JobPositionID != activeJob {
			t.Fatalf("solo debería quedar el puesto activo, se obtuvo %+v", result.Items)
		}
	})

	t.Run("batch por un puesto eliminado", func(t *testing.T) {
		jobPositionID := newTestJobPosition(t, db, employerID)
		employeeID := newTestEmployee(t, db)

		batch, err := repo.CreateBatch(ctx, NewJobPositionSubject(jobPositionID))
		if err != nil {
			t.Fatalf("crear batch: %v", err)
		}
		if _, err := repo.CompleteBatch(ctx, batch.ID, []Candidate{
			{EmployeeID: employeeID, JobPositionID: jobPositionID, Score: scorePtr(0.9)},
		}); err != nil {
			t.Fatalf("completar: %v", err)
		}

		if _, err := db.ExecContext(ctx, `UPDATE job_positions SET deleted_at = now() WHERE id = $1`, jobPositionID); err != nil {
			t.Fatalf("eliminar el puesto: %v", err)
		}

		result, err := repo.EmployeeRecommendationsForJobPosition(ctx, jobPositionID, fullPage())
		if err != nil {
			t.Fatalf("leer: %v", err)
		}
		if len(result.Items) != 0 {
			t.Fatalf("un puesto eliminado no debería devolver candidatos, se obtuvieron %d", len(result.Items))
		}
	})
}

func TestOrdenYPaginacionDePuestos(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	base := time.Now().UTC().Add(-72 * time.Hour)

	// Dos puestos empatados en puntaje: desempata la publicación más reciente.
	older := newTestJobPosition(t, db, employerID, withTestJobPositionPosition("Empate viejo"), withTestJobPositionCreatedAt(base))
	newer := newTestJobPosition(t, db, employerID, withTestJobPositionPosition("Empate nuevo"), withTestJobPositionCreatedAt(base.Add(24*time.Hour)))
	top := newTestJobPosition(t, db, employerID, withTestJobPositionPosition("Mejor puntaje"), withTestJobPositionCreatedAt(base))
	unscored := newTestJobPosition(t, db, employerID, withTestJobPositionPosition("Sin puntaje"), withTestJobPositionCreatedAt(base.Add(48*time.Hour)))

	batch, err := repo.CreateBatch(ctx, NewEmployeeSubject(employeeID))
	if err != nil {
		t.Fatalf("crear batch: %v", err)
	}
	if _, err := repo.CompleteBatch(ctx, batch.ID, []Candidate{
		{EmployeeID: employeeID, JobPositionID: older, Score: scorePtr(0.5)},
		{EmployeeID: employeeID, JobPositionID: unscored, Score: nil},
		{EmployeeID: employeeID, JobPositionID: top, Score: scorePtr(0.9)},
		{EmployeeID: employeeID, JobPositionID: newer, Score: scorePtr(0.5)},
	}); err != nil {
		t.Fatalf("completar: %v", err)
	}

	result, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
	if err != nil {
		t.Fatalf("leer: %v", err)
	}

	want := []int32{top, newer, older, unscored}
	if len(result.Items) != len(want) {
		t.Fatalf("se esperaban %d items, se obtuvieron %d", len(want), len(result.Items))
	}
	for i, expected := range want {
		if result.Items[i].JobPositionID != expected {
			t.Fatalf("en la posición %d se esperaba el puesto %d, se obtuvo %d", i, expected, result.Items[i].JobPositionID)
		}
	}

	page, err := repo.JobRecommendationsForEmployee(ctx, employeeID, Page{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("leer la página: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("la página debería traer 2 items, trajo %d", len(page.Items))
	}
	if page.Items[0].JobPositionID != newer || page.Items[1].JobPositionID != older {
		t.Fatalf("la página no respeta el orden global: %+v", page.Items)
	}
}

func TestOrdenYPaginacionDeEmpleados(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employerID := newTestEmployer(t, db)
	jobPositionID := newTestJobPosition(t, db, employerID)
	base := time.Now().UTC().Add(-72 * time.Hour)

	// Dos empleados empatados en puntaje: desempata el perfil actualizado más recientemente.
	staleTie := newTestEmployee(t, db, withTestEmployeePosition("Empate viejo"), withTestEmployeeUpdatedAt(base))
	freshTie := newTestEmployee(t, db, withTestEmployeePosition("Empate nuevo"), withTestEmployeeUpdatedAt(base.Add(24*time.Hour)))
	top := newTestEmployee(t, db, withTestEmployeePosition("Mejor puntaje"), withTestEmployeeUpdatedAt(base))
	unscored := newTestEmployee(t, db, withTestEmployeePosition("Sin puntaje"), withTestEmployeeUpdatedAt(base.Add(48*time.Hour)))

	batch, err := repo.CreateBatch(ctx, NewJobPositionSubject(jobPositionID))
	if err != nil {
		t.Fatalf("crear batch: %v", err)
	}
	if _, err := repo.CompleteBatch(ctx, batch.ID, []Candidate{
		{EmployeeID: staleTie, JobPositionID: jobPositionID, Score: scorePtr(0.5)},
		{EmployeeID: unscored, JobPositionID: jobPositionID, Score: nil},
		{EmployeeID: top, JobPositionID: jobPositionID, Score: scorePtr(0.9)},
		{EmployeeID: freshTie, JobPositionID: jobPositionID, Score: scorePtr(0.5)},
	}); err != nil {
		t.Fatalf("completar: %v", err)
	}

	result, err := repo.EmployeeRecommendationsForJobPosition(ctx, jobPositionID, fullPage())
	if err != nil {
		t.Fatalf("leer: %v", err)
	}

	want := []int32{top, freshTie, staleTie, unscored}
	if len(result.Items) != len(want) {
		t.Fatalf("se esperaban %d items, se obtuvieron %d", len(want), len(result.Items))
	}
	for i, expected := range want {
		if result.Items[i].EmployeeID != expected {
			t.Fatalf("en la posición %d se esperaba el empleado %d, se obtuvo %d", i, expected, result.Items[i].EmployeeID)
		}
	}

	page, err := repo.EmployeeRecommendationsForJobPosition(ctx, jobPositionID, Page{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("leer la página: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("la página debería traer 2 items, trajo %d", len(page.Items))
	}
	if page.Items[0].EmployeeID != staleTie || page.Items[1].EmployeeID != unscored {
		t.Fatalf("la página no respeta el orden global: %+v", page.Items)
	}
}
