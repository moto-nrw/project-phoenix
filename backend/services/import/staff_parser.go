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

// staffRequiredColumns are the normalized header keys a staff import file must
// contain. E-Mail is no longer required (#2600): a row without one becomes a
// Stammdatensatz without a login.
var staffRequiredColumns = []string{"vorname", "nachname", "rolle"}

// MapStaffRow maps column values to a StaffImportRow using the shared mapper.
func MapStaffRow(mapper *ColumnMapper) importModels.StaffImportRow {
	return importModels.StaffImportRow{
		FirstName:  mapper.GetCol("vorname"),
		LastName:   mapper.GetCol("nachname"),
		Email:      mapper.GetCol("email"),
		RoleName:   mapper.GetCol("rolle"),
		Position:   mapper.GetCol("position"),
		Birthday:   mapper.GetCol("geburtstag"),
		Gender:     mapper.GetCol("geschlecht"),
		StaffNotes: mapper.GetCol("notizen"),

		PersonnelNumber:  mapper.GetCol("personalnummer"),
		EmploymentType:   mapper.GetCol("beschäftigungsart"),
		EntryDate:        mapper.GetCol("eintritt"),
		ContractEndDate:  mapper.GetCol("vertragsende"),
		ProbationEndDate: mapper.GetCol("probezeit bis"),
		WeeklyHours:      mapper.GetCol("wochenstunden"),

		AddressStreet:         mapper.GetCol("straße"),
		AddressPostalCode:     mapper.GetCol("plz"),
		AddressCity:           mapper.GetCol("ort"),
		Phone:                 mapper.GetRawCol("telefon"),
		ContactEmail:          mapper.GetCol("kontakt-email"),
		EmergencyContactName:  mapper.GetCol("notfallkontakt"),
		EmergencyContactPhone: mapper.GetRawCol("notfallkontakt telefon"),

		Qualifications:          mapper.GetCol("qualifikationen"),
		HasQualificationsColumn: mapper.HasColumn("qualifikationen"),
	}
}

// missingStaffColumns returns the required staff columns absent from the mapping.
func missingStaffColumns(mapping map[string]int) []string {
	var missing []string
	for _, col := range staffRequiredColumns {
		if _, ok := mapping[col]; !ok {
			missing = append(missing, col)
		}
	}
	return missing
}

// ParseStaffCSV parses a CSV file into staff import rows.
func ParseStaffCSV(reader io.Reader) ([]importModels.StaffImportRow, error) {
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
	if missing := missingStaffColumns(mapping); len(missing) > 0 {
		return nil, fmt.Errorf("fehlende erforderliche Spalten: %s", strings.Join(missing, ", "))
	}

	var rows []importModels.StaffImportRow
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
		rows = append(rows, MapStaffRow(mapper))
		rowNum++
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("die CSV-Datei enthält keine Datenzeilen. Möglicherweise haben Sie versehentlich die Vorlage hochgeladen")
	}

	return rows, nil
}

// ParseStaffXLSX parses an Excel (.xlsx) file into staff import rows.
func ParseStaffXLSX(reader io.Reader) ([]importModels.StaffImportRow, error) {
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
	if missing := missingStaffColumns(mapping); len(missing) > 0 {
		return nil, fmt.Errorf("fehlende erforderliche Spalten: %s", strings.Join(missing, ", "))
	}

	var rows []importModels.StaffImportRow
	for rowNum := 2; rowNum <= len(sheetRows); rowNum++ {
		values := sheetRows[rowNum-1]
		if isEmptyRow(values) {
			continue
		}
		mapper := NewColumnMapper(mapping, values)
		rows = append(rows, MapStaffRow(mapper))
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("die Excel-Datei enthält keine Datenzeilen. Möglicherweise haben Sie versehentlich die Vorlage hochgeladen")
	}

	return rows, nil
}
