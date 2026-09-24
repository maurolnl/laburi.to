package employee

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

// Estos tests ejercen las tres lecturas nuevas contra el esquema real. Los tests de servicio
// usan un store falso y afirman qué se hace con la respuesta; acá se afirma la respuesta misma,
// que es donde viven el filtro por empleado y la ausencia de coordenadas de S3.

func insertProfileEmployee(t *testing.T, db *sql.DB) (employeeID, userID int32) {
	t.Helper()

	email := fmt.Sprintf("profile-%d@laburi.to", time.Now().UnixNano())
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO users (email, hashed_password, role, created_at, updated_at)
		VALUES ($1, 'hash', 'employee', now(), now())
		RETURNING id
	`, email).Scan(&userID); err != nil {
		t.Fatalf("crear usuario: %v", err)
	}

	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO employees (user_id, position, role, years_of_experience, created_at, updated_at)
		VALUES ($1, 'Backend Engineer', 'Go developer', '2_to_5y', now(), now())
		RETURNING id
	`, userID).Scan(&employeeID); err != nil {
		t.Fatalf("crear empleado: %v", err)
	}

	return employeeID, userID
}

func insertProfileFile(t *testing.T, db *sql.DB, employeeID int32, filename, status string) int32 {
	t.Helper()

	var fileID int32
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO employee_files (
			employee_id, type, bucket, object_key, original_filename, content_type,
			size_bytes, status, created_at, updated_at
		)
		VALUES ($1, 'certification', 'laburito-bucket', $2, $3, 'application/pdf', 1024, $4, now(), now())
		RETURNING id
	`, employeeID, fmt.Sprintf("certifications/%d-%s", time.Now().UnixNano(), filename), filename, status).Scan(&fileID); err != nil {
		t.Fatalf("crear archivo: %v", err)
	}

	return fileID
}

func insertProfileEducation(t *testing.T, db *sql.DB, employeeID int32, title string, document *string) int32 {
	t.Helper()

	var educationID int32
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO employee_education (employee_id, education_type, title, status, certification, created_at, updated_at)
		VALUES ($1, 'university', $2, 'completed', $3, now(), now())
		RETURNING id
	`, employeeID, title, document).Scan(&educationID); err != nil {
		t.Fatalf("crear educación: %v", err)
	}

	return educationID
}

func TestGetEmployeeProfileByID(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("identifica archivos y documentos sin revelar su ubicación", func(t *testing.T) {
		employeeID, userID := insertProfileEmployee(t, db)
		fileID := insertProfileFile(t, db, employeeID, "certificado.pdf", "uploaded")
		document := "certifications/titulo.pdf"
		withDocument := insertProfileEducation(t, db, employeeID, "Ingeniería", &document)
		insertProfileEducation(t, db, employeeID, "Tecnicatura", nil)

		profile, err := repo.GetEmployeeProfileByID(ctx, employeeID)
		if err != nil {
			t.Fatalf("leer el perfil: %v", err)
		}

		if profile.ID != employeeID || profile.UserID != userID {
			t.Fatalf("el perfil no corresponde al empleado: %+v", profile)
		}
		if len(profile.Files) != 1 || profile.Files[0].ID != fileID || profile.Files[0].Title != "certificado.pdf" {
			t.Fatalf("archivos inesperados: %+v", profile.Files)
		}
		if len(profile.Education) != 2 {
			t.Fatalf("educación inesperada: %+v", profile.Education)
		}

		byTitle := map[string]ProfileEducationItem{}
		for _, item := range profile.Education {
			byTitle[item.Title] = item
		}
		if got := byTitle["Ingeniería"].CertificationDocumentID; got == nil || *got != withDocument {
			t.Fatalf("el título con documento no trae su identificador: %+v", byTitle["Ingeniería"])
		}
		// Un título sin documento no puede traer identificador: el cliente lo leería como un
		// documento descargable que no existe.
		if got := byTitle["Tecnicatura"].CertificationDocumentID; got != nil {
			t.Fatalf("el título sin documento trae identificador %d", *got)
		}
	})

	// El certificado a medio subir no se lista porque tampoco se entrega: lista y entrega tienen
	// que describir el mismo conjunto.
	t.Run("omite los certificados sin subir", func(t *testing.T) {
		employeeID, _ := insertProfileEmployee(t, db)
		insertProfileFile(t, db, employeeID, "pendiente.pdf", "pending")
		insertProfileFile(t, db, employeeID, "fallido.pdf", "failed")
		uploaded := insertProfileFile(t, db, employeeID, "listo.pdf", "uploaded")

		profile, err := repo.GetEmployeeProfileByID(ctx, employeeID)
		if err != nil {
			t.Fatalf("leer el perfil: %v", err)
		}

		if len(profile.Files) != 1 || profile.Files[0].ID != uploaded {
			t.Fatalf("archivos inesperados: %+v", profile.Files)
		}
	})

	t.Run("empleado inexistente", func(t *testing.T) {
		if _, err := repo.GetEmployeeProfileByID(ctx, 999_999); !errors.Is(err, ErrEmployeeProfileNotFound) {
			t.Fatalf("se esperaba ErrEmployeeProfileNotFound, se obtuvo %v", err)
		}
	})
}

func TestGetEmployeeFile(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("archivo propio", func(t *testing.T) {
		employeeID, _ := insertProfileEmployee(t, db)
		fileID := insertProfileFile(t, db, employeeID, "certificado.pdf", "uploaded")

		file, err := repo.GetEmployeeFile(ctx, employeeID, fileID)
		if err != nil {
			t.Fatalf("leer el archivo: %v", err)
		}
		if file.Bucket != "laburito-bucket" || file.ObjectKey == "" {
			t.Fatalf("el archivo no trae su ubicación para firmar: %+v", file)
		}
		if file.Filename != "certificado.pdf" || file.ContentType != "application/pdf" {
			t.Fatalf("metadatos inesperados: %+v", file)
		}
	})

	// El archivo ajeno y el inexistente devuelven el mismo error porque el filtro por empleado
	// está en la consulta: no hay forma de que el borde los distinga por accidente.
	t.Run("archivo ajeno, inexistente o sin subir", func(t *testing.T) {
		employeeID, _ := insertProfileEmployee(t, db)
		otherID, _ := insertProfileEmployee(t, db)
		otherFile := insertProfileFile(t, db, otherID, "ajeno.pdf", "uploaded")
		pendingFile := insertProfileFile(t, db, employeeID, "pendiente.pdf", "pending")

		for name, fileID := range map[string]int32{
			"ajeno":       otherFile,
			"inexistente": 999_999,
			"sin subir":   pendingFile,
		} {
			if _, err := repo.GetEmployeeFile(ctx, employeeID, fileID); !errors.Is(err, ErrFileNotFound) {
				t.Fatalf("%s: se esperaba ErrFileNotFound, se obtuvo %v", name, err)
			}
		}
	})
}

func TestGetEmployeeEducationDocument(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	t.Run("documento propio", func(t *testing.T) {
		employeeID, _ := insertProfileEmployee(t, db)
		document := "certifications/titulo.pdf"
		educationID := insertProfileEducation(t, db, employeeID, "Ingeniería", &document)

		file, err := repo.GetEmployeeEducationDocument(ctx, employeeID, educationID)
		if err != nil {
			t.Fatalf("leer el documento: %v", err)
		}
		if file.ObjectKey != document {
			t.Fatalf("se resolvió otra clave: %+v", file)
		}
		// El nombre original no se persistió al subirlo, así que se deriva del título: es lo
		// único que identifica al documento para quien lo descarga.
		if !strings.HasPrefix(file.Filename, "Ingeniería") || !strings.HasSuffix(file.Filename, ".pdf") {
			t.Fatalf("nombre de descarga inesperado: %q", file.Filename)
		}
		// El bucket queda vacío a propósito: employee_education no lo persiste y quien firma
		// resuelve el suyo. Inventarlo acá sería adivinar.
		if file.Bucket != "" {
			t.Fatalf("la lectura inventó un bucket: %q", file.Bucket)
		}
	})

	t.Run("documento ajeno, inexistente o ausente", func(t *testing.T) {
		employeeID, _ := insertProfileEmployee(t, db)
		otherID, _ := insertProfileEmployee(t, db)
		document := "certifications/ajeno.pdf"
		otherEducation := insertProfileEducation(t, db, otherID, "Ajena", &document)
		withoutDocument := insertProfileEducation(t, db, employeeID, "Sin documento", nil)
		blank := " "
		withBlankDocument := insertProfileEducation(t, db, employeeID, "En blanco", &blank)

		for name, educationID := range map[string]int32{
			"ajeno":         otherEducation,
			"inexistente":   999_999,
			"sin documento": withoutDocument,
			"en blanco":     withBlankDocument,
		} {
			if _, err := repo.GetEmployeeEducationDocument(ctx, employeeID, educationID); !errors.Is(err, ErrFileNotFound) {
				t.Fatalf("%s: se esperaba ErrFileNotFound, se obtuvo %v", name, err)
			}
		}
	})
}
