package importpkg

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	importModels "github.com/moto-nrw/project-phoenix/internal/core/domain/import"
	"github.com/stretchr/testify/assert"
)

func TestStudentImportConfig_Validate_RequiredFields(t *testing.T) {
	config := &StudentImportConfig{
		resolver: &RelationshipResolver{
			groupCache: make(map[string]*education.Group),
		},
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
	config := &StudentImportConfig{
		resolver: &RelationshipResolver{
			groupCache: make(map[string]*education.Group),
		},
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

func TestStudentImportConfig_Validate_DataRetention(t *testing.T) {
	config := &StudentImportConfig{
		resolver: &RelationshipResolver{
			groupCache: make(map[string]*education.Group),
		},
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
	config := &StudentImportConfig{
		resolver: &RelationshipResolver{
			groupCache: make(map[string]*education.Group),
		},
	}

	tests := []struct {
		name      string
		birthday  string
		wantError bool
	}{
		{"valid ISO format", "2015-08-15", false},
		{"valid ISO format 2", "2014-03-22", false},
		{"empty (optional)", "", false},
		{"invalid format DD.MM.YYYY", "15.08.2015", true},
		{"invalid format DD/MM/YYYY", "15/08/2015", true},
		{"invalid format YYYY/MM/DD", "2015/08/15", true},
		{"invalid date", "2015-13-45", true},
		{"just text", "invalid", true},
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
					if err.Code == "invalid_date_format" {
						hasBirthdayError = true
						assert.Contains(t, err.Message, "JJJJ-MM-TT")
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
			}
		})
	}
}

func TestStudentImportConfig_Validate_ErrorSeverity(t *testing.T) {
	config := &StudentImportConfig{
		resolver: &RelationshipResolver{
			groupCache: make(map[string]*education.Group),
		},
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
// mapRelationshipType Tests
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
			result := mapRelationshipType(tt.input)
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
		result := stringPtr("test")
		assert.NotNil(t, result)
		assert.Equal(t, "test", *result)
	})

	t.Run("returns nil for empty string", func(t *testing.T) {
		result := stringPtr("")
		assert.Nil(t, result)
	})

	t.Run("returns nil for whitespace-only string", func(t *testing.T) {
		result := stringPtr("   ")
		assert.Nil(t, result)
	})

	t.Run("trims whitespace", func(t *testing.T) {
		result := stringPtr("  test  ")
		assert.NotNil(t, result)
		assert.Equal(t, "test", *result)
	})
}

// ============================================================================
// parseOptionalDate Tests
// ============================================================================

func TestParseOptionalDate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantDate  bool
		wantError bool
	}{
		{"empty string returns nil", "", false, false},
		{"valid ISO date", "2015-08-15", true, false},
		{"valid ISO date 2", "2020-01-01", true, false},
		{"invalid format DD.MM.YYYY", "15.08.2015", false, true},
		{"invalid date", "2015-13-45", false, true},
		{"random text", "invalid", false, true},
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
