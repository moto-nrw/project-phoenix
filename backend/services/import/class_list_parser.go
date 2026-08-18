package importpkg

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// classListRequiredColumns are the normalized header keys a class-list entry
// import file must contain (#2382). All three are required — the entry IS
// name + class.
var classListRequiredColumns = []string{"vorname", "nachname", "klasse"}

// MapClassListRow maps column values to a ClassListEntryImportRow.
func MapClassListRow(mapper *ColumnMapper) importModels.ClassListEntryImportRow {
	return importModels.ClassListEntryImportRow{
		FirstName:   mapper.GetCol("vorname"),
		LastName:    mapper.GetCol("nachname"),
		SchoolClass: mapper.GetCol("klasse"),
	}
}

// missingClassListColumns returns the required columns absent from the mapping.
func missingClassListColumns(mapping map[string]int) []string {
	var missing []string
	for _, col := range classListRequiredColumns {
		if _, ok := mapping[col]; !ok {
			missing = append(missing, col)
		}
	}
	return missing
}

// ParseClassListCSV parses a CSV file into class-list entry import rows.
func ParseClassListCSV(reader io.Reader) ([]importModels.ClassListEntryImportRow, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true
	csvReader.LazyQuotes = true

	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	mapping := make(map[string]int)
	for i, col := range header {
		mapping[normalizeHeaderKey(col)] = i
	}
	if missing := missingClassListColumns(mapping); len(missing) > 0 {
		return nil, fmt.Errorf("fehlende erforderliche Spalten: %s", strings.Join(missing, ", "))
	}

	var rows []importModels.ClassListEntryImportRow
	rowNum := 2
	for {
		values, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowNum, err)
		}

		if isEmptyRow(values) {
			rowNum++
			continue
		}

		mapper := NewColumnMapper(mapping, values)
		rows = append(rows, MapClassListRow(mapper))
		rowNum++
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("die CSV-Datei enthält keine Datenzeilen. Möglicherweise haben Sie versehentlich die Vorlage hochgeladen")
	}

	return rows, nil
}

// ParseClassListXLSX parses an Excel (.xlsx) file into class-list entry
// import rows.
func ParseClassListXLSX(reader io.Reader) ([]importModels.ClassListEntryImportRow, error) {
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	f, err := excelize.OpenReader(buf)
	if err != nil {
		return nil, fmt.Errorf("open excel file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	sheetRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}
	if len(sheetRows) == 0 {
		return nil, fmt.Errorf("empty Excel file")
	}

	mapping := make(map[string]int)
	for i, col := range sheetRows[0] {
		mapping[normalizeHeaderKey(col)] = i
	}
	if missing := missingClassListColumns(mapping); len(missing) > 0 {
		return nil, fmt.Errorf("fehlende erforderliche Spalten: %s", strings.Join(missing, ", "))
	}

	var rows []importModels.ClassListEntryImportRow
	for rowNum := 2; rowNum <= len(sheetRows); rowNum++ {
		values := sheetRows[rowNum-1]
		if isEmptyRow(values) {
			continue
		}
		mapper := NewColumnMapper(mapping, values)
		rows = append(rows, MapClassListRow(mapper))
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("die Excel-Datei enthält keine Datenzeilen. Möglicherweise haben Sie versehentlich die Vorlage hochgeladen")
	}

	return rows, nil
}
