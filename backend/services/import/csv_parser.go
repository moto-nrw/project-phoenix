package importpkg

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
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
	row := importModels.StudentImportRow{
		DataRetentionDays: 30, // Default
	}

	// Helper: Get column value safely with CSV injection protection
	getCol := func(colName string) string {
		idx, exists := p.columnMapping[colName]
		if !exists || idx < 0 || idx >= len(values) {
			return "" // Column doesn't exist or out of range
		}
		return sanitizeCellValue(strings.TrimSpace(values[idx]))
	}

	// Parse boolean ("Ja"/"Nein")
	parseBool := func(val string) bool {
		normalized := strings.ToLower(strings.TrimSpace(val))
		return normalized == "ja" || normalized == "yes" || normalized == "true" || normalized == "1"
	}

	// Map student fields
	row.FirstName = getCol("vorname")
	row.LastName = getCol("nachname")
	row.SchoolClass = getCol("klasse")
	row.GroupName = getCol("gruppe") // Human-readable name (e.g., "Gruppe 1A")
	row.Birthday = getCol("geburtstag")
	row.TagID = getCol("rfid")
	row.HealthInfo = getCol("gesundheitsinfo")
	row.SupervisorNotes = getCol("betreuernotizen")
	row.ExtraInfo = getCol("zusatzinfo")
	row.PickupStatus = getCol("abholstatus")
	row.BusPermission = parseBool(getCol("bus"))

	// Privacy consent
	row.PrivacyAccepted = parseBool(getCol("datenschutz"))
	if retentionStr := getCol("aufbewahrung(tage)"); retentionStr != "" {
		retention, err := strconv.Atoi(retentionStr)
		if err != nil {
			// GDPR CRITICAL: User provided invalid retention value (e.g., "30 Tage" instead of "30")
			// We MUST return an error instead of silently defaulting to 30 days
			// This prevents GDPR violations where user thinks they set 7 days but got 30
			return row, fmt.Errorf("ungültiger Wert für Aufbewahrung(Tage): '%s'. Bitte nur Zahlen verwenden (z.B. 7, 14, 30)", retentionStr)
		}
		// Store the parsed value - validation layer will handle range checking:
		// - < 1 returns error
		// - > 31 returns warning and caps to 31
		row.DataRetentionDays = retention
	}

	// AUTO-DETECT GUARDIANS (Erz1, Erz2, Erz3, ...)
	guardianNum := 1
	for {
		emailKey := fmt.Sprintf("erz%d.email", guardianNum)
		phoneKey := fmt.Sprintf("erz%d.telefon", guardianNum)
		mobileKey := fmt.Sprintf("erz%d.mobil", guardianNum)

		// Check if this guardian number exists in CSV
		_, hasEmail := p.columnMapping[emailKey]
		_, hasPhone := p.columnMapping[phoneKey]
		_, hasMobile := p.columnMapping[mobileKey]

		if !hasEmail && !hasPhone && !hasMobile {
			break // No more guardians
		}

		guardian := importModels.GuardianImportData{
			FirstName:          getCol(fmt.Sprintf("erz%d.vorname", guardianNum)),
			LastName:           getCol(fmt.Sprintf("erz%d.nachname", guardianNum)),
			Email:              getCol(emailKey),
			Phone:              getCol(phoneKey),
			MobilePhone:        getCol(mobileKey),
			RelationshipType:   getCol(fmt.Sprintf("erz%d.verhältnis", guardianNum)),
			IsPrimary:          parseBool(getCol(fmt.Sprintf("erz%d.primär", guardianNum))),
			IsEmergencyContact: parseBool(getCol(fmt.Sprintf("erz%d.notfall", guardianNum))),
			CanPickup:          parseBool(getCol(fmt.Sprintf("erz%d.abholung", guardianNum))),
		}

		// Parse flexible phone numbers into PhoneNumbers array
		guardian.PhoneNumbers = p.parseGuardianPhoneNumbers(guardianNum, getCol)

		// Only add if has contact info (skip empty guardians)
		hasPhoneNumbers := len(guardian.PhoneNumbers) > 0
		if guardian.Email != "" || guardian.Phone != "" || guardian.MobilePhone != "" || hasPhoneNumbers {
			row.Guardians = append(row.Guardians, guardian)
		}

		guardianNum++
	}

	return row, nil
}

// parseGuardianPhoneNumbers extracts phone numbers from CSV columns into PhoneImportData array
// Supported columns: Erz{N}.Telefon, Erz{N}.Telefon2, Erz{N}.Mobil, Erz{N}.Mobil2,
// Erz{N}.Dienstlich, Erz{N}.Dienstlich2
func (p *CSVParser) parseGuardianPhoneNumbers(guardianNum int, getCol func(string) string) []importModels.PhoneImportData {
	var phones []importModels.PhoneImportData
	priority := 1

	// Define phone column mappings: column suffix → (phone_type, label)
	phoneMappings := []struct {
		suffix    string
		phoneType string
		label     string
	}{
		// Home phones (Telefon)
		{"telefon", "home", ""},
		{"telefon2", "home", ""},
		// Mobile phones (Mobil)
		{"mobil", "mobile", ""},
		{"mobil2", "mobile", ""},
		// Work phones with labels
		{"dienstlich", "work", "Dienstlich"},
		{"dienstlich2", "work", "Dienstlich"},
	}

	for _, mapping := range phoneMappings {
		colKey := fmt.Sprintf("erz%d.%s", guardianNum, mapping.suffix)
		value := getCol(colKey)
		if value != "" {
			phones = append(phones, importModels.PhoneImportData{
				PhoneNumber: value,
				PhoneType:   mapping.phoneType,
				Label:       mapping.label,
				IsPrimary:   priority == 1, // First phone is primary
			})
			priority++
		}
	}

	return phones
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
