package importapi

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/uptrace/bun"
	"github.com/xuri/excelize/v2"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// getStaffImportHeaders returns the header row for the staff import template.
func getStaffImportHeaders() []string {
	return []string{"Vorname", "Nachname", "Email", "Rolle", "Position (optional)"}
}

// getStaffImportExamples returns example data rows for the staff template.
func getStaffImportExamples() [][]any {
	return [][]any{
		{"Anna", "Lehmann", "anna.lehmann@example.com", "Betreuer", "Klassenlehrerin"},
		{"Bernd", "Schulz", "bernd.schulz@example.com", "Administrator", ""},
	}
}

// DownloadStaffTemplate handles the staff template download (CSV or Excel).
func (rs *Resource) DownloadStaffTemplate(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("format") == "xlsx" {
		rs.downloadStaffTemplateXLSX(w, r)
		return
	}
	rs.downloadStaffTemplateCSV(w, r)
}

// downloadStaffTemplateCSV generates the staff CSV template.
func (rs *Resource) downloadStaffTemplateCSV(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=mitarbeiter-import-vorlage.csv")

	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write(getStaffImportHeaders()); err != nil {
		slog.Default().Error("Error writing CSV headers", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}
	for _, row := range getStaffImportExamples() {
		strRow := make([]string, len(row))
		for i, v := range row {
			strRow[i] = fmt.Sprintf("%v", v)
		}
		if err := csvWriter.Write(strRow); err != nil {
			slog.Default().Error("Error writing CSV row", slog.String("error", err.Error()))
		}
	}
	csvWriter.Flush()
}

// downloadStaffTemplateXLSX generates the staff Excel (.xlsx) template.
func (rs *Resource) downloadStaffTemplateXLSX(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=mitarbeiter-import-vorlage.xlsx")

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			slog.Default().Error("Error closing Excel file", slog.String("error", err.Error()))
		}
	}()

	sheetName := "Mitarbeiter"
	if err := setupExcelSheet(f, sheetName); err != nil {
		slog.Default().Error("Error setting up sheet", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}

	headers := getStaffImportHeaders()
	writeExcelHeaders(f, sheetName, headers)
	writeExcelExampleRows(f, sheetName, getStaffImportExamples())
	setExcelColumnWidths(f, sheetName, len(headers), 25)
	writeStaffHinweiseSheet(f)

	if err := f.Write(w); err != nil {
		slog.Default().Error("Error writing Excel file", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
	}
}

// writeStaffHinweiseSheet adds a "Hinweise" sheet describing the staff columns.
func writeStaffHinweiseSheet(f *excelize.File) {
	sheetName := "Hinweise"
	if _, err := f.NewSheet(sheetName); err != nil {
		slog.Default().Error("Error creating Hinweise sheet", slog.String("error", err.Error()))
		return
	}

	rows := [][]string{
		{"Spalte", "Pflicht?", "Beschreibung"},
		{"Vorname", "Ja", "Vorname des Mitarbeiters"},
		{"Nachname", "Ja", "Nachname des Mitarbeiters"},
		{"Email", "Ja", "E-Mail-Adresse — die Einladung wird an diese Adresse geschickt"},
		{"Rolle", "Ja", "Rolle des Mitarbeiters (muss exakt einer vorhandenen Rolle entsprechen)"},
		{"Position", "Nein", "Optionale Berufsbezeichnung (z.B. Klassenlehrerin)"},
		{"", "", ""},
		{"Hinweis", "", "Der Import legt Einladungen an. Mitarbeitende setzen ihr Passwort selbst über den Link in der E-Mail."},
		{"Hinweis", "", "Bereits in dieser Schule vorhandene Konten werden übersprungen."},
	}
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			_ = f.SetCellValue(sheetName, cell, val)
		}
	}
	_ = f.SetColWidth(sheetName, "A", "A", 14)
	_ = f.SetColWidth(sheetName, "B", "B", 10)
	_ = f.SetColWidth(sheetName, "C", "C", 70)
}

// PreviewStaffImport handles the staff import preview (dry-run).
func (rs *Resource) PreviewStaffImport(w http.ResponseWriter, r *http.Request) {
	uploadResult, ok := rs.validateAndParseStaffFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseStaffFile
	}

	accountID, err := getAccountIDFromContext(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	ctx := importService.ContextWithImporterPermissions(r.Context(), jwt.ClaimsFromCtx(r.Context()).Permissions)
	request := importModels.ImportRequest[importModels.StaffImportRow]{
		Rows:            uploadResult.Rows,
		Mode:            importModels.ImportModeCreate,
		DryRun:          true,
		StopOnError:     false,
		UserID:          accountID,
		SkipInvalidRows: false,
	}

	result, err := rs.staffImportService.Import(ctx, request)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("vorschau fehlgeschlagen: %s", err.Error())))
		return
	}

	// GDPR Compliance: Audit log for preview (Article 30). The route's tenant
	// transaction supplies the RLS context required by the audit repository.
	if err := rs.staffImportService.RecordAuditInTransaction(ctx, "staff", uploadResult.Filename, result, accountID, true, tenant.FromContext(ctx)); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("vorschau konnte nicht protokolliert werden: %w", err)))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
}

// ImportStaff handles the actual staff import (bulk invitation creation).
func (rs *Resource) ImportStaff(w http.ResponseWriter, r *http.Request) {
	uploadResult, ok := rs.validateAndParseStaffFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseStaffFile
	}

	accountID, err := getAccountIDFromContext(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	ctx := importService.ContextWithImporterPermissions(r.Context(), jwt.ClaimsFromCtx(r.Context()).Permissions)
	tenantID := tenant.FromContext(ctx)
	var result *importModels.ImportResult[importModels.StaffImportRow]
	if err := tenant.WithTenantTx(ctx, rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		request := importModels.ImportRequest[importModels.StaffImportRow]{
			Rows:            uploadResult.Rows,
			Mode:            importModels.ImportModeCreate,
			DryRun:          false,
			StopOnError:     false,
			UserID:          accountID,
			SkipInvalidRows: true,
		}

		var txErr error
		result, txErr = rs.staffImportService.Import(ctx, request)
		if txErr != nil {
			return txErr
		}
		// GDPR Compliance: Audit log for actual import (Article 30). Written
		// inside the import transaction so the import is only acknowledged
		// once its audit record is persisted.
		return rs.staffImportService.RecordAuditInTransaction(ctx, "staff", uploadResult.Filename, result, accountID, false, tenantID)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("import fehlgeschlagen: %s", err.Error())))
		return
	}

	slog.Default().Info("Staff import completed",
		slog.Int("created", result.CreatedCount),
		slog.Int("updated", result.UpdatedCount),
		slog.Int("errors", result.ErrorCount),
		slog.String("filename", uploadResult.Filename))

	message := fmt.Sprintf("Import abgeschlossen: %d erstellt, %d aktualisiert, %d Fehler",
		result.CreatedCount, result.UpdatedCount, result.ErrorCount)

	common.Respond(w, r, http.StatusOK, result, message)
}
