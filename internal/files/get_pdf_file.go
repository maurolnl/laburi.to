package files

import (
	"fmt"
	"io"
	"mime"
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

	headers := multipartFileHeaders(r, keyName)
	if len(headers) > 1 {
		return nil, ErrInvalidFile
	}

	if len(headers) == 0 {
		return nil, nil
	}
	header := headers[0]

	file, err := header.Open()
	if err != nil {
		return nil, ErrInvalidFile
	}

	if header.Size > maxSize {
		file.Close()
		return nil, ErrFileTooLarge
	}

	contentType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil || contentType != "application/pdf" {
		file.Close()
		return nil, ErrUnsupportedFileType
	}

	buffer := make([]byte, 512)
	bytesRead, readErr := file.Read(buffer)
	if readErr != nil && readErr != io.EOF {
		file.Close()
		return nil, ErrInvalidFile
	}
	if http.DetectContentType(buffer[:bytesRead]) != "application/pdf" {
		file.Close()
		return nil, ErrUnsupportedFileType
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return nil, ErrInvalidFile
	}

	return &PDFFile{
		File:        file,
		Filename:    header.Filename,
		ContentType: contentType,
		Size:        header.Size,
	}, nil
}

func multipartFileHeaders(r *http.Request, key string) []*multipart.FileHeader {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil
	}

	return r.MultipartForm.File[key]
}
