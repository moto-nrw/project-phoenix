package importapi

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/xuri/excelize/v2"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	userSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	maxFileSize = 10 * 1024 * 1024 // 10MB

	// Error messages (S1192 - avoid duplicate string literals)
	errTemplateCreation = "Fehler beim Erstellen der Vorlage"

	// Test data constants (S1192 - avoid duplicate string literals)
	testLastNameMueller  = "Müller"
	testAddressMusterstr = "Musterstr. 1"
	hintYesNo            = "Ja / Nein"

	// Route paths shared by every import domain (S1192)
	routeTemplate = "/template"
	routePreview  = "/preview"
	routeImport   = "/import"

	// Permissions (S1192)
	permUsersRead          = "users:read"
	permUsersCreate        = "users:create"
	permTimeTrackingManage = "time_tracking:manage"
)

// Resource defines the import resource
type Resource struct {
	studentImportService *importService.ImportService[importModels.StudentImportRow]
	staffImportService   *importService.ImportService[importModels.StaffImportRow]
	// openingBalanceImportFactory builds a request-scoped opening balance
	// import service (#2132) — Stichtag/Begründung/actor come from the
	// upload form. Wired via SetOpeningBalanceImportFactory.
	openingBalanceImportFactory importService.OpeningBalanceImportFactory
	personService               userSvc.PersonService
	db                          *bun.DB
}

// SetOpeningBalanceImportFactory wires the opening balance import (#2132).
// Setter injection so existing NewResource call sites stay unchanged.
func (rs *Resource) SetOpeningBalanceImportFactory(factory importService.OpeningBalanceImportFactory) {
	rs.openingBalanceImportFactory = factory
}

// NewResource creates a new import resource
func NewResource(
	studentImportService *importService.ImportService[importModels.StudentImportRow],
	staffImportService *importService.ImportService[importModels.StaffImportRow],
	personService userSvc.PersonService,
	db *bun.DB,
) *Resource {
	return &Resource{
		studentImportService: studentImportService,
		staffImportService:   staffImportService,
		personService:        personService,
		db:                   db,
	}
}

// Router returns a configured router for import endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustNewTokenAuth()

	// Protected routes - require UsersCreate permission
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		// Student import endpoints
		r.Route("/students", func(r chi.Router) {
			// Template download - requires UsersRead
			r.With(authorize.RequiresPermission(permUsersRead), withTx).Get(routeTemplate, rs.downloadStudentTemplate)

			// Preview - requires UsersCreate
			r.With(authorize.RequiresPermission(permUsersCreate), withTx).Post(routePreview, rs.previewStudentImport)

			// Actual import - requires UsersCreate
			// Note: no withTx here — the handler manages its own WithTenantTx
			// to control commit/rollback based on import results.
			r.With(authorize.RequiresPermission(permUsersCreate)).Post(routeImport, rs.importStudents)
		})

		// Opening balance (Eröffnungssalden) import endpoints (#2132).
		// Stundenkonto and vacation takeover values are payroll data —
		// everything sits behind time_tracking:manage.
		r.Route("/opening-balances", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permTimeTrackingManage), withTx).Get(routeTemplate, rs.DownloadOpeningBalanceTemplate)
			r.With(authorize.RequiresPermission(permTimeTrackingManage), withTx).Post(routePreview, rs.PreviewOpeningBalanceImport)
			// Note: no withTx here — the handler manages its own WithTenantTx
			// to control commit/rollback based on import results.
			r.With(authorize.RequiresPermission(permTimeTrackingManage)).Post(routeImport, rs.ImportOpeningBalances)
		})

		// Staff (Mitarbeiter) import endpoints
		r.Route("/teachers", func(r chi.Router) {
			// Template download - requires UsersRead
			r.With(authorize.RequiresPermission(permUsersRead), withTx).Get(routeTemplate, rs.DownloadStaffTemplate)

			// Preview - requires UsersCreate
			r.With(authorize.RequiresPermission(permUsersCreate), withTx).Post(routePreview, rs.PreviewStaffImport)

			// Actual import - requires UsersCreate
			// Note: no withTx here — the handler manages its own WithTenantTx
			// to control commit/rollback based on import results.
			r.With(authorize.RequiresPermission(permUsersCreate)).Post(routeImport, rs.ImportStaff)
		})
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

	headers := getStudentImportHeaders()
	if err := csvWriter.Write(headers); err != nil {
		slog.Default().Error("Error writing CSV headers", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}

	for _, row := range getStudentImportExamples() {
		// Convert []any to []string for CSV writer
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

// downloadStudentTemplateXLSX generates Excel (.xlsx) template
func (rs *Resource) downloadStudentTemplateXLSX(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=schueler-import-vorlage.xlsx")

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			slog.Default().Error("Error closing Excel file", slog.String("error", err.Error()))
		}
	}()

	sheetName := "Kinder"
	if err := setupExcelSheet(f, sheetName); err != nil {
		slog.Default().Error("Error setting up sheet", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}

	headers := getStudentImportHeaders()
	writeExcelHeaders(f, sheetName, headers)
	writeExcelExampleRows(f, sheetName, getStudentImportExamples())
	setExcelColumnWidths(f, sheetName, len(headers), 15)

	// Add "Hinweise" sheet with field descriptions and allowed values
	writeHinweiseSheet(f)

	if err := f.Write(w); err != nil {
		slog.Default().Error("Error writing Excel file", slog.String("error", err.Error()))
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
		"Vorname", "Nachname", "Klasse", "Gruppe (optional)", "Geburtstag (optional)",
		"Erz1.Vorname", "Erz1.Nachname", "Erz1.Email", "Erz1.Telefon (optional)", "Erz1.Telefon2 (optional)", "Erz1.Mobil (optional)", "Erz1.Mobil2 (optional)", "Erz1.Dienstlich (optional)", "Erz1.Dienstlich2 (optional)", "Erz1.Verhältnis (optional)", "Erz1.Hauptansprechpartner (optional)", "Erz1.Notfall (optional)", "Erz1.Abholberechtigt (optional)",
		"Erz1.Straße (optional)", "Erz1.Stadt (optional)", "Erz1.PLZ (optional)", "Erz1.Notizen (optional)", "Erz1.Sprache (optional)",
		"Erz2.Vorname (optional)", "Erz2.Nachname (optional)", "Erz2.Email (optional)", "Erz2.Telefon (optional)", "Erz2.Telefon2 (optional)", "Erz2.Mobil (optional)", "Erz2.Mobil2 (optional)", "Erz2.Dienstlich (optional)", "Erz2.Dienstlich2 (optional)", "Erz2.Verhältnis (optional)", "Erz2.Hauptansprechpartner (optional)", "Erz2.Notfall (optional)", "Erz2.Abholberechtigt (optional)",
		"Erz2.Straße (optional)", "Erz2.Stadt (optional)", "Erz2.PLZ (optional)", "Erz2.Notizen (optional)", "Erz2.Sprache (optional)",
		"Gesundheitsinfo (optional)", "Betreuernotizen (optional)", "Zusatzinfo (optional)", "Datenschutz", "Aufbewahrung(Tage) (optional)", "Gehweise.Mo", "Gehweise.Di", "Gehweise.Mi", "Gehweise.Do", "Gehweise.Fr", "Begleitung (optional)", "Einschreibung von (optional)", "Einschreibung bis (optional)", "AGB akzeptiert am (optional)", "Datenverarbeitung akzeptiert am (optional)", "E-Mail-Kontakt akzeptiert am (optional)", "Foto-Einwilligung am (optional)",
		"Ankunft.Mo (optional)", "Ankunft.Mo.Notizen (optional)", "Ankunft.Di (optional)", "Ankunft.Di.Notizen (optional)", "Ankunft.Mi (optional)", "Ankunft.Mi.Notizen (optional)", "Ankunft.Do (optional)", "Ankunft.Do.Notizen (optional)", "Ankunft.Fr (optional)", "Ankunft.Fr.Notizen (optional)",
		"Abholung.Mo (optional)", "Abholung.Mo.Notizen (optional)", "Abholung.Di (optional)", "Abholung.Di.Notizen (optional)", "Abholung.Mi (optional)", "Abholung.Mi.Notizen (optional)", "Abholung.Do (optional)", "Abholung.Do.Notizen (optional)", "Abholung.Fr (optional)", "Abholung.Fr.Notizen (optional)",
	}
}

// getStudentImportExamples returns example data rows for the template
func getStudentImportExamples() [][]any {
	return [][]any{
		{"Max", "Mustermann", "1A", "Gruppe 1A", "15.08.2015",
			// Guardian 1: phones, relationship
			"Maria", testLastNameMueller, "maria.mueller@example.com", "0123-456789", "", "", "", "0221-9876543", "", "Mutter", "Ja", "Ja", "Ja",
			// Guardian 1: address, notes, language
			testAddressMusterstr, "Köln", "50667", "", "de",
			// Guardian 2: phones, relationship
			"Hans", testLastNameMueller, "hans.mueller@example.com", "", "", "0176-12345678", "", "", "", "Vater", "Nein", "Ja", "Ja",
			// Guardian 2: address, notes, language
			testAddressMusterstr, "Köln", "50667", "", "de",
			// Additional info (per-day Gehweise: Bus Mo/Mi/Fr, abholung Di, alleine Do; keine Begleitung)
			"", "Sehr ruhiges Kind", "", "Ja", 30, "bus", "abholung", "bus", "alleine", "bus", "", "01.08.2024", "31.07.2025", "01.08.2024", "01.08.2024", "01.08.2024", "01.08.2024",
			// Arrival schedule (Mon-Fri)
			"08:00", "", "08:00", "", "08:00", "", "08:00", "", "08:30", "Frühbetreuung",
			// Pickup schedule (Mon-Fri)
			"16:00", "", "15:30", "", "16:00", "", "15:30", "", "14:00", "Frühschluss"},
		{"Anna", "Schmidt", "2B", "Gruppe 2B", "22.03.14",
			// Guardian 1: phones, relationship
			"Petra", "Schmidt", "petra.schmidt@example.com", "0234-567890", "", "", "", "0211-5551234", "", "Mutter", "Ja", "Ja", "Ja",
			// Guardian 1: address, notes, language
			"Hauptstr. 5", "Düsseldorf", "40210", "Allergien beachten", "de",
			// Guardian 2 (empty)
			"", "", "", "", "", "", "", "", "", "", "", "", "",
			// Guardian 2: empty profile fields
			"", "", "", "", "",
			// Additional info (per-day Gehweise: Mo "mit anderem Kind" + Begleitung, Di–Fr Bus)
			"Allergie: Nüsse", "", "Kann gut malen", "Ja", 15, "mit anderem Kind", "bus", "bus", "bus", "bus", "Geschwisterkind Lena", "01.08.2024", "", "01.08.2024", "01.08.2024", "", "",
			// Arrival schedule (partial)
			"07:45", "", "07:45", "", "07:45", "", "", "", "", "",
			// Pickup schedule (partial)
			"15:00", "", "15:00", "", "15:00", "", "15:00", "", "", ""},
	}
}

// writeExcelHeaders writes headers to the first row
func writeExcelHeaders(f *excelize.File, sheetName string, headers []string) {
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheetName, cell, header); err != nil {
			slog.Default().Error("Error setting header", slog.String("error", err.Error()))
		}
	}
}

// writeExcelExampleRows writes example data rows starting from row 2
func writeExcelExampleRows(f *excelize.File, sheetName string, examples [][]any) {
	for rowIdx, row := range examples {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			if err := f.SetCellValue(sheetName, cell, value); err != nil {
				slog.Default().Error("Error setting cell value", slog.String("error", err.Error()))
			}
		}
	}
}

// setExcelColumnWidths sets uniform column widths
func setExcelColumnWidths(f *excelize.File, sheetName string, numCols int, width float64) {
	for i := 1; i <= numCols; i++ {
		col, _ := excelize.ColumnNumberToName(i)
		if err := f.SetColWidth(sheetName, col, col, width); err != nil {
			slog.Default().Error("Error setting column width", slog.String("error", err.Error()))
		}
	}
}

// writeHinweiseSheet adds a "Hinweise" sheet with field descriptions and allowed values
func writeHinweiseSheet(f *excelize.File) {
	sheetName := "Hinweise"
	if _, err := f.NewSheet(sheetName); err != nil {
		slog.Default().Error("Error creating Hinweise sheet", slog.String("error", err.Error()))
		return
	}

	// Section headers (row index → label) — rendered as merged, bold section dividers.
	// Indices below "Kinder-Zusatzinfos" account for the extra "Begleitung" doc
	// row added after Gehweise.Mo (#1694), which shifts every later row down by one.
	sectionRows := map[int]string{
		7:  "Erziehungsberechtigte (Erz1, Erz2, ...)",
		23: "Kinder-Zusatzinfos",
		38: "Abholzeiten (Montag bis Freitag)",
		42: "Allgemeine Hinweise",
	}

	dataRows := [][]string{
		// row 1: header
		{"Spalte", "Pflicht?", "Erlaubte Werte / Format", "Beschreibung"},
		// rows 2-6: student fields
		{"Vorname", "Ja", "Text", "Vorname des Kindes"},
		{"Nachname", "Ja", "Text", "Nachname des Kindes"},
		{"Klasse", "Ja", "Text (z.B. 1A, 2B)", "Schulklasse"},
		{"Gruppe", "Nein", "Text (exakter Gruppenname)", "OGS-Gruppe — muss in der Datenbank existieren"},
		{"Geburtstag", "Nein", "JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ", "Geburtsdatum, z.B. 2015-08-15, 15.08.2015 oder 15.08.15"},
		// row 7: section header (injected)
		// rows 8-20: guardian fields
		{"Erz1.Vorname", "Nein", "Text", "Vorname des Erziehungsberechtigten"},
		{"Erz1.Nachname", "Nein", "Text", "Nachname des Erziehungsberechtigten"},
		{"Erz1.Email", "Nein*", "gültige E-Mail", "* Mindestens Email ODER Telefon erforderlich"},
		{"Erz1.Telefon", "Nein*", "z.B. 0123-456789, +49 123 456789", "Festnetznummer"},
		{"Erz1.Mobil", "Nein", "z.B. 0176-12345678", "Mobilnummer"},
		{"Erz1.Dienstlich", "Nein", "z.B. 0221-9876543", "Dienstliche Telefonnummer"},
		{"Erz1.Verhältnis", "Nein", "Mutter, Vater, Oma, Opa, Tante, Onkel, Vormund, Sonstige", "Beziehung zum Kind"},
		{"Erz1.Hauptansprechpartner", "Nein", hintYesNo, "Erster Ansprechpartner für die OGS"},
		{"Erz1.Notfall", "Nein", hintYesNo, "Als Notfallkontakt hinterlegt"},
		{"Erz1.Abholberechtigt", "Nein", hintYesNo, "Darf das Kind abholen"},
		{"Erz1.Straße", "Nein", "Text", "Straße und Hausnummer"},
		{"Erz1.Stadt", "Nein", "Text", "Ort / Stadt"},
		{"Erz1.PLZ", "Nein", "5-stellig (z.B. 50667)", "Postleitzahl"},
		{"Erz1.Notizen", "Nein", "Text", "Interne Notizen zum Erziehungsberechtigten"},
		{"Erz1.Sprache", "Nein", "de, en, tr, ar, ...", "Bevorzugte Sprache (ISO 639-1, Standard: de)"},
		// row 23: section header "Kinder-Zusatzinfos" (injected)
		{"Gesundheitsinfo", "Nein", "Text", "Allergien, Medikamente, etc."},
		{"Betreuernotizen", "Nein", "Text", "Interne Notizen für Betreuer"},
		{"Zusatzinfo", "Nein", "Text", "Sonstige Informationen (Elternnotizen)"},
		{"Datenschutz", "Ja", hintYesNo, "Datenschutzerklärung akzeptiert"},
		{"Aufbewahrung(Tage)", "Nein", "1-31 (Standard: 30)", "Datenaufbewahrungsfrist in Tagen"},
		{"Gehweise.Mo", "Nein", "alleine, bus, abholung oder mit anderem Kind", "Wie das Kind am Montag nach Hause geht. Di, Mi, Do, Fr analog: Gehweise.Di … Gehweise.Fr. Leer = geht alleine. (Ältere Dateien mit 'Abholstatus' und 'Bus.Mo'–'Bus.Fr' bzw. einer einzelnen Spalte 'Bus' werden weiterhin akzeptiert.)"},
		{"Begleitung", "Nein", "Text", "Mit wem das Kind geht, wenn ein Tag auf 'mit anderem Kind' steht (z.B. Geschwisterkind, Freund). Ohne einen solchen Tag wird der Eintrag ignoriert."},
		{"Einschreibung von", "Nein", "JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ", "Beginn der Betreuung. Zukünftiges Datum: Kind wird erst dann aktiv."},
		{"Einschreibung bis", "Nein", "JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ", "Ende der Betreuung; darf nicht vor 'Einschreibung von' liegen"},
		{"AGB akzeptiert am", "Nein", "JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ", "Datum der AGB-Einwilligung. Leer = keine Einwilligung erfasst. Kein Zukunftsdatum."},
		{"Datenverarbeitung akzeptiert am", "Nein", "JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ", "Datum der Einwilligung zur Datenverarbeitung. Leer = keine Einwilligung."},
		{"E-Mail-Kontakt akzeptiert am", "Nein", "JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ", "Datum der Einwilligung zur E-Mail-Kontaktaufnahme. Leer = keine Einwilligung."},
		{"Foto-Einwilligung am", "Nein", "JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ", "Datum der Foto-Einwilligung. Leer = keine Einwilligung."},
		// row 37: section header "Abholzeiten" (injected)
		{"Abholung.Mo", "Nein", "HH:MM (z.B. 15:30, 16:00)", "Regelmäßige Gehzeit am Montag (wann das Kind geht)"},
		{"Abholung.Mo.Notizen", "Nein", "Text", "Notiz zur Gehzeit am Montag"},
		{"", "", "(Di, Mi, Do, Fr analog)", "Gleiche Spalten für alle Wochentage"},
		// row 33: section header (injected)
		// row 34: general hints
		{"Ja/Nein-Felder", "", "Ja, Nein, Yes, No, true, false, 1, 0", "Groß-/Kleinschreibung egal"},
		{"Erz2, Erz3, ...", "", "Gleiche Spalten wie Erz1", "Beliebig viele Erziehungsberechtigte möglich"},
		{"Geburtstag", "", "Beispiele: 2015-08-15, 15.08.2015, 15.08.15", "Alle drei Formate werden beim Import akzeptiert"},
	}

	// Create styles
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"2B579A"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	sectionStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "2B579A"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D6E4F0"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	requiredStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "C00000"},
	})

	// Write data, inserting section headers at the right positions
	excelRow := 1
	dataIdx := 0
	totalRows := len(dataRows) + len(sectionRows)
	for excelRow <= totalRows {
		// Check if this row is a section header
		if label, ok := sectionRows[excelRow]; ok {
			cell, _ := excelize.CoordinatesToCellName(1, excelRow)
			_ = f.SetCellValue(sheetName, cell, label)
			_ = f.MergeCell(sheetName, cell, fmt.Sprintf("D%d", excelRow))
			_ = f.SetCellStyle(sheetName, cell, fmt.Sprintf("D%d", excelRow), sectionStyle)
			excelRow++
			continue
		}

		if dataIdx >= len(dataRows) {
			break
		}

		row := dataRows[dataIdx]
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, excelRow)
			_ = f.SetCellValue(sheetName, cell, val)

			// Style header row
			if excelRow == 1 {
				_ = f.SetCellStyle(sheetName, cell, cell, headerStyle)
			}
			// Bold red "Ja" in Pflicht column
			if colIdx == 1 && val == "Ja" {
				_ = f.SetCellStyle(sheetName, cell, cell, requiredStyle)
			}
		}

		dataIdx++
		excelRow++
	}

	// Column widths
	_ = f.SetColWidth(sheetName, "A", "A", 30)
	_ = f.SetColWidth(sheetName, "B", "B", 10)
	_ = f.SetColWidth(sheetName, "C", "C", 45)
	_ = f.SetColWidth(sheetName, "D", "D", 50)

	// Freeze header row
	_ = f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})
}

// previewStudentImport handles import preview (dry-run)
func (rs *Resource) previewStudentImport(w http.ResponseWriter, r *http.Request) {
	// Validate and parse CSV file
	uploadResult, ok := rs.validateAndParseCSVFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseCSVFile
	}

	// Resolve staff ID from JWT (pickup schedule FK references users.staff, not auth.accounts)
	staffID, err := rs.getStaffIDFromJWT(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Get account ID for audit logging (GDPR: audit tracks auth identity)
	accountID, err := getAccountIDFromContext(r.Context())
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
		UserID:          staffID,
		SkipInvalidRows: false,
	}

	result, err := rs.studentImportService.Import(ctx, request)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("vorschau fehlgeschlagen: %s", err.Error())))
		return
	}

	// GDPR Compliance: Audit log for preview (Article 30). The route's tenant
	// transaction supplies the RLS context required by the audit repository.
	if err := rs.studentImportService.RecordAuditInTransaction(r.Context(), "student", uploadResult.Filename, result, accountID, true, tenant.FromContext(r.Context())); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("vorschau konnte nicht protokolliert werden: %w", err)))
		return
	}

	common.Respond(w, r, http.StatusOK, result, "Import-Vorschau erfolgreich")
}

// importStudents handles actual student import
func (rs *Resource) importStudents(w http.ResponseWriter, r *http.Request) {
	// Validate and parse CSV file
	uploadResult, ok := rs.validateAndParseCSVFile(w, r)
	if !ok {
		return // Error already handled by validateAndParseCSVFile
	}

	// Get account ID for audit logging (GDPR: audit tracks auth identity)
	accountID, err := getAccountIDFromContext(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Run actual import inside tenant transaction.
	// Staff ID resolution must happen inside the TX because RLS requires tenant context.
	tenantID := tenant.FromContext(r.Context())
	var result *importModels.ImportResult[importModels.StudentImportRow]
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		// Resolve staff ID within tenant TX (pickup schedule FK references users.staff, not auth.accounts)
		staffID, staffErr := rs.getStaffIDFromJWT(ctx)
		if staffErr != nil {
			return fmt.Errorf("staff resolution failed: %w", staffErr)
		}

		request := importModels.ImportRequest[importModels.StudentImportRow]{
			Rows:            uploadResult.Rows,
			Mode:            importModels.ImportModeCreate, // Create-only: duplicates will error
			DryRun:          false,                         // ACTUAL IMPORT
			StopOnError:     false,                         // Continue on errors
			UserID:          staffID,
			SkipInvalidRows: true, // Skip invalid rows, import valid ones
		}

		var txErr error
		result, txErr = rs.studentImportService.Import(ctx, request)
		if txErr != nil {
			return txErr
		}
		// GDPR Compliance: Audit log for actual import (Article 30). Written
		// inside the import transaction so the import is only acknowledged
		// once its audit record is persisted.
		return rs.studentImportService.RecordAuditInTransaction(ctx, "student", uploadResult.Filename, result, accountID, false, tenantID)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("import fehlgeschlagen: %s", err.Error())))
		return
	}

	// Log import summary
	slog.Default().Info("Student import completed",
		slog.Int("created", result.CreatedCount),
		slog.Int("updated", result.UpdatedCount),
		slog.Int("errors", result.ErrorCount),
		slog.String("filename", uploadResult.Filename))

	// Build success message
	message := fmt.Sprintf("Import abgeschlossen: %d erstellt, %d aktualisiert, %d Fehler",
		result.CreatedCount, result.UpdatedCount, result.ErrorCount)

	common.Respond(w, r, http.StatusOK, result, message)
}

// getAccountIDFromContext extracts the account ID from the JWT context
func getAccountIDFromContext(ctx context.Context) (int64, error) {
	claims, ok := ctx.Value(jwt.CtxClaims).(jwt.AppClaims)
	if !ok {
		return 0, fmt.Errorf("no claims in context")
	}

	return int64(claims.ID), nil
}

// getStaffIDFromJWT resolves the staff ID from JWT claims by looking up account → person → staff.
// The pickup schedule FK (created_by) references users.staff, not auth.accounts.
func (rs *Resource) getStaffIDFromJWT(ctx context.Context) (int64, error) {
	accountID, err := getAccountIDFromContext(ctx)
	if err != nil {
		return 0, err
	}

	person, err := rs.personService.FindByAccountID(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("find person for account %d: %w", accountID, err)
	}
	if person == nil {
		return 0, fmt.Errorf("person not found for account %d", accountID)
	}

	staff, err := rs.personService.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		return 0, fmt.Errorf("find staff for person %d: %w", person.ID, err)
	}
	if staff == nil {
		return 0, fmt.Errorf("user is not a staff member (person %d)", person.ID)
	}

	return staff.ID, nil
}
