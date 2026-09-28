package employee

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/files"
)

// certificationsFormField es el único campo con el que viajan las certificaciones del paso
// base: un arreglo JSON de ítems. legacyCertificationsFormField es el formato anterior, que se
// rechaza en vez de ignorarse: un cliente viejo que lo mande borraría las certificaciones en
// silencio.
const (
	certificationsFormField       = "certifications"
	legacyCertificationsFormField = "certifications[]"
)

// CertificationItem es un ítem del campo multipart `certifications`. Document es la clave de un
// archivo nuevo del mismo multipart y DocumentID el certificado ya cargado que se conserva;
// son excluyentes, y sin ninguno la certificación queda sin PDF.
type CertificationItem struct {
	Name       string  `json:"name"`
	Document   *string `json:"document"`
	DocumentID *int32  `json:"document_id"`
}

// CertificationUpload es el PDF nuevo de una certificación, ya validado en el borde.
type CertificationUpload struct {
	File        multipart.File
	Filename    string
	ContentType string
	Size        int64
}

// CertificationEntry es una certificación ya parseada y validada: su nombre y, a lo sumo, uno
// de los dos orígenes de su PDF.
type CertificationEntry struct {
	Name       string
	KeepFileID *int32
	Upload     *CertificationUpload
}

// KeptCertificationFile es un certificado que una actualización conserva, con el nombre de la
// certificación a la que queda asociado.
type KeptCertificationFile struct {
	FileID int32
	Name   string
}

// RemovedFile es la ubicación de un certificado dado de baja, que se borra del almacenamiento
// después de confirmar la transacción.
type RemovedFile struct {
	Bucket    string
	ObjectKey string
}

func certificationNames(entries []CertificationEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}

	return names
}

func closeCertificationUploads(entries []CertificationEntry) {
	for _, entry := range entries {
		if entry.Upload != nil {
			entry.Upload.File.Close()
		}
	}
}

func invalidCertifications(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidCertifications, fmt.Sprintf(format, args...))
}

// parseCertificationsForm lee y valida las certificaciones del multipart ya parseado. Todas las
// reglas se comprueban acá, antes de subir nada al almacenamiento: un request inválido no deja
// archivos que limpiar. Si devuelve error, no queda ningún archivo abierto; si no, el llamador
// cierra los que recibe con closeCertificationUploads.
func parseCertificationsForm(r *http.Request) ([]CertificationEntry, error) {
	if len(r.MultipartForm.Value[legacyCertificationsFormField]) > 0 {
		return nil, invalidCertifications("unsupported format: send %q as a JSON array", certificationsFormField)
	}

	items, err := decodeCertificationItems(r.MultipartForm.Value[certificationsFormField])
	if err != nil {
		return nil, err
	}

	entries := make([]CertificationEntry, 0, len(items))
	// keys guarda, por índice de entries, la clave del archivo nuevo: se abre recién después de
	// validar todo lo que no requiere leerlo.
	keys := make([]string, 0, len(items))
	names := map[string]bool{}
	documentKeys := map[string]bool{}
	documentIDs := map[int32]bool{}

	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return nil, invalidCertifications("name is required")
		}

		normalized := strings.ToLower(name)
		if names[normalized] {
			return nil, invalidCertifications("duplicated certification %q", name)
		}
		names[normalized] = true

		documentKey := ""
		if item.Document != nil {
			documentKey = strings.TrimSpace(*item.Document)
		}
		if documentKey != "" && item.DocumentID != nil {
			return nil, invalidCertifications("certification %q cannot have both document and document_id", name)
		}

		entry := CertificationEntry{Name: name}
		switch {
		case documentKey != "":
			if documentKeys[documentKey] {
				return nil, invalidCertifications("document %q is referenced more than once", documentKey)
			}
			documentKeys[documentKey] = true
		case item.DocumentID != nil:
			id := *item.DocumentID
			if id <= 0 {
				return nil, invalidCertifications("certification %q has an invalid document_id", name)
			}
			if documentIDs[id] {
				return nil, invalidCertifications("document_id %d is referenced more than once", id)
			}
			documentIDs[id] = true
			entry.KeepFileID = &id
		}

		entries = append(entries, entry)
		keys = append(keys, documentKey)
	}

	for key := range r.MultipartForm.File {
		if !documentKeys[key] {
			return nil, invalidCertifications("file %q is not associated with any certification", key)
		}
	}

	for i, key := range keys {
		if key == "" {
			continue
		}

		pdf, err := files.GetPDF(r, key, maxUploadSize)
		if err == nil && pdf == nil {
			err = invalidCertifications("document %q not found", key)
		}
		if err != nil {
			closeCertificationUploads(entries[:i])
			return nil, fmt.Errorf("certification %q: %w", entries[i].Name, err)
		}

		entries[i].Upload = &CertificationUpload{
			File:        pdf.File,
			Filename:    pdf.Filename,
			ContentType: pdf.ContentType,
			Size:        pdf.Size,
		}
	}

	return entries, nil
}

// decodeCertificationItems acepta el campo ausente, vacío o `null` como la lista vacía: el
// empleado que no declara certificaciones no tiene que mandar nada.
func decodeCertificationItems(values []string) ([]CertificationItem, error) {
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) > 1 {
		return nil, invalidCertifications("%q must be sent once", certificationsFormField)
	}

	raw := strings.TrimSpace(values[0])
	if raw == "" || raw == "null" {
		return nil, nil
	}

	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()

	var items []CertificationItem
	if err := decoder.Decode(&items); err != nil {
		return nil, invalidCertifications("%q must be a JSON array of {name, document, document_id}", certificationsFormField)
	}

	return items, nil
}
