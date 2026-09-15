package fileformat

import (
	"encoding/csv"
	"fmt"
	"io"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
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
		key := normalizeHeaderKey(col)
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

		// Use shared mapping logic
		mapper := NewColumnMapper(p.columnMapping, values)
		row, err := MapStudentRow(mapper)
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
