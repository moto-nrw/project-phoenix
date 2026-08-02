package common

import (
	"errors"
	"io"
	"net/http"
	"strings"
)

// DocxContentType is the DOCX MIME type. http.DetectContentType cannot
// produce it — DOCX is a ZIP container, so magic bytes only prove
// application/zip and the extension disambiguates (same approach as the
// XLSX check in api/import).
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
// container whose original filename must end in .docx (a bare .zip is
// rejected). Enforces the advertised file-size cap separately from the
// multipart body cap. The caller must close UploadedFile.File.
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
		contentType = DocxContentType
	default:
		return "", errors.New(invalidDocumentTypeMessage)
	}

	if _, err := file.Seek(0, 0); err != nil {
		return "", errors.New("failed to process file")
	}
	return contentType, nil
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
