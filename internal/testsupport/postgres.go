// Package testsupport levanta una base PostgreSQL efímera para los tests que necesitan
// verificar el comportamiento real del esquema: atomicidad de las transacciones,
// constraints, índices únicos parciales y orden de las consultas.
//
// La base nunca es una base preexistente del entorno: este paquete ignora deliberadamente
// DB_URL y cualquier otra configuración de conexión, y solo usa el contenedor que él mismo
// crea.
package testsupport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	postgresImage  = "postgres:16-alpine"
	startupTimeout = 120 * time.Second
	// healthTimeout acota el pre-chequeo del entorno de contenedores. Sin él, una máquina
	// sin Docker esperaría el timeout de arranque completo antes de omitir cada test.
	healthTimeout = 5 * time.Second
)

// shared guarda la única instancia por ejecución de tests. Levantar un contenedor cuesta
// segundos, así que todos los tests comparten la misma base y se aíslan entre sí por
// transacción o por truncado.
var (
	once   sync.Once
	shared *sql.DB
	// sharedErr distingue "no hay entorno de contenedores" (skip) de "el esquema no
	// aplica" (fallo), que son dos situaciones muy distintas para quien lee el reporte.
	sharedErr        error
	sharedUnavailabe bool
)

// PostgresDB devuelve una conexión a la base efímera con todas las migraciones aplicadas.
// Las tablas se vacían al terminar el test, de modo que sirve para verificar el efecto de
// commits reales.
//
// Si no hay entorno de contenedores disponible, omite el test con un motivo explícito en
// lugar de fallar: la suite completa debe seguir siendo ejecutable en una máquina sin
// Docker.
func PostgresDB(t *testing.T) *sql.DB {
	t.Helper()

	db := mustSharedDB(t)
	t.Cleanup(func() { truncateAll(t, db) })

	return db
}

// PostgresTx devuelve una transacción sobre la base efímera que se revierte al terminar el
// test. Es el aislamiento por defecto para los tests que solo verifican constraints del
// esquema y no necesitan observar un commit.
func PostgresTx(t *testing.T) *sql.Tx {
	t.Helper()

	db := mustSharedDB(t)

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("testsupport: no se pudo abrir la transacción: %v", err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Errorf("testsupport: no se pudo revertir la transacción: %v", err)
		}
	})

	return tx
}

func mustSharedDB(t *testing.T) *sql.DB {
	t.Helper()

	once.Do(startShared)

	if sharedUnavailabe {
		t.Skipf("testsupport: se omite el test de integración porque no hay entorno de contenedores disponible: %v", sharedErr)
	}
	if sharedErr != nil {
		t.Fatalf("testsupport: no se pudo preparar la base de integración: %v", sharedErr)
	}

	return shared
}

func startShared() {
	if err := checkContainerEnvironment(); err != nil {
		sharedUnavailabe = true
		sharedErr = err
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()

	container, err := postgres.Run(ctx, postgresImage,
		postgres.WithDatabase("laburito_test"),
		postgres.WithUsername("laburito"),
		postgres.WithPassword("laburito"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		// Un fallo al arrancar el contenedor se interpreta como ausencia de entorno:
		// es la causa abrumadoramente más común y la que no debe romper la suite.
		sharedUnavailabe = true
		sharedErr = err
		return
	}
	// El contenedor no se termina explícitamente: no hay un hook de teardown global
	// compartido entre paquetes, y el reaper de testcontainers lo elimina cuando el
	// proceso de test termina.
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		sharedErr = fmt.Errorf("obtener la cadena de conexión: %w", err)
		return
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		sharedErr = fmt.Errorf("abrir la conexión: %w", err)
		return
	}
	if err := db.PingContext(ctx); err != nil {
		sharedErr = fmt.Errorf("conectar con la base efímera: %w", err)
		return
	}

	if err := applyMigrations(ctx, db); err != nil {
		sharedErr = err
		return
	}

	shared = db
}

// checkContainerEnvironment responde rápido si no hay un daemon de contenedores al que
// conectarse, para que la suite en una máquina sin Docker omita los tests de integración
// en segundos en lugar de esperar el timeout de arranque.
func checkContainerEnvironment() error {
	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return fmt.Errorf("no hay proveedor de contenedores: %w", err)
	}
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()

	if err := provider.Health(ctx); err != nil {
		return fmt.Errorf("el daemon de contenedores no responde: %w", err)
	}

	return nil
}

// applyMigrations aplica en orden la sección `-- +goose Up` de cada archivo de
// sql/schema. No usa el binario de goose para no exigir una herramienta externa en la
// suite, pero respeta exactamente el mismo contenido que goose ejecutaría.
func applyMigrations(ctx context.Context, db *sql.DB) error {
	dir, err := schemaDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("leer %s: %w", dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	if len(names) == 0 {
		return fmt.Errorf("no se encontraron migraciones en %s", dir)
	}

	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("leer la migración %s: %w", name, err)
		}

		up := UpSection(string(content))
		if strings.TrimSpace(up) == "" {
			return fmt.Errorf("la migración %s no tiene sección -- +goose Up", name)
		}

		if _, err := db.ExecContext(ctx, up); err != nil {
			return fmt.Errorf("aplicar la migración %s: %w", name, err)
		}
	}

	return nil
}

// UpSection recorta el contenido de una migración goose entre `-- +goose Up` y
// `-- +goose Down`. Se exporta porque los tests de contrato del esquema verifican esa
// misma sección.
func UpSection(migration string) string {
	const (
		upMarker   = "-- +goose Up"
		downMarker = "-- +goose Down"
	)

	start := strings.Index(migration, upMarker)
	if start < 0 {
		return ""
	}
	up := migration[start+len(upMarker):]

	if end := strings.Index(up, downMarker); end >= 0 {
		up = up[:end]
	}

	return up
}

// DownSection recorta el contenido de una migración goose a partir de `-- +goose Down`.
// Los tests de rollback la aplican dentro de una transacción para verificar que deshace
// exactamente lo que la sección Up creó.
func DownSection(migration string) string {
	const downMarker = "-- +goose Down"

	start := strings.Index(migration, downMarker)
	if start < 0 {
		return ""
	}

	return migration[start+len(downMarker):]
}

// ReadMigration devuelve el contenido de un archivo de sql/schema por su nombre.
func ReadMigration(name string) (string, error) {
	dir, err := schemaDir()
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return "", fmt.Errorf("leer la migración %s: %w", name, err)
	}

	return string(content), nil
}

// schemaDir resuelve sql/schema desde el directorio del paquete bajo test, que es el
// directorio de trabajo que usa `go test`. Sube hasta encontrar el go.mod del módulo.
func schemaDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("obtener el directorio de trabajo: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "sql", "schema"), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no se encontró la raíz del módulo desde el directorio de trabajo")
		}
		dir = parent
	}
}

// truncateAll vacía todas las tablas de la base salvo las de control de migraciones, para
// que un test nunca observe las filas escritas por otro. RESTART IDENTITY deja además los
// SERIAL en un estado previsible.
func truncateAll(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()

	rows, err := db.QueryContext(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		  AND tablename <> 'goose_db_version'
	`)
	if err != nil {
		t.Fatalf("testsupport: no se pudieron listar las tablas: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("testsupport: no se pudo leer el nombre de la tabla: %v", err)
		}
		tables = append(tables, fmt.Sprintf("%q", table))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("testsupport: no se pudieron listar las tablas: %v", err)
	}
	if len(tables) == 0 {
		return
	}

	statement := fmt.Sprintf("TRUNCATE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	if _, err := db.ExecContext(ctx, statement); err != nil {
		t.Fatalf("testsupport: no se pudieron vaciar las tablas: %v", err)
	}
}
