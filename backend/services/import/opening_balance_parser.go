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

// openingBalanceRequiredColumns are the normalized header keys the opening
// balance import file must contain. The value columns are all optional.
var openingBalanceRequiredColumns = []string{"vorname", "nachname"}

// MapOpeningBalanceRow maps column values to an OpeningBalanceImportRow.
// The four value columns use GetRawCol: sanitizeCellValue's CSV-injection
// guard prefixes leading '-' with a quote, which would break exactly the
// negative balances this import exists for. The raw values go straight into
// strict float parsing, so there is no injection surface.
func MapOpeningBalanceRow(mapper *ColumnMapper) importModels.OpeningBalanceImportRow {
	return importModels.OpeningBalanceImportRow{
		PersonnelNumber:   mapper.GetCol("personalnummer"),
		FirstName:         mapper.GetCol("vorname"),
		LastName:          mapper.GetCol("nachname"),
		HoursBalance:      mapper.GetRawCol("stundensaldo"),
		VacationEntitled:  mapper.GetRawCol("jahresanspruch"),
		VacationCarryover: mapper.GetRawCol("vorjahresübertrag"),
		VacationRemaining: mapper.GetRawCol("resturlaub"),
	}
}

// missingOpeningBalanceColumns returns the required columns absent from the mapping.
func missingOpeningBalanceColumns(mapping map[string]int) []string {
	var missing []string
	for _, col := range openingBalanceRequiredColumns {
		if _, ok := mapping[col]; !ok {
			missing = append(missing, col)
		}
	}
	return missing
}

// ParseOpeningBalanceCSV parses a CSV file into opening balance import rows.
func ParseOpeningBalanceCSV(reader io.Reader) ([]importModels.OpeningBalanceImportRow, error) {
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
	if missing := missingOpeningBalanceColumns(mapping); len(missing) > 0 {
		return nil, fmt.Errorf("fehlende erforderliche Spalten: %s", strings.Join(missing, ", "))
	}

	var rows []importModels.OpeningBalanceImportRow
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
		rows = append(rows, MapOpeningBalanceRow(mapper))
		rowNum++
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("die CSV-Datei enthält keine Datenzeilen. Möglicherweise haben Sie versehentlich die Vorlage hochgeladen")
	}

	return rows, nil
}

// ParseOpeningBalanceXLSX parses an Excel (.xlsx) file into opening balance import rows.
func ParseOpeningBalanceXLSX(reader io.Reader) ([]importModels.OpeningBalanceImportRow, error) {
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
	if missing := missingOpeningBalanceColumns(mapping); len(missing) > 0 {
		return nil, fmt.Errorf("fehlende erforderliche Spalten: %s", strings.Join(missing, ", "))
	}

	var rows []importModels.OpeningBalanceImportRow
	for rowNum := 2; rowNum <= len(sheetRows); rowNum++ {
		values := sheetRows[rowNum-1]
		if isEmptyRow(values) {
			continue
		}
		mapper := NewColumnMapper(mapping, values)
		rows = append(rows, MapOpeningBalanceRow(mapper))
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("die Excel-Datei enthält keine Datenzeilen. Möglicherweise haben Sie versehentlich die Vorlage hochgeladen")
	}

	return rows, nil
}
