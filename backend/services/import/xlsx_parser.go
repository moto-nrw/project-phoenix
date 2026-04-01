package importpkg

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// XLSXParser parses Excel (.xlsx) files into import rows
type XLSXParser struct {
	columnMapping map[string]int // Excel column name → index (lowercase)
}

const (
	studentImportShortDatePattern = "yyyy-MM-dd"
	minValidBirthdaySerial        = 61
)

// NewXLSXParser creates a new XLSX parser
func NewXLSXParser() *XLSXParser {
	return &XLSXParser{}
}

// ParseStudents parses an Excel file into student import rows
func (p *XLSXParser) ParseStudents(reader io.Reader) ([]importModels.StudentImportRow, error) {
	// Read the file into memory
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Open Excel file
	f, err := excelize.OpenReader(buf, excelize.Options{
		ShortDatePattern: studentImportShortDatePattern,
	})
	if err != nil {
		return nil, fmt.Errorf("open excel file: %w", err)
	}
	defer func() {
		_ = f.Close() // Ignore close errors in defer
	}()

	// Get the first sheet
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	// Read all rows from the sheet
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("empty Excel file")
	}

	// First row is the header
	header := rows[0]

	// Build column mapping (case-insensitive)
	p.columnMapping = make(map[string]int)
	for i, col := range header {
		key := normalizeHeaderKey(col)
		p.columnMapping[key] = i
	}

	// Parse data rows (skip header)
	var studentRows []importModels.StudentImportRow
	for rowNum := 2; rowNum <= len(rows); rowNum++ {
		if rowNum-1 >= len(rows) {
			break
		}

		values := normalizeXLSXRowValues(f, sheetName, rowNum, header, rows[rowNum-1])

		// Skip empty rows
		if isEmptyRow(values) {
			continue
		}

		// Use shared mapping logic
		mapper := NewColumnMapper(p.columnMapping, values)
		row, err := MapStudentRow(mapper)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowNum, err)
		}

		studentRows = append(studentRows, row)
	}

	// Validate that we have at least one data row
	// Empty files (only headers) likely indicate user uploaded the template by mistake
	if len(studentRows) == 0 {
		return nil, fmt.Errorf("die Excel-Datei enthält keine Datenzeilen. Möglicherweise haben Sie versehentlich die Vorlage hochgeladen")
	}

	return studentRows, nil
}

// GetColumnMapping returns the detected column mapping
func (p *XLSXParser) GetColumnMapping() map[string]int {
	return p.columnMapping
}

// ValidateHeader checks if the Excel file has required columns
func (p *XLSXParser) ValidateHeader(reader io.Reader) error {
	// Read the file into memory
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Open Excel file
	f, err := excelize.OpenReader(buf)
	if err != nil {
		return fmt.Errorf("open excel file: %w", err)
	}
	defer func() {
		_ = f.Close() // Ignore close errors in defer
	}()

	// Get the first sheet
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return fmt.Errorf("no sheets found in Excel file")
	}

	// Read header row
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("read rows: %w", err)
	}

	if len(rows) == 0 {
		return fmt.Errorf("empty Excel file")
	}

	header := rows[0]

	// Build column mapping
	mapping := make(map[string]int)
	for i, col := range header {
		key := normalizeHeaderKey(col)
		mapping[key] = i
	}

	// Required columns
	requiredColumns := []string{"vorname", "nachname", "klasse"}
	missing := []string{}

	for _, col := range requiredColumns {
		if _, exists := mapping[col]; !exists {
			missing = append(missing, col)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("fehlende erforderliche Spalten: %s", strings.Join(missing, ", "))
	}

	return nil
}

// isEmptyRow checks if an Excel row is completely empty
func isEmptyRow(values []string) bool {
	for _, val := range values {
		if strings.TrimSpace(val) != "" {
			return false
		}
	}
	return true
}

func normalizeXLSXRowValues(f *excelize.File, sheetName string, rowNum int, header, values []string) []string {
	normalized := append([]string(nil), values...)
	limit := len(header)
	if len(normalized) < limit {
		limit = len(normalized)
	}

	for colIdx := 0; colIdx < limit; colIdx++ {
		headerKey := normalizeHeaderKey(header[colIdx])
		cellValue := strings.TrimSpace(normalized[colIdx])
		if cellValue == "" {
			continue
		}

		cellName, err := excelize.CoordinatesToCellName(colIdx+1, rowNum)
		if err != nil {
			continue
		}

		style, err := getCellStyle(f, sheetName, cellName)
		if err != nil {
			continue
		}

		if headerKey != "geburtstag" || !isDateNumberFormat(style) {
			continue
		}

		if rawValue, ok := suspiciousBirthdayRawValue(f, sheetName, cellName); ok {
			// Force obviously invalid Excel serials back through normal validation.
			normalized[colIdx] = rawValue
		}
	}

	return normalized
}

func getCellStyle(f *excelize.File, sheetName, cellName string) (*excelize.Style, error) {
	styleID, err := f.GetCellStyle(sheetName, cellName)
	if err != nil {
		return nil, err
	}
	return f.GetStyle(styleID)
}

func suspiciousBirthdayRawValue(f *excelize.File, sheetName, cellName string) (string, bool) {
	rawValue, err := f.GetCellValue(sheetName, cellName, excelize.Options{RawCellValue: true})
	if err != nil {
		return "", false
	}

	serial, err := strconv.ParseFloat(strings.TrimSpace(rawValue), 64)
	if err != nil {
		return "", false
	}

	if serial <= 0 || serial > minValidBirthdaySerial {
		return "", false
	}

	return rawValue, true
}

func isDateNumberFormat(style *excelize.Style) bool {
	if style == nil {
		return false
	}
	if style.CustomNumFmt != nil {
		custom := strings.ToLower(*style.CustomNumFmt)
		return strings.Contains(custom, "d") && strings.Contains(custom, "y")
	}

	switch style.NumFmt {
	case 14, 15, 16, 17, 22, 27, 30, 36, 50, 57, 58:
		return true
	default:
		return false
	}
}
