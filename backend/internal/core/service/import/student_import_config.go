package importpkg

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	importModels "github.com/moto-nrw/project-phoenix/internal/core/domain/import"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
	"github.com/uptrace/bun"
)

var (
	emailRegex = regexp.MustCompile(`^[A-Za-z0-9._+%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$`)
	phoneRegex = regexp.MustCompile(`^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$`)
)

// mapRelationshipType converts German relationship types to valid English types
func mapRelationshipType(germanType string) string {
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
	personRepo   userPort.PersonRepository
	studentRepo  userPort.StudentRepository
	guardianRepo userPort.GuardianProfileRepository
	relationRepo userPort.StudentGuardianRepository
	privacyRepo  userPort.PrivacyConsentRepository
	resolver     *RelationshipResolver
	txHandler    *base.TxHandler
}

// NewStudentImportConfig creates a new student import configuration
// Note: RFID cards are not supported in CSV import and must be assigned separately
func NewStudentImportConfig(
	personRepo userPort.PersonRepository,
	studentRepo userPort.StudentRepository,
	guardianRepo userPort.GuardianProfileRepository,
	relationRepo userPort.StudentGuardianRepository,
	privacyRepo userPort.PrivacyConsentRepository,
	resolver *RelationshipResolver,
	db *bun.DB,
) *StudentImportConfig {
	return &StudentImportConfig{
		personRepo:   personRepo,
		studentRepo:  studentRepo,
		guardianRepo: guardianRepo,
		relationRepo: relationRepo,
		privacyRepo:  privacyRepo,
		resolver:     resolver,
		txHandler:    base.NewTxHandler(db),
	}
}

// PreloadReferenceData loads all reference data (groups) for relationship resolution
func (c *StudentImportConfig) PreloadReferenceData(ctx context.Context) error {
	// Pre-load all groups for relationship resolution
	return c.resolver.PreloadGroups(ctx)
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
		groupID, groupErrors := c.resolver.ResolveGroup(ctx, row.GroupName)
		if len(groupErrors) > 0 {
			errors = append(errors, groupErrors...)
		} else if groupID != nil {
			row.GroupID = groupID // Cache resolved ID
		}
	} else {
		// INFO: Group empty - student will be created without group
		errors = append(errors, importModels.ValidationError{
			Field:    "group",
			Message:  "Keine Gruppe zugewiesen. Der Schüler wird ohne Gruppe erstellt.",
			Code:     "group_empty",
			Severity: importModels.ErrorSeverityInfo, // Non-blocking
		})
	}

	// 5. OPTIONAL: Guardian validation
	for i, guardian := range row.Guardians {
		guardianErrors := c.validateGuardian(i+1, guardian)
		errors = append(errors, guardianErrors...)
	}

	// 6. Birthday validation (if provided)
	if row.Birthday != "" {
		if _, err := time.Parse("2006-01-02", row.Birthday); err != nil {
			errors = append(errors, importModels.ValidationError{
				Field:    "birthday",
				Message:  "Ungültiges Datumsformat. Bitte verwenden Sie JJJJ-MM-TT (z.B. 2015-08-15)",
				Code:     "invalid_date_format",
				Severity: importModels.ErrorSeverityError,
			})
		}
	}

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

// validateGuardian validates a single guardian's data
func (c *StudentImportConfig) validateGuardian(num int, guardian importModels.GuardianImportData) []importModels.ValidationError {
	errors := []importModels.ValidationError{}
	fieldPrefix := fmt.Sprintf("guardian_%d", num)

	// At least one contact method required
	if guardian.Email == "" && guardian.Phone == "" && guardian.MobilePhone == "" {
		errors = append(errors, importModels.ValidationError{
			Field:    fieldPrefix,
			Message:  fmt.Sprintf("Erziehungsberechtigter %d benötigt mindestens eine Kontaktmethode (Email, Telefon oder Mobil)", num),
			Code:     "guardian_contact_required",
			Severity: importModels.ErrorSeverityError,
		})
		return errors // Return early if no contact info
	}

	// Email format validation (if provided)
	if guardian.Email != "" && !emailRegex.MatchString(guardian.Email) {
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_email", fieldPrefix),
			Message:  fmt.Sprintf("Ungültiges Email-Format für Erziehungsberechtigten %d: %s", num, guardian.Email),
			Code:     "invalid_email",
			Severity: importModels.ErrorSeverityError,
		})
	}

	// Phone format validation (if provided)
	if guardian.Phone != "" && !phoneRegex.MatchString(guardian.Phone) {
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_phone", fieldPrefix),
			Message:  fmt.Sprintf("Ungültiges Telefon-Format für Erziehungsberechtigten %d: %s", num, guardian.Phone),
			Code:     "invalid_phone",
			Severity: importModels.ErrorSeverityError,
		})
	}

	// Mobile phone format validation (if provided)
	if guardian.MobilePhone != "" && !phoneRegex.MatchString(guardian.MobilePhone) {
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_mobile", fieldPrefix),
			Message:  fmt.Sprintf("Ungültiges Mobiltelefon-Format für Erziehungsberechtigten %d: %s", num, guardian.MobilePhone),
			Code:     "invalid_phone",
			Severity: importModels.ErrorSeverityError,
		})
	}

	return errors
}

// FindExisting checks if a student already exists (for duplicate detection)
func (c *StudentImportConfig) FindExisting(ctx context.Context, row importModels.StudentImportRow) (*int64, error) {
	// Strategy: Find by exact first_name + last_name + school_class match
	students, err := c.studentRepo.FindByNameAndClass(ctx, row.FirstName, row.LastName, row.SchoolClass)
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
	return nil, fmt.Errorf("mehrere Schüler gefunden mit Name '%s %s' in Klasse '%s'",
		row.FirstName, row.LastName, row.SchoolClass)
}

// Create creates a new student with all related entities
func (c *StudentImportConfig) Create(ctx context.Context, row importModels.StudentImportRow) (int64, error) {
	var studentID int64

	err := c.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		person, err := c.createPersonFromRow(txCtx, row)
		if err != nil {
			return err
		}

		student, err := c.createStudentFromRow(txCtx, person.ID, row)
		if err != nil {
			return err
		}
		studentID = student.ID

		if err := c.createGuardianRelationships(txCtx, studentID, row.Guardians); err != nil {
			return err
		}

		return c.createPrivacyConsentIfNeeded(txCtx, studentID, row)
	})

	return studentID, err
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

	if err := c.personRepo.Create(ctx, person); err != nil {
		return nil, fmt.Errorf("create person: %w", err)
	}

	return person, nil
}

// createStudentFromRow creates a student from person and row
func (c *StudentImportConfig) createStudentFromRow(ctx context.Context, personID int64, row importModels.StudentImportRow) (*users.Student, error) {
	student := &users.Student{
		PersonID:        personID,
		SchoolClass:     strings.TrimSpace(row.SchoolClass),
		GroupID:         row.GroupID,
		ExtraInfo:       stringPtr(row.ExtraInfo),
		SupervisorNotes: stringPtr(row.SupervisorNotes),
		HealthInfo:      stringPtr(row.HealthInfo),
		PickupStatus:    stringPtr(row.PickupStatus),
	}

	if err := c.studentRepo.Create(ctx, student); err != nil {
		return nil, fmt.Errorf("create student: %w", err)
	}

	return student, nil
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
		RelationshipType:   mapRelationshipType(guardianData.RelationshipType),
		IsPrimary:          guardianData.IsPrimary,
		IsEmergencyContact: guardianData.IsEmergencyContact,
		CanPickup:          guardianData.CanPickup,
	}

	if err := c.relationRepo.Create(ctx, relationship); err != nil {
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
	if err := c.privacyRepo.Create(ctx, consent); err != nil {
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
		existing, err := c.guardianRepo.FindByEmail(ctx, data.Email)
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
			return existing.ID, nil
		}
		// Guardian not found - will create new one below
	}

	// Create new guardian
	guardian := &users.GuardianProfile{
		FirstName:   strings.TrimSpace(data.FirstName),
		LastName:    strings.TrimSpace(data.LastName),
		Email:       stringPtr(data.Email),
		Phone:       stringPtr(data.Phone),
		MobilePhone: stringPtr(data.MobilePhone),
	}

	if err := c.guardianRepo.Create(ctx, guardian); err != nil {
		return 0, err
	}

	return guardian.ID, nil
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

// parseOptionalDate parses a date string or returns nil
func parseOptionalDate(dateStr string) (*time.Time, error) {
	if dateStr == "" {
		return nil, nil
	}

	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// stringPtr returns a pointer to a string, or nil if empty
func stringPtr(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
