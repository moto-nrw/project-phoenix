package importpkg

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

	importModels "github.com/moto-nrw/project-phoenix/internal/core/domain/import"
)

// XLSXParser parses Excel (.xlsx) files into import rows
type XLSXParser struct {
	columnMapping map[string]int // Excel column name → index (lowercase)
}

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
	f, err := excelize.OpenReader(buf)
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
		key := strings.ToLower(strings.TrimSpace(col))
		p.columnMapping[key] = i
	}

	// Parse data rows (skip header)
	var studentRows []importModels.StudentImportRow
	for rowNum := 2; rowNum <= len(rows); rowNum++ {
		if rowNum-1 >= len(rows) {
			break
		}

		values := rows[rowNum-1]

		// Skip empty rows
		if isEmptyRow(values) {
			continue
		}

		row, err := p.mapStudentRow(values)
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

// mapStudentRow maps Excel values to StudentImportRow
func (p *XLSXParser) mapStudentRow(values []string) (importModels.StudentImportRow, error) {
	// Helper: Get column value safely with CSV injection protection
	getCol := func(colName string) string {
		idx, exists := p.columnMapping[colName]
		if !exists || idx < 0 || idx >= len(values) {
			return "" // Column doesn't exist or out of range
		}
		return sanitizeCellValue(strings.TrimSpace(values[idx]))
	}

	// Helper: Check if column exists
	hasColumn := func(colName string) bool {
		_, exists := p.columnMapping[colName]
		return exists
	}

	return mapStudentRowWithHelpers(getCol, hasColumn)
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
		key := strings.ToLower(strings.TrimSpace(col))
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
