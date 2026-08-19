package importpkg

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/education"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func futureBirthdayForTests() time.Time {
	now := time.Now().In(time.UTC)
	return time.Date(now.Year()+1, now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func futureBirthdayISOForTests() string {
	return futureBirthdayForTests().Format("2006-01-02")
}

func futureBirthdayGermanLongForTests() string {
	return futureBirthdayForTests().Format("02.01.2006")
}

func futureBirthdayGermanShortForTests() string {
	return futureBirthdayForTests().Format("02.01.06")
}

func TestEnrollmentStartsInFuture_UsesBusinessDate(t *testing.T) {
	today := timezone.TodayDate()
	tomorrow := today.AddDays(1)

	assert.False(t, enrollmentStartsInFuture(nil))
	assert.False(t, enrollmentStartsInFuture(&today), "today must be active, not pending")
	assert.True(t, enrollmentStartsInFuture(&tomorrow))
}

func TestStudentImportConfig_CreateSingleGuardianRelationship_AssignsRolePermissions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	factory := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(1)
	student := testpkg.CreateTestStudent(t, db, "Import", "Guardian", "1a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, db, student.ID) })

	config := NewStudentImportConfig(StudentImportDeps{
		GuardianRepo:      factory.GuardianProfile,
		GuardianPhoneRepo: factory.GuardianPhoneNumber,
		RelationRepo:      factory.StudentGuardian,
	}, db)

	err := config.createSingleGuardianRelationship(ctx, student.ID, importModels.GuardianImportData{
		FirstName:        "Import",
		LastName:         "Parent",
		Email:            "import-parent@example.test",
		RelationshipType: "Elternteil",
	}, 1)
	require.NoError(t, err)

	relationships, err := factory.StudentGuardian.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, relationships, 1)
	assert.Equal(t, authorize.GuardianRoleLegalGuardian, relationships[0].GuardianRole)
	assert.True(t, authorize.StudentGuardianHasPermission(relationships[0], authorize.GuardianPermissionPortalAccess))
}

func TestStudentImportConfig_Validate_RequiredFields(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	tests := []struct {
		name     string
		row      importModels.StudentImportRow
		wantErrs int
		errCodes []string
	}{
		{
			name: "all required fields present",
			row: importModels.StudentImportRow{
				FirstName:         "Max",
				LastName:          "Mustermann",
				SchoolClass:       "1A",
				DataRetentionDays: 30,
			},
			wantErrs: 1, // INFO about empty group
			errCodes: []string{"group_empty"},
		},
		{
			name: "missing first name",
			row: importModels.StudentImportRow{
				LastName:          "Mustermann",
				SchoolClass:       "1A",
				DataRetentionDays: 30,
			},
			wantErrs: 2, // ERROR: first_name required + INFO: group_empty
			errCodes: []string{"required", "group_empty"},
		},
		{
			name: "missing last name",
			row: importModels.StudentImportRow{
				FirstName:         "Max",
				SchoolClass:       "1A",
				DataRetentionDays: 30,
			},
			wantErrs: 2, // ERROR: last_name required + INFO: group_empty
			errCodes: []string{"required", "group_empty"},
		},
		{
			name: "missing school class",
			row: importModels.StudentImportRow{
				FirstName:         "Max",
				LastName:          "Mustermann",
				DataRetentionDays: 30,
			},
			wantErrs: 2, // ERROR: school_class required + INFO: group_empty
			errCodes: []string{"required", "group_empty"},
		},
		{
			name:     "all required fields missing",
			row:      importModels.StudentImportRow{},
			wantErrs: 5, // 3 ERROR + 1 INFO (group_empty) + 1 ERROR (data_retention out of range)
			errCodes: []string{"required", "required", "required", "group_empty", "invalid_range"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := config.Validate(context.Background(), &tt.row)
			assert.Len(t, errors, tt.wantErrs, "Error count mismatch")

			// Verify error codes
			for _, expectedCode := range tt.errCodes {
				found := false
				for _, err := range errors {
					if err.Code == expectedCode {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error code '%s' not found", expectedCode)
			}
		})
	}
}

func TestStudentImportConfig_Validate_GuardianValidation(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	tests := []struct {
		name      string
		guardians []importModels.GuardianImportData
		wantErrs  int
		errCodes  []string
	}{
		{
			name: "valid guardian with email",
			guardians: []importModels.GuardianImportData{
				{Email: "maria@example.com", FirstName: "Maria", LastName: "Müller"},
			},
			wantErrs: 1, // INFO: group_empty
			errCodes: []string{"group_empty"},
		},
		{
			name: "valid guardian with phone",
			guardians: []importModels.GuardianImportData{
				{Phone: "0123-456789", FirstName: "Maria", LastName: "Müller"},
			},
			wantErrs: 1, // INFO: group_empty
			errCodes: []string{"group_empty"},
		},
		{
			name: "guardian without contact method",
			guardians: []importModels.GuardianImportData{
				{FirstName: "Maria", LastName: "Müller"},
			},
			wantErrs: 2, // ERROR: contact required + INFO: group_empty
			errCodes: []string{"guardian_contact_required", "group_empty"},
		},
		{
			name: "invalid email format",
			guardians: []importModels.GuardianImportData{
				{Email: "not-an-email", FirstName: "Maria", LastName: "Müller"},
			},
			wantErrs: 2, // ERROR: invalid_email + INFO: group_empty
			errCodes: []string{"invalid_email", "group_empty"},
		},
		{
			name: "invalid phone format",
			guardians: []importModels.GuardianImportData{
				{Phone: "abc", FirstName: "Maria", LastName: "Müller"},
			},
			wantErrs: 2, // ERROR: invalid_phone + INFO: group_empty
			errCodes: []string{"invalid_phone", "group_empty"},
		},
		{
			name: "multiple guardians all valid",
			guardians: []importModels.GuardianImportData{
				{Email: "maria@example.com"},
				{Phone: "0123-456789"},
				{Email: "hans@example.com", Phone: "0987-654321"},
			},
			wantErrs: 1, // INFO: group_empty
			errCodes: []string{"group_empty"},
		},
		{
			name: "multiple guardians with errors",
			guardians: []importModels.GuardianImportData{
				{Email: "invalid-email"},  // Invalid email
				{FirstName: "No Contact"}, // No contact method
			},
			wantErrs: 3, // 2 ERROR + INFO: group_empty
			errCodes: []string{"invalid_email", "guardian_contact_required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := importModels.StudentImportRow{
				FirstName:         "Max",
				LastName:          "Mustermann",
				SchoolClass:       "1A",
				Guardians:         tt.guardians,
				DataRetentionDays: 30,
			}

			errors := config.Validate(context.Background(), &row)
			assert.Len(t, errors, tt.wantErrs, "Error count mismatch")

			// Verify expected error codes exist
			for _, expectedCode := range tt.errCodes {
				found := false
				for _, err := range errors {
					if err.Code == expectedCode {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error code '%s' not found", expectedCode)
			}
		})
	}
}

func TestStudentImportConfig_Validate_EnrollmentDates(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	baseRow := func() importModels.StudentImportRow {
		return importModels.StudentImportRow{
			FirstName:         "Max",
			LastName:          "Mustermann",
			SchoolClass:       "1A",
			DataRetentionDays: 30,
		}
	}

	hasCode := func(errs []importModels.ValidationError, code string) bool {
		for _, e := range errs {
			if e.Code == code {
				return true
			}
		}
		return false
	}

	t.Run("valid range passes and normalizes to ISO", func(t *testing.T) {
		row := baseRow()
		row.EnrolledFrom = "01.08.2024"
		row.EnrolledUntil = "31.07.2025"
		errs := config.Validate(context.Background(), &row)
		assert.False(t, hasCode(errs, "invalid_date_format"))
		assert.False(t, hasCode(errs, "invalid_date_range"))
		assert.Equal(t, "2024-08-01", row.EnrolledFrom)
		assert.Equal(t, "2025-07-31", row.EnrolledUntil)
	})

	t.Run("future enrolled_from is allowed", func(t *testing.T) {
		row := baseRow()
		row.EnrolledFrom = futureBirthdayISOForTests()
		errs := config.Validate(context.Background(), &row)
		assert.False(t, hasCode(errs, "invalid_date_format"),
			"a future enrollment start date must be accepted (unlike a birthday)")
	})

	t.Run("invalid format reports error", func(t *testing.T) {
		row := baseRow()
		row.EnrolledFrom = "kein-datum"
		errs := config.Validate(context.Background(), &row)
		assert.True(t, hasCode(errs, "invalid_date_format"))
	})

	t.Run("until before from reports range error", func(t *testing.T) {
		row := baseRow()
		row.EnrolledFrom = "01.08.2024"
		row.EnrolledUntil = "01.08.2023"
		errs := config.Validate(context.Background(), &row)
		assert.True(t, hasCode(errs, "invalid_date_range"))
	})
}

func TestStudentImportConfig_Validate_ConsentDates(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	baseRow := func() importModels.StudentImportRow {
		return importModels.StudentImportRow{
			FirstName:         "Max",
			LastName:          "Mustermann",
			SchoolClass:       "1A",
			DataRetentionDays: 30,
		}
	}

	hasCode := func(errs []importModels.ValidationError, code string) bool {
		for _, e := range errs {
			if e.Code == code {
				return true
			}
		}
		return false
	}

	t.Run("valid consent dates pass and normalize to ISO", func(t *testing.T) {
		row := baseRow()
		row.AGBAcceptedAt = "01.08.2024"
		row.PhotoConsentGivenAt = "2024-08-01"
		errs := config.Validate(context.Background(), &row)
		assert.False(t, hasCode(errs, "invalid_date_format"))
		assert.False(t, hasCode(errs, "invalid_date"))
		assert.Equal(t, "2024-08-01", row.AGBAcceptedAt)
		assert.Equal(t, "2024-08-01", row.PhotoConsentGivenAt)
	})

	t.Run("future consent date is rejected", func(t *testing.T) {
		row := baseRow()
		row.DataProcessingAcceptedAt = futureBirthdayISOForTests()
		errs := config.Validate(context.Background(), &row)
		assert.True(t, hasCode(errs, "invalid_date"),
			"a consent cannot have been given in the future")
	})

	t.Run("invalid format is rejected", func(t *testing.T) {
		row := baseRow()
		row.EmailContactAcceptedAt = "kein-datum"
		errs := config.Validate(context.Background(), &row)
		assert.True(t, hasCode(errs, "invalid_date_format"))
	})
}

// TestStudentImportConfig_Validate_AccompaniedCompanionNote pins the import
// preview gate for the accompanied departure mode (#1694): a row that sets any
// Gehweise cell to "Mit anderem Kind" but leaves Begleitung blank must surface
// a row error in the preview pass, before createStudentFromRow builds a student
// the model rejects mid-import.
func TestStudentImportConfig_Validate_AccompaniedCompanionNote(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	hasBegleitungError := func(errs []importModels.ValidationError) bool {
		for _, e := range errs {
			if e.Field == "begleitung" && e.Severity == importModels.ErrorSeverityError {
				return true
			}
		}
		return false
	}

	t.Run("accompanied day without Begleitung is flagged", func(t *testing.T) {
		row := importModels.StudentImportRow{
			FirstName:         "Max",
			LastName:          "Mustermann",
			SchoolClass:       "1A",
			DataRetentionDays: 30,
			DepartureDays:     map[string]string{"mon": "accompanied"},
		}
		errs := config.Validate(context.Background(), &row)
		assert.True(t, hasBegleitungError(errs), "expected a begleitung error, got %+v", errs)
	})

	t.Run("accompanied day with Begleitung passes", func(t *testing.T) {
		row := importModels.StudentImportRow{
			FirstName:              "Max",
			LastName:               "Mustermann",
			SchoolClass:            "1A",
			DataRetentionDays:      30,
			DepartureDays:          map[string]string{"mon": "accompanied"},
			DepartureCompanionNote: "Geschwisterkind Lena",
		}
		errs := config.Validate(context.Background(), &row)
		assert.False(t, hasBegleitungError(errs), "unexpected begleitung error: %+v", errs)
	})

	t.Run("non-accompanied plan needs no Begleitung", func(t *testing.T) {
		row := importModels.StudentImportRow{
			FirstName:         "Max",
			LastName:          "Mustermann",
			SchoolClass:       "1A",
			DataRetentionDays: 30,
			DepartureDays:     map[string]string{"mon": "bus"},
		}
		errs := config.Validate(context.Background(), &row)
		assert.False(t, hasBegleitungError(errs), "unexpected begleitung error: %+v", errs)
	})
}

func TestStudentImportConfig_Validate_DataRetention(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	tests := []struct {
		name          string
		retentionDays int
		wantError     bool
		wantWarning   bool
	}{
		{"minimum valid (1 day)", 1, false, false},
		{"default (30 days)", 30, false, false},
		{"maximum valid (31 days)", 31, false, false},
		{"too low (0 days)", 0, true, false},
		{"negative", -5, true, false},
		{"too high (32 days)", 32, false, true},       // Warning, not error
		{"way too high (365 days)", 365, false, true}, // Warning, not error
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := importModels.StudentImportRow{
				FirstName:         "Max",
				LastName:          "Mustermann",
				SchoolClass:       "1A",
				DataRetentionDays: tt.retentionDays,
			}

			errors := config.Validate(context.Background(), &row)

			if tt.wantError {
				// Should have at least the retention error (severity: Error)
				hasRetentionError := false
				for _, err := range errors {
					if err.Field == "data_retention_days" && err.Severity == importModels.ErrorSeverityError {
						hasRetentionError = true
						assert.Contains(t, err.Message, "mindestens 1 Tag")
						break
					}
				}
				assert.True(t, hasRetentionError, "Expected data retention error")
			} else if tt.wantWarning {
				// Should have warning (severity: Warning) for values > 31
				hasRetentionWarning := false
				for _, err := range errors {
					if err.Field == "data_retention_days" && err.Severity == importModels.ErrorSeverityWarning {
						hasRetentionWarning = true
						assert.Contains(t, err.Message, "Maximum")
						assert.Contains(t, err.Message, "31 Tage")
						break
					}
				}
				assert.True(t, hasRetentionWarning, "Expected data retention warning for value > 31")
			} else {
				// Should not have retention error or warning
				for _, err := range errors {
					if err.Field == "data_retention_days" {
						t.Errorf("Unexpected data retention validation for %d days: %s", tt.retentionDays, err.Message)
					}
				}
			}
		})
	}
}

func TestStudentImportConfig_Validate_BirthdayFormat(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}
	futureISO := futureBirthdayISOForTests()
	futureGermanLong := futureBirthdayGermanLongForTests()
	futureGermanShort := futureBirthdayGermanShortForTests()

	tests := []struct {
		name           string
		birthday       string
		wantError      bool
		wantErrorCode  string
		wantMessage    string
		wantNormalized string
	}{
		{"valid ISO format", "2015-08-15", false, "", "", "2015-08-15"},
		{"valid ISO format 2", "2014-03-22", false, "", "", "2014-03-22"},
		{"valid German format DD.MM.YYYY", "15.08.2015", false, "", "", "2015-08-15"},
		{"valid German format DD.MM.YY", "15.08.15", false, "", "", "2015-08-15"},
		{"empty (optional)", "", false, "", "", ""},
		{"future ISO date", futureISO, true, "invalid_date", "Zukunft", ""},
		{"future German format DD.MM.YYYY", futureGermanLong, true, "invalid_date", "Zukunft", ""},
		{"invalid format DD/MM/YYYY", "15/08/2015", true, "invalid_date_format", "JJJJ-MM-TT", ""},
		{"invalid format YYYY/MM/DD", "2015/08/15", true, "invalid_date_format", "JJJJ-MM-TT", ""},
		{"invalid date", "2015-13-45", true, "invalid_date_format", "JJJJ-MM-TT", ""},
		{"future German format DD.MM.YY", futureGermanShort, true, "invalid_date", "Zukunft", ""},
		{"just text", "invalid", true, "invalid_date_format", "JJJJ-MM-TT", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := importModels.StudentImportRow{
				FirstName:         "Max",
				LastName:          "Mustermann",
				SchoolClass:       "1A",
				Birthday:          tt.birthday,
				DataRetentionDays: 30,
			}

			errors := config.Validate(context.Background(), &row)

			if tt.wantError {
				hasBirthdayError := false
				for _, err := range errors {
					if err.Field == "birthday" {
						hasBirthdayError = true
						assert.Equal(t, tt.wantErrorCode, err.Code)
						assert.Contains(t, err.Message, tt.wantMessage)
						if tt.wantErrorCode == "invalid_date_format" {
							assert.Contains(t, err.Message, "TT.MM.JJJJ")
							assert.Contains(t, err.Message, "TT.MM.JJ")
						}
						break
					}
				}
				assert.True(t, hasBirthdayError, "Expected birthday format error")
			} else {
				for _, err := range errors {
					if err.Code == "invalid_date_format" {
						t.Errorf("Unexpected birthday error for '%s'", tt.birthday)
					}
				}
				assert.Equal(t, tt.wantNormalized, row.Birthday)
			}
		})
	}
}

func TestStudentImportConfig_Validate_ErrorSeverity(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	row := importModels.StudentImportRow{
		FirstName:         "", // ERROR: required
		LastName:          "Mustermann",
		SchoolClass:       "1A",
		GroupName:         "", // INFO: empty group
		DataRetentionDays: 30,
	}

	errors := config.Validate(context.Background(), &row)

	// Should have at least 2 errors: 1 ERROR + 1 INFO
	assert.GreaterOrEqual(t, len(errors), 2)

	// Check severity levels
	hasError := false
	hasInfo := false

	for _, err := range errors {
		switch err.Severity {
		case importModels.ErrorSeverityError:
			hasError = true
		case importModels.ErrorSeverityInfo:
			hasInfo = true
		}
	}

	assert.True(t, hasError, "Should have at least one ERROR severity")
	assert.True(t, hasInfo, "Should have at least one INFO severity")
}

func TestStudentImportConfig_ValidateGuardian_EmailFormats(t *testing.T) {
	config := &StudentImportConfig{}

	tests := []struct {
		email     string
		wantError bool
	}{
		{"valid@example.com", false},
		{"user.name@example.com", false},
		{"user+tag@example.co.uk", false},
		{"123@test.de", false},
		{"not-an-email", true},
		{"missing@domain", true},
		{"@example.com", true},
		{"user@", true},
		{"user example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			guardian := importModels.GuardianImportData{
				Email: tt.email,
			}

			errors := config.validateGuardian(1, guardian)

			if tt.wantError {
				hasEmailError := false
				for _, err := range errors {
					if err.Code == "invalid_email" {
						hasEmailError = true
						break
					}
				}
				assert.True(t, hasEmailError, "Expected email validation error")
			} else {
				for _, err := range errors {
					if err.Code == "invalid_email" {
						t.Errorf("Unexpected email error for '%s'", tt.email)
					}
				}
			}
		})
	}
}

func TestStudentImportConfig_ValidateGuardian_PhoneFormats(t *testing.T) {
	config := &StudentImportConfig{}

	tests := []struct {
		phone     string
		wantError bool
	}{
		{"0123-456789", false},
		{"+49 123 456789", false},
		{"0176-12345678", false},
		{"+49-176-12345678", false},
		{"abc", true},
		{"12", true}, // Too short
		{"", false},  // Empty is ok (validated separately)
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			guardian := importModels.GuardianImportData{
				Email: "valid@example.com", // Provide email so contact validation passes
				Phone: tt.phone,
			}

			errors := config.validateGuardian(1, guardian)

			if tt.wantError {
				hasPhoneError := false
				for _, err := range errors {
					if err.Code == "invalid_phone" {
						hasPhoneError = true
						break
					}
				}
				assert.True(t, hasPhoneError, "Expected phone validation error for '%s'", tt.phone)
			} else {
				for _, err := range errors {
					if err.Code == "invalid_phone" {
						t.Errorf("Unexpected phone error for '%s'", tt.phone)
					}
				}
			}
		})
	}
}

// ============================================================================
// Language Preference Validation Tests
// ============================================================================

func TestValidateGuardianLanguage(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		wantWarn bool
	}{
		{"empty is valid", "", false},
		{"de is valid", "de", false},
		{"en is valid", "en", false},
		{"tr is valid", "tr", false},
		{"ar is valid", "ar", false},
		{"DE uppercase is valid", "DE", false},
		{"unknown code warns", "klingon", true},
		{"xx warns", "xx", true},
		{"ja not in list warns", "ja", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateGuardianLanguage(1, tt.lang, "guardian_1")
			if tt.wantWarn {
				assert.Len(t, errs, 1)
				assert.Equal(t, "unknown_language", errs[0].Code)
				assert.Equal(t, importModels.ErrorSeverityWarning, errs[0].Severity)
			} else {
				assert.Empty(t, errs)
			}
		})
	}
}

func TestStudentImportConfig_Validate_GuardianLanguageWarning(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	row := importModels.StudentImportRow{
		FirstName:   "Max",
		LastName:    "Mustermann",
		SchoolClass: "1A",
		Guardians: []importModels.GuardianImportData{
			{
				Email:              "test@example.com",
				LanguagePreference: "klingon",
			},
		},
		DataRetentionDays: 30,
	}

	errors := config.Validate(context.Background(), &row)

	hasLangWarning := false
	for _, err := range errors {
		if err.Code == "unknown_language" {
			hasLangWarning = true
			assert.Equal(t, importModels.ErrorSeverityWarning, err.Severity,
				"unknown language should be a warning, not an error")
			assert.Contains(t, err.Message, "klingon")
		}
	}
	assert.True(t, hasLangWarning, "should warn about unknown language code")
}

// ============================================================================
// Pickup Schedule Validation Tests
// ============================================================================

func TestStudentImportConfig_Validate_PickupSchedule(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	tests := []struct {
		name      string
		schedules []importModels.PickupScheduleImportData
		wantCodes []string
	}{
		{
			name: "valid schedule",
			schedules: []importModels.PickupScheduleImportData{
				{Weekday: 1, PickupTime: "16:00"},
				{Weekday: 5, PickupTime: "14:30"},
			},
			wantCodes: nil,
		},
		{
			name: "invalid weekday 0",
			schedules: []importModels.PickupScheduleImportData{
				{Weekday: 0, PickupTime: "16:00"},
			},
			wantCodes: []string{"invalid_weekday"},
		},
		{
			name: "invalid weekday 6",
			schedules: []importModels.PickupScheduleImportData{
				{Weekday: 6, PickupTime: "16:00"},
			},
			wantCodes: []string{"invalid_weekday"},
		},
		{
			name: "invalid time format",
			schedules: []importModels.PickupScheduleImportData{
				{Weekday: 1, PickupTime: "25:00"},
			},
			wantCodes: []string{"invalid_time_format"},
		},
		{
			name: "invalid time text",
			schedules: []importModels.PickupScheduleImportData{
				{Weekday: 1, PickupTime: "nachmittags"},
			},
			wantCodes: []string{"invalid_time_format"},
		},
		{
			name: "valid edge times",
			schedules: []importModels.PickupScheduleImportData{
				{Weekday: 1, PickupTime: "00:00"},
				{Weekday: 2, PickupTime: "23:59"},
				{Weekday: 3, PickupTime: "9:30"},
			},
			wantCodes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := importModels.StudentImportRow{
				FirstName:         "Max",
				LastName:          "Mustermann",
				SchoolClass:       "1A",
				PickupSchedules:   tt.schedules,
				DataRetentionDays: 30,
			}

			errors := config.Validate(context.Background(), &row)

			for _, expectedCode := range tt.wantCodes {
				found := false
				for _, err := range errors {
					if err.Code == expectedCode {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error code '%s' not found", expectedCode)
			}

			if tt.wantCodes == nil {
				for _, err := range errors {
					if err.Code == "invalid_weekday" || err.Code == "invalid_time_format" {
						t.Errorf("Unexpected pickup schedule error: %s", err.Message)
					}
				}
			}
		})
	}
}

func TestStudentImportConfig_Validate_ArrivalSchedule(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	tests := []struct {
		name      string
		schedules []importModels.ArrivalScheduleImportData
		wantCodes []string
	}{
		{
			name: "valid schedule",
			schedules: []importModels.ArrivalScheduleImportData{
				{Weekday: 1, ExpectedArrival: "08:00"},
				{Weekday: 5, ExpectedArrival: "08:30"},
			},
			wantCodes: nil,
		},
		{
			name: "invalid weekday",
			schedules: []importModels.ArrivalScheduleImportData{
				{Weekday: 6, ExpectedArrival: "08:00"},
			},
			wantCodes: []string{"invalid_weekday"},
		},
		{
			name: "invalid time format",
			schedules: []importModels.ArrivalScheduleImportData{
				{Weekday: 1, ExpectedArrival: "morgens"},
			},
			wantCodes: []string{"invalid_time_format"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := importModels.StudentImportRow{
				FirstName:         "Max",
				LastName:          "Mustermann",
				SchoolClass:       "1A",
				ArrivalSchedules:  tt.schedules,
				DataRetentionDays: 30,
			}

			errors := config.Validate(context.Background(), &row)

			for _, expectedCode := range tt.wantCodes {
				found := false
				for _, err := range errors {
					if err.Code == expectedCode {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error code '%s' not found", expectedCode)
			}

			if tt.wantCodes == nil {
				for _, err := range errors {
					if err.Field == "arrival_schedule" {
						t.Errorf("Unexpected arrival schedule error: %s", err.Message)
					}
				}
			}
		})
	}
}

// ============================================================================
// isValidTimeFormat Tests
// ============================================================================

func TestIsValidTimeFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"16:00", true},
		{"09:30", true},
		{"9:30", true},
		{"00:00", true},
		{"23:59", true},
		{"25:00", false},
		{"12:60", false},
		{"abc", false},
		{"", false},
		{"1630", false},
		{"16:00:00", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isValidTimeFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// Guardian Profile Helper Tests
// ============================================================================

func TestGuardianLanguagePreference(t *testing.T) {
	assert.Equal(t, "de", guardianLanguagePreference(""))
	assert.Equal(t, "de", guardianLanguagePreference("de"))
	assert.Equal(t, "en", guardianLanguagePreference("EN"))
	assert.Equal(t, "tr", guardianLanguagePreference("  TR  "))
}

// ============================================================================
// Combined Validation: Guardian + Pickup Schedule
// ============================================================================

func TestStudentImportConfig_Validate_CombinedGuardianAndPickupErrors(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	row := importModels.StudentImportRow{
		FirstName:   "Max",
		LastName:    "Mustermann",
		SchoolClass: "1A",
		Guardians: []importModels.GuardianImportData{
			{
				Email: "test@example.com",
			},
		},
		PickupSchedules: []importModels.PickupScheduleImportData{
			{Weekday: 0, PickupTime: "16:00"},   // Invalid weekday
			{Weekday: 1, PickupTime: "invalid"}, // Invalid time
		},
		DataRetentionDays: 30,
	}

	errors := config.Validate(context.Background(), &row)

	// Should have: invalid_weekday + invalid_time_format + group_empty
	codeCount := map[string]int{}
	for _, err := range errors {
		codeCount[err.Code]++
	}

	assert.Equal(t, 1, codeCount["invalid_weekday"], "should have 1 invalid_weekday error")
	assert.Equal(t, 1, codeCount["invalid_time_format"], "should have 1 invalid_time_format error")
	assert.Equal(t, 1, codeCount["group_empty"], "should have 1 group_empty info")
}

func TestStudentImportConfig_Validate_MultipleInvalidPickupSchedules(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	row := importModels.StudentImportRow{
		FirstName:   "Max",
		LastName:    "Mustermann",
		SchoolClass: "1A",
		PickupSchedules: []importModels.PickupScheduleImportData{
			{Weekday: 0, PickupTime: "16:00"}, // Invalid weekday
			{Weekday: 6, PickupTime: "15:00"}, // Invalid weekday
			{Weekday: 7, PickupTime: "14:00"}, // Invalid weekday
			{Weekday: 1, PickupTime: "25:99"}, // Invalid time
			{Weekday: 2, PickupTime: "abc"},   // Invalid time
		},
		DataRetentionDays: 30,
	}

	errors := config.Validate(context.Background(), &row)

	weekdayErrors := 0
	timeErrors := 0
	for _, err := range errors {
		if err.Code == "invalid_weekday" {
			weekdayErrors++
		}
		if err.Code == "invalid_time_format" {
			timeErrors++
		}
	}

	assert.Equal(t, 3, weekdayErrors, "should have 3 invalid weekday errors")
	assert.Equal(t, 2, timeErrors, "should have 2 invalid time errors")
}

func TestStudentImportConfig_Validate_PickupScheduleEmptyListIsValid(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	row := importModels.StudentImportRow{
		FirstName:         "Max",
		LastName:          "Mustermann",
		SchoolClass:       "1A",
		PickupSchedules:   nil, // No schedules
		DataRetentionDays: 30,
	}

	errors := config.Validate(context.Background(), &row)

	// Should only have group_empty info, no pickup errors
	for _, err := range errors {
		if err.Code == "invalid_weekday" || err.Code == "invalid_time_format" {
			t.Errorf("Unexpected pickup schedule error with empty list: %s", err.Message)
		}
	}
}

func TestStudentImportConfig_Validate_PickupScheduleErrorMessages(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	t.Run("weekday error message contains weekday number", func(t *testing.T) {
		row := importModels.StudentImportRow{
			FirstName:   "Max",
			LastName:    "Mustermann",
			SchoolClass: "1A",
			PickupSchedules: []importModels.PickupScheduleImportData{
				{Weekday: 8, PickupTime: "16:00"},
			},
			DataRetentionDays: 30,
		}

		errors := config.Validate(context.Background(), &row)

		for _, err := range errors {
			if err.Code == "invalid_weekday" {
				assert.Contains(t, err.Message, "8", "error message should contain the invalid weekday")
				assert.Contains(t, err.Message, "1 (Mo)", "error message should explain valid range")
				assert.Contains(t, err.Message, "5 (Fr)", "error message should explain valid range")
			}
		}
	})

	t.Run("time error message contains invalid value", func(t *testing.T) {
		row := importModels.StudentImportRow{
			FirstName:   "Max",
			LastName:    "Mustermann",
			SchoolClass: "1A",
			PickupSchedules: []importModels.PickupScheduleImportData{
				{Weekday: 1, PickupTime: "nachmittags"},
			},
			DataRetentionDays: 30,
		}

		errors := config.Validate(context.Background(), &row)

		for _, err := range errors {
			if err.Code == "invalid_time_format" {
				assert.Contains(t, err.Message, "nachmittags", "error message should contain the invalid time")
				assert.Contains(t, err.Message, "HH:MM", "error message should explain correct format")
			}
		}
	})
}

// ============================================================================
// createPickupSchedules nil-repo safety
// ============================================================================

func TestStudentImportConfig_CreateArrivalSchedules_NilRepo(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{ArrivalScheduleRepo: nil}}

	schedules := []importModels.ArrivalScheduleImportData{
		{Weekday: 1, ExpectedArrival: "08:00"},
	}

	err := config.createArrivalSchedules(context.Background(), 123, schedules)
	assert.NoError(t, err, "should not error when arrivalScheduleRepo is nil")
}

func TestStudentImportConfig_CreateArrivalSchedules_EmptySchedules(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{ArrivalScheduleRepo: nil}}

	err := config.createArrivalSchedules(context.Background(), 123, nil)
	assert.NoError(t, err)

	err = config.createArrivalSchedules(context.Background(), 123, []importModels.ArrivalScheduleImportData{})
	assert.NoError(t, err)
}

func TestStudentImportConfig_CreatePickupSchedules_NilRepo(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{
		// No repo
		PickupScheduleRepo: nil},
	}

	schedules := []importModels.PickupScheduleImportData{
		{Weekday: 1, PickupTime: "16:00"},
	}

	// Should return nil (no-op) when repo is nil
	err := config.createPickupSchedules(context.Background(), 123, schedules)
	assert.NoError(t, err, "should not error when pickupScheduleRepo is nil")
}

func TestStudentImportConfig_CreatePickupSchedules_EmptySchedules(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{
		// Would panic if called, but shouldn't be
		PickupScheduleRepo: nil},
	}

	// Should return nil (no-op) when schedules is empty
	err := config.createPickupSchedules(context.Background(), 123, nil)
	assert.NoError(t, err)

	err = config.createPickupSchedules(context.Background(), 123, []importModels.PickupScheduleImportData{})
	assert.NoError(t, err)
}

// ============================================================================
// Validate: guardian with new fields passes existing validation
// ============================================================================

func TestStudentImportConfig_Validate_GuardianWithProfileFieldsStillValidatesContact(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	// Guardian has profile fields but NO contact info -> should still require contact
	row := importModels.StudentImportRow{
		FirstName:   "Max",
		LastName:    "Mustermann",
		SchoolClass: "1A",
		Guardians: []importModels.GuardianImportData{
			{
				FirstName:     "Maria",
				LastName:      "Müller",
				AddressStreet: "Musterstr. 1",
				AddressCity:   "Köln",
				// No email, phone, or phone numbers
			},
		},
		DataRetentionDays: 30,
	}

	errors := config.Validate(context.Background(), &row)

	hasContactRequired := false
	for _, err := range errors {
		if err.Code == "guardian_contact_required" {
			hasContactRequired = true
			break
		}
	}

	assert.True(t, hasContactRequired,
		"guardian with profile fields but no contact should still require contact info")
}

// ============================================================================
// Validate: pickup schedule boundary values
// ============================================================================

func TestStudentImportConfig_Validate_PickupScheduleBoundaryWeekdays(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	tests := []struct {
		name      string
		weekday   int
		wantError bool
	}{
		{"weekday -1 invalid", -1, true},
		{"weekday 0 invalid", 0, true},
		{"weekday 1 valid (Monday)", 1, false},
		{"weekday 3 valid (Wednesday)", 3, false},
		{"weekday 5 valid (Friday)", 5, false},
		{"weekday 6 invalid (Saturday)", 6, true},
		{"weekday 7 invalid (Sunday)", 7, true},
		{"weekday 100 invalid", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := importModels.StudentImportRow{
				FirstName:   "Max",
				LastName:    "Mustermann",
				SchoolClass: "1A",
				PickupSchedules: []importModels.PickupScheduleImportData{
					{Weekday: tt.weekday, PickupTime: "16:00"},
				},
				DataRetentionDays: 30,
			}

			errors := config.Validate(context.Background(), &row)

			hasWeekdayError := false
			for _, err := range errors {
				if err.Code == "invalid_weekday" {
					hasWeekdayError = true
					break
				}
			}

			if tt.wantError {
				assert.True(t, hasWeekdayError, "Expected weekday error for %d", tt.weekday)
			} else {
				assert.False(t, hasWeekdayError, "Unexpected weekday error for %d", tt.weekday)
			}
		})
	}
}

func TestStudentImportConfig_Validate_PickupTimeBoundaryValues(t *testing.T) {
	config := &StudentImportConfig{StudentImportDeps: StudentImportDeps{Resolver: &RelationshipResolver{
		groupCache: make(map[string]*education.Group),
	}},
	}

	tests := []struct {
		name      string
		time      string
		wantError bool
	}{
		{"midnight", "00:00", false},
		{"early morning", "06:30", false},
		{"afternoon", "15:30", false},
		{"late evening", "23:59", false},
		{"single digit hour", "9:00", false},
		{"invalid 24:00", "24:00", true},
		{"invalid 25:00", "25:00", true},
		{"invalid 12:60", "12:60", true},
		{"no colon", "1630", true},
		{"with seconds", "16:00:00", true},
		{"text", "four pm", true},
		{"empty", "", true},
		{"only colon", ":", true},
		{"negative", "-1:00", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := importModels.StudentImportRow{
				FirstName:   "Max",
				LastName:    "Mustermann",
				SchoolClass: "1A",
				PickupSchedules: []importModels.PickupScheduleImportData{
					{Weekday: 1, PickupTime: tt.time},
				},
				DataRetentionDays: 30,
			}

			errors := config.Validate(context.Background(), &row)

			hasTimeError := false
			for _, err := range errors {
				if err.Code == "invalid_time_format" {
					hasTimeError = true
					break
				}
			}

			if tt.wantError {
				assert.True(t, hasTimeError, "Expected time format error for '%s'", tt.time)
			} else {
				assert.False(t, hasTimeError, "Unexpected time format error for '%s'", tt.time)
			}
		})
	}
}

// ============================================================================
// MapRelationshipType Tests
// ============================================================================

func TestMapRelationshipType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Parent types
		{"Mutter", "parent"},
		{"mutter", "parent"},
		{"MUTTER", "parent"},
		{"Vater", "parent"},
		{"vater", "parent"},
		{"mama", "parent"},
		{"papa", "parent"},
		// Relative types
		{"Großmutter", "relative"},
		{"Oma", "relative"},
		{"Großvater", "relative"},
		{"Opa", "relative"},
		{"Tante", "relative"},
		{"Onkel", "relative"},
		{"Geschwister", "relative"},
		{"Schwester", "relative"},
		{"Bruder", "relative"},
		// Other types
		{"Andere", "other"},
		{"sonstige", "other"},
		{"unknown", "other"}, // Unknown values map to "other"
		{"", "other"},        // Empty maps to "other"
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MapRelationshipType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// EntityName Tests
// ============================================================================

func TestStudentImportConfig_EntityName(t *testing.T) {
	config := &StudentImportConfig{}

	name := config.EntityName()

	assert.Equal(t, "student", name)
}

// ============================================================================
// stringPtr Tests
// ============================================================================

func TestStringPtr(t *testing.T) {
	t.Run("returns pointer to string", func(t *testing.T) {
		result := strutil.TrimToNil("test")
		assert.NotNil(t, result)
		assert.Equal(t, "test", *result)
	})

	t.Run("returns nil for empty string", func(t *testing.T) {
		result := strutil.TrimToNil("")
		assert.Nil(t, result)
	})

	t.Run("returns nil for whitespace-only string", func(t *testing.T) {
		result := strutil.TrimToNil("   ")
		assert.Nil(t, result)
	})

	t.Run("trims whitespace", func(t *testing.T) {
		result := strutil.TrimToNil("  test  ")
		assert.NotNil(t, result)
		assert.Equal(t, "test", *result)
	})
}

// ============================================================================
// parseOptionalDate Tests
// ============================================================================

func TestParseOptionalDate(t *testing.T) {
	futureISO := futureBirthdayISOForTests()
	futureGermanLong := futureBirthdayGermanLongForTests()
	futureGermanShort := futureBirthdayGermanShortForTests()

	tests := []struct {
		name      string
		input     string
		wantDate  bool
		wantError bool
		wantISO   string
	}{
		{"empty string returns nil", "", false, false, ""},
		{"valid ISO date", "2015-08-15", true, false, "2015-08-15"},
		{"valid ISO date 2", "2020-01-01", true, false, "2020-01-01"},
		{"valid German date DD.MM.YYYY", "15.08.2015", true, false, "2015-08-15"},
		{"valid German date DD.MM.YY", "15.08.15", true, false, "2015-08-15"},
		{"future ISO date rejected", futureISO, false, true, ""},
		{"future German date DD.MM.YYYY rejected", futureGermanLong, false, true, ""},
		{"future German short date rejected", futureGermanShort, false, true, ""},
		{"invalid date", "2015-13-45", false, true, ""},
		{"random text", "invalid", false, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseOptionalDate(tt.input)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantDate {
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantISO, result.Format("2006-01-02"))
			} else if !tt.wantError {
				assert.Nil(t, result)
			}
		})
	}
}

// ============================================================================
// validateRetentionDays Tests
// ============================================================================

func TestValidateRetentionDays(t *testing.T) {
	tests := []struct {
		name     string
		days     int
		expected int
	}{
		{"valid minimum", 1, 1},
		{"valid default", 30, 30},
		{"valid maximum", 31, 31},
		{"invalid zero clamps to default", 0, 30},
		{"invalid negative clamps to default", -5, 30},
		{"over maximum clamps to 31", 32, 31},
		{"way over maximum clamps to 31", 365, 31},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateRetentionDays(tt.days)
			assert.Equal(t, tt.expected, result)
		})
	}
}
