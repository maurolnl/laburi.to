package testsupport

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestUpSectionRecortaLaSeccionUp(t *testing.T) {
	migration := `-- +goose Up
CREATE TABLE ejemplo (id SERIAL PRIMARY KEY);

-- +goose Down
DROP TABLE ejemplo;
`

	up := UpSection(migration)

	if !strings.Contains(up, "CREATE TABLE ejemplo") {
		t.Fatalf("la sección Up debería contener el CREATE TABLE, se obtuvo %q", up)
	}
	if strings.Contains(up, "DROP TABLE") {
		t.Fatalf("la sección Up no debería contener el Down, se obtuvo %q", up)
	}
}

func TestUpSectionSinMarcador(t *testing.T) {
	if up := UpSection("CREATE TABLE ejemplo (id SERIAL PRIMARY KEY);"); up != "" {
		t.Fatalf("una migración sin marcador Up debería producir una sección vacía, se obtuvo %q", up)
	}
}

func TestPostgresDBAplicaTodasLasMigraciones(t *testing.T) {
	db := PostgresDB(t)

	// Las tablas de la última migración son la prueba de que se aplicaron todas en orden:
	// recommendations depende de recommendation_batches, de employees y de job_positions.
	for _, table := range []string{"users", "employees", "employers", "job_positions", "recommendation_batches", "recommendations"} {
		var exists bool
		err := db.QueryRowContext(context.Background(), `
			SELECT EXISTS(
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("consultar la existencia de %s: %v", table, err)
		}
		if !exists {
			t.Errorf("la tabla %s debería existir en la base efímera", table)
		}
	}
}

func TestPostgresDBReutilizaLaInstancia(t *testing.T) {
	first := PostgresDB(t)
	second := PostgresDB(t)

	if first != second {
		t.Fatal("dos llamados dentro de la misma ejecución deberían compartir la misma instancia")
	}
}

// Los dos tests siguientes escriben en la misma tabla: si el aislamiento no funcionara,
// el segundo en ejecutarse vería la fila del primero.

func TestAislamientoPrimerEscritor(t *testing.T) {
	db := PostgresDB(t)
	assertUsuariosVacio(t, db)
	insertarUsuario(t, db, "primero@laburi.to")
}

func TestAislamientoSegundoEscritor(t *testing.T) {
	db := PostgresDB(t)
	assertUsuariosVacio(t, db)
	insertarUsuario(t, db, "segundo@laburi.to")
}

func TestPostgresTxSeRevierte(t *testing.T) {
	db := PostgresDB(t)

	func() {
		tx := PostgresTx(t)
		if _, err := tx.ExecContext(context.Background(),
			`INSERT INTO users (email, hashed_password, role, created_at, updated_at)
			 VALUES ($1, $2, $3, now(), now())`,
			"transitorio@laburi.to", "hash", "employee"); err != nil {
			t.Fatalf("insertar dentro de la transacción: %v", err)
		}
	}()

	// La transacción se revierte recién en el Cleanup del test, así que aquí todavía no
	// se puede observar el rollback. Lo que sí se verifica es que la escritura quedó
	// confinada a la transacción y no en la conexión compartida.
	var count int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM users WHERE email = $1`, "transitorio@laburi.to").Scan(&count); err != nil {
		t.Fatalf("contar usuarios: %v", err)
	}
	if count != 0 {
		t.Fatalf("la escritura no confirmada no debería ser visible fuera de la transacción, se vieron %d filas", count)
	}
}

func TestPostgresDBIgnoraDBURLDelEntorno(t *testing.T) {
	t.Setenv("DB_URL", "postgres://no-existe:5432/no-existe?sslmode=disable")

	db := PostgresDB(t)

	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("la base efímera debería seguir funcionando con un DB_URL inválido en el entorno: %v", err)
	}
}

func assertUsuariosVacio(t *testing.T, db *sql.DB) {
	t.Helper()

	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM users`).Scan(&count); err != nil {
		t.Fatalf("contar usuarios: %v", err)
	}
	if count != 0 {
		t.Fatalf("el test no debería ver filas escritas por otro test, se vieron %d", count)
	}
}

func insertarUsuario(t *testing.T, db *sql.DB, email string) {
	t.Helper()

	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO users (email, hashed_password, role, created_at, updated_at)
		 VALUES ($1, $2, $3, now(), now())`,
		email, "hash", "employee"); err != nil {
		t.Fatalf("insertar usuario: %v", err)
	}
}
