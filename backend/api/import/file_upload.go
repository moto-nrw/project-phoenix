package importapi

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	importService "github.com/moto-nrw/project-phoenix/services/import"
)

// FileUploadResult contains the parsed CSV data and file metadata
type FileUploadResult struct {
	Rows     []importModels.StudentImportRow
	Filename string
}

// validateAndParseCSVFile handles common file upload validation and parsing
// Supports both CSV and Excel (.xlsx) files
func (rs *Resource) validateAndParseCSVFile(w http.ResponseWriter, r *http.Request) (*FileUploadResult, bool) {
	// Security: File size limit
	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)

	// Parse multipart form
	if r.ParseMultipartForm(maxFileSize) != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("datei zu groß (max 10MB)")))
		return nil, false
	}

	// Get the file from the request
	file, header, err := r.FormFile("file")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("datei fehlt")))
		return nil, false
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	// Validate file type (MIME type and extension)
	if !isValidImportFile(header) {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("ungültiger Dateityp (nur CSV oder Excel erlaubt)")))
		return nil, false
	}

	// SECURITY: Verify actual file content using magic bytes
	// Protects against file type spoofing (e.g., malware.exe renamed to students.csv)
	if err := verifyFileContent(file, header); err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return nil, false
	}

	// Select appropriate parser based on file extension
	var parser importService.FileParser
	if isExcelFile(header) {
		parser = importService.NewXLSXParser()
	} else {
		parser = importService.NewCSVParser()
	}

	// Parse file
	rows, err := parser.ParseStudents(file)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("Datei-Fehler: %s", err.Error())))
		return nil, false
	}

	return &FileUploadResult{
		Rows:     rows,
		Filename: header.Filename,
	}, true
}

// verifyFileContent verifies the actual file content using magic bytes
// SECURITY: Protects against file type spoofing by checking actual file content
// instead of relying on MIME type or extension alone
func verifyFileContent(file multipart.File, header *multipart.FileHeader) error {
	// Read first 512 bytes for magic byte detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("fehler beim Lesen der Datei")
	}
	buffer = buffer[:n]

	// Reset file position to beginning for subsequent reads
	if seeker, ok := file.(io.Seeker); ok {
		if _, err := seeker.Seek(0, 0); err != nil {
			return fmt.Errorf("fehler beim Zurücksetzen der Dateiposition")
		}
	}

	filename := strings.ToLower(header.Filename)

	// Excel (.xlsx) files are ZIP archives starting with PK\x03\x04
	if strings.HasSuffix(filename, ".xlsx") {
		if len(buffer) < 4 || !bytes.Equal(buffer[:4], []byte{0x50, 0x4B, 0x03, 0x04}) {
			return fmt.Errorf("ungültiges Excel-Format: Die Datei ist keine echte .xlsx-Datei")
		}
		return nil
	}

	// CSV files should be valid UTF-8 text
	if strings.HasSuffix(filename, ".csv") {
		// Check if content is valid UTF-8 text
		if !utf8.Valid(buffer) {
			// Try to detect if it's binary data
			if isBinaryData(buffer) {
				return fmt.Errorf("ungültiges CSV-Format: Die Datei scheint binäre Daten zu enthalten")
			}
		}
		return nil
	}

	return fmt.Errorf("nicht unterstützter Dateityp")
}

// isBinaryData checks if the buffer contains binary (non-text) data
// Returns true if more than 30% of bytes are non-printable (excluding common whitespace)
func isBinaryData(buffer []byte) bool {
	if len(buffer) == 0 {
		return false
	}

	nonPrintable := 0
	for _, b := range buffer {
		// Allow common text characters: printable ASCII + whitespace
		if b < 32 && b != 9 && b != 10 && b != 13 { // tab, LF, CR
			nonPrintable++
		} else if b == 127 || b > 127 && b < 160 { // DEL and control characters
			nonPrintable++
		}
	}

	// If more than 30% is non-printable, consider it binary
	threshold := len(buffer) * 30 / 100
	return nonPrintable > threshold
}

// isValidImportFile checks if the uploaded file is a valid CSV or Excel file
func isValidImportFile(header *multipart.FileHeader) bool {
	allowedTypes := map[string]bool{
		"text/csv":                 true,
		"application/vnd.ms-excel": true,
		"text/plain":               true,
		"application/csv":          true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true, // .xlsx
	}
	contentType := header.Header.Get("Content-Type")

	// Check content type or file extension as fallback
	if allowedTypes[contentType] {
		return true
	}

	// Fallback to extension check for browsers that don't set proper MIME types
	filename := strings.ToLower(header.Filename)
	return strings.HasSuffix(filename, ".csv") || strings.HasSuffix(filename, ".xlsx")
}

// isExcelFile checks if the uploaded file is an Excel file
func isExcelFile(header *multipart.FileHeader) bool {
	contentType := header.Header.Get("Content-Type")
	filename := strings.ToLower(header.Filename)

	return contentType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
		strings.HasSuffix(filename, ".xlsx")
}
