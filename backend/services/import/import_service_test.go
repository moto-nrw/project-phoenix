package importpkg

import (
	"context"
	"errors"
	"testing"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRow is a simple test struct for the generic import service
type testRow struct {
	Name  string
	Value string
}

// mockImportConfig is a mock implementation of ImportConfig[testRow]
type mockImportConfig struct {
	preloadErr      error
	validateErrors  []importModels.ValidationError
	findExistingID  *int64
	findExistingErr error
	createID        int64
	createErr       error
	updateErr       error
	entityName      string
}

func (m *mockImportConfig) PreloadReferenceData(ctx context.Context) error {
	return m.preloadErr
}

func (m *mockImportConfig) Validate(ctx context.Context, row *testRow) []importModels.ValidationError {
	return m.validateErrors
}

func (m *mockImportConfig) FindExisting(ctx context.Context, row testRow) (*int64, error) {
	return m.findExistingID, m.findExistingErr
}

func (m *mockImportConfig) Create(ctx context.Context, row testRow) (int64, error) {
	return m.createID, m.createErr
}

func (m *mockImportConfig) Update(ctx context.Context, id int64, row testRow) error {
	return m.updateErr
}

func (m *mockImportConfig) EntityName() string {
	if m.entityName == "" {
		return "TestEntity"
	}
	return m.entityName
}

// ============================================================================
// NewImportService Tests
// ============================================================================

func TestNewImportService(t *testing.T) {
	t.Run("creates service with config", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{}

		// ACT
		service := NewImportService[testRow](config, nil)

		// ASSERT
		require.NotNil(t, service)
		assert.Equal(t, 100, service.batchSize)
	})
}

// ============================================================================
// Import Tests
// ============================================================================

func TestImportService_Import(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when preload fails", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{
			preloadErr: errors.New("preload failed"),
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows: []testRow{{Name: "test", Value: "value"}},
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "preload")
	})

	t.Run("dry run creates count without actual import", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{
			findExistingID: nil, // New entity
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test1", Value: "value1"}, {Name: "test2", Value: "value2"}},
			DryRun: true,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.CreatedCount)
		assert.Equal(t, 0, result.UpdatedCount)
		assert.True(t, result.DryRun)
	})

	t.Run("dry run update count for existing entities", func(t *testing.T) {
		// ARRANGE
		existingID := int64(123)
		config := &mockImportConfig{
			findExistingID: &existingID, // Existing entity
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test1", Value: "value1"}},
			DryRun: true,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 0, result.CreatedCount)
		assert.Equal(t, 1, result.UpdatedCount)
	})

	t.Run("creates new entities in upsert mode", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{
			findExistingID: nil, // New entity
			createID:       1,
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test", Value: "value"}},
			DryRun: false,
			Mode:   importModels.ImportModeUpsert,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.CreatedCount)
	})

	t.Run("updates existing entities in upsert mode", func(t *testing.T) {
		// ARRANGE
		existingID := int64(123)
		config := &mockImportConfig{
			findExistingID: &existingID,
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test", Value: "value"}},
			DryRun: false,
			Mode:   importModels.ImportModeUpsert,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.UpdatedCount)
	})

	t.Run("records validation errors", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{
			validateErrors: []importModels.ValidationError{
				{Field: "name", Message: "Name is required", Code: "required", Severity: importModels.ErrorSeverityError},
			},
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "", Value: "value"}},
			DryRun: false,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.ErrorCount)
		assert.Len(t, result.Errors, 1)
	})

	t.Run("records validation warnings", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{
			validateErrors: []importModels.ValidationError{
				{Field: "name", Message: "Name is unusual", Code: "warning", Severity: importModels.ErrorSeverityWarning},
			},
			createID: 1,
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test", Value: "value"}},
			DryRun: false,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.WarningCount)
		assert.Equal(t, 0, result.ErrorCount)
		assert.Equal(t, 1, result.CreatedCount)
	})

	t.Run("stops on error when StopOnError is true", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{
			validateErrors: []importModels.ValidationError{
				{Field: "name", Message: "Invalid", Code: "invalid", Severity: importModels.ErrorSeverityError},
			},
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:        []testRow{{Name: "test1"}, {Name: "test2"}},
			DryRun:      false,
			StopOnError: true,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.ErrorCount) // Only first row processed due to StopOnError
	})

	t.Run("records creation errors", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{
			findExistingID: nil,
			createErr:      errors.New("creation failed"),
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test", Value: "value"}},
			DryRun: false,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.ErrorCount)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "creation_failed", result.Errors[0].Errors[0].Code)
	})

	t.Run("records update errors", func(t *testing.T) {
		// ARRANGE
		existingID := int64(123)
		config := &mockImportConfig{
			findExistingID: &existingID,
			updateErr:      errors.New("update failed"),
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test", Value: "value"}},
			DryRun: false,
			Mode:   importModels.ImportModeUpsert,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.ErrorCount)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "update_failed", result.Errors[0].Errors[0].Code)
	})

	t.Run("records duplicate check errors", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{
			findExistingErr: errors.New("db error"),
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test", Value: "value"}},
			DryRun: false,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.ErrorCount)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "duplicate_check_failed", result.Errors[0].Errors[0].Code)
	})

	t.Run("in create mode records error for existing entity", func(t *testing.T) {
		// ARRANGE
		existingID := int64(123)
		config := &mockImportConfig{
			findExistingID: &existingID,
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test", Value: "value"}},
			DryRun: false,
			Mode:   importModels.ImportModeCreate,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.ErrorCount)
		assert.Equal(t, "already_exists", result.Errors[0].Errors[0].Code)
	})

	t.Run("in update mode records error for new entity", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{
			findExistingID: nil, // Not found
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test", Value: "value"}},
			DryRun: false,
			Mode:   importModels.ImportModeUpdate,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.ErrorCount)
		assert.Equal(t, "not_found", result.Errors[0].Errors[0].Code)
	})

	t.Run("records timing information", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test", Value: "value"}},
			DryRun: true,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.StartedAt.IsZero())
		assert.False(t, result.CompletedAt.IsZero())
		assert.True(t, result.CompletedAt.After(result.StartedAt) || result.CompletedAt.Equal(result.StartedAt))
	})
}

// ============================================================================
// categorizeValidationErrors Tests
// ============================================================================

func TestCategorizeValidationErrors(t *testing.T) {
	t.Run("separates errors and warnings", func(t *testing.T) {
		// ARRANGE
		errors := []importModels.ValidationError{
			{Field: "name", Message: "Required", Severity: importModels.ErrorSeverityError},
			{Field: "value", Message: "Unusual", Severity: importModels.ErrorSeverityWarning},
			{Field: "other", Message: "Invalid", Severity: importModels.ErrorSeverityError},
		}

		// ACT
		blockingErrors, warnings := categorizeValidationErrors(errors)

		// ASSERT
		assert.Len(t, blockingErrors, 2)
		assert.Len(t, warnings, 1)
	})

	t.Run("handles empty input", func(t *testing.T) {
		// ACT
		blockingErrors, warnings := categorizeValidationErrors([]importModels.ValidationError{})

		// ASSERT
		assert.Empty(t, blockingErrors)
		assert.Empty(t, warnings)
	})
}

// ============================================================================
// generateBulkActions Tests
// ============================================================================

func TestGenerateBulkActions(t *testing.T) {
	t.Run("generates bulk actions for repeated errors", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{}
		service := NewImportService[testRow](config, nil)

		errors := []importModels.ImportError[testRow]{
			{
				RowNumber: 2,
				Data:      testRow{Name: "test1"},
				Errors: []importModels.ValidationError{
					{
						Field:       "name",
						Message:     "Invalid",
						ActualValue: "wrong_value",
						AutoFix: &importModels.AutoFix{
							Action:      "replace",
							Replacement: "correct_value",
							Description: "Fix the value",
						},
					},
				},
			},
			{
				RowNumber: 3,
				Data:      testRow{Name: "test2"},
				Errors: []importModels.ValidationError{
					{
						Field:       "name",
						Message:     "Invalid",
						ActualValue: "wrong_value",
						AutoFix: &importModels.AutoFix{
							Action:      "replace",
							Replacement: "correct_value",
							Description: "Fix the value",
						},
					},
				},
			},
		}

		// ACT
		bulkActions := service.generateBulkActions(errors)

		// ASSERT
		require.Len(t, bulkActions, 1)
		assert.Equal(t, "replace_all", bulkActions[0].Action)
		assert.Equal(t, "name", bulkActions[0].Field)
		assert.Equal(t, "wrong_value", bulkActions[0].OldValue)
		assert.Equal(t, "correct_value", bulkActions[0].NewValue)
		assert.Len(t, bulkActions[0].AffectedRows, 2)
	})

	t.Run("ignores single occurrences", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{}
		service := NewImportService[testRow](config, nil)

		errors := []importModels.ImportError[testRow]{
			{
				RowNumber: 2,
				Data:      testRow{Name: "test1"},
				Errors: []importModels.ValidationError{
					{
						Field:       "name",
						Message:     "Invalid",
						ActualValue: "unique_wrong_value",
						AutoFix: &importModels.AutoFix{
							Action:      "replace",
							Replacement: "correct_value",
							Description: "Fix the value",
						},
					},
				},
			},
		}

		// ACT
		bulkActions := service.generateBulkActions(errors)

		// ASSERT
		assert.Empty(t, bulkActions)
	})

	t.Run("ignores errors without AutoFix", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{}
		service := NewImportService[testRow](config, nil)

		errors := []importModels.ImportError[testRow]{
			{
				RowNumber: 2,
				Data:      testRow{Name: "test1"},
				Errors: []importModels.ValidationError{
					{
						Field:       "name",
						Message:     "Invalid",
						ActualValue: "wrong_value",
						AutoFix:     nil, // No AutoFix
					},
				},
			},
			{
				RowNumber: 3,
				Data:      testRow{Name: "test2"},
				Errors: []importModels.ValidationError{
					{
						Field:       "name",
						Message:     "Invalid",
						ActualValue: "wrong_value",
						AutoFix:     nil, // No AutoFix
					},
				},
			},
		}

		// ACT
		bulkActions := service.generateBulkActions(errors)

		// ASSERT
		assert.Empty(t, bulkActions)
	})
}

// ============================================================================
// Dry Run Duplicate Check Error Tests
// ============================================================================

func TestImportService_DryRunDuplicateCheckError(t *testing.T) {
	ctx := context.Background()

	t.Run("records error in dry run mode when duplicate check fails", func(t *testing.T) {
		// ARRANGE
		config := &mockImportConfig{
			findExistingErr: errors.New("db connection failed"),
		}
		service := NewImportService[testRow](config, nil)
		request := importModels.ImportRequest[testRow]{
			Rows:   []testRow{{Name: "test", Value: "value"}},
			DryRun: true,
		}

		// ACT
		result, err := service.Import(ctx, request)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.ErrorCount)
		assert.Equal(t, "duplicate_check_failed", result.Errors[0].Errors[0].Code)
	})
}
