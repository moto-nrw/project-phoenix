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
	"github.com/xuri/excelize/v2"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/audit"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	importService "github.com/moto-nrw/project-phoenix/services/import"
)

const (
	maxFileSize = 10 * 1024 * 1024 // 10MB
)

// Resource defines the import resource
type Resource struct {
	studentImportService *importService.ImportService[importModels.StudentImportRow]
	auditRepo            audit.DataImportRepository
}

// NewResource creates a new import resource
func NewResource(studentImportService *importService.ImportService[importModels.StudentImportRow], auditRepo audit.DataImportRepository) *Resource {
	return &Resource{
		studentImportService: studentImportService,
		auditRepo:            auditRepo,
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

// downloadStudentTemplate handles template download (CSV or Excel)
func (rs *Resource) downloadStudentTemplate(w http.ResponseWriter, r *http.Request) {
	// Get format from query parameter (default: csv)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}

	if format == "xlsx" {
		rs.downloadStudentTemplateXLSX(w, r)
	} else {
		rs.downloadStudentTemplateCSV(w, r)
	}
}

// downloadStudentTemplateCSV generates CSV template
func (rs *Resource) downloadStudentTemplateCSV(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=schueler-import-vorlage.csv")

	csvWriter := csv.NewWriter(w)

	// Header row with all supported columns (RFID removed)
	headers := []string{
		"Vorname", "Nachname", "Klasse", "Gruppe", "Geburtstag",
		"Erz1.Vorname", "Erz1.Nachname", "Erz1.Email", "Erz1.Telefon", "Erz1.Mobil", "Erz1.Verhältnis", "Erz1.Primär", "Erz1.Notfall", "Erz1.Abholung",
		"Erz2.Vorname", "Erz2.Nachname", "Erz2.Email", "Erz2.Telefon", "Erz2.Mobil", "Erz2.Verhältnis", "Erz2.Primär", "Erz2.Notfall", "Erz2.Abholung",
		"Gesundheitsinfo", "Betreuernotizen", "Zusatzinfo", "Abholstatus", "Datenschutz", "Aufbewahrung(Tage)", "Bus",
	}

	if err := csvWriter.Write(headers); err != nil {
		log.Printf("Error writing CSV headers: %v", err)
		http.Error(w, "Fehler beim Erstellen der Vorlage", http.StatusInternalServerError)
		return
	}

	// Example rows with realistic data (RFID removed)
	examples := [][]string{
		{
			// Student info
			"Max", "Mustermann", "1A", "Gruppe 1A", "2015-08-15",
			// Guardian 1 (Mother)
			"Maria", "Müller", "maria.mueller@example.com", "0123-456789", "", "Mutter", "Ja", "Ja", "Ja",
			// Guardian 2 (Father)
			"Hans", "Müller", "hans.mueller@example.com", "0123-987654", "0176-12345678", "Vater", "Nein", "Ja", "Ja",
			// Additional info
			"", "Sehr ruhiges Kind", "", "Wird abgeholt", "Ja", "30", "Nein",
		},
		{
			// Student info
			"Anna", "Schmidt", "2B", "Gruppe 2B", "2014-03-22",
			// Guardian 1 (Mother) - only one guardian
			"Petra", "Schmidt", "petra.schmidt@example.com", "0234-567890", "", "Mutter", "Ja", "Ja", "Ja",
			// Guardian 2 (empty - optional!)
			"", "", "", "", "", "", "", "", "",
			// Additional info
			"Allergie: Nüsse", "", "Kann gut malen", "Geht alleine nach Hause", "Ja", "15", "Ja",
		},
	}

	for _, row := range examples {
		if err := csvWriter.Write(row); err != nil {
			log.Printf("Error writing CSV row: %v", err)
		}
	}

	csvWriter.Flush()
}

// downloadStudentTemplateXLSX generates Excel (.xlsx) template
func (rs *Resource) downloadStudentTemplateXLSX(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=schueler-import-vorlage.xlsx")

	// Create a new Excel file
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Error closing Excel file: %v", err)
		}
	}()

	sheetName := "Schüler"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		log.Printf("Error creating sheet: %v", err)
		http.Error(w, "Fehler beim Erstellen der Vorlage", http.StatusInternalServerError)
		return
	}

	// Delete default sheet
	if err := f.DeleteSheet("Sheet1"); err != nil {
		log.Printf("Error deleting default sheet: %v", err)
	}
	f.SetActiveSheet(index)

	// Header row with all supported columns (RFID removed)
	headers := []string{
		"Vorname", "Nachname", "Klasse", "Gruppe", "Geburtstag",
		"Erz1.Vorname", "Erz1.Nachname", "Erz1.Email", "Erz1.Telefon", "Erz1.Mobil", "Erz1.Verhältnis", "Erz1.Primär", "Erz1.Notfall", "Erz1.Abholung",
		"Erz2.Vorname", "Erz2.Nachname", "Erz2.Email", "Erz2.Telefon", "Erz2.Mobil", "Erz2.Verhältnis", "Erz2.Primär", "Erz2.Notfall", "Erz2.Abholung",
		"Gesundheitsinfo", "Betreuernotizen", "Zusatzinfo", "Abholstatus", "Datenschutz", "Aufbewahrung(Tage)", "Bus",
	}

	// Write headers
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheetName, cell, header); err != nil {
			log.Printf("Error setting header: %v", err)
		}
	}

	// Example rows with realistic data (RFID removed)
	examples := [][]interface{}{
		{
			// Student info
			"Max", "Mustermann", "1A", "Gruppe 1A", "2015-08-15",
			// Guardian 1 (Mother)
			"Maria", "Müller", "maria.mueller@example.com", "0123-456789", "", "Mutter", "Ja", "Ja", "Ja",
			// Guardian 2 (Father)
			"Hans", "Müller", "hans.mueller@example.com", "0123-987654", "0176-12345678", "Vater", "Nein", "Ja", "Ja",
			// Additional info
			"", "Sehr ruhiges Kind", "", "Wird abgeholt", "Ja", 30, "Nein",
		},
		{
			// Student info
			"Anna", "Schmidt", "2B", "Gruppe 2B", "2014-03-22",
			// Guardian 1 (Mother) - only one guardian
			"Petra", "Schmidt", "petra.schmidt@example.com", "0234-567890", "", "Mutter", "Ja", "Ja", "Ja",
			// Guardian 2 (empty - optional!)
			"", "", "", "", "", "", "", "", "",
			// Additional info
			"Allergie: Nüsse", "", "Kann gut malen", "Geht alleine nach Hause", "Ja", 15, "Ja",
		},
	}

	// Write example rows
	for rowIdx, row := range examples {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			if err := f.SetCellValue(sheetName, cell, value); err != nil {
				log.Printf("Error setting cell value: %v", err)
			}
		}
	}

	// Auto-fit column widths
	for i := 1; i <= len(headers); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		if err := f.SetColWidth(sheetName, col, col, 15); err != nil {
			log.Printf("Error setting column width: %v", err)
		}
	}

	// Write to response
	if err := f.Write(w); err != nil {
		log.Printf("Error writing Excel file: %v", err)
		http.Error(w, "Fehler beim Erstellen der Vorlage", http.StatusInternalServerError)
	}
}

// previewStudentImport handles import preview (dry-run)
func (rs *Resource) previewStudentImport(w http.ResponseWriter, r *http.Request) {
	// Validate and parse CSV file
	uploadResult, ok := rs.validateAndParseCSVFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseCSVFile
	}

	// Get user ID from context
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		render.Status(r, http.StatusUnauthorized)
		if err := render.Render(w, r, common.ErrorUnauthorized(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Run dry-run import (preview only, no database changes)
	ctx := r.Context()
	request := importModels.ImportRequest[importModels.StudentImportRow]{
		Rows:            uploadResult.Rows,
		Mode:            importModels.ImportModeCreate, // Create-only: duplicates will error
		DryRun:          true,                          // PREVIEW ONLY
		StopOnError:     false,                         // Collect all errors
		UserID:          userID,
		SkipInvalidRows: false,
	}

	result, err := rs.studentImportService.Import(ctx, request)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(fmt.Errorf("vorschau fehlgeschlagen: %s", err.Error()))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// GDPR Compliance: Audit log for preview (Article 30)
	rs.logImportAudit(uploadResult.Filename, result, userID, true)

	common.Respond(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
}

// importStudents handles actual student import
func (rs *Resource) importStudents(w http.ResponseWriter, r *http.Request) {
	// Validate and parse CSV file
	uploadResult, ok := rs.validateAndParseCSVFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseCSVFile
	}

	// Get user ID from context
	userID, err := getUserIDFromContext(r.Context())
	if err != nil {
		render.Status(r, http.StatusUnauthorized)
		if err := render.Render(w, r, common.ErrorUnauthorized(err)); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Run actual import
	ctx := r.Context()
	request := importModels.ImportRequest[importModels.StudentImportRow]{
		Rows:            uploadResult.Rows,
		Mode:            importModels.ImportModeCreate, // Create-only: duplicates will error
		DryRun:          false,                         // ACTUAL IMPORT
		StopOnError:     false,                         // Continue on errors
		UserID:          userID,
		SkipInvalidRows: true, // Skip invalid rows, import valid ones
	}

	result, err := rs.studentImportService.Import(ctx, request)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		if err := render.Render(w, r, common.ErrorInternalServer(fmt.Errorf("import fehlgeschlagen: %s", err.Error()))); err != nil {
			log.Printf(common.LogRenderError, err)
		}
		return
	}

	// Log import summary (using %q to prevent log injection via filename)
	log.Printf("Student import completed: created=%d, updated=%d, errors=%d, filename=%q",
		result.CreatedCount, result.UpdatedCount, result.ErrorCount, uploadResult.Filename)

	// GDPR Compliance: Audit log for actual import (Article 30)
	rs.logImportAudit(uploadResult.Filename, result, userID, false)

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

// logImportAudit creates an audit record for import operations (GDPR compliance)
func (rs *Resource) logImportAudit(filename string, result *importModels.ImportResult[importModels.StudentImportRow], userID int64, dryRun bool) {
	go func() {
		auditCtx := context.Background()
		auditRecord := &audit.DataImport{
			EntityType:   "student",
			Filename:     filename,
			TotalRows:    result.TotalRows,
			CreatedCount: result.CreatedCount,
			UpdatedCount: result.UpdatedCount,
			SkippedCount: 0, // Not tracked separately
			ErrorCount:   result.ErrorCount,
			WarningCount: result.WarningCount,
			DryRun:       dryRun,
			ImportedBy:   userID,
			StartedAt:    result.StartedAt,
			CompletedAt:  &result.CompletedAt,
			Metadata:     audit.JSONBMap{},
		}
		if err := rs.auditRepo.Create(auditCtx, auditRecord); err != nil {
			logLevel := "WARNING"
			if !dryRun {
				logLevel = "ERROR"
			}
			log.Printf("%s: Failed to create audit log for import: %v", logLevel, err)
		}
	}()
}
