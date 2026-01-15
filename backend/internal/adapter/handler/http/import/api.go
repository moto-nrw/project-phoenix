package importapi

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/xuri/excelize/v2"

	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/audit"
	importModels "github.com/moto-nrw/project-phoenix/internal/core/domain/import"
	importService "github.com/moto-nrw/project-phoenix/internal/core/service/import"
)

const (
	maxFileSize = 10 * 1024 * 1024 // 10MB

	// Error messages (S1192 - avoid duplicate string literals)
	errTemplateCreation = "Fehler beim Erstellen der Vorlage"

	// Test data constants (S1192 - avoid duplicate string literals)
	testLastNameMueller = "Müller"
)

// Resource defines the import resource
type Resource struct {
	studentImportService *importService.ImportService[importModels.StudentImportRow]
}

// NewResource creates a new import resource
func NewResource(studentImportService *importService.ImportService[importModels.StudentImportRow]) *Resource {
	return &Resource{
		studentImportService: studentImportService,
	}
}

// Router returns a configured router for import endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

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
		logger.Logger.WithError(err).Error("Error writing CSV headers")
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}

	// Example rows with realistic data (RFID removed)
	examples := [][]string{
		{
			// Student info
			"Max", "Mustermann", "1A", "Gruppe 1A", "2015-08-15",
			// Guardian 1 (Mother)
			"Maria", testLastNameMueller, "maria.mueller@example.com", "0123-456789", "", "Mutter", "Ja", "Ja", "Ja",
			// Guardian 2 (Father)
			"Hans", testLastNameMueller, "hans.mueller@example.com", "0123-987654", "0176-12345678", "Vater", "Nein", "Ja", "Ja",
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
			logger.Logger.WithError(err).Warn("Error writing CSV row")
		}
	}

	csvWriter.Flush()
}

// downloadStudentTemplateXLSX generates Excel (.xlsx) template
func (rs *Resource) downloadStudentTemplateXLSX(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=schueler-import-vorlage.xlsx")

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			logger.Logger.WithError(err).Warn("Error closing Excel file")
		}
	}()

	sheetName := "Schüler"
	if err := setupExcelSheet(f, sheetName); err != nil {
		logger.Logger.WithError(err).Error("Error setting up sheet")
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}

	headers := getStudentImportHeaders()
	writeExcelHeaders(f, sheetName, headers)
	writeExcelExampleRows(f, sheetName, getStudentImportExamples())
	setExcelColumnWidths(f, sheetName, len(headers), 15)

	if err := f.Write(w); err != nil {
		logger.Logger.WithError(err).Error("Error writing Excel file")
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
	}
}

// setupExcelSheet creates the sheet and removes the default one
func setupExcelSheet(f *excelize.File, sheetName string) error {
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}
	_ = f.DeleteSheet("Sheet1") // Ignore error for default sheet deletion
	f.SetActiveSheet(index)
	return nil
}

// getStudentImportHeaders returns the header row for student import template
func getStudentImportHeaders() []string {
	return []string{
		"Vorname", "Nachname", "Klasse", "Gruppe", "Geburtstag",
		"Erz1.Vorname", "Erz1.Nachname", "Erz1.Email", "Erz1.Telefon", "Erz1.Mobil", "Erz1.Verhältnis", "Erz1.Primär", "Erz1.Notfall", "Erz1.Abholung",
		"Erz2.Vorname", "Erz2.Nachname", "Erz2.Email", "Erz2.Telefon", "Erz2.Mobil", "Erz2.Verhältnis", "Erz2.Primär", "Erz2.Notfall", "Erz2.Abholung",
		"Gesundheitsinfo", "Betreuernotizen", "Zusatzinfo", "Abholstatus", "Datenschutz", "Aufbewahrung(Tage)", "Bus",
	}
}

// getStudentImportExamples returns example data rows for the template
func getStudentImportExamples() [][]interface{} {
	return [][]interface{}{
		{"Max", "Mustermann", "1A", "Gruppe 1A", "2015-08-15",
			"Maria", testLastNameMueller, "maria.mueller@example.com", "0123-456789", "", "Mutter", "Ja", "Ja", "Ja",
			"Hans", testLastNameMueller, "hans.mueller@example.com", "0123-987654", "0176-12345678", "Vater", "Nein", "Ja", "Ja",
			"", "Sehr ruhiges Kind", "", "Wird abgeholt", "Ja", 30, "Nein"},
		{"Anna", "Schmidt", "2B", "Gruppe 2B", "2014-03-22",
			"Petra", "Schmidt", "petra.schmidt@example.com", "0234-567890", "", "Mutter", "Ja", "Ja", "Ja",
			"", "", "", "", "", "", "", "", "",
			"Allergie: Nüsse", "", "Kann gut malen", "Geht alleine nach Hause", "Ja", 15, "Ja"},
	}
}

// writeExcelHeaders writes headers to the first row
func writeExcelHeaders(f *excelize.File, sheetName string, headers []string) {
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheetName, cell, header); err != nil {
			logger.Logger.WithError(err).WithField("cell", cell).Warn("Error setting header")
		}
	}
}

// writeExcelExampleRows writes example data rows starting from row 2
func writeExcelExampleRows(f *excelize.File, sheetName string, examples [][]interface{}) {
	for rowIdx, row := range examples {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			if err := f.SetCellValue(sheetName, cell, value); err != nil {
				logger.Logger.WithError(err).WithField("cell", cell).Warn("Error setting cell value")
			}
		}
	}
}

// setExcelColumnWidths sets uniform column widths
func setExcelColumnWidths(f *excelize.File, sheetName string, numCols int, width float64) {
	for i := 1; i <= numCols; i++ {
		col, _ := excelize.ColumnNumberToName(i)
		if err := f.SetColWidth(sheetName, col, col, width); err != nil {
			logger.Logger.WithError(err).WithField("column", col).Warn("Error setting column width")
		}
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
		common.RenderError(w, r, common.ErrorUnauthorized(err))
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
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("vorschau fehlgeschlagen: %s", err.Error())))
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
		common.RenderError(w, r, common.ErrorUnauthorized(err))
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
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("import fehlgeschlagen: %s", err.Error())))
		return
	}

	// Log import summary
	logger.Logger.WithFields(map[string]interface{}{
		"created":  result.CreatedCount,
		"updated":  result.UpdatedCount,
		"errors":   result.ErrorCount,
		"filename": uploadResult.Filename,
	}).Info("Student import completed")

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
		if err := rs.studentImportService.CreateAuditRecord(auditCtx, auditRecord); err != nil {
			logEntry := logger.Logger.WithError(err).WithField("dry_run", dryRun)
			if dryRun {
				logEntry.Warn("Failed to create audit log for import preview")
			} else {
				logEntry.Error("Failed to create audit log for import")
			}
		}
	}()
}
