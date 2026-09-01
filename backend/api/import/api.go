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
	studentImportService   *importService.ImportService[importModels.StudentImportRow]
	staffImportService     *importService.ImportService[importModels.StaffImportRow]
	classListImportService *importService.ImportService[importModels.ClassListEntryImportRow]
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
	classListImportService *importService.ImportService[importModels.ClassListEntryImportRow],
	personService userSvc.PersonService,
	db *bun.DB,
) *Resource {
	return &Resource{
		studentImportService:   studentImportService,
		staffImportService:     staffImportService,
		classListImportService: classListImportService,
		personService:          personService,
		db:                     db,
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
		r.Use(common.ReadOnlyPreviewMiddleware)
		r.Use(jwt.TenantMiddleware)
		r.Use(common.SecurityPrincipalMiddleware)
		r.Use(common.TenantOperationMiddleware)
		withTx := common.TenantTxMiddleware

		// Student import endpoints
		r.Route("/students", func(r chi.Router) {
			// Template download - requires UsersRead
			r.With(common.RequiresPermission(permUsersRead), withTx).Get(routeTemplate, rs.downloadStudentTemplate)

			// Preview - requires UsersCreate
			// Note: no withTx here — the handler owns its tenant transaction
			// so the GDPR audit row is committed before the success response.
			r.With(common.RequiresPermission(permUsersCreate)).Post(routePreview, rs.previewStudentImport)

			// Actual import - requires UsersCreate
			// Note: no withTx here — the handler manages its own WithTenantTx
			// to control commit/rollback based on import results.
			r.With(common.RequiresPermission(permUsersCreate)).Post(routeImport, rs.importStudents)
		})

		// Opening balance (Eröffnungssalden) import endpoints (#2132).
		// Stundenkonto and vacation takeover values are payroll data —
		// everything sits behind time_tracking:manage.
		r.Route("/opening-balances", func(r chi.Router) {
			r.With(common.RequiresPermission(permTimeTrackingManage), withTx).Get(routeTemplate, rs.DownloadOpeningBalanceTemplate)
			// Note: no withTx on the preview either — the handler owns its
			// tenant transaction so the GDPR audit row is committed before
			// the success response.
			r.With(common.RequiresPermission(permTimeTrackingManage)).Post(routePreview, rs.PreviewOpeningBalanceImport)
			// Note: no withTx here — the handler manages its own WithTenantTx
			// to control commit/rollback based on import results.
			r.With(common.RequiresPermission(permTimeTrackingManage)).Post(routeImport, rs.ImportOpeningBalances)
		})

		// Class-list entry (Klassenlisteneintrag, #2382) import endpoints
		r.Route("/class-list-entries", func(r chi.Router) {
			r.With(common.RequiresPermission(permUsersRead), withTx).Get(routeTemplate, rs.DownloadClassListTemplate)
			// Note: no withTx on preview/import — the handler owns its tenant
			// transaction so the GDPR audit row is committed before the
			// success response.
			r.With(common.RequiresPermission(permUsersCreate)).Post(routePreview, rs.PreviewClassListImport)
			r.With(common.RequiresPermission(permUsersCreate)).Post(routeImport, rs.ImportClassList)
		})

		// Staff (Mitarbeiter) import endpoints
		r.Route("/teachers", func(r chi.Router) {
			// Template download - requires UsersRead
			r.With(common.RequiresPermission(permUsersRead), withTx).Get(routeTemplate, rs.DownloadStaffTemplate)

			// Preview - requires UsersCreate
			// Note: no withTx here — the handler owns its tenant transaction
			// so the GDPR audit row is committed before the success response.
			r.With(common.RequiresPermission(permUsersCreate)).Post(routePreview, rs.PreviewStaffImport)

			// Actual import - requires UsersCreate
			// Note: no withTx here — the handler manages its own WithTenantTx
			// to control commit/rollback based on import results.
			r.With(common.RequiresPermission(permUsersCreate)).Post(routeImport, rs.ImportStaff)
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

// studentGuardianColumns are the per-guardian columns of the template, in
// order. Erz1's name and e-mail are labelled without "(optional)" so the
// template signals that at least one contact is expected.
var studentGuardianColumns = []string{
	"Vorname", "Nachname", "Email", "Telefon", "Telefon2", "Mobil", "Mobil2", "Dienstlich", "Dienstlich2",
	"Verhältnis", "Rolle", "Hauptansprechpartner", "Notfall", "Abholberechtigt", "Abholhinweis", "Notfallpriorität",
	"Straße", "Stadt", "PLZ", "Notizen", "Sprache",
}

// studentTemplateGuardianCount is how many Erz blocks the template ships with.
// The parser accepts any number (Erz5, Erz6, ...) as long as the columns follow
// the same naming.
const studentTemplateGuardianCount = 4

// studentTemplateColumns is the template header row without the "(optional)"
// annotation, paired with whether the column is required.
func studentTemplateColumns() []struct {
	name     string
	required bool
} {
	type col = struct {
		name     string
		required bool
	}
	cols := []col{
		{"Vorname", true}, {"Nachname", true}, {"Klasse", true}, {"Gruppe", false}, {"Geburtstag", false},
		{"RFID", false}, {"Straße", false}, {"PLZ", false}, {"Ort", false},
	}
	for n := 1; n <= studentTemplateGuardianCount; n++ {
		for _, c := range studentGuardianColumns {
			cols = append(cols, col{fmt.Sprintf("Erz%d.%s", n, c), false})
		}
	}
	cols = append(cols,
		col{"Gesundheitsinfo", false}, col{"Betreuernotizen", false}, col{"Zusatzinfo", false},
		col{"Datenschutz", true}, col{"Aufbewahrung(Tage)", false},
		col{"Gehweise.Mo", true}, col{"Gehweise.Di", true}, col{"Gehweise.Mi", true}, col{"Gehweise.Do", true}, col{"Gehweise.Fr", true},
		col{"Begleitung", false}, col{"Einschreibung von", false}, col{"Einschreibung bis", false},
		col{"AGB akzeptiert am", false}, col{"Datenverarbeitung akzeptiert am", false},
		col{"E-Mail-Kontakt akzeptiert am", false}, col{"Foto-Einwilligung am", false},
	)
	for _, prefix := range []string{"Ankunft", "Abholung"} {
		for _, day := range []string{"Mo", "Di", "Mi", "Do", "Fr"} {
			cols = append(cols, col{fmt.Sprintf("%s.%s", prefix, day), false}, col{fmt.Sprintf("%s.%s.Notizen", prefix, day), false})
		}
	}
	return cols
}

// getStudentImportHeaders returns the header row for student import template
func getStudentImportHeaders() []string {
	cols := studentTemplateColumns()
	headers := make([]string, 0, len(cols))
	for _, c := range cols {
		if c.required {
			headers = append(headers, c.name)
		} else {
			headers = append(headers, c.name+" (optional)")
		}
	}
	return headers
}

// studentExampleRow projects a sparse header→value map onto the template
// column order, so example rows never drift out of alignment when a column
// is added.
func studentExampleRow(values map[string]any) []any {
	cols := studentTemplateColumns()
	row := make([]any, len(cols))
	for i, c := range cols {
		if v, ok := values[c.name]; ok {
			row[i] = v
		} else {
			row[i] = ""
		}
	}
	return row
}

// getStudentImportExamples returns example data rows for the template
func getStudentImportExamples() [][]any {
	return [][]any{
		studentExampleRow(map[string]any{
			"Vorname": "Max", "Nachname": "Mustermann", "Klasse": "1A", "Gruppe": "Gruppe 1A", "Geburtstag": "15.08.2015",
			"Straße": testAddressMusterstr, "PLZ": "50667", "Ort": "Köln",
			"Erz1.Vorname": "Maria", "Erz1.Nachname": testLastNameMueller, "Erz1.Email": "maria.mueller@example.com",
			"Erz1.Telefon": "0123-456789", "Erz1.Dienstlich": "0221-9876543", "Erz1.Verhältnis": "Mutter", "Erz1.Rolle": "Hauptsorgeberechtigt",
			"Erz1.Hauptansprechpartner": "Ja", "Erz1.Notfall": "Ja", "Erz1.Abholberechtigt": "Ja", "Erz1.Notfallpriorität": 1,
			"Erz1.Straße": testAddressMusterstr, "Erz1.Stadt": "Köln", "Erz1.PLZ": "50667", "Erz1.Sprache": "de",
			"Erz2.Vorname": "Hans", "Erz2.Nachname": testLastNameMueller, "Erz2.Email": "hans.mueller@example.com",
			"Erz2.Mobil": "0176-12345678", "Erz2.Verhältnis": "Vater", "Erz2.Rolle": "Sorgeberechtigt",
			"Erz2.Hauptansprechpartner": "Nein", "Erz2.Notfall": "Ja", "Erz2.Abholberechtigt": "Ja", "Erz2.Notfallpriorität": 2,
			"Erz2.Straße": testAddressMusterstr, "Erz2.Stadt": "Köln", "Erz2.PLZ": "50667", "Erz2.Sprache": "de",
			"Erz3.Vorname": "Gisela", "Erz3.Nachname": testLastNameMueller, "Erz3.Mobil": "0171-2223344",
			"Erz3.Verhältnis": "Oma", "Erz3.Rolle": "Nur Abholung", "Erz3.Abholberechtigt": "Ja", "Erz3.Abholhinweis": "Nur dienstags",
			"Betreuernotizen": "Sehr ruhiges Kind", "Datenschutz": "Ja", "Aufbewahrung(Tage)": 30,
			"Gehweise.Mo": "bus", "Gehweise.Di": "abholung", "Gehweise.Mi": "bus", "Gehweise.Do": "alleine", "Gehweise.Fr": "bus",
			"Einschreibung von": "01.08.2024", "Einschreibung bis": "31.07.2025",
			"AGB akzeptiert am": "01.08.2024", "Datenverarbeitung akzeptiert am": "01.08.2024", "E-Mail-Kontakt akzeptiert am": "01.08.2024", "Foto-Einwilligung am": "01.08.2024",
			"Ankunft.Mo": "08:00", "Ankunft.Di": "08:00", "Ankunft.Mi": "08:00", "Ankunft.Do": "08:00", "Ankunft.Fr": "08:30", "Ankunft.Fr.Notizen": "Frühbetreuung",
			"Abholung.Mo": "16:00", "Abholung.Di": "15:30", "Abholung.Mi": "16:00", "Abholung.Do": "15:30", "Abholung.Fr": "14:00", "Abholung.Fr.Notizen": "Frühschluss",
		}),
		studentExampleRow(map[string]any{
			"Vorname": "Anna", "Nachname": "Schmidt", "Klasse": "2B", "Gruppe": "Gruppe 2B", "Geburtstag": "22.03.14",
			"Erz1.Vorname": "Petra", "Erz1.Nachname": "Schmidt", "Erz1.Email": "petra.schmidt@example.com",
			"Erz1.Telefon": "0234-567890", "Erz1.Dienstlich": "0211-5551234", "Erz1.Verhältnis": "Mutter",
			"Erz1.Hauptansprechpartner": "Ja", "Erz1.Notfall": "Ja", "Erz1.Abholberechtigt": "Ja",
			"Erz1.Straße": "Hauptstr. 5", "Erz1.Stadt": "Düsseldorf", "Erz1.PLZ": "40210", "Erz1.Notizen": "Allergien beachten", "Erz1.Sprache": "de",
			"Gesundheitsinfo": "Allergie: Nüsse", "Zusatzinfo": "Kann gut malen", "Datenschutz": "Ja", "Aufbewahrung(Tage)": 15,
			"Gehweise.Mo": "mit anderem Kind", "Gehweise.Di": "bus", "Gehweise.Mi": "bus", "Gehweise.Do": "bus", "Gehweise.Fr": "bus",
			"Begleitung": "Geschwisterkind Lena", "Einschreibung von": "01.08.2024",
			"AGB akzeptiert am": "01.08.2024", "Datenverarbeitung akzeptiert am": "01.08.2024",
			"Ankunft.Mo": "07:45", "Ankunft.Di": "07:45", "Ankunft.Mi": "07:45",
			"Abholung.Mo": "15:00", "Abholung.Di": "15:00", "Abholung.Mi": "15:00", "Abholung.Do": "15:00",
		}),
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
		11: "Erziehungsberechtigte (Erz1, Erz2, ...)",
		30: "Kinder-Zusatzinfos",
		45: "Abholzeiten (Montag bis Freitag)",
		49: "Allgemeine Hinweise",
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
		{"RFID", "Nein", "Chip-ID der Karte", "Karte des Kindes. Die Karte muss an dieser Schule angelegt und noch niemandem zugeordnet sein, sonst wird die Zeile abgelehnt."},
		{"Straße", "Nein", "Text", "Anschrift des Kindes: Straße und Hausnummer"},
		{"PLZ", "Nein", "5-stellig", "Anschrift des Kindes: Postleitzahl"},
		{"Ort", "Nein", "Text", "Anschrift des Kindes: Ort"},
		// row 11: section header (injected)
		// rows 8-20: guardian fields
		{"Erz1.Vorname", "Nein", "Text", "Vorname des Erziehungsberechtigten"},
		{"Erz1.Nachname", "Nein", "Text", "Nachname des Erziehungsberechtigten"},
		{"Erz1.Email", "Nein*", "gültige E-Mail", "* Mindestens Email ODER Telefon erforderlich"},
		{"Erz1.Telefon", "Nein*", "z.B. 0123-456789, +49 123 456789", "Festnetznummer"},
		{"Erz1.Mobil", "Nein", "z.B. 0176-12345678", "Mobilnummer"},
		{"Erz1.Dienstlich", "Nein", "z.B. 0221-9876543", "Dienstliche Telefonnummer"},
		{"Erz1.Verhältnis", "Nein", "Mutter, Vater, Oma, Opa, Tante, Onkel, Vormund, Sonstige", "Beziehung zum Kind"},
		{"Erz1.Rolle", "Nein", "Hauptsorgeberechtigt, Sorgeberechtigt, Mitsorgeberechtigt, Notfallkontakt, Nur Abholung, Sozialarbeit", "Rolle im Elternportal. Leer = wird aus Verhältnis und Ja/Nein-Feldern abgeleitet."},
		{"Erz1.Hauptansprechpartner", "Nein", hintYesNo, "Erster Ansprechpartner für die OGS"},
		{"Erz1.Notfall", "Nein", hintYesNo, "Als Notfallkontakt hinterlegt"},
		{"Erz1.Abholberechtigt", "Nein", hintYesNo, "Darf das Kind abholen"},
		{"Erz1.Abholhinweis", "Nein", "Text", "Hinweis zur Abholung durch diese Person (z.B. nur dienstags)"},
		{"Erz1.Notfallpriorität", "Nein", "1, 2, 3, ...", "Reihenfolge im Notfall (1 = zuerst anrufen)"},
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
		{"Erz2, Erz3, ...", "", "Gleiche Spalten wie Erz1", "Beliebig viele Erziehungsberechtigte möglich; die Vorlage enthält Erz1 bis Erz4"},
		{"Geburtstag", "", "Beispiele: 2015-08-15, 15.08.2015, 15.08.15", "Alle drei Formate werden beim Import akzeptiert"},
		{"Aktualisieren", "", "Modus 'Bestehende aktualisieren' oder 'Beides'", "Bestehende Kinder werden über Vorname + Nachname + Klasse erkannt, bei Klassenwechsel über RFID oder Vorname + Nachname + Geburtstag. Leere Zellen ändern nichts."},
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
	mode, ok := importModeFromRequest(w, r)
	if !ok {
		return
	}

	// Get account ID for audit logging (GDPR: audit tracks auth identity)
	accountID, err := getAccountIDFromContext(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// The preview owns its tenant transaction (no route-level withTx): the
	// GDPR audit row must be committed before the success response is
	// written — a middleware transaction commits only after the handler
	// returns, when a commit failure can no longer be reported. Staff ID
	// resolution happens inside the TX because the lookup is RLS-scoped.
	tenantID := tenant.FromContext(r.Context())
	var result *importModels.ImportResult[importModels.StudentImportRow]
	var staffResolutionErr error
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		staffID, staffErr := rs.getStaffIDFromJWT(ctx)
		if staffErr != nil {
			staffResolutionErr = staffErr
			return staffErr
		}

		request := importModels.ImportRequest[importModels.StudentImportRow]{
			Rows:            uploadResult.Rows,
			Mode:            mode,
			DryRun:          true,  // PREVIEW ONLY
			StopOnError:     false, // Collect all errors
			UserID:          staffID,
			SkipInvalidRows: false,
		}

		var txErr error
		result, txErr = rs.studentImportService.Import(ctx, request)
		if txErr != nil {
			return txErr
		}
		// GDPR Compliance: Audit log for preview (Article 30).
		return rs.studentImportService.RecordAuditInTransaction(ctx, "student", uploadResult.Filename, result, accountID, true, tenantID)
	}); err != nil {
		if staffResolutionErr != nil {
			common.RenderError(w, r, common.ErrorUnauthorized(staffResolutionErr))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("Import-Vorschau fehlgeschlagen", err))
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
	mode, ok := importModeFromRequest(w, r)
	if !ok {
		return
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
			Mode:            mode,
			DryRun:          false, // ACTUAL IMPORT
			StopOnError:     false, // Continue on errors
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
		common.RenderError(w, r, common.ErrorInternalServerWrap("Import fehlgeschlagen", err))
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
