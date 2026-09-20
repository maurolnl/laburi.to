package recommendation

import (
	"context"
	"database/sql"
	"sort"
	"testing"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

const migration0007 = "0007_recommendations.sql"

// TestMigracion0007EsReversible verifica que `down` deshace exactamente lo que `up` creó.
// Corre dentro de una transacción que se revierte al terminar: el DDL de PostgreSQL es
// transaccional, así que el resto de los tests del paquete no se ve afectado.
func TestMigracion0007EsReversible(t *testing.T) {
	tx := testsupport.PostgresTx(t)
	ctx := context.Background()

	content, err := testsupport.ReadMigration(migration0007)
	if err != nil {
		t.Fatalf("leer la migración: %v", err)
	}

	before := relations(t, tx)

	if _, err := tx.ExecContext(ctx, testsupport.DownSection(content)); err != nil {
		t.Fatalf("aplicar la sección Down: %v", err)
	}

	after := relations(t, tx)
	for _, relation := range []string{"recommendation_batches", "recommendations"} {
		if contains(after, relation) {
			t.Errorf("la sección Down debería eliminar %s", relation)
		}
	}
	// El rollback no debe tocar objetos de migraciones anteriores.
	for _, relation := range []string{"users", "employees", "employers", "job_positions"} {
		if !contains(after, relation) {
			t.Errorf("la sección Down no debería eliminar %s", relation)
		}
	}

	if _, err := tx.ExecContext(ctx, testsupport.UpSection(content)); err != nil {
		t.Fatalf("reaplicar la sección Up: %v", err)
	}

	restored := relations(t, tx)
	if len(restored) != len(before) {
		t.Fatalf("up seguido de down debería dejar el esquema igual: antes %v, después %v", before, restored)
	}
	for i := range before {
		if before[i] != restored[i] {
			t.Fatalf("up seguido de down debería dejar el esquema igual: antes %v, después %v", before, restored)
		}
	}
}

// relations lista tablas e índices del esquema público, que es lo que la migración crea y
// destruye.
func relations(t *testing.T, tx *sql.Tx) []string {
	t.Helper()

	rows, err := tx.QueryContext(context.Background(), `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public'
		  AND c.relkind IN ('r', 'i')
	`)
	if err != nil {
		t.Fatalf("listar relaciones: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("leer el nombre de la relación: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("listar relaciones: %v", err)
	}
	sort.Strings(names)

	return names
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
