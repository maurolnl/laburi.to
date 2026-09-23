package recommendation

import (
	"context"
	"errors"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

func TestClaimBatchProsperaDesdeEstadosNoTerminales(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// processing se reclama igual que pending: un procesamiento interrumpido tiene que poder
	// retomarse, porque si no el batch queda bloqueando al sujeto por el índice único parcial.
	for _, initial := range []BatchStatus{BatchPending, BatchProcessing} {
		t.Run(string(initial), func(t *testing.T) {
			employeeID := newTestEmployee(t, db)
			batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID), withTestBatchStatus(initial))

			batch, err := repo.ClaimBatch(ctx, batchID)
			if err != nil {
				t.Fatalf("ClaimBatch() error = %v, se esperaba nil", err)
			}
			if batch.Status != BatchProcessing {
				t.Fatalf("estado tras el reclamo = %q, se esperaba %q", batch.Status, BatchProcessing)
			}
			if batch.ID != batchID {
				t.Fatalf("el reclamo devolvió el batch %d, se esperaba %d", batch.ID, batchID)
			}
			if batch.EmployeeID == nil || *batch.EmployeeID != employeeID {
				t.Fatalf("el reclamo perdió el sujeto del batch: %+v", batch)
			}
		})
	}
}

func TestClaimBatchNoProsperaDesdeEstadosTerminales(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	for _, terminal := range []BatchStatus{BatchCompleted, BatchFailed} {
		t.Run(string(terminal), func(t *testing.T) {
			employeeID := newTestEmployee(t, db)
			batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID), withTestBatchStatus(terminal))

			_, err := repo.ClaimBatch(ctx, batchID)
			if !errors.Is(err, ErrBatchNotClaimable) {
				t.Fatalf("ClaimBatch() error = %v, se esperaba ErrBatchNotClaimable", err)
			}

			batch, err := repo.GetBatch(ctx, batchID)
			if err != nil {
				t.Fatalf("GetBatch() error = %v", err)
			}
			if batch.Status != terminal {
				t.Fatalf("un reclamo rechazado no debe cambiar el estado: quedó %q, era %q", batch.Status, terminal)
			}
		})
	}
}

// Es la garantía que impide que un redelivery destruya el conjunto vigente: si el reclamo de
// un batch completed prosperara, el reemplazo siguiente borraría recomendaciones correctas.
func TestClaimRechazadoConservaElConjuntoDelBatchCompletado(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employeeID := newTestEmployee(t, db)
	employerID := newTestEmployer(t, db)
	jobID := newTestJobPosition(t, db, employerID)

	batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID), withTestBatchStatus(BatchCompleted))
	newTestRecommendation(t, db, batchID, employeeID, jobID, scorePtr(0.5))

	if _, err := repo.ClaimBatch(ctx, batchID); !errors.Is(err, ErrBatchNotClaimable) {
		t.Fatalf("ClaimBatch() error = %v, se esperaba ErrBatchNotClaimable", err)
	}

	if count := countRecommendations(t, db, batchID); count != 1 {
		t.Fatalf("el conjunto del batch completado debía quedar intacto, quedaron %d recomendaciones", count)
	}

	result, err := repo.JobRecommendationsForEmployee(ctx, employeeID, fullPage())
	if err != nil {
		t.Fatalf("JobRecommendationsForEmployee() error = %v", err)
	}
	if result.Status != BatchCompleted || len(result.Items) != 1 {
		t.Fatalf("el conjunto vigente cambió tras un reclamo rechazado: %+v", result)
	}
}

func TestClaimBatchInexistente(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)

	_, err := repo.ClaimBatch(context.Background(), 999_999)
	if !errors.Is(err, ErrBatchNotFound) {
		t.Fatalf("ClaimBatch() error = %v, se esperaba ErrBatchNotFound", err)
	}
	// La distinción importa: no encontrado significa que el mensaje sobrevivió a su batch, y
	// no reclamable que otro procesamiento ya lo cerró.
	if errors.Is(err, ErrBatchNotClaimable) {
		t.Fatal("un batch inexistente no debe confundirse con uno ya terminal")
	}
}

func TestGetBatchDevuelveSujetoYEstado(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employerID := newTestEmployer(t, db)
	jobID := newTestJobPosition(t, db, employerID)
	batchID := newTestBatch(t, db, NewJobPositionSubject(jobID), withTestBatchStatus(BatchFailed))

	batch, err := repo.GetBatch(ctx, batchID)
	if err != nil {
		t.Fatalf("GetBatch() error = %v", err)
	}
	if batch.SubjectType != SubjectJobPosition || batch.JobPositionID == nil || *batch.JobPositionID != jobID {
		t.Fatalf("GetBatch() perdió el sujeto: %+v", batch)
	}
	if batch.EmployeeID != nil {
		t.Fatalf("un batch de puesto no debe traer empleado: %+v", batch)
	}
	if batch.Status != BatchFailed {
		t.Fatalf("estado = %q, se esperaba %q", batch.Status, BatchFailed)
	}
	if batch.CreatedAt.IsZero() || batch.UpdatedAt.IsZero() {
		t.Fatalf("GetBatch() debe traer las marcas temporales: %+v", batch)
	}
}

func TestGetBatchInexistente(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)

	batch, err := repo.GetBatch(context.Background(), 999_999)
	if !errors.Is(err, ErrBatchNotFound) {
		t.Fatalf("GetBatch() error = %v, se esperaba ErrBatchNotFound", err)
	}
	if batch.ID != 0 {
		t.Fatalf("GetBatch() no debe devolver ningún batch cuando no existe: %+v", batch)
	}
}
