package importpkg

import (
	"context"
	stdErrors "errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
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

// StudentImportConfig implements ports.ImportConfig for student imports. Every
// write goes through the owner commands in ports.StudentImportDeps (#2708).
type StudentImportConfig struct {
	ports.RelationshipResolver
	ports.StudentImportDeps
}

// NewStudentImportConfig creates a new student import configuration.
func NewStudentImportConfig(deps ports.StudentImportDeps) *StudentImportConfig {
	deps.Persons = ports.ObservePersonDirectory(deps.Persons)
	deps.Students = ports.ObserveStudentDirectory(deps.Students)
	deps.Guardians = ports.ObserveGuardianDirectory(deps.Guardians)
	deps.Schedules = ports.ObserveStudentSchedules(deps.Schedules)
	deps.PrivacyConsents = ports.ObservePrivacyConsents(deps.PrivacyConsents)
	return &StudentImportConfig{RelationshipResolver: deps.Resolver, StudentImportDeps: deps}
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

	errors = append(errors, validateStudentPersonNames(row, requiresCreateFields)...)

	// 2. RFID: the card must exist at this school and must not be worn by
	// somebody else (#2600). A typo here would check the wrong child in at
	// the door without anyone noticing, so the row is blocked, not warned.
	errors = append(errors, c.validateTag(ctx, row)...)

	errors = append(errors, validateStudentSchoolClass(row, requiresCreateFields)...)

	errors = append(errors, c.resolveStudentGroup(ctx, row)...)

	// 5. OPTIONAL: Guardian validation
	for i, guardian := range row.Guardians {
		guardianErrors := c.validateGuardian(i+1, guardian)
		errors = append(errors, guardianErrors...)

	}

	// 5b. OPTIONAL: Arrival and pickup schedule validation
	errors = append(errors, validateArrivalSchedules(row.ArrivalSchedules)...)
	errors = append(errors, validatePickupSchedules(row.PickupSchedules)...)

	errors = append(errors, validateStudentDepartureNote(row)...)

	errors = append(errors, normalizeStudentBirthday(row)...)

	// 6b. Enrollment date range validation (if provided)
	errors = append(errors, validateEnrollmentDates(row)...)

	// 6c. Consent date validation (if provided)
	errors = append(errors, validateConsentDates(row)...)

	errors = append(errors, validateStudentRetention(row)...)

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
	if c.FindRFIDCard == nil {
		row.TagID = ""
		return []importModels.ValidationError{{
			Field:    "tag_id",
			Message:  "RFID-Karten können in dieser Installation nicht importiert werden. Bitte die Karte nach dem Import über die Geräteverwaltung zuweisen.",
			Code:     "rfid_not_supported",
			Severity: importModels.ErrorSeverityWarning,
		}}
	}

	tagID, found, err := c.FindRFIDCard(ctx, raw)
	if err != nil {
		return rfidLookupFailed(err)
	}
	if !found {
		return []importModels.ValidationError{{
			Field:       "tag_id",
			Message:     fmt.Sprintf("RFID-Karte '%s' ist an dieser Schule nicht angelegt. Bitte die Karte zuerst in der Geräteverwaltung erfassen.", raw),
			Code:        "rfid_unknown",
			Severity:    importModels.ErrorSeverityError,
			ActualValue: raw,
		}}
	}
	row.TagID = tagID

	return c.validateTagWearer(ctx, row, tagID, raw)
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

// supportedLanguages contains ISO 639-1 codes common in German school contexts
var supportedLanguages = map[string]bool{
	"de": true, "en": true, "tr": true, "ar": true, "ru": true,
	"pl": true, "fa": true, "ku": true, "fr": true, "es": true,
	"it": true, "pt": true, "ro": true, "uk": true, "sr": true,
	"hr": true, "bg": true, "el": true, "vi": true, "zh": true,
}

// FindExisting resolves the row to an existing student (duplicate detection
// in create mode, match key in update mode). Keys, in order: the RFID card
// (survives a class change), first + last name + class, and first + last name
// + birthday (the class-change case without a card).
func busDaysFromImportRow(row importModels.StudentImportRow) ports.BusDays {
	if row.BusDays != nil {
		days := ports.BusDays{}
		for _, key := range ports.BusDayOrder {
			if row.BusDays[key] {
				days[key] = true
			}
		}
		return days
	}
	return ports.BusDaysFromLegacyFlag(row.BusPermission)
}

// departurePlanFromImportRow resolves the unified per-day departure plan from an
// import row. The current per-day "Gehweise.Mo".."Gehweise.Fr" columns take
// precedence; otherwise the legacy Bus(.Mo..Fr) and Abholstatus columns are
// folded into the plan so old templates keep importing. departure_days is the
// single source of truth (#1610).
func departurePlanFromImportRow(row importModels.StudentImportRow) ports.DepartureDays {
	if row.DepartureDays != nil {
		out := ports.DepartureDays{}
		for _, key := range ports.PickupDayOrder {
			switch row.DepartureDays[key] {
			case string(ports.DepartureAlone):
				out[key] = ports.DepartureAlone
			case string(ports.DepartureBus):
				out[key] = ports.DepartureBus
			case string(ports.DeparturePickup):
				out[key] = ports.DeparturePickup
			case string(ports.DepartureAccompanied):
				out[key] = ports.DepartureAccompanied
			}
		}
		return out
	}
	bus := busDaysFromImportRow(row)
	pickup := ports.PickupDaysFromLegacyStatus(row.PickupStatus)
	return ports.DepartureDaysFromLegacy(bus, pickup)
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

// EntityName returns the entity type name
func (c *StudentImportConfig) EntityName() string {
	return "student"
}

// Helper functions

// boundedNotePtr trims the value, truncates it to the companion-note cap by
// rune count (multibyte-safe), and returns nil when empty. Import truncates
// rather than rejecting the whole row, mirroring the enrollment intake (#1694).
func boundedNotePtr(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > ports.MaxDepartureCompanionNoteLen {
		trimmed = string([]rune(trimmed)[:ports.MaxDepartureCompanionNoteLen])
	}
	return &trimmed
}

// createArrivalSchedules creates weekly arrival schedule records for a student

func validateStudentPersonNames(row *importModels.StudentImportRow, requiresCreateFields bool) []importModels.ValidationError {
	errors := []importModels.ValidationError{}
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

	return errors
}

func validateStudentSchoolClass(row *importModels.StudentImportRow, requiresCreateFields bool) []importModels.ValidationError {
	errors := []importModels.ValidationError{}
	// 3. REQUIRED: Student validation
	if requiresCreateFields && strings.TrimSpace(row.SchoolClass) == "" {
		errors = append(errors, importModels.ValidationError{
			Field:    "school_class",
			Message:  "Klasse ist erforderlich",
			Code:     "required",
			Severity: importModels.ErrorSeverityError,
		})
	}

	return errors
}

func (c *StudentImportConfig) resolveStudentGroup(ctx context.Context, row *importModels.StudentImportRow) []importModels.ValidationError {
	errors := []importModels.ValidationError{}
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

	return errors
}

func validateStudentDepartureNote(row *importModels.StudentImportRow) []importModels.ValidationError {
	errors := []importModels.ValidationError{}
	// 5c. Coupled "mit wem" note: a row that sets any Gehweise.* cell to "Mit
	// anderem Kind" (accompanied) needs a non-blank Begleitung. Surface it in the
	// preview pass so the user sees the row error before importing, rather than
	// having createStudentFromRow build an accompanied student the model rejects
	// mid-import (#1694).
	if departurePlanFromImportRow(*row).HasMode(ports.DepartureAccompanied) &&
		strings.TrimSpace(row.DepartureCompanionNote) == "" {
		errors = append(errors, importModels.ValidationError{
			Field:    "begleitung",
			Message:  "Begleitung ist erforderlich, wenn an einem Tag 'Mit anderem Kind' als Heimweg gewählt ist.",
			Code:     "required",
			Severity: importModels.ErrorSeverityError,
		})
	}

	return errors
}

func normalizeStudentBirthday(row *importModels.StudentImportRow) []importModels.ValidationError {
	errors := []importModels.ValidationError{}
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

	return errors
}

func validateStudentRetention(row *importModels.StudentImportRow) []importModels.ValidationError {
	errors := []importModels.ValidationError{}
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

func rfidLookupFailed(err error) []importModels.ValidationError {
	return []importModels.ValidationError{{
		Field:    "tag_id",
		Message:  fmt.Sprintf("RFID-Karte konnte nicht geprüft werden: %s", err.Error()),
		Code:     "rfid_lookup_failed",
		Severity: importModels.ErrorSeverityError,
	}}
}

func (c *StudentImportConfig) validateTagWearer(ctx context.Context, row *importModels.StudentImportRow, tagID, raw string) []importModels.ValidationError {
	wearer, err := c.Persons.FindPersonByTag(ctx, tagID)
	if err != nil {
		if stdErrors.Is(err, ports.ErrPersonNotFound) {
			return nil
		}
		return rfidLookupFailed(err)
	}
	students, err := c.Students.ListStudentsByPersonID(ctx, []int64{wearer.ID})
	if err != nil {
		return rfidLookupFailed(err)
	}
	if len(students) > 0 {
		student := students[0]
		if strings.TrimSpace(row.FirstName) != "" && strings.TrimSpace(row.LastName) != "" && strings.TrimSpace(row.SchoolClass) != "" {
			matched, err := c.findStudentWithoutTag(ctx, *row)
			if err != nil {
				return rfidLookupFailed(err)
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
