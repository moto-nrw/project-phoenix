package importpkg

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// CSVParser parses CSV files into import rows
type CSVParser struct {
	columnMapping map[string]int // CSV column name → index (lowercase)
}

// NewCSVParser creates a new CSV parser
func NewCSVParser() *CSVParser {
	return &CSVParser{}
}

// ParseStudents parses a CSV file into student import rows
func (p *CSVParser) ParseStudents(reader io.Reader) ([]importModels.StudentImportRow, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1 // Variable columns (support any number of guardians)
	csvReader.TrimLeadingSpace = true
	csvReader.LazyQuotes = true // Handle quotes more leniently

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	// Build column mapping (case-insensitive)
	p.columnMapping = make(map[string]int)
	for i, col := range header {
		key := strings.ToLower(strings.TrimSpace(col))
		p.columnMapping[key] = i
	}

	// Read data rows
	var rows []importModels.StudentImportRow
	rowNum := 2 // Start at 2 (1=header)

	for {
		values, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowNum, err)
		}

		row, err := p.mapStudentRow(values)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowNum, err)
		}

		rows = append(rows, row)
		rowNum++
	}

	// Validate that we have at least one data row
	// Empty files (only headers) likely indicate user uploaded the template by mistake
	if len(rows) == 0 {
		return nil, fmt.Errorf("die CSV-Datei enthält keine Datenzeilen. Möglicherweise haben Sie versehentlich die Vorlage hochgeladen")
	}

	return rows, nil
}

// mapStudentRow maps CSV values to StudentImportRow
func (p *CSVParser) mapStudentRow(values []string) (importModels.StudentImportRow, error) {
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
func (p *CSVParser) GetColumnMapping() map[string]int {
	return p.columnMapping
}

// ValidateHeader checks if the CSV has required columns
func (p *CSVParser) ValidateHeader(reader io.Reader) error {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1

	// Read only the header
	header, err := csvReader.Read()
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}

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
