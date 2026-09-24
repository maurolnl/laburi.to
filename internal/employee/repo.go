package employee

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/database"
)

type EmployeeRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *EmployeeRepository {
	return &EmployeeRepository{db: db}
}

func (r *EmployeeRepository) CreateEmployee(ctx context.Context, employee CreateEmployeeRequest, userID int32, file *EmployeeFileMetadata) (int32, error) {
	q := database.New(r.db)
	if file == nil {
		return q.CreateEmployeeWithoutFile(ctx, database.CreateEmployeeWithoutFileParams{
			Position:          employee.Position,
			Role:              employee.Role,
			YearsOfExperience: string(employee.YearsOfExperience),
			Certifications:    employee.Certifications,
			PortfolioUrl: sql.NullString{
				String: employee.PortfolioURL,
				Valid:  employee.PortfolioURL != "",
			},
			UserID: userID,
		})
	}

	employeeID, err := q.CreateEmployee(ctx, database.CreateEmployeeParams{
		Position:          employee.Position,
		Role:              employee.Role,
		YearsOfExperience: string(employee.YearsOfExperience),
		Certifications:    employee.Certifications,
		PortfolioUrl: sql.NullString{
			String: employee.PortfolioURL,
			Valid:  employee.PortfolioURL != "",
		},
		UserID:           userID,
		Type:             file.Type,
		Bucket:           file.Bucket,
		ObjectKey:        file.ObjectKey,
		OriginalFilename: file.OriginalFilename,
		ContentType:      file.ContentType,
		SizeBytes:        file.SizeBytes,
		ChecksumSha256: sql.NullString{
			String: file.ChecksumSHA256,
			Valid:  file.ChecksumSHA256 != "",
		},
		Status: file.Status,
	})
	if err != nil {
		return 0, err
	}

	return employeeID, nil
}

func (r *EmployeeRepository) UpdateEmployee(ctx context.Context, employeeID int32, employee CreateEmployeeRequest, file *EmployeeFileMetadata) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := database.New(r.db).WithTx(tx)
	if err := qtx.UpdateEmployee(ctx, database.UpdateEmployeeParams{
		ID:                employeeID,
		Position:          employee.Position,
		Role:              employee.Role,
		YearsOfExperience: string(employee.YearsOfExperience),
		Certifications:    employee.Certifications,
		PortfolioUrl: sql.NullString{
			String: employee.PortfolioURL,
			Valid:  employee.PortfolioURL != "",
		},
	}); err != nil {
		return err
	}

	if file != nil {
		if err := qtx.CreateEmployeeFile(ctx, database.CreateEmployeeFileParams{
			EmployeeID:       employeeID,
			Type:             file.Type,
			Bucket:           file.Bucket,
			ObjectKey:        file.ObjectKey,
			OriginalFilename: file.OriginalFilename,
			ContentType:      file.ContentType,
			SizeBytes:        file.SizeBytes,
			ChecksumSha256:   sql.NullString{String: file.ChecksumSHA256, Valid: file.ChecksumSHA256 != ""},
			Status:           file.Status,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func nullInt16ToPtr(n sql.NullInt16) *int16 {
	if !n.Valid {
		return nil
	}
	return &n.Int16
}

func (r *EmployeeRepository) GetEmployee(ctx context.Context, ID int32) (Employee, error) {
	q := database.New(r.db)
	row, err := q.GetEmployee(ctx, ID)
	if err != nil {
		return Employee{}, err
	}

	var internetConnections []InternetConnection
	if err := json.Unmarshal([]byte(row.InternetConnections), &internetConnections); err != nil {
		return Employee{}, err
	}

	var education []EducationItem
	if err := json.Unmarshal([]byte(row.Education), &education); err != nil {
		return Employee{}, err
	}

	var files []FileItem
	if err := json.Unmarshal([]byte(row.Files), &files); err != nil {
		return Employee{}, err
	}

	return Employee{
		ID:                   row.ID,
		UserID:               row.UserID,
		Email:                row.Email,
		Position:             row.Position,
		Role:                 row.Role,
		YearsOfExperience:    row.YearsOfExperience,
		Certifications:       row.Certifications,
		PortfolioURL:         row.PortfolioUrl.String,
		Timezone:             row.Timezone.String,
		Os:                   row.Os.String,
		PaidSoftware:         row.PaidSoftware,
		AvailableHoursPerDay: row.AvailableHoursPerDay.Int16,
		CompatibleProjects:   nullInt16ToPtr(row.CompatibleProjects),
		IncompatibleProjects: nullInt16ToPtr(row.IncompatibleProjects),
		InternetConnections:  internetConnections,
		Education:            education,
		Files:                files,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}, nil
}

// IsProfileComplete delega en la consulta que combina la existencia de las cinco filas. Se
// resuelve en una sola ida a la base y no reutiliza GetEmployee, que toma el userID, agrega
// tres subconsultas JSON y devuelve el perfil entero para responder una pregunta booleana.
func (r *EmployeeRepository) IsProfileComplete(ctx context.Context, employeeID int32) (bool, error) {
	q := database.New(r.db)
	return q.IsEmployeeProfileComplete(ctx, employeeID)
}

func (r *EmployeeRepository) GetEmployeeByID(ctx context.Context, ID int32) (Employee, error) {
	q := database.New(r.db)
	row, err := q.GetEmployeeByID(ctx, ID)
	if errors.Is(err, sql.ErrNoRows) {
		return Employee{}, ErrEmployeeProfileNotFound
	}
	if err != nil {
		return Employee{}, err
	}

	return Employee{
		ID:     row.ID,
		UserID: row.UserID,
	}, nil
}

func (r *EmployeeRepository) CreateLocationWithConnections(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := database.New(r.db).WithTx(tx)
	if err := validateTimezone(ctx, tx, locationRequest.Timezone); err != nil {
		return err
	}

	for _, conn := range locationRequest.InternetConnections {
		if _, err := qtx.CreateEmployeeConnection(ctx, database.CreateEmployeeConnectionParams{
			EmployeeID: employeeID,
			Type:       conn.Type,
			Speed:      conn.Speed,
		}); err != nil {
			return err
		}
	}

	if _, err := qtx.CreateEmployeeLocation(ctx, database.CreateEmployeeLocationParams{
		EmployeeID: employeeID,
		Timezone:   locationRequest.Timezone,
	}); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *EmployeeRepository) UpdateLocationWithConnections(ctx context.Context, employeeID int32, locationRequest CreateEmployeeLocationRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := database.New(r.db).WithTx(tx)
	if err := validateTimezone(ctx, tx, locationRequest.Timezone); err != nil {
		return err
	}

	if err := qtx.DeleteEmployeeConnections(ctx, employeeID); err != nil {
		return err
	}

	for _, conn := range locationRequest.InternetConnections {
		if _, err := qtx.CreateEmployeeConnection(ctx, database.CreateEmployeeConnectionParams{
			EmployeeID: employeeID,
			Type:       conn.Type,
			Speed:      conn.Speed,
		}); err != nil {
			return err
		}
	}

	if _, err := qtx.UpsertEmployeeLocation(ctx, database.UpsertEmployeeLocationParams{
		EmployeeID: employeeID,
		Timezone:   locationRequest.Timezone,
	}); err != nil {
		return err
	}

	return tx.Commit()
}

func validateTimezone(ctx context.Context, db database.DBTX, timezone string) error {
	const query = `
		SELECT EXISTS(
			SELECT 1
			FROM pg_timezone_names
			WHERE name = $1
		)
	`

	var exists bool
	if err := db.QueryRowContext(ctx, query, timezone).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrInvalidTimezone
	}

	return nil
}

// GetEmployeeProfileByID lee el perfil por el identificador de empleado. Comparte con
// GetEmployee la forma de la fila salvo en los dos agregados de archivos, así que la
// deserialización común vive en unmarshalProfileArrays y el mapeo no se duplica.
func (r *EmployeeRepository) GetEmployeeProfileByID(ctx context.Context, employeeID int32) (EmployeeProfile, error) {
	q := database.New(r.db)
	row, err := q.GetEmployeeProfileByID(ctx, employeeID)
	if errors.Is(err, sql.ErrNoRows) {
		return EmployeeProfile{}, ErrEmployeeProfileNotFound
	}
	if err != nil {
		return EmployeeProfile{}, err
	}

	var internetConnections []InternetConnection
	if err := json.Unmarshal([]byte(row.InternetConnections), &internetConnections); err != nil {
		return EmployeeProfile{}, err
	}

	var education []ProfileEducationItem
	if err := json.Unmarshal([]byte(row.Education), &education); err != nil {
		return EmployeeProfile{}, err
	}

	var files []ProfileFileItem
	if err := json.Unmarshal([]byte(row.Files), &files); err != nil {
		return EmployeeProfile{}, err
	}

	return EmployeeProfile{
		ID:                   row.ID,
		UserID:               row.UserID,
		Email:                row.Email,
		Position:             row.Position,
		Role:                 row.Role,
		YearsOfExperience:    row.YearsOfExperience,
		Certifications:       row.Certifications,
		PortfolioURL:         row.PortfolioUrl.String,
		Timezone:             row.Timezone.String,
		Os:                   row.Os.String,
		PaidSoftware:         row.PaidSoftware,
		AvailableHoursPerDay: row.AvailableHoursPerDay.Int16,
		CompatibleProjects:   nullInt16ToPtr(row.CompatibleProjects),
		IncompatibleProjects: nullInt16ToPtr(row.IncompatibleProjects),
		InternetConnections:  internetConnections,
		Education:            education,
		Files:                files,
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}, nil
}

// GetEmployeeFile y GetEmployeeEducationDocument filtran por el par empleado/identificador en
// la consulta y no acá: un archivo ajeno no devuelve fila, así que la traducción a
// ErrFileNotFound es la misma que la del archivo inexistente sin que este código tenga que
// acordarse de igualarlas.
func (r *EmployeeRepository) GetEmployeeFile(ctx context.Context, employeeID, fileID int32) (StoredFile, error) {
	q := database.New(r.db)
	row, err := q.GetEmployeeFileForEmployee(ctx, database.GetEmployeeFileForEmployeeParams{
		ID:         fileID,
		EmployeeID: employeeID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return StoredFile{}, ErrFileNotFound
	}
	if err != nil {
		return StoredFile{}, err
	}

	return StoredFile{
		Bucket:      row.Bucket,
		ObjectKey:   row.ObjectKey,
		Filename:    row.OriginalFilename,
		ContentType: row.ContentType,
	}, nil
}

// El documento de un título no es una fila de employee_files sino un object_key guardado en la
// propia fila de educación, así que no hay bucket ni content type persistidos. El bucket lo
// aporta quien firma, que es el único que conoce la configuración, y el content type es siempre
// PDF por la validación de carga.
func (r *EmployeeRepository) GetEmployeeEducationDocument(ctx context.Context, employeeID, educationID int32) (StoredFile, error) {
	q := database.New(r.db)
	row, err := q.GetEmployeeEducationDocumentForEmployee(ctx, database.GetEmployeeEducationDocumentForEmployeeParams{
		ID:         educationID,
		EmployeeID: employeeID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return StoredFile{}, ErrFileNotFound
	}
	if err != nil {
		return StoredFile{}, err
	}

	return StoredFile{
		ObjectKey:   row.Certification.String,
		Filename:    educationDocumentFilename(row.Title),
		ContentType: educationDocumentContentType,
	}, nil
}

// educationDocumentFilename arma el nombre con el que el navegador guarda el documento. El
// nombre original no se persistió al subirlo, así que se deriva del título del estudio, que es
// lo único que identifica al documento para quien lo descarga.
func educationDocumentFilename(title string) string {
	sanitized := strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r == '/' || r < ' ' {
			return '-'
		}
		return r
	}, strings.TrimSpace(title))

	if sanitized == "" {
		sanitized = "documento"
	}

	return sanitized + ".pdf"
}
