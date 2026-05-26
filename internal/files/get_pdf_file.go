package files

import (
	"fmt"
	"mime/multipart"
	"net/http"
)

type PDFFile struct {
	File        multipart.File
	Filename    string
	ContentType string
	Size        int64
}

var (
	ErrInvalidFile         = fmt.Errorf("invalid file")
	ErrFileTooLarge        = fmt.Errorf("file too large")
	ErrUnsupportedFileType = fmt.Errorf("unsupported file type")
)

func GetPDF(r *http.Request, keyName string, maxSizeParam ...int64) (*PDFFile, error) {
	maxSize := int64(5 << 20) // 5 MB
	if len(maxSizeParam) > 0 {
		maxSize = maxSizeParam[0]
	}

	header := getMultipartFileHeader(r, keyName)

	if header == nil {
		return nil, ErrInvalidFile
	}

	file, err := header.Open()
	if err != nil {
		return nil, ErrInvalidFile
	}

	if header.Size > maxSize {
		file.Close()
		return nil, ErrFileTooLarge
	}

	contentType := header.Header.Get("Content-Type")
	if contentType != "application/pdf" {
		file.Close()
		return nil, ErrUnsupportedFileType
	}

	return &PDFFile{
		File:        file,
		Filename:    header.Filename,
		ContentType: contentType,
		Size:        header.Size,
	}, nil
}

func getMultipartFileHeader(r *http.Request, key string) *multipart.FileHeader {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil
	}

	headers := r.MultipartForm.File[key]

	if len(headers) > 0 {
		return headers[0]
	}

	return nil
}
