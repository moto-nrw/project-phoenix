package importapi

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
	"github.com/uptrace/bun"
	"github.com/xuri/excelize/v2"

	"github.com/moto-nrw/project-phoenix/api/common"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Class-list entry import (#2382): the bulk form of the minimal
// Klassenlisteneintrag — Vorname, Nachname, Klasse, nothing else.

// getClassListImportHeaders returns the header row for the template.
func getClassListImportHeaders() []string {
	return []string{"Vorname", "Nachname", "Klasse"}
}

// getClassListImportExamples returns example data rows for the template.
func getClassListImportExamples() [][]any {
	return [][]any{
		{"Lena", "Beispiel", "1a"},
		{"Jonas", "Muster", "3b"},
	}
}

// ClassListFileUploadResult carries the parsed rows plus the original
// filename for the audit trail.
type ClassListFileUploadResult struct {
	Rows     []importModels.ClassListEntryImportRow
	Filename string
}

// validateAndParseClassListFile handles upload validation and parsing for
// class-list entry imports. Supports both CSV and Excel (.xlsx) files.
func (rs *Resource) validateAndParseClassListFile(w http.ResponseWriter, r *http.Request) (*ClassListFileUploadResult, bool) {
	file, header, isExcel, ok := rs.openValidatedUploadFile(w, r)
	if !ok {
		return nil, false
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Default().Error("failed to close file", slog.String("error", err.Error()))
		}
	}()

	var rows []importModels.ClassListEntryImportRow
	var err error
	if isExcel {
		rows, err = importService.ParseClassListXLSX(file)
	} else {
		rows, err = importService.ParseClassListCSV(file)
	}
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("Datei-Fehler: %s", err.Error())))
		return nil, false
	}

	return &ClassListFileUploadResult{
		Rows:     rows,
		Filename: header.Filename,
	}, true
}

// DownloadClassListTemplate handles the template download (CSV or Excel).
func (rs *Resource) DownloadClassListTemplate(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("format") == "xlsx" {
		rs.downloadClassListTemplateXLSX(w, r)
		return
	}
	rs.downloadClassListTemplateCSV(w, r)
}

func (rs *Resource) downloadClassListTemplateCSV(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=klassenliste-import-vorlage.csv")

	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write(getClassListImportHeaders()); err != nil {
		slog.Default().Error("Error writing CSV headers", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}
	for _, row := range getClassListImportExamples() {
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

func (rs *Resource) downloadClassListTemplateXLSX(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=klassenliste-import-vorlage.xlsx")

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			slog.Default().Error("Error closing Excel file", slog.String("error", err.Error()))
		}
	}()

	sheetName := "Klassenliste"
	if err := setupExcelSheet(f, sheetName); err != nil {
		slog.Default().Error("Error setting up sheet", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}

	headers := getClassListImportHeaders()
	writeExcelHeaders(f, sheetName, headers)
	writeExcelExampleRows(f, sheetName, getClassListImportExamples())
	setExcelColumnWidths(f, sheetName, len(headers), 20)
	writeClassListHinweiseSheet(f)

	if err := f.Write(w); err != nil {
		slog.Default().Error("Error writing Excel file", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
	}
}

// writeClassListHinweiseSheet adds a "Hinweise" sheet describing the columns.
func writeClassListHinweiseSheet(f *excelize.File) {
	sheetName := "Hinweise"
	if _, err := f.NewSheet(sheetName); err != nil {
		slog.Default().Error("Error creating Hinweise sheet", slog.String("error", err.Error()))
		return
	}

	rows := [][]string{
		{"Spalte", "Pflicht?", "Beschreibung"},
		{"Vorname", "Ja", "Vorname des Kindes"},
		{"Nachname", "Ja", "Nachname des Kindes"},
		{"Klasse", "Ja", "Schulklasse (z.B. 1a) — wie bei den regulären Kindern geschrieben"},
		{"", "", ""},
		{"Hinweis", "", "Klassenlisteneinträge sind Kinder OHNE OGS-Betreuung: Sie erscheinen nur auf Klassenlisten und in der Klassenansicht, nie in Anwesenheit oder Betreuungsplanung."},
		{"Hinweis", "", "Kinder, die bereits in moto angelegt sind, werden übersprungen — sie stehen schon auf der Klassenliste."},
	}
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			_ = f.SetCellValue(sheetName, cell, val)
		}
	}
	_ = f.SetColWidth(sheetName, "A", "A", 14)
	_ = f.SetColWidth(sheetName, "B", "B", 10)
	_ = f.SetColWidth(sheetName, "C", "C", 80)
}

// PreviewClassListImport handles the class-list import preview (dry-run).
func (rs *Resource) PreviewClassListImport(w http.ResponseWriter, r *http.Request) {
	rs.runClassListImport(w, r, true)
}

// ImportClassList handles the actual class-list entry import.
func (rs *Resource) ImportClassList(w http.ResponseWriter, r *http.Request) {
	rs.runClassListImport(w, r, false)
}

// runClassListImport shares the preview/import flow: both own their tenant
// transaction so the GDPR audit row is committed before the success response
// (same contract as the staff import).
func (rs *Resource) runClassListImport(w http.ResponseWriter, r *http.Request, dryRun bool) {
	uploadResult, ok := rs.validateAndParseClassListFile(w, r)
	if !ok {
		return
	}

	accountID, err := getAccountIDFromContext(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	var result *importModels.ImportResult[importModels.ClassListEntryImportRow]
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		request := importModels.ImportRequest[importModels.ClassListEntryImportRow]{
			Rows:            uploadResult.Rows,
			Mode:            importModels.ImportModeCreate,
			DryRun:          dryRun,
			StopOnError:     false,
			UserID:          accountID,
			SkipInvalidRows: !dryRun,
		}

		var txErr error
		result, txErr = rs.classListImportService.Import(ctx, request)
		if txErr != nil {
			return txErr
		}
		// GDPR Compliance: audit log for preview and import (Article 30).
		return rs.classListImportService.RecordAuditInTransaction(ctx, "class_list_entries", uploadResult.Filename, result, accountID, dryRun, tenantID)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Import fehlgeschlagen", err))
		return
	}

	if dryRun {
		common.Respond(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
		return
	}

	slog.Default().Info("Class list entry import completed",
		slog.Int("created", result.CreatedCount),
		slog.Int("errors", result.ErrorCount),
		slog.String("filename", uploadResult.Filename))

	message := fmt.Sprintf("Import abgeschlossen: %d erstellt, %d Fehler",
		result.CreatedCount, result.ErrorCount)

	common.Respond(w, r, http.StatusOK, result, message)
}
