package importpkg

import (
	"context"
	stdErrors "errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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

// StudentImportConfig implements ImportConfig for student imports
type StudentImportConfig struct {
	StudentImportDeps
	txHandler *base.TxHandler
}

// StudentImportDeps contains dependencies for StudentImportConfig
type StudentImportDeps struct {
	PersonRepo          users.PersonRepository
	StudentRepo         users.StudentRepository
	GuardianRepo        users.GuardianProfileRepository
	GuardianPhoneRepo   users.GuardianPhoneNumberRepository
	RelationRepo        users.StudentGuardianRepository
	PrivacyRepo         users.PrivacyConsentRepository
	ArrivalScheduleRepo scheduleModels.StudentArrivalScheduleRepository
	PickupScheduleRepo  scheduleModels.StudentPickupScheduleRepository
	Resolver            *RelationshipResolver
}

// NewStudentImportConfig creates a new student import configuration
// Note: RFID cards are not supported in CSV import and must be assigned separately
func NewStudentImportConfig(deps StudentImportDeps, db *bun.DB) *StudentImportConfig {
	return &StudentImportConfig{
		StudentImportDeps: deps,
		txHandler:         base.NewTxHandler(db),
	}
}

// PreloadReferenceData loads all reference data (groups) for relationship resolution
func (c *StudentImportConfig) PreloadReferenceData(ctx context.Context) error {
	// Pre-load all groups for relationship resolution
	return c.Resolver.PreloadGroups(ctx)
}

// Validate validates a single row of student import data
func (c *StudentImportConfig) Validate(ctx context.Context, row *importModels.StudentImportRow) []importModels.ValidationError {
	errors := []importModels.ValidationError{}

	// 1. REQUIRED: Person validation
	if strings.TrimSpace(row.FirstName) == "" {
		errors = append(errors, importModels.ValidationError{
			Field:    "first_name",
			Message:  "Vorname ist erforderlich",
			Code:     "required",
			Severity: importModels.ErrorSeverityError,
		})
	}

	if strings.TrimSpace(row.LastName) == "" {
		errors = append(errors, importModels.ValidationError{
			Field:    "last_name",
			Message:  "Nachname ist erforderlich",
			Code:     "required",
			Severity: importModels.ErrorSeverityError,
		})
	}

	// 2. RFID cards are not supported in CSV import - always clear
	// RFID cards must be assigned separately after import via the device management interface
	if row.TagID != "" {
		errors = append(errors, importModels.ValidationError{
			Field:    "tag_id",
			Message:  "RFID-Karten werden beim CSV-Import nicht unterstützt. Bitte weisen Sie RFID-Karten nach dem Import über die Geräteverwaltung zu.",
			Code:     "rfid_not_supported",
			Severity: importModels.ErrorSeverityInfo,
		})
		row.TagID = "" // Always clear - RFID cards not supported in CSV import
	}

	// 3. REQUIRED: Student validation
	if strings.TrimSpace(row.SchoolClass) == "" {
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

// FindExisting checks if a student already exists (for duplicate detection)
func (c *StudentImportConfig) FindExisting(ctx context.Context, row importModels.StudentImportRow) (*int64, error) {
	// Strategy: Find by exact first_name + last_name + school_class match
	students, err := c.StudentRepo.FindByNameAndClass(ctx, row.FirstName, row.LastName, row.SchoolClass)
	if err != nil {
		return nil, err
	}

	if len(students) == 0 {
		return nil, nil // No existing student
	}

	if len(students) == 1 {
		return &students[0].ID, nil
	}

	// Multiple matches - ambiguous
	return nil, fmt.Errorf("mehrere Kinder gefunden mit Name '%s %s' in Klasse '%s'",
		row.FirstName, row.LastName, row.SchoolClass)
}

// Create creates a new student with all related entities.
// Uses a PostgreSQL savepoint for per-row atomicity within the outer tenant transaction.
// If any step fails (e.g. person created but student fails), ROLLBACK TO SAVEPOINT
// cleans up partial records while keeping the outer tx alive for other rows.
func (c *StudentImportConfig) Create(ctx context.Context, row importModels.StudentImportRow) (int64, error) {
	tx, hasTx := base.TxFromContext(ctx)
	if hasTx {
		if _, err := tx.ExecContext(ctx, "SAVEPOINT import_row"); err != nil {
			return 0, fmt.Errorf("savepoint: %w", err)
		}

		studentID, err := c.createAllEntities(ctx, row)
		if err != nil {
			_, _ = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT import_row")
			return 0, err
		}

		_, _ = tx.ExecContext(ctx, "RELEASE SAVEPOINT import_row")
		return studentID, nil
	}

	// Fallback: no outer tx (shouldn't happen in normal HTTP flow)
	return c.createAllEntities(ctx, row)
}

// createAllEntities creates person, student, guardians, privacy consent, and weekly schedules.
func (c *StudentImportConfig) createAllEntities(ctx context.Context, row importModels.StudentImportRow) (int64, error) {
	person, err := c.createPersonFromRow(ctx, row)
	if err != nil {
		return 0, err
	}

	student, err := c.createStudentFromRow(ctx, person.ID, row)
	if err != nil {
		return 0, err
	}

	if err := c.createGuardianRelationships(ctx, student.ID, row.Guardians); err != nil {
		return 0, err
	}

	if err := c.createPrivacyConsentIfNeeded(ctx, student.ID, row); err != nil {
		return 0, err
	}

	if err := c.createArrivalSchedules(ctx, student.ID, row.ArrivalSchedules); err != nil {
		return 0, err
	}

	if err := c.createPickupSchedules(ctx, student.ID, row.PickupSchedules); err != nil {
		return 0, err
	}

	return student.ID, nil
}

// createPersonFromRow creates a person from import row
func (c *StudentImportConfig) createPersonFromRow(ctx context.Context, row importModels.StudentImportRow) (*users.Person, error) {
	birthday, _ := parseOptionalDate(row.Birthday)
	person := &users.Person{
		FirstName: strings.TrimSpace(row.FirstName),
		LastName:  strings.TrimSpace(row.LastName),
		Birthday:  birthday,
		TagID:     nil, // RFID cards not supported in CSV import
	}
	person.SetTenantID(tenant.FromContext(ctx))

	if err := c.PersonRepo.Create(ctx, person); err != nil {
		return nil, fmt.Errorf("create person: %w", err)
	}

	return person, nil
}

// busDaysFromImportRow resolves the student's bus_days from an import row.
// Per-day "Bus.Mo".."Bus.Fr" columns take precedence; otherwise the legacy
// single "Bus" column maps to all weekdays (Mo–Fr) when true, no days when
// false. bus_days is the single source of truth (#1582).
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
func (c *StudentImportConfig) createStudentFromRow(ctx context.Context, personID int64, row importModels.StudentImportRow) (*users.Student, error) {
	enrolledFrom := parseOptionalImportCalendarDate(row.EnrolledFrom)
	enrolledUntil := parseOptionalImportCalendarDate(row.EnrolledUntil)

	student := &users.Student{
		PersonID:        personID,
		SchoolClass:     strings.TrimSpace(row.SchoolClass),
		GroupID:         row.GroupID,
		ExtraInfo:       strutil.TrimToNil(row.ExtraInfo),
		SupervisorNotes: strutil.TrimToNil(row.SupervisorNotes),
		HealthInfo:      strutil.TrimToNil(row.HealthInfo),
		// DepartureDays is the unified source of truth; the repository derives
		// bus_days, pickup_days and pickup_status from it on persist (#1610).
		DepartureDays: departurePlanFromImportRow(row),
		// Free-text "mit wem" for the accompanied mode; the repository clears it
		// on persist when no day is accompanied, so it never outlives the mode.
		DepartureCompanionNote:   boundedNotePtr(row.DepartureCompanionNote),
		EnrolledFrom:             enrolledFrom,
		EnrolledUntil:            enrolledUntil,
		AGBAcceptedAt:            parseOptionalImportDate(row.AGBAcceptedAt),
		DataProcessingAcceptedAt: parseOptionalImportDate(row.DataProcessingAcceptedAt),
		EmailContactAcceptedAt:   parseOptionalImportDate(row.EmailContactAcceptedAt),
		// Photo consent date is set; "given_by" is intentionally left nil on import.
		PhotoConsentGivenAt: parseOptionalImportDate(row.PhotoConsentGivenAt),
	}

	// A future enrollment start means the student isn't active yet. Mark them
	// pending so the activate-students scheduler flips them to active once
	// enrolled_from arrives (mirrors the parent-enrollment flow). Without a
	// future start date the DB default ('active') applies.
	if enrollmentStartsInFuture(enrolledFrom) {
		student.Status = users.StudentStatusPending
	}

	student.SetTenantID(tenant.FromContext(ctx))

	if err := c.StudentRepo.Create(ctx, student); err != nil {
		return nil, fmt.Errorf("create student: %w", err)
	}

	return student, nil
}

func enrollmentStartsInFuture(enrolledFrom *timezone.Date) bool {
	return enrolledFrom != nil && enrolledFrom.After(timezone.TodayDate())
}

// createGuardianRelationships creates all guardian relationships
func (c *StudentImportConfig) createGuardianRelationships(ctx context.Context, studentID int64, guardians []importModels.GuardianImportData) error {
	for i, guardianData := range guardians {
		if err := c.createSingleGuardianRelationship(ctx, studentID, guardianData, i+1); err != nil {
			return err
		}
	}
	return nil
}

// createSingleGuardianRelationship creates a single guardian relationship
func (c *StudentImportConfig) createSingleGuardianRelationship(ctx context.Context, studentID int64, guardianData importModels.GuardianImportData, index int) error {
	guardianID, err := c.createOrFindGuardian(ctx, guardianData)
	if err != nil {
		return fmt.Errorf("guardian %d: %w", index, err)
	}

	relationship := &users.StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  guardianID,
		RelationshipType:   MapRelationshipType(guardianData.RelationshipType),
		IsPrimary:          guardianData.IsPrimary,
		IsEmergencyContact: guardianData.IsEmergencyContact,
		CanPickup:          guardianData.CanPickup,
	}
	authorize.ApplyDefaultStudentGuardianRole(relationship)
	relationship.SetTenantID(tenant.FromContext(ctx))

	if err := c.RelationRepo.Create(ctx, relationship); err != nil {
		return fmt.Errorf("create relationship %d: %w", index, err)
	}

	return nil
}

// createPrivacyConsentIfNeeded creates privacy consent if specified in row.
// Only creates consent if privacy is explicitly accepted OR a valid retention period (>0) is specified.
func (c *StudentImportConfig) createPrivacyConsentIfNeeded(ctx context.Context, studentID int64, row importModels.StudentImportRow) error {
	// Skip if privacy not accepted AND no valid retention days specified
	// This prevents creating consent for negative/zero/missing retention values
	if !row.PrivacyAccepted && row.DataRetentionDays <= 0 {
		return nil
	}

	consent := buildPrivacyConsent(studentID, row)
	if err := c.PrivacyRepo.Create(ctx, consent); err != nil {
		return fmt.Errorf("create privacy consent: %w", err)
	}

	return nil
}

// buildPrivacyConsent builds a privacy consent object
func buildPrivacyConsent(studentID int64, row importModels.StudentImportRow) *users.PrivacyConsent {
	retentionDays := validateRetentionDays(row.DataRetentionDays)

	consent := &users.PrivacyConsent{
		StudentID:         studentID,
		PolicyVersion:     "1.0",
		Accepted:          row.PrivacyAccepted,
		DataRetentionDays: retentionDays,
	}

	if row.PrivacyAccepted {
		now := time.Now()
		consent.AcceptedAt = &now
	}

	return consent
}

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
func (c *StudentImportConfig) createOrFindGuardian(ctx context.Context, data importModels.GuardianImportData) (int64, error) {
	// Deduplication strategy: Email is unique identifier
	if data.Email != "" {
		existing, err := c.GuardianRepo.FindByEmail(ctx, data.Email)
		// CRITICAL: Distinguish between "not found" and real DB errors
		// The repository converts sql.ErrNoRows to "guardian profile not found" message
		// This is NORMAL and means we should create a new guardian
		if err != nil {
			// Check if it's a "not found" error (expected and normal)
			if strings.Contains(err.Error(), "guardian profile not found") {
				// Guardian doesn't exist yet - will create new one below
				// This is the expected flow for new guardians
			} else {
				// Real database error (connection timeout, constraint violation, etc.)
				return 0, fmt.Errorf("database error checking existing guardian: %w", err)
			}
		} else if existing != nil {
			// Guardian found - reuse it (deduplication)
			// Update profile fields if the import provides new data
			c.updateExistingGuardianProfile(ctx, existing, data)

			// Add any new phone numbers from the import data
			if err := c.createGuardianPhoneNumbers(ctx, existing.ID, data.PhoneNumbers); err != nil {
				// Log but don't fail - phone numbers are additive
				// Duplicates will be handled gracefully
				return existing.ID, fmt.Errorf("add phone numbers to existing guardian: %w", err)
			}
			return existing.ID, nil
		}
		// Guardian not found - will create new one below
	}

	// Create new guardian (phone numbers are added via createGuardianPhoneNumbers below)
	guardian := &users.GuardianProfile{
		FirstName:          strings.TrimSpace(data.FirstName),
		LastName:           strings.TrimSpace(data.LastName),
		Email:              strutil.TrimToNil(data.Email),
		AddressStreet:      strutil.TrimToNil(data.AddressStreet),
		AddressCity:        strutil.TrimToNil(data.AddressCity),
		AddressPostalCode:  strutil.TrimToNil(data.AddressPostalCode),
		Notes:              strutil.TrimToNil(data.Notes),
		LanguagePreference: guardianLanguagePreference(data.LanguagePreference),
	}

	if err := c.GuardianRepo.Create(ctx, guardian); err != nil {
		return 0, err
	}

	// Create phone numbers from PhoneNumbers array (flexible phone support)
	if err := c.createGuardianPhoneNumbers(ctx, guardian.ID, data.PhoneNumbers); err != nil {
		return 0, fmt.Errorf("create phone numbers: %w", err)
	}

	return guardian.ID, nil
}

// updateExistingGuardianProfile merges non-empty import fields into an existing guardian.
// Only overwrites fields that are provided in the import data (non-empty).
// Errors are logged but don't fail the import — the guardian link still works.
func (c *StudentImportConfig) updateExistingGuardianProfile(ctx context.Context, existing *users.GuardianProfile, data importModels.GuardianImportData) {
	updated := false

	if v := strings.TrimSpace(data.AddressStreet); v != "" && !ptrEquals(existing.AddressStreet, v) {
		existing.AddressStreet = strutil.TrimToNil(v)
		updated = true
	}
	if v := strings.TrimSpace(data.AddressCity); v != "" && !ptrEquals(existing.AddressCity, v) {
		existing.AddressCity = strutil.TrimToNil(v)
		updated = true
	}
	if v := strings.TrimSpace(data.AddressPostalCode); v != "" && !ptrEquals(existing.AddressPostalCode, v) {
		existing.AddressPostalCode = strutil.TrimToNil(v)
		updated = true
	}
	if v := strings.TrimSpace(data.Notes); v != "" && !ptrEquals(existing.Notes, v) {
		existing.Notes = strutil.TrimToNil(v)
		updated = true
	}
	if v := guardianLanguagePreference(data.LanguagePreference); data.LanguagePreference != "" && v != existing.LanguagePreference {
		existing.LanguagePreference = v
		updated = true
	}

	if !updated {
		return
	}

	// Best-effort update — don't fail the import if profile update fails
	_ = c.GuardianRepo.Update(ctx, existing)
}

// ptrEquals checks if a *string equals a plain string value
func ptrEquals(ptr *string, val string) bool {
	return ptr != nil && *ptr == val
}

// createGuardianPhoneNumbers creates phone numbers for a guardian from import data
func (c *StudentImportConfig) createGuardianPhoneNumbers(ctx context.Context, guardianID int64, phones []importModels.PhoneImportData) error {
	for i, phoneData := range phones {
		if phoneData.PhoneNumber == "" {
			continue // Skip empty phone numbers
		}

		// Map phone type string to enum
		phoneType := mapPhoneType(phoneData.PhoneType)

		// Set label pointer (nil if empty)
		var label *string
		if phoneData.Label != "" {
			label = &phoneData.Label
		}

		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardianID,
			PhoneNumber:       phoneData.PhoneNumber,
			PhoneType:         phoneType,
			Label:             label,
			IsPrimary:         phoneData.IsPrimary,
			Priority:          i + 1, // Priority based on order in import
		}

		if err := c.GuardianPhoneRepo.Create(ctx, phone); err != nil {
			return fmt.Errorf("phone %d: %w", i+1, err)
		}
	}
	return nil
}

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

// Update updates an existing student (not implemented for MVP - see #556 for Phase 2)
func (c *StudentImportConfig) Update(_ context.Context, _ int64, _ importModels.StudentImportRow) error {
	return fmt.Errorf("update mode not supported in MVP - use create-only mode or manually update students")
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
func (c *StudentImportConfig) createArrivalSchedules(ctx context.Context, studentID int64, schedules []importModels.ArrivalScheduleImportData) error {
	if len(schedules) == 0 || c.ArrivalScheduleRepo == nil {
		return nil
	}

	for i, sched := range schedules {
		parsed, err := time.Parse("15:04", sched.ExpectedArrival)
		if err != nil {
			return fmt.Errorf("arrival schedule %d: invalid time '%s': %w", i+1, sched.ExpectedArrival, err)
		}
		arrivalTime := time.Date(2024, 1, 1, parsed.Hour(), parsed.Minute(), 0, 0, time.UTC)

		record := &scheduleModels.StudentArrivalSchedule{
			StudentID:       studentID,
			Weekday:         sched.Weekday,
			ExpectedArrival: arrivalTime,
			Notes:           strutil.TrimToNil(sched.Notes),
			CreatedBy:       ImporterIDFromContext(ctx),
		}
		record.SetTenantID(tenant.FromContext(ctx))

		if err := c.ArrivalScheduleRepo.Create(ctx, record); err != nil {
			return fmt.Errorf("create arrival schedule (weekday %d): %w", sched.Weekday, err)
		}
	}

	return nil
}

// createPickupSchedules creates weekly pickup schedule records for a student
func (c *StudentImportConfig) createPickupSchedules(ctx context.Context, studentID int64, schedules []importModels.PickupScheduleImportData) error {
	if len(schedules) == 0 || c.PickupScheduleRepo == nil {
		return nil
	}

	for i, sched := range schedules {
		parsed, err := time.Parse("15:04", sched.PickupTime)
		if err != nil {
			return fmt.Errorf("pickup schedule %d: invalid time '%s': %w", i+1, sched.PickupTime, err)
		}
		// Use a valid reference date — time.Parse("15:04") produces year 0000 which PostgreSQL rejects
		pickupTime := time.Date(2024, 1, 1, parsed.Hour(), parsed.Minute(), 0, 0, time.UTC)

		record := &scheduleModels.StudentPickupSchedule{
			StudentID:  studentID,
			Weekday:    sched.Weekday,
			PickupTime: pickupTime,
			Notes:      strutil.TrimToNil(sched.Notes),
			CreatedBy:  ImporterIDFromContext(ctx),
		}
		record.SetTenantID(tenant.FromContext(ctx))

		if err := c.PickupScheduleRepo.Create(ctx, record); err != nil {
			return fmt.Errorf("create pickup schedule (weekday %d): %w", sched.Weekday, err)
		}
	}

	return nil
}
