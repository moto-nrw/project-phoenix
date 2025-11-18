package importapi

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	importService "github.com/moto-nrw/project-phoenix/services/import"
)

const (
	maxFileSize = 10 * 1024 * 1024 // 10MB
)

// Resource defines the import resource
type Resource struct {
	studentImportService *importService.ImportService[importModels.StudentImportRow]
	csvParser            *importService.CSVParser
}

// NewResource creates a new import resource
func NewResource(studentImportService *importService.ImportService[importModels.StudentImportRow]) *Resource {
	return &Resource{
		studentImportService: studentImportService,
		csvParser:            importService.NewCSVParser(),
	}
}

// Router returns a configured router for import endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes - require UsersCreate permission
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)

		// Student import endpoints
		r.Route("/students", func(r chi.Router) {
			// Template download - requires UsersRead
			r.With(authorize.RequiresPermission("users:read")).Get("/template", rs.downloadStudentTemplate)

			// Preview - requires UsersCreate
			r.With(authorize.RequiresPermission("users:create")).Post("/preview", rs.previewStudentImport)

			// Actual import - requires UsersCreate
			r.With(authorize.RequiresPermission("users:create")).Post("/import", rs.importStudents)
		})

		// Future: Teacher import endpoints
		// r.Route("/teachers", func(r chi.Router) {
		//     r.Get("/template", rs.downloadTeacherTemplate)
		//     r.Post("/preview", rs.previewTeacherImport)
		//     r.Post("/import", rs.importTeachers)
		// })
	})

	return r
}

// downloadStudentTemplate handles CSV template download
func (rs *Resource) downloadStudentTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=schueler-import-vorlage.csv")

	csvWriter := csv.NewWriter(w)

	// Header row with all supported columns
	headers := []string{
		"Vorname", "Nachname", "Klasse", "Gruppe", "Geburtstag", "RFID",
		"Erz1.Vorname", "Erz1.Nachname", "Erz1.Email", "Erz1.Telefon", "Erz1.Mobil", "Erz1.Verhältnis", "Erz1.Primär", "Erz1.Notfall", "Erz1.Abholung",
		"Erz2.Vorname", "Erz2.Nachname", "Erz2.Email", "Erz2.Telefon", "Erz2.Mobil", "Erz2.Verhältnis", "Erz2.Primär", "Erz2.Notfall", "Erz2.Abholung",
		"Gesundheitsinfo", "Betreuernotizen", "Zusatzinfo", "Datenschutz", "Aufbewahrung(Tage)", "Bus",
	}

	if err := csvWriter.Write(headers); err != nil {
		log.Printf("Error writing CSV headers: %v", err)
		http.Error(w, "Fehler beim Erstellen der Vorlage", http.StatusInternalServerError)
		return
	}

	// Example rows with realistic data
	examples := [][]string{
		{
			// Student info
			"Max", "Mustermann", "1A", "Gruppe 1A", "2015-08-15", "",
			// Guardian 1 (Mother)
			"Maria", "Müller", "maria.mueller@example.com", "0123-456789", "", "Mutter", "Ja", "Ja", "Ja",
			// Guardian 2 (Father)
			"Hans", "Müller", "hans.mueller@example.com", "0123-987654", "0176-12345678", "Vater", "Nein", "Ja", "Ja",
			// Additional info
			"", "Sehr ruhiges Kind", "", "Ja", "30", "Nein",
		},
		{
			// Student info
			"Anna", "Schmidt", "2B", "Gruppe 2B", "2014-03-22", "ABC123",
			// Guardian 1 (Mother) - only one guardian
			"Petra", "Schmidt", "petra.schmidt@example.com", "0234-567890", "", "Mutter", "Ja", "Ja", "Ja",
			// Guardian 2 (empty - optional!)
			"", "", "", "", "", "", "", "", "",
			// Additional info
			"Allergie: Nüsse", "", "Kann gut malen", "Ja", "15", "Ja",
		},
	}

	for _, row := range examples {
		if err := csvWriter.Write(row); err != nil {
			log.Printf("Error writing CSV row: %v", err)
		}
	}

	csvWriter.Flush()
}

// previewStudentImport handles import preview (dry-run)
func (rs *Resource) previewStudentImport(w http.ResponseWriter, r *http.Request) {
	// Security: File size limit
	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)

	// Parse multipart form
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("datei zu groß (max 10MB)"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get the file from the request
	file, header, err := r.FormFile("file")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("datei fehlt"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	// Validate file type
	allowedTypes := map[string]bool{
		"text/csv":                 true,
		"application/vnd.ms-excel": true,
		"text/plain":               true,
		"application/csv":          true,
	}
	contentType := header.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("ungültiger Dateityp (nur CSV erlaubt)"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse CSV
	rows, err := rs.csvParser.ParseStudents(file)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("CSV-Fehler: %s", err.Error()))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get user ID from context
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		render.Status(r, http.StatusUnauthorized)
		if err := render.Render(w, r, common.ErrorUnauthorized(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Run dry-run import (preview only, no database changes)
	ctx := r.Context()
	request := importModels.ImportRequest[importModels.StudentImportRow]{
		Rows:            rows,
		Mode:            importModels.ImportModeUpsert,
		DryRun:          true,  // PREVIEW ONLY
		StopOnError:     false, // Collect all errors
		UserID:          userID,
		SkipInvalidRows: false,
	}

	result, err := rs.studentImportService.Import(ctx, request)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(fmt.Errorf("vorschau fehlgeschlagen: %s", err.Error()))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
}

// importStudents handles actual student import
func (rs *Resource) importStudents(w http.ResponseWriter, r *http.Request) {
	// Security: File size limit
	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)

	// Parse multipart form
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("datei zu groß (max 10MB)"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get the file from the request
	file, header, err := r.FormFile("file")
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("datei fehlt"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	// Validate file type
	allowedTypes := map[string]bool{
		"text/csv":                 true,
		"application/vnd.ms-excel": true,
		"text/plain":               true,
		"application/csv":          true,
	}
	contentType := header.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("ungültiger Dateityp (nur CSV erlaubt)"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse CSV
	rows, err := rs.csvParser.ParseStudents(file)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		if err := render.Render(w, r, common.ErrorInvalidRequest(fmt.Errorf("CSV-Fehler: %s", err.Error()))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get user ID from context
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		render.Status(r, http.StatusUnauthorized)
		if err := render.Render(w, r, common.ErrorUnauthorized(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Run actual import
	ctx := r.Context()
	request := importModels.ImportRequest[importModels.StudentImportRow]{
		Rows:            rows,
		Mode:            importModels.ImportModeUpsert,
		DryRun:          false, // ACTUAL IMPORT
		StopOnError:     false, // Continue on errors
		UserID:          userID,
		SkipInvalidRows: true, // Skip invalid rows, import valid ones
	}

	result, err := rs.studentImportService.Import(ctx, request)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(fmt.Errorf("import fehlgeschlagen: %s", err.Error()))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Log import summary
	log.Printf("Student import completed: created=%d, updated=%d, errors=%d, filename=%s",
		result.CreatedCount, result.UpdatedCount, result.ErrorCount, header.Filename)

	// Build success message
	message := fmt.Sprintf("Import abgeschlossen: %d erstellt, %d aktualisiert, %d Fehler",
		result.CreatedCount, result.UpdatedCount, result.ErrorCount)

	common.Respond(w, r, http.StatusOK, result, message)
}

// getUserIDFromContext extracts the user ID from the JWT context
func getUserIDFromContext(ctx context.Context) (int64, error) {
	claims, ok := ctx.Value(jwt.CtxClaims).(jwt.AppClaims)
	if !ok {
		return 0, fmt.Errorf("no claims in context")
	}

	return int64(claims.ID), nil
}
