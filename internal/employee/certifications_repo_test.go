package employee

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/testsupport"
)

// Estos tests ejercen la asociación entre certificación y PDF contra el esquema real: el orden
// de las bajas y reasignaciones solo se puede probar contra el índice único de verdad.

func insertCertificationUser(t *testing.T, db *sql.DB) int32 {
	t.Helper()

	var userID int32
	email := fmt.Sprintf("certifications-%d@laburi.to", time.Now().UnixNano())
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO users (email, hashed_password, role, created_at, updated_at)
		VALUES ($1, 'hash', 'employee', now(), now())
		RETURNING id
	`, email).Scan(&userID); err != nil {
		t.Fatalf("crear usuario: %v", err)
	}

	return userID
}

func certificationFile(name, filename string) EmployeeFileMetadata {
	return EmployeeFileMetadata{
		Type:              certificationFileType,
		Bucket:            "laburito-bucket",
		ObjectKey:         fmt.Sprintf("certifications/%d-%s", time.Now().UnixNano(), filename),
		OriginalFilename:  filename,
		ContentType:       "application/pdf",
		SizeBytes:         1024,
		ChecksumSHA256:    "deadbeef",
		Status:            employeeFileStatusUploaded,
		CertificationName: name,
	}
}

func baseRequest(names ...string) CreateEmployeeRequest {
	return CreateEmployeeRequest{BaseEmployeeRequest: BaseEmployeeRequest{
		Position:          "Backend Engineer",
		Role:              "Go developer",
		YearsOfExperience: Years2To5Y,
		Certifications:    names,
	}}
}

// fileIDByName resuelve el certificado activo de cada certificación tal como lo ve el perfil.
func fileIDByName(t *testing.T, repo *EmployeeRepository, employeeID int32) map[string]*int32 {
	t.Helper()

	profile, err := repo.GetEmployeeProfileByID(context.Background(), employeeID)
	if err != nil {
		t.Fatalf("leer el perfil: %v", err)
	}

	byName := map[string]*int32{}
	for _, certification := range profile.Certifications {
		byName[certification.Name] = certification.DocumentID
	}

	return byName
}

func fileStatus(t *testing.T, db *sql.DB, fileID int32) (status string, name sql.NullString) {
	t.Helper()

	if err := db.QueryRowContext(context.Background(),
		`SELECT status, certification_name FROM employee_files WHERE id = $1`, fileID,
	).Scan(&status, &name); err != nil {
		t.Fatalf("leer el archivo %d: %v", fileID, err)
	}

	return status, name
}

func TestCreateEmployeeAssociatesEachPDF(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	userID := insertCertificationUser(t, db)
	employeeID, err := repo.CreateEmployee(ctx, baseRequest("Scrum Master", "AWS Cloud Practitioner", "ITIL"), userID, []EmployeeFileMetadata{
		certificationFile("Scrum Master", "scrum.pdf"),
		certificationFile("AWS Cloud Practitioner", "aws.pdf"),
	})
	if err != nil {
		t.Fatalf("crear el empleado: %v", err)
	}

	profile, err := repo.GetEmployeeProfileByID(ctx, employeeID)
	if err != nil {
		t.Fatalf("leer el perfil: %v", err)
	}
	if len(profile.Certifications) != 3 {
		t.Fatalf("certificaciones inesperadas: %+v", profile.Certifications)
	}
	// El orden es el declarado por el empleado, no el de los archivos.
	for i, name := range []string{"Scrum Master", "AWS Cloud Practitioner", "ITIL"} {
		if profile.Certifications[i].Name != name {
			t.Fatalf("orden inesperado: %+v", profile.Certifications)
		}
	}
	if profile.Certifications[0].DocumentID == nil || profile.Certifications[1].DocumentID == nil {
		t.Fatalf("las certificaciones con PDF no traen su identificador: %+v", profile.Certifications)
	}
	if *profile.Certifications[0].DocumentID == *profile.Certifications[1].DocumentID {
		t.Fatalf("dos certificaciones comparten PDF: %+v", profile.Certifications)
	}
	if profile.Certifications[2].DocumentID != nil {
		t.Fatalf("la certificación sin PDF trae identificador: %+v", profile.Certifications[2])
	}
	if len(profile.Files) != 0 {
		t.Fatalf("un certificado asociado no debe listarse como sin asociar: %+v", profile.Files)
	}

	file, err := repo.GetEmployeeFile(ctx, employeeID, *profile.Certifications[1].DocumentID)
	if err != nil || file.Filename != "aws.pdf" {
		t.Fatalf("el PDF de la certificación no se entrega: %+v %v", file, err)
	}

	// El perfil propio, direccionado por usuario, describe lo mismo.
	own, err := repo.GetEmployee(ctx, userID)
	if err != nil {
		t.Fatalf("leer el perfil propio: %v", err)
	}
	if len(own.Certifications) != 3 || own.Certifications[1].DocumentID == nil ||
		*own.Certifications[1].DocumentID != *profile.Certifications[1].DocumentID || own.Certifications[2].DocumentID != nil {
		t.Fatalf("perfil propio inesperado: %+v", own.Certifications)
	}
	if len(own.Files) != 0 {
		t.Fatalf("perfil propio con archivos sin asociar inesperados: %+v", own.Files)
	}
}

func TestLegacyCertificateIsListedUnassigned(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	employeeID, userID := insertProfileEmployee(t, db)
	if _, err := db.ExecContext(ctx, `UPDATE employees SET certifications = ARRAY['Scrum Master', 'AWS'] WHERE id = $1`, employeeID); err != nil {
		t.Fatalf("declarar certificaciones: %v", err)
	}
	legacy := insertProfileFile(t, db, employeeID, "viejo.pdf", "uploaded")

	for name, read := range map[string]func() ([]CertificationResponseItem, []ProfileFileItem){
		"por empleado": func() ([]CertificationResponseItem, []ProfileFileItem) {
			profile, err := repo.GetEmployeeProfileByID(ctx, employeeID)
			if err != nil {
				t.Fatalf("leer el perfil: %v", err)
			}
			return profile.Certifications, profile.Files
		},
		"por usuario": func() ([]CertificationResponseItem, []ProfileFileItem) {
			employee, err := repo.GetEmployee(ctx, userID)
			if err != nil {
				t.Fatalf("leer el perfil propio: %v", err)
			}
			return employee.Certifications, employee.Files
		},
	} {
		t.Run(name, func(t *testing.T) {
			certifications, files := read()
			if len(certifications) != 2 || certifications[0].DocumentID != nil || certifications[1].DocumentID != nil {
				t.Fatalf("un PDF viejo no debe atribuirse a ninguna certificación: %+v", certifications)
			}
			if len(files) != 1 || files[0].ID != legacy || files[0].Title != "viejo.pdf" {
				t.Fatalf("el PDF viejo no se lista sin asociar: %+v", files)
			}
		})
	}
}

func TestUpdateEmployeeCertificationSet(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	userID := insertCertificationUser(t, db)
	scrumFile := certificationFile("Scrum Master", "scrum.pdf")
	awsFile := certificationFile("AWS", "aws.pdf")
	k8sFile := certificationFile("Kubernetes", "k8s.pdf")
	employeeID, err := repo.CreateEmployee(ctx, baseRequest("Scrum Master", "AWS", "Kubernetes"), userID, []EmployeeFileMetadata{scrumFile, awsFile, k8sFile})
	if err != nil {
		t.Fatalf("crear el empleado: %v", err)
	}
	legacy := insertProfileFile(t, db, employeeID, "viejo.pdf", "uploaded")
	untouchedLegacy := insertProfileFile(t, db, employeeID, "otro-viejo.pdf", "uploaded")

	before := fileIDByName(t, repo, employeeID)
	scrumID, awsID, k8sID := *before["Scrum Master"], *before["AWS"], *before["Kubernetes"]

	// Scrum conserva su PDF, AWS lo reemplaza, Kubernetes lo quita y la certificación nueva
	// adopta un PDF viejo sin asociar.
	newAWS := certificationFile("AWS", "aws-nuevo.pdf")
	removed, err := repo.UpdateEmployee(ctx, employeeID, baseRequest("Scrum Master", "AWS", "Kubernetes", "ITIL"), []KeptCertificationFile{
		{FileID: scrumID, Name: "Scrum Master"},
		{FileID: legacy, Name: "ITIL"},
	}, []EmployeeFileMetadata{newAWS})
	if err != nil {
		t.Fatalf("actualizar el empleado: %v", err)
	}

	removedKeys := map[string]bool{}
	for _, file := range removed {
		removedKeys[file.ObjectKey] = true
	}
	if len(removed) != 2 || !removedKeys[awsFile.ObjectKey] || !removedKeys[k8sFile.ObjectKey] {
		t.Fatalf("bajas inesperadas: %+v", removed)
	}

	for _, id := range []int32{awsID, k8sID} {
		if status, _ := fileStatus(t, db, id); status != "deleted" {
			t.Fatalf("el archivo %d quedó %q", id, status)
		}
		if _, err := repo.GetEmployeeFile(ctx, employeeID, id); !errors.Is(err, ErrFileNotFound) {
			t.Fatalf("un certificado dado de baja se sigue entregando: %v", err)
		}
	}

	after := fileIDByName(t, repo, employeeID)
	if after["Scrum Master"] == nil || *after["Scrum Master"] != scrumID {
		t.Fatalf("Scrum no conservó su PDF: %+v", after)
	}
	if after["AWS"] == nil || *after["AWS"] == awsID {
		t.Fatalf("AWS no tiene el PDF nuevo: %+v", after)
	}
	if after["Kubernetes"] != nil {
		t.Fatalf("Kubernetes conserva un PDF quitado: %+v", after)
	}
	if after["ITIL"] == nil || *after["ITIL"] != legacy {
		t.Fatalf("ITIL no adoptó el PDF viejo: %+v", after)
	}

	profile, err := repo.GetEmployeeProfileByID(ctx, employeeID)
	if err != nil {
		t.Fatalf("leer el perfil: %v", err)
	}
	if len(profile.Files) != 1 || profile.Files[0].ID != untouchedLegacy {
		t.Fatalf("el PDF viejo no referenciado debe quedar sin asociar: %+v", profile.Files)
	}
}

func TestUpdateEmployeeSwapsCertificationNames(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	userID := insertCertificationUser(t, db)
	employeeID, err := repo.CreateEmployee(ctx, baseRequest("AWS", "GCP"), userID, []EmployeeFileMetadata{
		certificationFile("AWS", "aws.pdf"),
		certificationFile("GCP", "gcp.pdf"),
	})
	if err != nil {
		t.Fatalf("crear el empleado: %v", err)
	}
	before := fileIDByName(t, repo, employeeID)

	// Intercambiar los PDFs en un solo paso chocaría con el índice único si los nombres no se
	// liberaran antes de reasignarlos.
	if _, err := repo.UpdateEmployee(ctx, employeeID, baseRequest("AWS", "GCP"), []KeptCertificationFile{
		{FileID: *before["GCP"], Name: "AWS"},
		{FileID: *before["AWS"], Name: "GCP"},
	}, nil); err != nil {
		t.Fatalf("intercambiar: %v", err)
	}

	after := fileIDByName(t, repo, employeeID)
	if *after["AWS"] != *before["GCP"] || *after["GCP"] != *before["AWS"] {
		t.Fatalf("intercambio inesperado: antes %v, después %v", before, after)
	}
}

func TestUpdateEmployeeRejectsForeignDocument(t *testing.T) {
	db := testsupport.PostgresDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	userID := insertCertificationUser(t, db)
	employeeID, err := repo.CreateEmployee(ctx, baseRequest("AWS"), userID, []EmployeeFileMetadata{certificationFile("AWS", "aws.pdf")})
	if err != nil {
		t.Fatalf("crear el empleado: %v", err)
	}
	otherID, _ := insertProfileEmployee(t, db)
	foreign := insertProfileFile(t, db, otherID, "ajeno.pdf", "uploaded")
	deleted := insertProfileFile(t, db, employeeID, "borrado.pdf", "deleted")
	before := fileIDByName(t, repo, employeeID)

	for name, fileID := range map[string]int32{"ajeno": foreign, "dado de baja": deleted, "inexistente": 999_999} {
		t.Run(name, func(t *testing.T) {
			_, err := repo.UpdateEmployee(ctx, employeeID, baseRequest("Otra"), []KeptCertificationFile{{FileID: fileID, Name: "Otra"}}, nil)
			if !errors.Is(err, ErrInvalidCertifications) {
				t.Fatalf("se esperaba ErrInvalidCertifications, se obtuvo %v", err)
			}

			// Nada se persistió: el PDF de AWS sigue activo y asociado.
			if status, name := fileStatus(t, db, *before["AWS"]); status != "uploaded" || name.String != "AWS" {
				t.Fatalf("la actualización rechazada modificó el certificado: %s %v", status, name)
			}
			if after := fileIDByName(t, repo, employeeID); after["AWS"] == nil {
				t.Fatalf("la actualización rechazada cambió las certificaciones: %v", after)
			}
		})
	}
}

func TestMigration0008IsIdempotent(t *testing.T) {
	db := testsupport.PostgresDB(t)

	contents, err := os.ReadFile("../../sql/schema/0008_employee_file_certification_name.sql")
	if err != nil {
		t.Fatalf("leer la migración: %v", err)
	}

	// PostgresDB ya aplicó la migración: reaplicarla dos veces más no debe fallar.
	for i := 0; i < 2; i++ {
		if _, err := db.ExecContext(context.Background(), testsupport.UpSection(string(contents))); err != nil {
			t.Fatalf("reaplicar la migración (%d): %v", i+1, err)
		}
	}

	var indexes int
	if err := db.QueryRowContext(context.Background(), `
		SELECT count(*) FROM pg_indexes WHERE indexname = 'employee_files_active_certification_key'
	`).Scan(&indexes); err != nil {
		t.Fatalf("consultar el índice: %v", err)
	}
	if indexes != 1 {
		t.Fatalf("se esperaba un índice, hay %d", indexes)
	}
}
