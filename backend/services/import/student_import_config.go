package importpkg

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

var (
	emailRegex = regexp.MustCompile(`^[A-Za-z0-9._+%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$`)
	phoneRegex = regexp.MustCompile(`^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$`)
)

// StudentImportConfig implements ImportConfig for student imports
type StudentImportConfig struct {
	personRepo   users.PersonRepository
	studentRepo  users.StudentRepository
	guardianRepo users.GuardianProfileRepository
	relationRepo users.StudentGuardianRepository
	privacyRepo  users.PrivacyConsentRepository
	resolver     *RelationshipResolver
	txHandler    *base.TxHandler
}

// NewStudentImportConfig creates a new student import configuration
func NewStudentImportConfig(
	personRepo users.PersonRepository,
	studentRepo users.StudentRepository,
	guardianRepo users.GuardianProfileRepository,
	relationRepo users.StudentGuardianRepository,
	privacyRepo users.PrivacyConsentRepository,
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

	// 2. REQUIRED: Student validation
	if strings.TrimSpace(row.SchoolClass) == "" {
		errors = append(errors, importModels.ValidationError{
			Field:    "school_class",
			Message:  "Klasse ist erforderlich",
			Code:     "required",
			Severity: importModels.ErrorSeverityError,
		})
	}

	// 3. OPTIONAL: Group resolution (with fuzzy matching)
	if row.GroupName != "" {
		groupID, groupErrors := c.resolver.ResolveGroup(ctx, row.GroupName)
		if len(groupErrors) > 0 {
			errors = append(errors, groupErrors...)
		} else {
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

	// 4. OPTIONAL: Guardian validation
	for i, guardian := range row.Guardians {
		guardianErrors := c.validateGuardian(i+1, guardian)
		errors = append(errors, guardianErrors...)
	}

	// 5. Birthday validation (if provided)
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

	// 6. Privacy validation
	if row.DataRetentionDays < 1 || row.DataRetentionDays > 31 {
		errors = append(errors, importModels.ValidationError{
			Field:    "data_retention_days",
			Message:  "Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen",
			Code:     "invalid_range",
			Severity: importModels.ErrorSeverityError,
		})
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
		// 1. Create Person
		birthday, _ := parseOptionalDate(row.Birthday)
		person := &users.Person{
			FirstName: strings.TrimSpace(row.FirstName),
			LastName:  strings.TrimSpace(row.LastName),
			Birthday:  birthday,
			TagID:     stringPtr(row.TagID),
		}

		if err := c.personRepo.Create(txCtx, person); err != nil {
			return fmt.Errorf("create person: %w", err)
		}

		// 2. Create Student
		student := &users.Student{
			PersonID:        person.ID,
			SchoolClass:     strings.TrimSpace(row.SchoolClass),
			GroupID:         row.GroupID, // May be nil (no group)
			ExtraInfo:       stringPtr(row.ExtraInfo),
			SupervisorNotes: stringPtr(row.SupervisorNotes),
			HealthInfo:      stringPtr(row.HealthInfo),
		}

		if err := c.studentRepo.Create(txCtx, student); err != nil {
			return fmt.Errorf("create student: %w", err)
		}
		studentID = student.ID

		// 3. Create/Link Multiple Guardians
		for i, guardianData := range row.Guardians {
			guardianID, err := c.createOrFindGuardian(txCtx, guardianData)
			if err != nil {
				return fmt.Errorf("guardian %d: %w", i+1, err)
			}

			// Create Student-Guardian Relationship
			relationship := &users.StudentGuardian{
				StudentID:          studentID,
				GuardianProfileID:  guardianID,
				RelationshipType:   guardianData.RelationshipType,
				IsPrimary:          guardianData.IsPrimary,
				IsEmergencyContact: guardianData.IsEmergencyContact,
				CanPickup:          guardianData.CanPickup,
			}

			if err := c.relationRepo.Create(txCtx, relationship); err != nil {
				return fmt.Errorf("create relationship %d: %w", i+1, err)
			}
		}

		// 4. Create Privacy Consent
		if row.PrivacyAccepted || row.DataRetentionDays > 0 {
			consent := &users.PrivacyConsent{
				StudentID:         studentID,
				Accepted:          row.PrivacyAccepted,
				DataRetentionDays: row.DataRetentionDays,
			}

			if row.PrivacyAccepted {
				now := time.Now()
				consent.AcceptedAt = &now
			}

			if err := c.privacyRepo.Create(txCtx, consent); err != nil {
				return fmt.Errorf("create privacy consent: %w", err)
			}
		}

		return nil
	})

	return studentID, err
}

// createOrFindGuardian deduplicates guardians by email
func (c *StudentImportConfig) createOrFindGuardian(ctx context.Context, data importModels.GuardianImportData) (int64, error) {
	// Deduplication strategy: Email is unique identifier
	if data.Email != "" {
		existing, err := c.guardianRepo.FindByEmail(ctx, data.Email)
		if err == nil && existing != nil {
			// Reuse existing guardian
			return existing.ID, nil
		}
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

// Update updates an existing student (not implemented for initial phase)
func (c *StudentImportConfig) Update(ctx context.Context, id int64, row importModels.StudentImportRow) error {
	// TODO: Implement update logic for upsert mode
	return fmt.Errorf("update not implemented yet")
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

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}
