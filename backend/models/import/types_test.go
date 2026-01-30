package importpkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// ImportMode Constants Tests
// =============================================================================

func TestImportModeConstants(t *testing.T) {
	assert.Equal(t, ImportMode("create"), ImportModeCreate)
	assert.Equal(t, ImportMode("update"), ImportModeUpdate)
	assert.Equal(t, ImportMode("upsert"), ImportModeUpsert)
}

// =============================================================================
// ErrorSeverity Constants Tests
// =============================================================================

func TestErrorSeverityConstants(t *testing.T) {
	assert.Equal(t, ErrorSeverity("error"), ErrorSeverityError)
	assert.Equal(t, ErrorSeverity("warning"), ErrorSeverityWarning)
	assert.Equal(t, ErrorSeverity("info"), ErrorSeverityInfo)
}

// =============================================================================
// ImportRequest Tests
// =============================================================================

func TestImportRequest_DefaultValues(t *testing.T) {
	req := ImportRequest[StudentImportRow]{}

	assert.Empty(t, req.Rows)
	assert.Equal(t, ImportMode(""), req.Mode)
	assert.False(t, req.DryRun)
	assert.False(t, req.StopOnError)
	assert.Equal(t, int64(0), req.UserID)
	assert.False(t, req.SkipInvalidRows)
}

func TestImportRequest_WithValues(t *testing.T) {
	req := ImportRequest[StudentImportRow]{
		Rows: []StudentImportRow{
			{FirstName: "Max", LastName: "Mustermann", SchoolClass: "1a"},
		},
		Mode:            ImportModeUpsert,
		DryRun:          true,
		StopOnError:     true,
		UserID:          42,
		SkipInvalidRows: true,
	}

	assert.Len(t, req.Rows, 1)
	assert.Equal(t, ImportModeUpsert, req.Mode)
	assert.True(t, req.DryRun)
	assert.True(t, req.StopOnError)
	assert.Equal(t, int64(42), req.UserID)
	assert.True(t, req.SkipInvalidRows)
}

// =============================================================================
// ImportResult Tests
// =============================================================================

func TestImportResult_DefaultValues(t *testing.T) {
	result := ImportResult[StudentImportRow]{}

	assert.Zero(t, result.TotalRows)
	assert.Zero(t, result.CreatedCount)
	assert.Zero(t, result.UpdatedCount)
	assert.Zero(t, result.SkippedCount)
	assert.Zero(t, result.ErrorCount)
	assert.Zero(t, result.WarningCount)
	assert.Empty(t, result.Errors)
	assert.Empty(t, result.BulkActions)
	assert.False(t, result.DryRun)
}

// =============================================================================
// ValidationError Tests
// =============================================================================

func TestValidationError_Fields(t *testing.T) {
	fix := &AutoFix{
		Action:      "replace",
		Replacement: "1A",
		Description: "Klasse korrigieren",
	}

	ve := ValidationError{
		Field:       "school_class",
		Message:     "Klasse nicht gefunden",
		Code:        "INVALID_CLASS",
		Severity:    ErrorSeverityError,
		Suggestions: []string{"1A", "1B", "1C"},
		AutoFix:     fix,
		ActualValue: "1X",
	}

	assert.Equal(t, "school_class", ve.Field)
	assert.Equal(t, "Klasse nicht gefunden", ve.Message)
	assert.Equal(t, "INVALID_CLASS", ve.Code)
	assert.Equal(t, ErrorSeverityError, ve.Severity)
	assert.Len(t, ve.Suggestions, 3)
	assert.NotNil(t, ve.AutoFix)
	assert.Equal(t, "1X", ve.ActualValue)
}

func TestValidationError_MinimalFields(t *testing.T) {
	ve := ValidationError{
		Field:    "first_name",
		Message:  "Pflichtfeld",
		Code:     "REQUIRED",
		Severity: ErrorSeverityError,
	}

	assert.Equal(t, "first_name", ve.Field)
	assert.Empty(t, ve.Suggestions)
	assert.Nil(t, ve.AutoFix)
	assert.Empty(t, ve.ActualValue)
}

// =============================================================================
// AutoFix Tests
// =============================================================================

func TestAutoFix_Fields(t *testing.T) {
	fix := AutoFix{
		Action:      "replace",
		Replacement: "Gruppe 1A",
		Description: "Gruppe umbenennen",
	}

	assert.Equal(t, "replace", fix.Action)
	assert.Equal(t, "Gruppe 1A", fix.Replacement)
	assert.Equal(t, "Gruppe umbenennen", fix.Description)
}

// =============================================================================
// BulkAction Tests
// =============================================================================

func TestBulkAction_Fields(t *testing.T) {
	ba := BulkAction{
		Title:        "5 Zeilen verwenden 'Gruppe A'",
		Description:  "Alle zu 'Gruppe 1A' ändern?",
		Action:       "replace_all",
		AffectedRows: []int{1, 3, 5, 7, 9},
		Field:        "group",
		OldValue:     "Gruppe A",
		NewValue:     "Gruppe 1A",
	}

	assert.Equal(t, "5 Zeilen verwenden 'Gruppe A'", ba.Title)
	assert.Equal(t, "replace_all", ba.Action)
	assert.Len(t, ba.AffectedRows, 5)
	assert.Equal(t, "group", ba.Field)
	assert.Equal(t, "Gruppe A", ba.OldValue)
	assert.Equal(t, "Gruppe 1A", ba.NewValue)
}

// =============================================================================
// ImportError Tests
// =============================================================================

func TestImportError_Fields(t *testing.T) {
	ie := ImportError[StudentImportRow]{
		RowNumber: 5,
		Data:      StudentImportRow{FirstName: "Test", LastName: "User"},
		Errors: []ValidationError{
			{Field: "school_class", Message: "Required", Code: "REQUIRED", Severity: ErrorSeverityError},
		},
	}

	assert.Equal(t, 5, ie.RowNumber)
	assert.Equal(t, "Test", ie.Data.FirstName)
	assert.Len(t, ie.Errors, 1)
}
