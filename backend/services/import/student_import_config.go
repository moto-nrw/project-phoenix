package importpkg

import (
	"context"
	"database/sql"
	stdErrors "errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/auth"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/import/ports"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

var (
	timeRegex   = regexp.MustCompile(`^([01]?[0-9]|2[0-3]):[0-5][0-9]$`)
	dateLayouts = []string{
		"2006-01-02",
		"02.01.2006",
	}
	errFutureBirthday = stdErrors.New("birthday cannot be in the future")
)

// isValidTimeFormat checks if a string is in HH:MM format
func isValidTimeFormat(s string) bool {
	return timeRegex.MatchString(s)
}

// MapRelationshipType converts German relationship types to valid English types
func MapRelationshipType(germanType string) string {
	normalized := strings.ToLower(strings.TrimSpace(germanType))

	// Map German terms to English types
	mapping := map[string]string{
		// Parent types
		"mutter":     "parent",
		"vater":      "parent",
		"mama":       "parent",
		"papa":       "parent",
		"elternteil": "parent",
		"parent":     "parent",

		// Guardian types
		"vormund":                "guardian",
		"erziehungsberechtigter": "guardian",
		"erziehungsberechtigte":  "guardian",
		"guardian":               "guardian",

		// Relative types
		"großmutter":  "relative",
		"großvater":   "relative",
		"oma":         "relative",
		"opa":         "relative",
		"tante":       "relative",
		"onkel":       "relative",
		"geschwister": "relative",
		"bruder":      "relative",
		"schwester":   "relative",
		"relative":    "relative",

		// Other types
		"sonstige": "other",
		"andere":   "other",
		"other":    "other",
	}

	if mapped, ok := mapping[normalized]; ok {
		return mapped
	}

	// Default to "other" for unknown types
	return "other"
}

// guardianRoleAliases maps the German labels of the "ErzN.Rolle" column (and
// the raw preset names) to the stored guardian_role presets.
var guardianRoleAliases = map[string]string{
	"hauptsorgeberechtigt": authorize.GuardianRolePrimaryGuardian, "hauptsorgeberechtigte": authorize.GuardianRolePrimaryGuardian, "hauptsorgeberechtigter": authorize.GuardianRolePrimaryGuardian,
	"sorgeberechtigt": authorize.GuardianRoleLegalGuardian, "sorgeberechtigte": authorize.GuardianRoleLegalGuardian, "sorgeberechtigter": authorize.GuardianRoleLegalGuardian,
	"mitsorgeberechtigt": authorize.GuardianRoleCoGuardian, "mitsorgeberechtigte": authorize.GuardianRoleCoGuardian, "mitsorgeberechtigter": authorize.GuardianRoleCoGuardian,
	"notfallkontakt": authorize.GuardianRoleEmergency,
	"nur abholung":   authorize.GuardianRolePickupOnly, "abholperson": authorize.GuardianRolePickupOnly, "abholung": authorize.GuardianRolePickupOnly,
	"sozialarbeit": authorize.GuardianRoleSocialWorker, "sozialarbeiter": authorize.GuardianRoleSocialWorker, "sozialarbeiterin": authorize.GuardianRoleSocialWorker,
	"benutzerdefiniert": authorize.GuardianRoleCustom,
}

// MapGuardianRole resolves a "ErzN.Rolle" cell to a stored preset. Returns
// ("", false) for an unknown label so the caller can report it; an empty cell
// maps to ("", true) meaning "derive the default".
func MapGuardianRole(raw string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "", true
	}
	if mapped, ok := guardianRoleAliases[normalized]; ok {
		return mapped, true
	}
	switch normalized {
	case authorize.GuardianRolePrimaryGuardian, authorize.GuardianRoleLegalGuardian, authorize.GuardianRoleCoGuardian,
		authorize.GuardianRoleEmergency, authorize.GuardianRolePickupOnly, authorize.GuardianRoleSocialWorker, authorize.GuardianRoleCustom:
		return normalized, true
	}
	return "", false
}

// StudentImportConfig implements ImportConfig for student imports. Every
// write goes through the owner commands in StudentImportDeps (#2708).
type StudentImportConfig struct {
	StudentImportDeps
}

// StudentImportDeps are the consumer-owned ports of the student import.
// Persons, Students and Guardians are the People Directory; Schedules is
// Care Plan; PrivacyConsents is Student Presence; ConsentHistory is the
// Audit platform recorder of the consent timestamps.
type StudentImportDeps struct {
	Persons         ports.PersonDirectory
	Students        ports.StudentDirectory
	Guardians       ports.GuardianDirectory
	Schedules       ports.StudentSchedules
	PrivacyConsents ports.PrivacyConsents
	// RFIDCardRepo resolves the optional RFID column to a card of this school
	// (#2600). nil disables RFID import (the column is then rejected).
	RFIDCardRepo   auth.RFIDCardRepository
	Resolver       *RelationshipResolver
	ConsentHistory usersService.StudentConsentChangeRecorder
}

// NewStudentImportConfig creates a new student import configuration.
func NewStudentImportConfig(deps StudentImportDeps) *StudentImportConfig {
	return &StudentImportConfig{StudentImportDeps: deps}
}

// GuardianPort is the guardian slice of the People Directory the student
// import consumes, and GuardianPortDecorator wraps it. Test compositions use
// the decorator to observe an owner failure mid-batch without naming the
// port package themselves.
type (
	GuardianPort          = ports.GuardianDirectory
	GuardianPortDecorator = func(GuardianPort) GuardianPort
)

// PreloadReferenceData loads all reference data (groups) for relationship resolution
func (c *StudentImportConfig) PreloadReferenceData(ctx context.Context) error {
	// Pre-load all groups for relationship resolution
	return c.Resolver.PreloadGroups(ctx)
}

// ValidateBatch flags rows that claim the same RFID card as an earlier row
// of the same file. The card is still free while the batch is validated, so
// only a batch-wide check can tell the second claim apart from a legitimate
// import; without it the owner refuses the second row at write time with a
// generic error instead of naming the card (#2708). The map is keyed by the
// zero-based slice index, and the message shows the 1-based file row
// including the header line, matching the engine's own row numbering.
func (c *StudentImportConfig) ValidateBatch(_ context.Context, rows []importModels.StudentImportRow) map[int][]importModels.ValidationError {
	result := make(map[int][]importModels.ValidationError)
	seen := make(map[string]int, len(rows))
	for i, row := range rows {
		tag := strings.TrimSpace(row.TagID)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if firstRow, duplicate := seen[key]; duplicate {
			result[i] = append(result[i], importModels.ValidationError{
				Field:       "tag_id",
				Message:     fmt.Sprintf("RFID-Karte '%s' ist in der Datei mehrfach vergeben (bereits in Zeile %d).", tag, firstRow+2),
				Code:        "duplicate_in_file",
				Severity:    importModels.ErrorSeverityError,
				ActualValue: tag,
			})
			continue
		}
		seen[key] = i
	}
	return result
}

// Validate validates a single row of student import data
func (c *StudentImportConfig) Validate(ctx context.Context, row *importModels.StudentImportRow) []importModels.ValidationError {
	errors := []importModels.ValidationError{}
	requiresCreateFields := true
	if mode := importModeFromContext(ctx); mode == importModels.ImportModeUpdate || mode == importModels.ImportModeUpsert {
		existing, findErr := c.FindExisting(ctx, *row)
		if findErr != nil {
			return []importModels.ValidationError{{Field: "student", Message: fmt.Sprintf("Kind konnte nicht geprüft werden: %s", findErr.Error()), Code: "existing_lookup_failed", Severity: importModels.ErrorSeverityError}}
		}
		requiresCreateFields = existing == nil
	}

	// 1. REQUIRED: Person validation
	if requiresCreateFields && strings.TrimSpace(row.FirstName) == "" {
		errors = append(errors, importModels.ValidationError{
			Field:    "first_name",
			Message:  "Vorname ist erforderlich",
			Code:     "required",
			Severity: importModels.ErrorSeverityError,
		})
	}

	if requiresCreateFields && strings.TrimSpace(row.LastName) == "" {
		errors = append(errors, importModels.ValidationError{
			Field:    "last_name",
			Message:  "Nachname ist erforderlich",
			Code:     "required",
			Severity: importModels.ErrorSeverityError,
		})
	}

	// 2. RFID: the card must exist at this school and must not be worn by
	// somebody else (#2600). A typo here would check the wrong child in at
	// the door without anyone noticing, so the row is blocked, not warned.
	errors = append(errors, c.validateTag(ctx, row)...)

	// 3. REQUIRED: Student validation
	if requiresCreateFields && strings.TrimSpace(row.SchoolClass) == "" {
		errors = append(errors, importModels.ValidationError{
			Field:    "school_class",
			Message:  "Klasse ist erforderlich",
			Code:     "required",
			Severity: importModels.ErrorSeverityError,
		})
	}

	// 4. OPTIONAL: Group resolution (with fuzzy matching)
	if row.GroupName != "" {
		groupID, groupErrors := c.Resolver.ResolveGroup(ctx, row.GroupName)
		if len(groupErrors) > 0 {
			errors = append(errors, groupErrors...)
		} else if groupID != nil {
			row.GroupID = groupID // Cache resolved ID
		}
	} else {
		// INFO: Group empty - student will be created without group
		errors = append(errors, importModels.ValidationError{
			Field:    "group",
			Message:  "Keine Gruppe zugewiesen. Das Kind wird ohne Gruppe erstellt.",
			Code:     "group_empty",
			Severity: importModels.ErrorSeverityInfo, // Non-blocking
		})
	}

	// 5. OPTIONAL: Guardian validation
	for i, guardian := range row.Guardians {
		guardianErrors := c.validateGuardian(i+1, guardian)
		errors = append(errors, guardianErrors...)

	}

	// 5b. OPTIONAL: Arrival and pickup schedule validation
	errors = append(errors, validateArrivalSchedules(row.ArrivalSchedules)...)
	errors = append(errors, validatePickupSchedules(row.PickupSchedules)...)

	// 5c. Coupled "mit wem" note: a row that sets any Gehweise.* cell to "Mit
	// anderem Kind" (accompanied) needs a non-blank Begleitung. Surface it in the
	// preview pass so the user sees the row error before importing, rather than
	// having createStudentFromRow build an accompanied student the model rejects
	// mid-import (#1694).
	if departurePlanFromImportRow(*row).HasMode(users.DepartureAccompanied) &&
		strings.TrimSpace(row.DepartureCompanionNote) == "" {
		errors = append(errors, importModels.ValidationError{
			Field:    "begleitung",
			Message:  "Begleitung ist erforderlich, wenn an einem Tag 'Mit anderem Kind' als Heimweg gewählt ist.",
			Code:     "required",
			Severity: importModels.ErrorSeverityError,
		})
	}

	// 6. Birthday validation (if provided)
	if trimmedBirthday := strings.TrimSpace(row.Birthday); trimmedBirthday != "" {
		parsedBirthday, err := parseSupportedDate(trimmedBirthday)
		if err != nil {
			message := "Ungültiges Datumsformat. Bitte verwenden Sie eines dieser Formate: JJJJ-MM-TT (z.B. 2015-08-15), TT.MM.JJJJ (z.B. 15.08.2015) oder TT.MM.JJ (z.B. 15.08.15)"
			code := "invalid_date_format"
			if stdErrors.Is(err, errFutureBirthday) {
				message = "Ungültiges Geburtsdatum. Geburtstage in der Zukunft sind nicht erlaubt."
				code = "invalid_date"
			}
			errors = append(errors, importModels.ValidationError{
				Field:    "birthday",
				Message:  message,
				Code:     code,
				Severity: importModels.ErrorSeverityError,
			})
		} else {
			row.Birthday = parsedBirthday.Format("2006-01-02")
		}
	} else {
		row.Birthday = ""
	}

	// 6b. Enrollment date range validation (if provided)
	errors = append(errors, validateEnrollmentDates(row)...)

	// 6c. Consent date validation (if provided)
	errors = append(errors, validateConsentDates(row)...)

	// 7. Privacy validation
	if row.DataRetentionDays < 1 {
		errors = append(errors, importModels.ValidationError{
			Field:    "data_retention_days",
			Message:  "Aufbewahrungsdauer muss mindestens 1 Tag sein",
			Code:     "invalid_range",
			Severity: importModels.ErrorSeverityError,
		})
	} else if row.DataRetentionDays > 31 {
		// Cap at 31 days with warning
		errors = append(errors, importModels.ValidationError{
			Field:    "data_retention_days",
			Message:  fmt.Sprintf("Aufbewahrungsdauer von %d Tagen überschreitet Maximum. Wird auf 31 Tage gesetzt.", row.DataRetentionDays),
			Code:     "value_capped",
			Severity: importModels.ErrorSeverityWarning,
		})
		row.DataRetentionDays = 31 // Cap to maximum
	}

	return errors
}

// validateTag resolves the RFID column to a card of this tenant and rewrites
// the cell to the stored spelling. Blocks the row when the card is unknown or
// already assigned to a different person. An occupied card is the import's
// strongest match key, so it is accepted only when its wearer is a student;
// names may legitimately change and are not an ownership proof.
func (c *StudentImportConfig) validateTag(ctx context.Context, row *importModels.StudentImportRow) []importModels.ValidationError {
	raw := strings.TrimSpace(row.TagID)
	if raw == "" {
		row.TagID = ""
		return nil
	}
	if c.RFIDCardRepo == nil {
		row.TagID = ""
		return []importModels.ValidationError{{
			Field:    "tag_id",
			Message:  "RFID-Karten können in dieser Installation nicht importiert werden. Bitte die Karte nach dem Import über die Geräteverwaltung zuweisen.",
			Code:     "rfid_not_supported",
			Severity: importModels.ErrorSeverityWarning,
		}}
	}

	lookupFailed := func(err error) []importModels.ValidationError {
		return []importModels.ValidationError{{
			Field:    "tag_id",
			Message:  fmt.Sprintf("RFID-Karte konnte nicht geprüft werden: %s", err.Error()),
			Code:     "rfid_lookup_failed",
			Severity: importModels.ErrorSeverityError,
		}}
	}
	card, err := c.RFIDCardRepo.FindByID(ctx, raw)
	if err != nil && !stdErrors.Is(err, sql.ErrNoRows) {
		return lookupFailed(err)
	}
	if card == nil {
		return []importModels.ValidationError{{
			Field:       "tag_id",
			Message:     fmt.Sprintf("RFID-Karte '%s' ist an dieser Schule nicht angelegt. Bitte die Karte zuerst in der Geräteverwaltung erfassen.", raw),
			Code:        "rfid_unknown",
			Severity:    importModels.ErrorSeverityError,
			ActualValue: raw,
		}}
	}
	row.TagID = card.ID

	wearer, err := c.Persons.FindPersonByTag(ctx, card.ID)
	if err != nil {
		if stdErrors.Is(err, ports.ErrPersonNotFound) {
			return nil
		}
		return lookupFailed(err)
	}
	students, err := c.Students.ListStudentsByPersonID(ctx, []int64{wearer.ID})
	if err != nil {
		return lookupFailed(err)
	}
	if len(students) > 0 {
		student := students[0]
		if strings.TrimSpace(row.FirstName) != "" && strings.TrimSpace(row.LastName) != "" && strings.TrimSpace(row.SchoolClass) != "" {
			matched, err := c.findStudentWithoutTag(ctx, *row)
			if err != nil {
				return lookupFailed(err)
			}
			if matched != nil && *matched != student.ID {
				return []importModels.ValidationError{{Field: "tag_id", Message: fmt.Sprintf("RFID-Karte '%s' ist bereits einer anderen Person zugeordnet.", raw), Code: "rfid_taken", Severity: importModels.ErrorSeverityError, ActualValue: raw}}
			}
		}
		return nil
	}
	return []importModels.ValidationError{{
		Field:       "tag_id",
		Message:     fmt.Sprintf("RFID-Karte '%s' ist bereits einer anderen Person zugeordnet.", raw),
		Code:        "rfid_taken",
		Severity:    importModels.ErrorSeverityError,
		ActualValue: raw,
	}}
}

// validateEnrollmentDates validates the optional enrollment date range and
// normalizes the row values to ISO format. Enrollment dates may legitimately
// lie in the future, so they are parsed without the birthday future-date check.
func validateEnrollmentDates(row *importModels.StudentImportRow) []importModels.ValidationError {
	var errors []importModels.ValidationError

	parseField := func(value *string, field, label string) *time.Time {
		trimmed := strings.TrimSpace(*value)
		if trimmed == "" {
			*value = ""
			return nil
		}
		parsed, err := parseDateFormats(trimmed)
		if err != nil {
			errors = append(errors, importModels.ValidationError{
				Field:    field,
				Message:  fmt.Sprintf("Ungültiges Datumsformat für '%s'. Bitte verwenden Sie JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ.", label),
				Code:     "invalid_date_format",
				Severity: importModels.ErrorSeverityError,
			})
			return nil
		}
		*value = parsed.Format("2006-01-02")
		return &parsed
	}

	from := parseField(&row.EnrolledFrom, "enrolled_from", "Einschreibung von")
	until := parseField(&row.EnrolledUntil, "enrolled_until", "Einschreibung bis")

	if from != nil && until != nil && from.After(*until) {
		errors = append(errors, importModels.ValidationError{
			Field:    "enrolled_until",
			Message:  "'Einschreibung bis' darf nicht vor 'Einschreibung von' liegen.",
			Code:     "invalid_date_range",
			Severity: importModels.ErrorSeverityError,
		})
	}

	return errors
}

// validateConsentDates validates the optional consent date columns (AGB, data
// processing, email contact, photo) and normalizes them to ISO format. A
// consent cannot have been given in the future, so future dates are rejected.
func validateConsentDates(row *importModels.StudentImportRow) []importModels.ValidationError {
	var errors []importModels.ValidationError

	validate := func(value *string, field, label string) {
		trimmed := strings.TrimSpace(*value)
		if trimmed == "" {
			*value = ""
			return
		}
		parsed, err := parseDateFormats(trimmed)
		if err != nil {
			errors = append(errors, importModels.ValidationError{
				Field:    field,
				Message:  fmt.Sprintf("Ungültiges Datumsformat für '%s'. Bitte verwenden Sie JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ.", label),
				Code:     "invalid_date_format",
				Severity: importModels.ErrorSeverityError,
			})
			return
		}
		if validateBirthdayDate(parsed) != nil {
			errors = append(errors, importModels.ValidationError{
				Field:    field,
				Message:  fmt.Sprintf("Einwilligungsdatum für '%s' darf nicht in der Zukunft liegen.", label),
				Code:     "invalid_date",
				Severity: importModels.ErrorSeverityError,
			})
			return
		}
		*value = parsed.Format("2006-01-02")
	}

	validate(&row.AGBAcceptedAt, "agb_accepted_at", "AGB akzeptiert am")
	validate(&row.DataProcessingAcceptedAt, "data_processing_accepted_at", "Datenverarbeitung akzeptiert am")
	validate(&row.EmailContactAcceptedAt, "email_contact_accepted_at", "E-Mail-Kontakt akzeptiert am")
	validate(&row.PhotoConsentGivenAt, "photo_consent_given_at", "Foto-Einwilligung am")

	return errors
}

// validateArrivalSchedules validates all arrival schedule entries
func validateArrivalSchedules(schedules []importModels.ArrivalScheduleImportData) []importModels.ValidationError {
	var errors []importModels.ValidationError
	for _, sched := range schedules {
		if sched.Weekday < 1 || sched.Weekday > 5 {
			errors = append(errors, importModels.ValidationError{
				Field:    "arrival_schedule",
				Message:  fmt.Sprintf("Ungültiger Wochentag %d. Erlaubt: 1 (Mo) bis 5 (Fr)", sched.Weekday),
				Code:     "invalid_weekday",
				Severity: importModels.ErrorSeverityError,
			})
		}
		if !isValidTimeFormat(sched.ExpectedArrival) {
			errors = append(errors, importModels.ValidationError{
				Field:    "arrival_schedule",
				Message:  fmt.Sprintf("Ungültiges Zeitformat '%s'. Bitte HH:MM verwenden (z.B. 08:00)", sched.ExpectedArrival),
				Code:     "invalid_time_format",
				Severity: importModels.ErrorSeverityError,
			})
		}
	}
	return errors
}

// validatePickupSchedules validates all pickup schedule entries
func validatePickupSchedules(schedules []importModels.PickupScheduleImportData) []importModels.ValidationError {
	var errors []importModels.ValidationError
	for _, sched := range schedules {
		if sched.Weekday < 1 || sched.Weekday > 5 {
			errors = append(errors, importModels.ValidationError{
				Field:    "pickup_schedule",
				Message:  fmt.Sprintf("Ungültiger Wochentag %d. Erlaubt: 1 (Mo) bis 5 (Fr)", sched.Weekday),
				Code:     "invalid_weekday",
				Severity: importModels.ErrorSeverityError,
			})
		}
		if !isValidTimeFormat(sched.PickupTime) {
			errors = append(errors, importModels.ValidationError{
				Field:    "pickup_schedule",
				Message:  fmt.Sprintf("Ungültiges Zeitformat '%s'. Bitte HH:MM verwenden (z.B. 15:30)", sched.PickupTime),
				Code:     "invalid_time_format",
				Severity: importModels.ErrorSeverityError,
			})
		}
	}
	return errors
}

// validateGuardian validates a single guardian's data
func (c *StudentImportConfig) validateGuardian(num int, guardian importModels.GuardianImportData) []importModels.ValidationError {
	errors := []importModels.ValidationError{}
	fieldPrefix := fmt.Sprintf("guardian_%d", num)

	// Check contact methods: either legacy fields or new PhoneNumbers array
	hasLegacyContact := guardian.Email != "" || guardian.Phone != "" || guardian.MobilePhone != ""
	hasNewPhoneNumbers := len(guardian.PhoneNumbers) > 0

	// At least one contact method required
	if !hasLegacyContact && !hasNewPhoneNumbers {
		errors = append(errors, importModels.ValidationError{
			Field:    fieldPrefix,
			Message:  fmt.Sprintf("Erziehungsberechtigter %d benötigt mindestens eine Kontaktmethode (Email, Telefon oder Mobil)", num),
			Code:     "guardian_contact_required",
			Severity: importModels.ErrorSeverityError,
		})
		return errors // Return early if no contact info
	}

	// Validate email, legacy phones, and new phone numbers
	errors = append(errors, validateGuardianEmail(num, guardian.Email, fieldPrefix)...)
	errors = append(errors, validateGuardianLegacyPhones(num, guardian, fieldPrefix)...)
	errors = append(errors, validateGuardianPhoneNumbers(num, guardian.PhoneNumbers, fieldPrefix)...)

	// Validate language preference (warning for unrecognized codes)
	errors = append(errors, validateGuardianLanguage(num, guardian.LanguagePreference, fieldPrefix)...)

	if _, ok := MapGuardianRole(guardian.GuardianRole); !ok {
		errors = append(errors, importModels.ValidationError{
			Field:       fmt.Sprintf("%s_role", fieldPrefix),
			Message:     fmt.Sprintf("Unbekannte Rolle '%s' für Erziehungsberechtigten %d. Erlaubt: Hauptsorgeberechtigt, Sorgeberechtigt, Mitsorgeberechtigt, Notfallkontakt, Nur Abholung, Sozialarbeit.", guardian.GuardianRole, num),
			Code:        "invalid_guardian_role",
			Severity:    importModels.ErrorSeverityError,
			ActualValue: guardian.GuardianRole,
		})
	}
	if guardian.EmergencyPriority < 0 {
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_emergency_priority", fieldPrefix),
			Message:  fmt.Sprintf("Notfallpriorität für Erziehungsberechtigten %d muss 1 oder größer sein.", num),
			Code:     "invalid_emergency_priority",
			Severity: importModels.ErrorSeverityError,
		})
	}

	return errors
}

// supportedLanguages contains ISO 639-1 codes common in German school contexts
var supportedLanguages = map[string]bool{
	"de": true, "en": true, "tr": true, "ar": true, "ru": true,
	"pl": true, "fa": true, "ku": true, "fr": true, "es": true,
	"it": true, "pt": true, "ro": true, "uk": true, "sr": true,
	"hr": true, "bg": true, "el": true, "vi": true, "zh": true,
}

// validateGuardianLanguage warns about unrecognized language codes
func validateGuardianLanguage(num int, lang, fieldPrefix string) []importModels.ValidationError {
	if lang == "" {
		return nil // Empty is fine — backend defaults to "de"
	}
	normalized := strings.ToLower(strings.TrimSpace(lang))
	if !supportedLanguages[normalized] {
		return []importModels.ValidationError{{
			Field:    fmt.Sprintf("%s_language", fieldPrefix),
			Message:  fmt.Sprintf("Unbekannter Sprachcode '%s' für Erziehungsberechtigten %d. Gängige Codes: de, en, tr, ar, ru, pl", normalized, num),
			Code:     "unknown_language",
			Severity: importModels.ErrorSeverityWarning,
		}}
	}
	return nil
}

// validateGuardianEmail validates email format
func validateGuardianEmail(num int, email, fieldPrefix string) []importModels.ValidationError {
	if email != "" && !users.IsValidEmailFormat(email) {
		return []importModels.ValidationError{{
			Field:    fmt.Sprintf("%s_email", fieldPrefix),
			Message:  fmt.Sprintf("Ungültiges Email-Format für Erziehungsberechtigten %d: %s", num, email),
			Code:     "invalid_email",
			Severity: importModels.ErrorSeverityError,
		}}
	}
	return nil
}

// validateGuardianLegacyPhones validates legacy phone and mobile_phone fields
func validateGuardianLegacyPhones(num int, guardian importModels.GuardianImportData, fieldPrefix string) []importModels.ValidationError {
	var errors []importModels.ValidationError

	if guardian.Phone != "" && users.ValidateOptionalPhone(guardian.Phone) != nil {
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_phone", fieldPrefix),
			Message:  fmt.Sprintf("Ungültiges Telefon-Format für Erziehungsberechtigten %d: %s", num, guardian.Phone),
			Code:     "invalid_phone",
			Severity: importModels.ErrorSeverityError,
		})
	}

	if guardian.MobilePhone != "" && users.ValidateOptionalPhone(guardian.MobilePhone) != nil {
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_mobile", fieldPrefix),
			Message:  fmt.Sprintf("Ungültiges Mobiltelefon-Format für Erziehungsberechtigten %d: %s", num, guardian.MobilePhone),
			Code:     "invalid_phone",
			Severity: importModels.ErrorSeverityError,
		})
	}

	return errors
}

// validateGuardianPhoneNumbers validates phone numbers from the new flexible PhoneNumbers array
func validateGuardianPhoneNumbers(num int, phones []importModels.PhoneImportData, fieldPrefix string) []importModels.ValidationError {
	var errors []importModels.ValidationError

	for i, phone := range phones {
		if phone.PhoneNumber == "" || users.ValidateOptionalPhone(phone.PhoneNumber) == nil {
			continue
		}
		label := phone.Label
		if label == "" {
			label = phone.PhoneType
		}
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_phone_%d", fieldPrefix, i+1),
			Message:  fmt.Sprintf("Ungültiges Telefon-Format für Erziehungsberechtigten %d (%s): %s", num, label, phone.PhoneNumber),
			Code:     "invalid_phone",
			Severity: importModels.ErrorSeverityError,
		})
	}

	return errors
}

// FindExisting resolves the row to an existing student (duplicate detection
// in create mode, match key in update mode). Keys, in order: the RFID card
// (survives a class change), first + last name + class, and first + last name
// + birthday (the class-change case without a card).
func busDaysFromImportRow(row importModels.StudentImportRow) users.BusDays {
	if row.BusDays != nil {
		days := users.BusDays{}
		for _, key := range users.BusDayOrder {
			if row.BusDays[key] {
				days[key] = true
			}
		}
		return days
	}
	return users.BusDaysFromLegacyFlag(row.BusPermission)
}

// departurePlanFromImportRow resolves the unified per-day departure plan from an
// import row. The current per-day "Gehweise.Mo".."Gehweise.Fr" columns take
// precedence; otherwise the legacy Bus(.Mo..Fr) and Abholstatus columns are
// folded into the plan so old templates keep importing. departure_days is the
// single source of truth (#1610).
func departurePlanFromImportRow(row importModels.StudentImportRow) users.DepartureDays {
	if row.DepartureDays != nil {
		out := users.DepartureDays{}
		for _, key := range users.PickupDayOrder {
			switch row.DepartureDays[key] {
			case string(users.DepartureAlone):
				out[key] = users.DepartureAlone
			case string(users.DepartureBus):
				out[key] = users.DepartureBus
			case string(users.DeparturePickup):
				out[key] = users.DeparturePickup
			case string(users.DepartureAccompanied):
				out[key] = users.DepartureAccompanied
			}
		}
		return out
	}
	bus := busDaysFromImportRow(row)
	pickup := users.PickupDaysFromLegacyStatus(row.PickupStatus)
	return users.DepartureDaysFromLegacy(bus, pickup)
}

// createStudentFromRow creates a student from person and row
func enrollmentStartsInFuture(enrolledFrom *timezone.Date) bool {
	return enrollmentStartsAfter(enrolledFrom, timezone.TodayDate())
}

func enrollmentStartsAfter(enrolledFrom *timezone.Date, today timezone.Date) bool {
	return enrolledFrom != nil && enrolledFrom.After(today)
}

// createGuardianRelationships creates all guardian relationships
// validateRetentionDays validates and normalizes retention days
func validateRetentionDays(days int) int {
	if days < 1 {
		return 30 // Default to 30 if invalid
	}
	if days > 31 {
		return 31 // Cap to maximum
	}
	return days
}

// createOrFindGuardian deduplicates guardians by email
// ptrEquals checks if a *string equals a plain string value
func ptrEquals(ptr *string, val string) bool {
	return ptr != nil && *ptr == val
}

// createGuardianPhoneNumbers creates phone numbers for a guardian from import data
// mapPhoneType converts import phone type string to users.PhoneType enum
func mapPhoneType(importType string) users.PhoneType {
	switch strings.ToLower(importType) {
	case "mobile":
		return users.PhoneTypeMobile
	case "home":
		return users.PhoneTypeHome
	case "work":
		return users.PhoneTypeWork
	default:
		return users.PhoneTypeOther
	}
}

// Update patches an existing student (#2600). Empty cells never clear a
// stored value; only what the row carries is written. Guardians are merged
// (matched by e-mail, new ones linked), schedules are replaced per weekday
// given, privacy consent is only created when none exists yet.
// importGuardianPhones collects every phone number of the row in normalized
// form, including the legacy single-column fields.
func importGuardianPhones(data importModels.GuardianImportData) map[string]struct{} {
	phones := make(map[string]struct{}, len(data.PhoneNumbers)+2)
	add := func(raw string) {
		if normalized := normalizeImportPhone(raw); normalized != "" {
			phones[normalized] = struct{}{}
		}
	}
	for _, phone := range data.PhoneNumbers {
		add(phone.PhoneNumber)
	}
	add(data.Phone)
	add(data.MobilePhone)
	return phones
}

// normalizeImportPhone reduces a phone number to its digits (a leading "+" is
// kept) so that "0221 / 123-45" and "0221 12345" compare equal.
func normalizeImportPhone(raw string) string {
	var b strings.Builder
	for i, r := range strings.TrimSpace(raw) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// EntityName returns the entity type name
func (c *StudentImportConfig) EntityName() string {
	return "student"
}

// Helper functions

// parseSupportedDate tries all supported import date formats in order.
// parseDateFormats parses a date string in any supported format
// (YYYY-MM-DD, DD.MM.YYYY, DD.MM.YY) without applying semantic restrictions.
// Use this for dates that may legitimately lie in the future (e.g. enrollment
// start dates). Birthdays must go through parseSupportedDate instead.
func parseDateFormats(dateStr string) (time.Time, error) {
	var lastErr error
	for _, layout := range dateLayouts {
		parsed, err := time.Parse(layout, dateStr)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}

	if shortDate, err := parseGermanShortDate(dateStr); err == nil {
		return shortDate, nil
	}

	return time.Time{}, lastErr
}

func parseSupportedDate(dateStr string) (time.Time, error) {
	parsed, err := parseDateFormats(dateStr)
	if err != nil {
		return time.Time{}, err
	}
	if err := validateBirthdayDate(parsed); err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func parseGermanShortDate(dateStr string) (time.Time, error) {
	parts := strings.Split(dateStr, ".")
	if len(parts) != 3 || len(parts[2]) != 2 {
		return time.Time{}, fmt.Errorf("invalid short German date format")
	}

	day, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	shortYear, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, err
	}

	year := 2000 + shortYear
	parsed := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if parsed.Year() != year || int(parsed.Month()) != month || parsed.Day() != day {
		return time.Time{}, fmt.Errorf("invalid short German date value")
	}

	return parsed, nil
}

func validateBirthdayDate(parsed time.Time) error {
	today := time.Now().In(time.UTC)
	currentDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if parsed.After(currentDate) {
		return errFutureBirthday
	}

	return nil
}

// parseOptionalImportDate parses an optional date in any supported format,
// allowing future dates. Returns nil for empty or unparseable input (format
// errors are surfaced separately during validation). Shared by enrollment and
// consent date columns.
func parseOptionalImportDate(dateStr string) *time.Time {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return nil
	}
	parsed, err := parseDateFormats(trimmed)
	if err != nil {
		return nil
	}
	return &parsed
}

// parseOptionalImportCalendarDate is the calendar-date sibling of
// parseOptionalImportDate for DATE-typed columns (enrollment window).
func parseOptionalImportCalendarDate(dateStr string) *timezone.Date {
	parsed := parseOptionalImportDate(dateStr)
	if parsed == nil {
		return nil
	}
	d := timezone.DateFromTime(*parsed)
	return &d
}

// parseOptionalDate parses a date string or returns nil
func parseOptionalDate(dateStr string) (*timezone.Date, error) {
	trimmed := strings.TrimSpace(dateStr)
	if trimmed == "" {
		return nil, nil
	}

	t, err := parseSupportedDate(trimmed)
	if err != nil {
		return nil, err
	}

	d := timezone.DateFromTime(t)
	return &d, nil
}

// boundedNotePtr trims the value, truncates it to the companion-note cap by
// rune count (multibyte-safe), and returns nil when empty. Import truncates
// rather than rejecting the whole row, mirroring the enrollment intake (#1694).
func boundedNotePtr(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > users.MaxDepartureCompanionNoteLen {
		trimmed = string([]rune(trimmed)[:users.MaxDepartureCompanionNoteLen])
	}
	return &trimmed
}

// guardianLanguagePreference returns a language preference, defaulting to "de"
func guardianLanguagePreference(val string) string {
	if val == "" {
		return "de"
	}
	return strings.ToLower(strings.TrimSpace(val))
}

// createArrivalSchedules creates weekly arrival schedule records for a student
