package active

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/xuri/excelize/v2"
)

// Writers for the cross-staff export (#1417 2b). They serialize MonthExportRow
// / day rows and NOTHING else — every number arrives computed. Conventions
// follow the existing single-staff export: semicolon CSV with UTF-8 BOM
// (Excel), excelize with a bold gray header for XLSX.

const (
	contentTypeCSV  = "text/csv; charset=utf-8"
	contentTypeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

// exportEmploymentTypeLabels mirrors the frontend's employmentTypeLabels — the
// file is read by people, not by code.
var exportEmploymentTypeLabels = map[string]string{
	userModels.EmploymentTypeFullTime: "Vollzeit",
	userModels.EmploymentTypePartTime: "Teilzeit",
	userModels.EmploymentTypeMinijob:  "Minijob",
}

var monthExportHeaders = []string{
	"Nachname", "Vorname", "Beschäftigungstyp", "Jahr", "Monat",
	"Übertrag Vormonat", "Soll", "Ist",
	"Gutschrift Krank", "Gutschrift Urlaub", "Gutschrift Fortbildung", "Gutschrift Sonstige",
	"Krank (Tage)", "Urlaub (Tage)", "Fortbildung (Tage)",
	"Auszahlung", "Freizeitausgleich", "Saldo-Reset", "Eröffnungssaldo",
	"Saldo Monat", "Übertrag Monatsende", "Abweichung seit Abschluss", "Status",
}

// formatExportMinutes renders a signed minute value. hhmm gives "-12:30";
// decimal gives "-12,50" (German decimal comma, two places) so the file can
// be summed in a spreadsheet without parsing clock strings.
func formatExportMinutes(minutes int, timeFormat string) string {
	if timeFormat == ExportTimeDecimal {
		return strings.Replace(strconv.FormatFloat(float64(minutes)/60.0, 'f', 2, 64), ".", ",", 1)
	}
	sign := ""
	if minutes < 0 {
		sign = "-"
		minutes = -minutes
	}
	return fmt.Sprintf("%s%d:%02d", sign, minutes/60, minutes%60)
}

// formatExportDays renders day counts (halves possible) with a German comma.
func formatExportDays(days float64) string {
	return strings.Replace(strconv.FormatFloat(days, 'f', -1, 64), ".", ",", 1)
}

func monthExportStatus(row MonthExportRow) string {
	if !row.IsClosed {
		return "offen"
	}
	if row.ClosedAt != nil {
		return "abgeschlossen am " + row.ClosedAt.In(timezone.Berlin).Format("02.01.2006")
	}
	return "abgeschlossen"
}

func monthExportCells(row MonthExportRow, timeFormat string) []string {
	employment := exportEmploymentTypeLabels[row.EmploymentType]
	if employment == "" {
		employment = row.EmploymentType
	}
	return []string{
		row.LastName,
		row.FirstName,
		employment,
		strconv.Itoa(row.Year),
		strconv.Itoa(row.Month),
		formatExportMinutes(row.CarryInMinutes, timeFormat),
		formatExportMinutes(row.TargetMinutes, timeFormat),
		formatExportMinutes(row.ActualMinutes, timeFormat),
		formatExportMinutes(row.CreditedSickMinutes, timeFormat),
		formatExportMinutes(row.CreditedVacationMinutes, timeFormat),
		formatExportMinutes(row.CreditedTrainingMinutes, timeFormat),
		formatExportMinutes(row.CreditedOtherMinutes, timeFormat),
		formatExportDays(row.SickDays),
		formatExportDays(row.VacationDays),
		formatExportDays(row.TrainingDays),
		formatExportMinutes(row.PayoutMinutes, timeFormat),
		formatExportMinutes(row.CompTimeMinutes, timeFormat),
		formatExportMinutes(row.ResetMinutes, timeFormat),
		formatExportMinutes(row.OpeningMinutes, timeFormat),
		formatExportMinutes(row.BalanceMinutes, timeFormat),
		formatExportMinutes(row.ClosingBalanceMinutes, timeFormat),
		formatExportMinutes(row.DriftMinutes, timeFormat),
		monthExportStatus(row),
	}
}

func writeMonthCSV(rows []MonthExportRow, timeFormat string) ([]byte, error) {
	cells := make([][]string, 0, len(rows))
	for _, row := range rows {
		cells = append(cells, monthExportCells(row, timeFormat))
	}
	return writeExportCSV(monthExportHeaders, cells, []int{0, 1, 2})
}

func writeMonthXLSX(rows []MonthExportRow, timeFormat string) ([]byte, error) {
	cells := make([][]any, 0, len(rows))
	for _, row := range rows {
		values := stringsToAny(monthExportCells(row, timeFormat))
		values[3] = row.Year
		values[4] = row.Month
		values[12] = row.SickDays
		values[13] = row.VacationDays
		values[14] = row.TrainingDays
		if timeFormat == ExportTimeDecimal {
			values[5] = float64(row.CarryInMinutes) / 60
			values[6] = float64(row.TargetMinutes) / 60
			values[7] = float64(row.ActualMinutes) / 60
			values[8] = float64(row.CreditedSickMinutes) / 60
			values[9] = float64(row.CreditedVacationMinutes) / 60
			values[10] = float64(row.CreditedTrainingMinutes) / 60
			values[11] = float64(row.CreditedOtherMinutes) / 60
			values[15] = float64(row.PayoutMinutes) / 60
			values[16] = float64(row.CompTimeMinutes) / 60
			values[17] = float64(row.ResetMinutes) / 60
			values[18] = float64(row.OpeningMinutes) / 60
			values[19] = float64(row.BalanceMinutes) / 60
			values[20] = float64(row.ClosingBalanceMinutes) / 60
			values[21] = float64(row.DriftMinutes) / 60
		}
		cells = append(cells, values)
	}
	var decimalColumns []int
	if timeFormat == ExportTimeDecimal {
		decimalColumns = []int{6, 7, 8, 9, 10, 11, 12, 16, 17, 18, 19, 20, 21, 22}
	}
	return writeExportXLSX("Zeiterfassung", monthExportHeaders, cells, decimalColumns)
}

// --- day granularity -------------------------------------------------------

// dayExportBlockRow is one per-day row of the cross-staff evidence export:
// the unchanged cells of the single-staff export, prefixed with the name.
type dayExportBlockRow struct {
	LastName  string
	FirstName string
	Cells     []string
}

// dayExportHeaders = name columns + the exact header of the single-staff
// export. Keeping the tail identical is deliberate: a day here must read the
// same as the same day in the per-staff download.
var dayExportHeaders = []string{
	"Nachname", "Vorname",
	"Datum", "Wochentag", "Start", "Ende", "Pause (Min)", "Netto (Std)", "Status", "Quelle", "Bemerkungen",
}

func dayExportCells(rows []dayExportBlockRow) [][]string {
	cells := make([][]string, 0, len(rows))
	for _, row := range rows {
		cells = append(cells, append([]string{row.LastName, row.FirstName}, row.Cells...))
	}
	return cells
}

func writeDayCSV(rows []dayExportBlockRow) ([]byte, error) {
	return writeExportCSV(dayExportHeaders, dayExportCells(rows), []int{0, 1, len(dayExportHeaders) - 1})
}

func writeDayXLSX(rows []dayExportBlockRow) ([]byte, error) {
	return writeExportXLSX("Zeiterfassung", dayExportHeaders, stringsRowsToAny(dayExportCells(rows)), nil)
}

// --- shared serializers ----------------------------------------------------

// sanitizeCSVFormula prevents spreadsheet software from evaluating untrusted
// text as a formula. The apostrophe is Excel's explicit text marker.
func sanitizeCSVFormula(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	default:
		return value
	}
}

func writeExportCSV(headers []string, rows [][]string, untrustedColumns []int) ([]byte, error) {
	var buf bytes.Buffer
	// UTF-8 BOM for Excel compatibility, like the single-staff export.
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	if err := w.Write(headers); err != nil {
		return nil, fmt.Errorf("failed to write CSV header: %w", err)
	}
	for _, row := range rows {
		safeRow := append([]string(nil), row...)
		for _, column := range untrustedColumns {
			if column >= 0 && column < len(safeRow) {
				safeRow[column] = sanitizeCSVFormula(safeRow[column])
			}
		}
		if err := w.Write(safeRow); err != nil {
			return nil, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("CSV write error: %w", err)
	}
	return buf.Bytes(), nil
}

func stringsToAny(values []string) []any {
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func stringsRowsToAny(rows [][]string) [][]any {
	result := make([][]any, len(rows))
	for i, row := range rows {
		result[i] = stringsToAny(row)
	}
	return result
}

func writeExportXLSX(sheet string, headers []string, rows [][]any, decimalColumns []int) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(idx)
	if sheet != "Sheet1" {
		_ = f.DeleteSheet("Sheet1")
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2E8F0"}, Pattern: 1},
	})
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, header)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
	}
	decimalColumnSet := make(map[int]bool, len(decimalColumns))
	for _, column := range decimalColumns {
		decimalColumnSet[column] = true
	}
	decimalStyle := 0
	if len(decimalColumnSet) > 0 {
		numberFormat := "0.00"
		decimalStyle, err = f.NewStyle(&excelize.Style{CustomNumFmt: &numberFormat})
		if err != nil {
			return nil, fmt.Errorf("failed to create decimal cell style: %w", err)
		}
	}
	for rowIdx, row := range rows {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheet, cell, value)
			if decimalColumnSet[colIdx+1] {
				_ = f.SetCellStyle(sheet, cell, cell, decimalStyle)
			}
		}
	}
	for i := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheet, col, col, 16)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to write XLSX: %w", err)
	}
	return buf.Bytes(), nil
}
