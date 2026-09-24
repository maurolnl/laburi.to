package employee

import "time"

const (
	maxUploadSize              = 5 << 20 // 5 MB
	certificationFileType      = "certification"
	employeeFileStatusUploaded = "uploaded"

	// educationDocumentContentType es siempre PDF: la carga de documentos de título solo acepta
	// PDF, y employee_education no persiste el content type del archivo.
	educationDocumentContentType = "application/pdf"
)

// presignTTL es cuánto vive una URL de descarga. Es una constante y no una variable de entorno
// a propósito: una URL prefirmada es una credencial, y su plazo es una decisión de seguridad
// del producto y no de despliegue. Una variable mal puesta en producción la convertiría en un
// enlace permanente sin que nadie lo note.
const presignTTL = 5 * time.Minute
