package importapi

import (
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"strings"

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
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("datei zu groß (max 10MB)"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return nil, false
	}

	// Get the file from the request
	file, header, err := r.FormFile("file")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("datei fehlt"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return nil, false
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	// Validate file type
	if !isValidImportFile(header) {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("ungültiger Dateityp (nur CSV oder Excel erlaubt)"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("Datei-Fehler: %s", err.Error()))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return nil, false
	}

	return &FileUploadResult{
		Rows:     rows,
		Filename: header.Filename,
	}, true
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
