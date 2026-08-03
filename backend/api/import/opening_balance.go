package importapi

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/uptrace/bun"
	"github.com/xuri/excelize/v2"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// getOpeningBalanceHeaders returns the header row for the opening balance
// import template (#2132). Header keys must normalize to the columns
// MapOpeningBalanceRow reads — units belong in the Hinweise sheet, not here.
func getOpeningBalanceHeaders() []string {
	return []string{"Personalnummer (optional)", "Vorname", "Nachname", "Stundensaldo", "Jahresanspruch", "Vorjahresübertrag", "Resturlaub"}
}

// getOpeningBalanceExamples returns example data rows for the template.
func getOpeningBalanceExamples() [][]any {
	return [][]any{
		{"1001", "Anna", "Lehmann", "12,5", "30", "2", "14,5"},
		{"", "Bernd", "Schulz", "-3,25", "", "", "8"},
		{"1003", "Clara", "Weber", "", "28", "0", "28"},
	}
}

// DownloadOpeningBalanceTemplate handles the template download (CSV or Excel).
func (rs *Resource) DownloadOpeningBalanceTemplate(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("format") == "xlsx" {
		rs.downloadOpeningBalanceTemplateXLSX(w, r)
		return
	}
	rs.downloadOpeningBalanceTemplateCSV(w, r)
}

func (rs *Resource) downloadOpeningBalanceTemplateCSV(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=eroeffnungssalden-import-vorlage.csv")

	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write(getOpeningBalanceHeaders()); err != nil {
		slog.Default().Error("Error writing CSV headers", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}
	for _, row := range getOpeningBalanceExamples() {
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

func (rs *Resource) downloadOpeningBalanceTemplateXLSX(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=eroeffnungssalden-import-vorlage.xlsx")

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			slog.Default().Error("Error closing Excel file", slog.String("error", err.Error()))
		}
	}()

	sheetName := "Eröffnungssalden"
	if err := setupExcelSheet(f, sheetName); err != nil {
		slog.Default().Error("Error setting up sheet", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}

	headers := getOpeningBalanceHeaders()
	writeExcelHeaders(f, sheetName, headers)
	writeExcelExampleRows(f, sheetName, getOpeningBalanceExamples())
	setExcelColumnWidths(f, sheetName, len(headers), 20)
	writeOpeningBalanceHinweiseSheet(f)

	if err := f.Write(w); err != nil {
		slog.Default().Error("Error writing Excel file", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
	}
}

// writeOpeningBalanceHinweiseSheet adds a "Hinweise" sheet describing the columns.
func writeOpeningBalanceHinweiseSheet(f *excelize.File) {
	sheetName := "Hinweise"
	if _, err := f.NewSheet(sheetName); err != nil {
		slog.Default().Error("Error creating Hinweise sheet", slog.String("error", err.Error()))
		return
	}

	rows := [][]string{
		{"Spalte", "Pflicht?", "Beschreibung"},
		{"Personalnummer", "Nein", "Eindeutige Zuordnung — empfohlen, wenn Namen mehrfach vorkommen"},
		{"Vorname", "Ja", "Vorname der Person (muss bereits in moto angelegt sein)"},
		{"Nachname", "Ja", "Nachname der Person"},
		{"Stundensaldo", "Nein", "Stundenkonto-Stand zum Stichtag in Stunden, auch negativ (z. B. 12,5 oder -3,25). Leer = keine Übernahme"},
		{"Jahresanspruch", "Nein", "Urlaubsanspruch des laufenden Jahres in Tagen. Leer = unverändert"},
		{"Vorjahresübertrag", "Nein", "Übertrag aus dem Vorjahr in Tagen. Leer = unverändert"},
		{"Resturlaub", "Nein", "Resturlaub zum Stichtag in Tagen. moto errechnet daraus die vor der Einführung genommenen Tage. Leer = keine Übernahme"},
		{"", "", ""},
		{"Hinweis", "", "Stichtag und Begründung werden beim Hochladen einmal für die ganze Datei angegeben."},
		{"Hinweis", "", "Pro Person ist nur eine Übernahme möglich. Korrekturen laufen über Löschen und Neuanlegen in der Personalakte."},
	}
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			_ = f.SetCellValue(sheetName, cell, val)
		}
	}
	_ = f.SetColWidth(sheetName, "A", "A", 18)
	_ = f.SetColWidth(sheetName, "B", "B", 10)
	_ = f.SetColWidth(sheetName, "C", "C", 90)
}

// openingBalanceRequestContext bundles the per-request import parameters
// parsed from the upload form.
type openingBalanceRequestContext struct {
	Rows          []importModels.OpeningBalanceImportRow
	Filename      string
	EffectiveDate timezone.Date
	Note          string
	DecidedBy     int64
	AccountID     int64
}

// parseOpeningBalanceRequest validates the upload, parses the file, and reads
// the shared Stichtag + Begründung form fields.
func (rs *Resource) parseOpeningBalanceRequest(w http.ResponseWriter, r *http.Request) (*openingBalanceRequestContext, bool) {
	file, header, isExcel, ok := rs.openValidatedUploadFile(w, r)
	if !ok {
		return nil, false
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Default().Error("failed to close file", slog.String("error", err.Error()))
		}
	}()

	var rows []importModels.OpeningBalanceImportRow
	var err error
	if isExcel {
		rows, err = importService.ParseOpeningBalanceXLSX(file)
	} else {
		rows, err = importService.ParseOpeningBalanceCSV(file)
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("Datei-Fehler: %s", err.Error())))
		return nil, false
	}

	effectiveDate, err := timezone.ParseDate(r.FormValue("effective_date"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("ungültiger Stichtag (erwartet JJJJ-MM-TT)")))
		return nil, false
	}
	note := strings.TrimSpace(r.FormValue("note"))
	if note == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("Begründung ist erforderlich")))
		return nil, false
	}

	accountID, err := getAccountIDFromContext(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return nil, false
	}
	decidedBy, err := rs.personService.ResolveStaffIDByAccountID(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(fmt.Errorf("kein Mitarbeiterprofil für dieses Konto")))
		return nil, false
	}

	return &openingBalanceRequestContext{
		Rows:          rows,
		Filename:      header.Filename,
		EffectiveDate: effectiveDate,
		Note:          note,
		DecidedBy:     decidedBy,
		AccountID:     accountID,
	}, true
}

// PreviewOpeningBalanceImport handles the dry-run preview.
func (rs *Resource) PreviewOpeningBalanceImport(w http.ResponseWriter, r *http.Request) {
	if rs.openingBalanceImportFactory == nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("opening balance import is not configured")))
		return
	}
	reqCtx, ok := rs.parseOpeningBalanceRequest(w, r)
	if !ok {
		return
	}

	svc := rs.openingBalanceImportFactory(reqCtx.EffectiveDate, reqCtx.Note, reqCtx.DecidedBy)
	result, err := svc.Import(r.Context(), importModels.ImportRequest[importModels.OpeningBalanceImportRow]{
		Rows:            reqCtx.Rows,
		Mode:            importModels.ImportModeCreate,
		DryRun:          true,
		StopOnError:     false,
		UserID:          reqCtx.AccountID,
		SkipInvalidRows: false,
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("vorschau fehlgeschlagen: %s", err.Error())))
		return
	}

	// GDPR Compliance: Audit log for preview (Article 30)
	svc.RecordAudit("opening_balance", reqCtx.Filename, result, reqCtx.AccountID, true, tenant.FromContext(r.Context()))

	common.Respond(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
}

// ImportOpeningBalances handles the actual import. The handler owns its
// tenant transaction so partial failures roll back per the import result.
func (rs *Resource) ImportOpeningBalances(w http.ResponseWriter, r *http.Request) {
	if rs.openingBalanceImportFactory == nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("opening balance import is not configured")))
		return
	}
	reqCtx, ok := rs.parseOpeningBalanceRequest(w, r)
	if !ok {
		return
	}

	svc := rs.openingBalanceImportFactory(reqCtx.EffectiveDate, reqCtx.Note, reqCtx.DecidedBy)
	tenantID := tenant.FromContext(r.Context())
	var result *importModels.ImportResult[importModels.OpeningBalanceImportRow]
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		result, txErr = svc.Import(ctx, importModels.ImportRequest[importModels.OpeningBalanceImportRow]{
			Rows:            reqCtx.Rows,
			Mode:            importModels.ImportModeCreate,
			DryRun:          false,
			StopOnError:     false,
			UserID:          reqCtx.AccountID,
			SkipInvalidRows: true,
		})
		return txErr
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("import fehlgeschlagen: %s", err.Error())))
		return
	}

	slog.Default().Info("Opening balance import completed",
		slog.Int("created", result.CreatedCount),
		slog.Int("errors", result.ErrorCount),
		slog.String("filename", reqCtx.Filename))

	// GDPR Compliance: Audit log for actual import (Article 30)
	svc.RecordAudit("opening_balance", reqCtx.Filename, result, reqCtx.AccountID, false, tenantID)

	message := fmt.Sprintf("Import abgeschlossen: %d übernommen, %d Fehler",
		result.CreatedCount, result.ErrorCount)

	common.Respond(w, r, http.StatusOK, result, message)
}
