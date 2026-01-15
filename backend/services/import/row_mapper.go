package importpkg

import (
	"fmt"
	"strconv"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// columnGetter is a function that retrieves a column value by name
type columnGetter func(colName string) string

// columnChecker is a function that checks if a column exists
type columnChecker func(colName string) bool

// mapStudentRowWithHelpers maps values to StudentImportRow using the provided helpers.
// This is the shared implementation used by both CSVParser and XLSXParser.
func mapStudentRowWithHelpers(getCol columnGetter, hasColumn columnChecker) (importModels.StudentImportRow, error) {
	row := importModels.StudentImportRow{
		DataRetentionDays: 30, // Default
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

		// Check if this guardian number exists
		if !hasColumn(emailKey) && !hasColumn(phoneKey) && !hasColumn(mobileKey) {
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

		// Only add if has contact info (skip empty guardians)
		if guardian.Email != "" || guardian.Phone != "" || guardian.MobilePhone != "" {
			row.Guardians = append(row.Guardians, guardian)
		}

		guardianNum++
	}

	return row, nil
}
