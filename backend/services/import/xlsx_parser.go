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

	// Helper: Get raw column value without sanitization (for phone numbers)
	// Phone numbers may start with + (international format) which would be corrupted by sanitization
	// Phone numbers go through validation anyway, so CSV injection is not a risk
	getRawCol := func(colName string) string {
		idx, exists := p.columnMapping[colName]
		if !exists || idx < 0 || idx >= len(values) {
			return ""
		}
		return strings.TrimSpace(values[idx])
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

		// Check if this guardian number exists in Excel
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
			Phone:              getRawCol(phoneKey),  // Use raw for phone (may start with +)
			MobilePhone:        getRawCol(mobileKey), // Use raw for phone (may start with +)
			RelationshipType:   getCol(fmt.Sprintf("erz%d.verhältnis", guardianNum)),
			IsPrimary:          parseBool(getCol(fmt.Sprintf("erz%d.primär", guardianNum))),
			IsEmergencyContact: parseBool(getCol(fmt.Sprintf("erz%d.notfall", guardianNum))),
			CanPickup:          parseBool(getCol(fmt.Sprintf("erz%d.abholung", guardianNum))),
		}

		// Parse flexible phone numbers into PhoneNumbers array (use getRawCol for phone columns)
		guardian.PhoneNumbers = p.parseGuardianPhoneNumbers(guardianNum, getRawCol)

		// Only add if has contact info (skip empty guardians)
		hasPhoneNumbers := len(guardian.PhoneNumbers) > 0
		if guardian.Email != "" || guardian.Phone != "" || guardian.MobilePhone != "" || hasPhoneNumbers {
			row.Guardians = append(row.Guardians, guardian)
		}

		guardianNum++
	}

	return row, nil
}

// parseGuardianPhoneNumbers extracts phone numbers from Excel columns into PhoneImportData array
// Supported columns: Erz{N}.Telefon, Erz{N}.Telefon2, Erz{N}.Mobil, Erz{N}.Mobil2,
// Erz{N}.Dienstlich, Erz{N}.Dienstlich2
func (p *XLSXParser) parseGuardianPhoneNumbers(guardianNum int, getCol func(string) string) []importModels.PhoneImportData {
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
