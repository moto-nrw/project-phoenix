package fileformat

import (
	"fmt"
	"log/slog"

	"github.com/xuri/excelize/v2"
)

const (
	testLastNameMueller  = "Müller"
	testAddressMusterstr = "Musterstr. 1"
	hintYesNo            = "Ja / Nein"
)

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

// getClassListImportHeaders returns the header row for the template.
func getClassListImportHeaders() []string {
	return []string{"Vorname", "Nachname", "Klasse"}
}

// The class-list template deliberately ships NO example data rows (#2399
// review round 10): a class-list row is nothing but a name and a class, so
// any rule that recognizes example rows again on upload is a name blocklist
// that would silently drop a real child of the same name. The columns are
// explained on the "Hinweise" sheet instead, and an unchanged template upload
// is rejected as "keine Datenzeilen" because it carries none.

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
		{"Hinweis", "", "Die Vorlage enthält nur die Kopfzeile: Bitte tragen Sie die Kinder ab Zeile 2 des Tabellenblatts \"Klassenliste\" ein, eine Zeile pro Kind."},
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
