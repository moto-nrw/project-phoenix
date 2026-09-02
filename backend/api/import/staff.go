package importapi

import (
	"context"
	"encoding/csv"
	"errors"
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
// Everything after Rolle is a Stammdaten column (#2600); the parser matches on
// the name without the "(optional)" annotation.
func getStaffImportHeaders() []string {
	return []string{
		"Vorname", "Nachname", "Rolle", "Email (optional)", "Position (optional)",
		"Personalnummer (optional)", "Geburtstag (optional)", "Geschlecht (optional)",
		"Beschäftigungsart (optional)", "Wochenstunden (optional)", "Eintritt (optional)", "Vertragsende (optional)", "Probezeit bis (optional)",
		"Straße (optional)", "PLZ (optional)", "Ort (optional)", "Telefon (optional)", "Kontakt-Email (optional)",
		"Notfallkontakt (optional)", "Notfallkontakt Telefon (optional)",
		"Qualifikationen (optional)", "Notizen (optional)",
	}
}

// getStaffImportExamples returns example data rows for the staff template.
func getStaffImportExamples() [][]any {
	return [][]any{
		{"Anna", "Lehmann", "Betreuer", "anna.lehmann@example.com", "Gruppenleitung",
			"P-1001", "12.05.1988", "w",
			"Teilzeit", "19,5", "01.08.2023", "", "31.01.2024",
			testAddressMusterstr, "50667", "Köln", "0221-1234567", "",
			"Peter Lehmann", "0171-9876543",
			"Erste Hilfe (01.03.2024 bis 01.03.2026); Schwimmschein",
			""},
		{"Bernd", "Schulz", "Administrator", "bernd.schulz@example.com", "",
			"P-1002", "", "m",
			"Vollzeit", "39", "01.02.2020", "", "",
			"", "", "", "", "",
			"", "",
			"",
			""},
		{"Cem", "Yilmaz", "Betreuer", "", "Honorarkraft",
			"", "", "",
			"Minijob", "8", "01.09.2026", "31.07.2027", "",
			"", "", "", "0176-5550001", "cem@example.com",
			"", "",
			"",
			"Kein Portalzugang gewünscht"},
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
		{"Vorname", "Ja", "Vorname der Person"},
		{"Nachname", "Ja", "Nachname der Person"},
		{"Rolle", "Ja", "Rolle im System (muss exakt einer vorhandenen Rolle entsprechen)"},
		{"Email", "Nein", "Login-Adresse. Wenn angegeben, wird eine Einladung an diese Adresse geschickt; die Person setzt ihr Passwort selbst. Leer = Datensatz ohne Zugang."},
		{"Position", "Nein", "Berufsbezeichnung (z.B. Gruppenleitung)"},
		{"Personalnummer", "Nein", "Personalnummer der Lohnabrechnung; eindeutig pro Schule. Dient beim Aktualisieren als Erkennungsmerkmal."},
		{"Geburtstag", "Nein", "JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ"},
		{"Geschlecht", "Nein", "w, m oder d (weiblich, männlich, divers)"},
		{"Beschäftigungsart", "Nein", "Vollzeit, Teilzeit oder Minijob"},
		{"Wochenstunden", "Nein", "Zahl zwischen 0 und 80, z.B. 39 oder 19,5"},
		{"Eintritt", "Nein", "Beginn des Arbeitsverhältnisses (Datum)"},
		{"Vertragsende", "Nein", "Ende des befristeten Vertrags (Datum); nicht vor Eintritt"},
		{"Probezeit bis", "Nein", "Ende der Probezeit (Datum); nicht vor Eintritt"},
		{"Straße / PLZ / Ort", "Nein", "Privatanschrift"},
		{"Telefon", "Nein", "Private Telefonnummer, z.B. 0221-1234567 oder +49 221 1234567"},
		{"Kontakt-Email", "Nein", "Kontaktadresse für die Personalakte (unabhängig vom Login)"},
		{"Notfallkontakt", "Nein", "Name der Person, die im Notfall zu verständigen ist"},
		{"Notfallkontakt Telefon", "Nein", "Telefonnummer des Notfallkontakts"},
		{"Qualifikationen", "Nein", "Mit Semikolon getrennt, optional mit Datum: Erste Hilfe (01.03.2024 bis 01.03.2026); Schwimmschein"},
		{"Notizen", "Nein", "Interne Notizen zur Person"},
		{"", "", ""},
		{"Hinweis", "", "Der Import legt die Person sofort in der Personalliste an. Mit E-Mail wird zusätzlich eine Einladung verschickt; beim Annehmen wird das Konto mit diesem Datensatz verknüpft."},
		{"Hinweis", "", "Bankdaten (IBAN, Steuer-ID, SV-Nummer) werden bewusst nicht importiert. Sie werden im Mitarbeiterprofil gepflegt."},
		{"Hinweis", "", "Modus 'Bestehende aktualisieren' oder 'Beides': Personen werden über Personalnummer, sonst E-Mail, sonst Vor- und Nachname erkannt. Leere Zellen ändern nichts. Rolle und Zugang werden dabei nicht verändert."},
	}
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
			_ = f.SetCellValue(sheetName, cell, val)
		}
	}
	_ = f.SetColWidth(sheetName, "A", "A", 22)
	_ = f.SetColWidth(sheetName, "B", "B", 10)
	_ = f.SetColWidth(sheetName, "C", "C", 90)
}

// PreviewStaffImport handles the staff import preview (dry-run).
func (rs *Resource) PreviewStaffImport(w http.ResponseWriter, r *http.Request) {
	uploadResult, ok := rs.validateAndParseStaffFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseStaffFile
	}
	mode, ok := importModeFromRequest(w, r)
	if !ok {
		return
	}

	accountID, err := getAccountIDFromContext(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// The preview owns its tenant transaction (no route-level withTx): the
	// GDPR audit row must be committed before the success response is
	// written — a middleware transaction commits only after the handler
	// returns, when a commit failure can no longer be reported.
	ctx := importService.ContextWithImporterPermissions(r.Context(), jwt.ClaimsFromCtx(r.Context()).Permissions)
	tenantID := tenant.FromContext(ctx)
	var result *importModels.ImportResult[importModels.StaffImportRow]
	if err := tenant.WithTenantTx(ctx, rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		request := importModels.ImportRequest[importModels.StaffImportRow]{
			Rows:            uploadResult.Rows,
			Mode:            mode,
			DryRun:          true,
			StopOnError:     false,
			UserID:          accountID,
			SkipInvalidRows: false,
		}

		var txErr error
		result, txErr = rs.staffImportService.Import(ctx, request)
		if txErr != nil {
			return txErr
		}
		// GDPR Compliance: Audit log for preview (Article 30).
		return rs.staffImportService.RecordAuditInTransaction(ctx, "staff", uploadResult.Filename, result, accountID, true, tenantID)
	}); err != nil {
		renderStaffImportError(w, r, err, "Import-Vorschau fehlgeschlagen")
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
}

// msgStaffImportModeForbidden is shown when a create-only importer asks for
// update or upsert mode (#2906).
const msgStaffImportModeForbidden = "Bestehende Mitarbeiter ändern geht mit Ihren Berechtigungen nicht. Wählen Sie „Nur neue anlegen“ oder fragen Sie die Leitung."

// renderStaffImportError maps a refused import mode to 403 and everything
// else to a 500 with the given client message.
func renderStaffImportError(w http.ResponseWriter, r *http.Request, err error, clientMsg string) {
	if errors.Is(err, importService.ErrImportModeForbidden) {
		common.RenderError(w, r, common.ErrorForbiddenMessage(msgStaffImportModeForbidden))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServerWrap(clientMsg, err))
}

// ImportStaff handles the actual staff import (Stammdatensätze plus optional invitations).
func (rs *Resource) ImportStaff(w http.ResponseWriter, r *http.Request) {
	uploadResult, ok := rs.validateAndParseStaffFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseStaffFile
	}
	mode, ok := importModeFromRequest(w, r)
	if !ok {
		return
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
			Mode:            mode,
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
		renderStaffImportError(w, r, err, "Import fehlgeschlagen")
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
