package importapi

import (
	"fmt"
	"log"
	"mime/multipart"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// FileUploadResult contains the parsed CSV data and file metadata
type FileUploadResult struct {
	Rows     []importModels.StudentImportRow
	Filename string
}

// validateAndParseCSVFile handles common file upload validation and parsing
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
	if !isValidCSVFile(header) {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("ungültiger Dateityp (nur CSV erlaubt)"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return nil, false
	}

	// Parse CSV
	rows, err := rs.csvParser.ParseStudents(file)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("CSV-Fehler: %s", err.Error()))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return nil, false
	}

	return &FileUploadResult{
		Rows:     rows,
		Filename: header.Filename,
	}, true
}

// isValidCSVFile checks if the uploaded file is a valid CSV
func isValidCSVFile(header *multipart.FileHeader) bool {
	allowedTypes := map[string]bool{
		"text/csv":                 true,
		"application/vnd.ms-excel": true,
		"text/plain":               true,
		"application/csv":          true,
	}
	contentType := header.Header.Get("Content-Type")
	return allowedTypes[contentType]
}
