package recommendation

import (
	"context"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

// El usuario dueño de cada empleador se lee de la base y no se devuelve desde la factoría, por
// el mismo motivo que en los tests de propiedad: así el test contrasta la consulta contra el
// esquema real y no contra lo que la factoría creyó haber insertado.
func TestEmployerHasCurrentRecommendation(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	userOf := func(t *testing.T, employerID int32) int32 {
		t.Helper()

		var userID int32
		if err := db.QueryRowContext(ctx, `SELECT user_id FROM employers WHERE id = $1`, employerID).Scan(&userID); err != nil {
			t.Fatalf("leer el usuario del empleador: %v", err)
		}

		return userID
	}

	assertAccess := func(t *testing.T, employeeID, employerUserID int32, want bool) {
		t.Helper()

		got, err := repo.EmployerHasCurrentRecommendation(ctx, employeeID, employerUserID)
		if err != nil {
			t.Fatalf("resolver el vínculo: %v", err)
		}
		if got != want {
			t.Fatalf("se esperaba vínculo=%v, se obtuvo %v", want, got)
		}
	}

	// El conjunto vigente del empleado contiene un puesto activo del empleador: el empleador ve
	// al empleado aunque nunca haya corrido un batch por su propio puesto.
	t.Run("vínculo por el conjunto vigente del empleado", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		employerID := newTestEmployer(t, db)
		jobPositionID := newTestJobPosition(t, db, employerID)

		batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID), withTestBatchStatus(BatchCompleted))
		newTestRecommendation(t, db, batchID, employeeID, jobPositionID, scorePtr(0.9))

		assertAccess(t, employeeID, userOf(t, employerID), true)
	})

	// La dirección inversa: el batch corrió por el puesto y el empleado figura entre sus
	// candidatos. Es el caso que espeja exactamente lo que el empleador ya puede listar.
	t.Run("vínculo por el conjunto vigente del puesto", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		employerID := newTestEmployer(t, db)
		jobPositionID := newTestJobPosition(t, db, employerID)

		batchID := newTestBatch(t, db, NewJobPositionSubject(jobPositionID), withTestBatchStatus(BatchCompleted))
		newTestRecommendation(t, db, batchID, employeeID, jobPositionID, scorePtr(0.9))

		assertAccess(t, employeeID, userOf(t, employerID), true)
	})

	t.Run("puesto eliminado lógicamente", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		employerID := newTestEmployer(t, db)
		jobPositionID := newTestJobPosition(t, db, employerID, withTestJobPositionDeleted())

		batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID), withTestBatchStatus(BatchCompleted))
		newTestRecommendation(t, db, batchID, employeeID, jobPositionID, scorePtr(0.9))

		assertAccess(t, employeeID, userOf(t, employerID), false)
	})

	// Dos batches completados del mismo empleado: el vínculo solo puede salir del más reciente.
	// Los batches se insertan directamente y no vía CompleteBatch, que poda los anteriores y
	// haría imposible construir este estado.
	t.Run("conjunto vigente reemplazado", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		employerID := newTestEmployer(t, db)
		staleJob := newTestJobPosition(t, db, employerID)
		otherEmployerID := newTestEmployer(t, db)
		currentJob := newTestJobPosition(t, db, otherEmployerID)

		staleBatch := newTestBatch(t, db, NewEmployeeSubject(employeeID), withTestBatchStatus(BatchCompleted))
		newTestRecommendation(t, db, staleBatch, employeeID, staleJob, scorePtr(0.9))

		currentBatch := newTestBatch(t, db, NewEmployeeSubject(employeeID), withTestBatchStatus(BatchCompleted))
		newTestRecommendation(t, db, currentBatch, employeeID, currentJob, scorePtr(0.9))

		assertAccess(t, employeeID, userOf(t, employerID), false)
		assertAccess(t, employeeID, userOf(t, otherEmployerID), true)
	})

	// Un batch abierto todavía no produjo un conjunto vigente, así que no habilita a nadie.
	t.Run("batch sin completar", func(t *testing.T) {
		for _, status := range []BatchStatus{BatchPending, BatchProcessing, BatchFailed} {
			t.Run(string(status), func(t *testing.T) {
				employeeID := newTestEmployee(t, db)
				employerID := newTestEmployer(t, db)
				jobPositionID := newTestJobPosition(t, db, employerID)

				batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID), withTestBatchStatus(status))
				newTestRecommendation(t, db, batchID, employeeID, jobPositionID, scorePtr(0.9))

				assertAccess(t, employeeID, userOf(t, employerID), false)
			})
		}
	})

	t.Run("empleador ajeno", func(t *testing.T) {
		employeeID := newTestEmployee(t, db)
		employerID := newTestEmployer(t, db)
		jobPositionID := newTestJobPosition(t, db, employerID)
		strangerID := newTestEmployer(t, db)

		batchID := newTestBatch(t, db, NewEmployeeSubject(employeeID), withTestBatchStatus(BatchCompleted))
		newTestRecommendation(t, db, batchID, employeeID, jobPositionID, scorePtr(0.9))

		assertAccess(t, employeeID, userOf(t, strangerID), false)
	})

	// Un empleado inexistente no es un error propio: para quien autoriza es indistinguible de
	// un empleado sin recomendaciones, y esa indistinción es justamente la que impide enumerar
	// identificadores.
	t.Run("empleado inexistente", func(t *testing.T) {
		employerID := newTestEmployer(t, db)

		assertAccess(t, 0, userOf(t, employerID), false)
	})
}
