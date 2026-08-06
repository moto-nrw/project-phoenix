package common

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
)

// DocxContentType is the DOCX MIME type. http.DetectContentType cannot
// produce it — DOCX is an OOXML ZIP container, so magic bytes only prove
// application/zip and the package contents disambiguate it.
const DocxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// AllowedDocumentTypes maps MIME types detected by http.DetectContentType to
// allowed document uploads (staff documents, #1424). DOCX is handled
// separately — see DocxContentType.
var AllowedDocumentTypes = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"image/jpg":       true,
}

const invalidDocumentTypeMessage = "invalid file type. Only PDF, DOCX, PNG and JPEG files are allowed"

// ParseDocumentWithLimits parses a multipart document upload and validates
// the content via magic bytes: PDF, PNG and JPEG directly, DOCX as a ZIP
// container with the required OOXML parts (a bare .zip is rejected).
// Enforces the advertised file-size cap separately from the multipart body
// cap. The caller must close UploadedFile.File.
func ParseDocumentWithLimits(w http.ResponseWriter, r *http.Request, fieldName string, maxFileSize, maxBodySize int64) (*UploadedFile, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	if err := r.ParseMultipartForm(maxBodySize); err != nil {
		return nil, errors.New("file too large")
	}

	file, header, err := r.FormFile(fieldName)
	if err != nil {
		return nil, errors.New("no file uploaded")
	}
	if header.Size > maxFileSize {
		_ = file.Close()
		return nil, errors.New("file too large")
	}

	contentType, err := detectDocumentContentType(file, header.Filename)
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	return &UploadedFile{
		File:        file,
		Filename:    header.Filename,
		ContentType: contentType,
	}, nil
}

// detectDocumentContentType reads the first 512 bytes to detect the MIME
// type via magic bytes and validates it against the allowed document types.
func detectDocumentContentType(file io.ReadSeeker, filename string) (string, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if n == 0 {
		if err != nil {
			return "", errors.New("cannot read file")
		}
		return "", errors.New("empty file")
	}

	contentType := http.DetectContentType(buf[:n])
	switch {
	case AllowedDocumentTypes[contentType]:
		// PDF or image, magic bytes are conclusive.
	case contentType == "application/zip" && strings.HasSuffix(strings.ToLower(filename), ".docx"):
		if !isDOCX(file) {
			return "", errors.New(invalidDocumentTypeMessage)
		}
		contentType = DocxContentType
	default:
		return "", errors.New(invalidDocumentTypeMessage)
	}

	if _, err := file.Seek(0, 0); err != nil {
		return "", errors.New("failed to process file")
	}
	return contentType, nil
}

// isDOCX verifies the minimum OOXML structure that distinguishes a Word
// document from an arbitrary ZIP archive. Uploads are capped at 10 MB, so
// reading the container here is bounded by the documented upload limit.
func isDOCX(file io.ReadSeeker) bool {
	if _, err := file.Seek(0, 0); err != nil {
		return false
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return false
	}
	reader, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return false
	}

	parts := make(map[string]struct{}, len(reader.File))
	for _, entry := range reader.File {
		parts[entry.Name] = struct{}{}
	}
	_, hasContentTypes := parts["[Content_Types].xml"]
	_, hasDocument := parts["word/document.xml"]
	return hasContentTypes && hasDocument
}

// DocumentFileExtension returns the canonical stored-file extension for a
// validated document content type.
func DocumentFileExtension(contentType string) string {
	switch contentType {
	case "application/pdf":
		return ".pdf"
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case DocxContentType:
		return ".docx"
	default:
		return ""
	}
}
