package importpkg

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/audit"
	importModels "github.com/moto-nrw/project-phoenix/internal/core/domain/import"
	auditPort "github.com/moto-nrw/project-phoenix/internal/core/port/audit"
	"github.com/uptrace/bun"
)

// ImportService handles generic import logic for any entity type
type ImportService[T any] struct {
	config    importModels.ImportConfig[T]
	auditRepo auditPort.DataImportRepository
	db        *bun.DB
	batchSize int
}

// NewImportService creates a new import service
func NewImportService[T any](config importModels.ImportConfig[T], db *bun.DB) *ImportService[T] {
	return &ImportService[T]{
		config:    config,
		db:        db,
		batchSize: 100, // Default batch size
	}
}

// SetAuditRepository sets the audit repository for GDPR compliance logging
func (s *ImportService[T]) SetAuditRepository(repo auditPort.DataImportRepository) {
	s.auditRepo = repo
}

// CreateAuditRecord creates a GDPR-compliant audit record for an import operation
func (s *ImportService[T]) CreateAuditRecord(ctx context.Context, record *audit.DataImport) error {
	if s.auditRepo == nil {
		return fmt.Errorf("audit repository not configured")
	}
	return s.auditRepo.Create(ctx, record)
}

// Import executes the import operation
func (s *ImportService[T]) Import(ctx context.Context, request importModels.ImportRequest[T]) (*importModels.ImportResult[T], error) {
	result := &importModels.ImportResult[T]{
		StartedAt: time.Now(),
		TotalRows: len(request.Rows),
		DryRun:    request.DryRun,
	}

	if err := s.config.PreloadReferenceData(ctx); err != nil {
		return nil, fmt.Errorf("preload reference data: %w", err)
	}

	// Process all rows (may terminate early if StopOnError is set)
	_ = s.processAllRows(ctx, request, result)

	result.CompletedAt = time.Now()
	result.BulkActions = s.generateBulkActions(result.Errors)

	return result, nil
}

// processAllRows processes all rows in the import request
func (s *ImportService[T]) processAllRows(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T]) bool {
	for i := range request.Rows {
		row := &request.Rows[i]
		rowNum := i + 2

		if s.processImportRow(ctx, request, result, row, rowNum) {
			return true
		}
	}
	return false
}

// processImportRow processes a single row
func (s *ImportService[T]) processImportRow(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int) bool {
	validationErrors := s.config.Validate(ctx, row)
	blockingErrors, warnings := categorizeValidationErrors(validationErrors)

	result.WarningCount += len(warnings)

	if len(blockingErrors) == 0 && len(warnings) > 0 {
		recordWarnings(result, rowNum, row, warnings)
	}

	if len(blockingErrors) > 0 {
		recordBlockingErrors(result, rowNum, row, blockingErrors, warnings)
		return request.StopOnError
	}

	if request.DryRun {
		return s.processDryRunRow(ctx, result, row, rowNum)
	}

	return s.processActualImportRow(ctx, request, result, row, rowNum)
}

// categorizeValidationErrors separates errors by severity
func categorizeValidationErrors(validationErrors []importModels.ValidationError) ([]importModels.ValidationError, []importModels.ValidationError) {
	blockingErrors := []importModels.ValidationError{}
	warnings := []importModels.ValidationError{}

	for _, err := range validationErrors {
		switch err.Severity {
		case importModels.ErrorSeverityError:
			blockingErrors = append(blockingErrors, err)
		case importModels.ErrorSeverityWarning:
			warnings = append(warnings, err)
		}
	}

	return blockingErrors, warnings
}

// recordWarnings records warnings in the result
func recordWarnings[T any](result *importModels.ImportResult[T], rowNum int, row *T, warnings []importModels.ValidationError) {
	result.Errors = append(result.Errors, importModels.ImportError[T]{
		RowNumber: rowNum,
		Data:      *row,
		Errors:    warnings,
		Timestamp: time.Now(),
	})
}

// recordBlockingErrors records blocking errors in the result
func recordBlockingErrors[T any](result *importModels.ImportResult[T], rowNum int, row *T, blockingErrors, warnings []importModels.ValidationError) {
	result.Errors = append(result.Errors, importModels.ImportError[T]{
		RowNumber: rowNum,
		Data:      *row,
		Errors:    append(blockingErrors, warnings...),
		Timestamp: time.Now(),
	})
	result.ErrorCount++
}

// processDryRunRow processes a row in dry run mode
func (s *ImportService[T]) processDryRunRow(ctx context.Context, result *importModels.ImportResult[T], row *T, rowNum int) bool {
	existingID, err := s.config.FindExisting(ctx, *row)
	if err != nil {
		recordDuplicateCheckError(result, rowNum, row, err)
		return false
	}

	if existingID != nil {
		result.UpdatedCount++
	} else {
		result.CreatedCount++
	}

	return false
}

// processActualImportRow processes a row for actual import
func (s *ImportService[T]) processActualImportRow(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int) bool {
	existingID, err := s.config.FindExisting(ctx, *row)
	if err != nil {
		recordDuplicateCheckError(result, rowNum, row, err)
		return false
	}

	action, shouldSkip := s.determineImportAction(request, result, row, rowNum, existingID)
	if shouldSkip {
		return request.StopOnError && result.ErrorCount > 0
	}

	return s.performImportAction(ctx, request, result, row, rowNum, action, existingID)
}

// determineImportAction determines whether to create or update based on mode
func (s *ImportService[T]) determineImportAction(request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int, existingID *int64) (string, bool) {
	if existingID != nil {
		return s.handleExistingEntity(request, result, row, rowNum)
	}
	return s.handleNewEntity(request, result, row, rowNum)
}

// handleExistingEntity handles the case when entity already exists
func (s *ImportService[T]) handleExistingEntity(request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int) (string, bool) {
	if request.Mode == importModels.ImportModeCreate {
		recordAlreadyExistsError(s, result, rowNum, row)
		return "", true
	}
	return "update", false
}

// handleNewEntity handles the case when entity doesn't exist
func (s *ImportService[T]) handleNewEntity(request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int) (string, bool) {
	if request.Mode == importModels.ImportModeUpdate {
		recordNotFoundError(s, result, rowNum, row)
		return "", true
	}
	return "create", false
}

// performImportAction performs the create or update operation
func (s *ImportService[T]) performImportAction(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int, action string, existingID *int64) bool {
	if action == "create" {
		return s.performCreateAction(ctx, request, result, row, rowNum)
	}
	return s.performUpdateAction(ctx, request, result, row, rowNum, existingID)
}

// performCreateAction performs the create operation
func (s *ImportService[T]) performCreateAction(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int) bool {
	if _, err := s.config.Create(ctx, *row); err != nil {
		recordCreationError(result, rowNum, row, err)
		return request.StopOnError
	}
	result.CreatedCount++
	return false
}

// performUpdateAction performs the update operation
func (s *ImportService[T]) performUpdateAction(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int, existingID *int64) bool {
	if err := s.config.Update(ctx, *existingID, *row); err != nil {
		recordUpdateError(result, rowNum, row, err)
		return request.StopOnError
	}
	result.UpdatedCount++
	return false
}

// recordDuplicateCheckError records a duplicate check error
func recordDuplicateCheckError[T any](result *importModels.ImportResult[T], rowNum int, row *T, err error) {
	result.Errors = append(result.Errors, importModels.ImportError[T]{
		RowNumber: rowNum,
		Data:      *row,
		Errors: []importModels.ValidationError{{
			Field:    "duplicate_check",
			Message:  fmt.Sprintf("Fehler bei Duplikatprüfung: %s", err.Error()),
			Code:     "duplicate_check_failed",
			Severity: importModels.ErrorSeverityError,
		}},
		Timestamp: time.Now(),
	})
	result.ErrorCount++
}

// recordAlreadyExistsError records an error when entity already exists
func recordAlreadyExistsError[T any](s *ImportService[T], result *importModels.ImportResult[T], rowNum int, row *T) {
	result.Errors = append(result.Errors, importModels.ImportError[T]{
		RowNumber: rowNum,
		Data:      *row,
		Errors: []importModels.ValidationError{{
			Field:    "duplicate",
			Message:  fmt.Sprintf("%s existiert bereits", s.config.EntityName()),
			Code:     "already_exists",
			Severity: importModels.ErrorSeverityError,
		}},
		Timestamp: time.Now(),
	})
	result.ErrorCount++
}

// recordNotFoundError records an error when entity is not found
func recordNotFoundError[T any](s *ImportService[T], result *importModels.ImportResult[T], rowNum int, row *T) {
	result.Errors = append(result.Errors, importModels.ImportError[T]{
		RowNumber: rowNum,
		Data:      *row,
		Errors: []importModels.ValidationError{{
			Field:    "not_found",
			Message:  fmt.Sprintf("%s nicht gefunden", s.config.EntityName()),
			Code:     "not_found",
			Severity: importModels.ErrorSeverityError,
		}},
		Timestamp: time.Now(),
	})
	result.ErrorCount++
}

// recordCreationError records a creation error
func recordCreationError[T any](result *importModels.ImportResult[T], rowNum int, row *T, err error) {
	result.Errors = append(result.Errors, importModels.ImportError[T]{
		RowNumber: rowNum,
		Data:      *row,
		Errors: []importModels.ValidationError{{
			Field:    "creation",
			Message:  fmt.Sprintf("Fehler beim Erstellen: %s", err.Error()),
			Code:     "creation_failed",
			Severity: importModels.ErrorSeverityError,
		}},
		Timestamp: time.Now(),
	})
	result.ErrorCount++
}

// recordUpdateError records an update error
func recordUpdateError[T any](result *importModels.ImportResult[T], rowNum int, row *T, err error) {
	result.Errors = append(result.Errors, importModels.ImportError[T]{
		RowNumber: rowNum,
		Data:      *row,
		Errors: []importModels.ValidationError{{
			Field:    "update",
			Message:  fmt.Sprintf("Fehler beim Aktualisieren: %s", err.Error()),
			Code:     "update_failed",
			Severity: importModels.ErrorSeverityError,
		}},
		Timestamp: time.Now(),
	})
	result.ErrorCount++
}

// generateBulkActions analyzes errors and suggests bulk corrections
func (s *ImportService[T]) generateBulkActions(errors []importModels.ImportError[T]) []importModels.BulkAction {
	// Group errors by field, old value, and suggested fix
	type actionKey struct {
		field       string
		oldValue    string
		newValue    string
		description string
	}

	actionMap := make(map[actionKey][]int) // key → affected row numbers

	for _, importErr := range errors {
		for _, validationErr := range importErr.Errors {
			// Only create bulk actions for errors with AutoFix and ActualValue
			if validationErr.AutoFix != nil && validationErr.AutoFix.Action == "replace" && validationErr.ActualValue != "" {
				key := actionKey{
					field:       validationErr.Field,
					oldValue:    validationErr.ActualValue,
					newValue:    validationErr.AutoFix.Replacement,
					description: validationErr.AutoFix.Description,
				}
				actionMap[key] = append(actionMap[key], importErr.RowNumber)
			}
		}
	}

	// Convert to bulk actions (only if affects 2+ rows)
	bulkActions := []importModels.BulkAction{}
	for key, rows := range actionMap {
		if len(rows) >= 2 {
			bulkActions = append(bulkActions, importModels.BulkAction{
				Title:        fmt.Sprintf("%d Zeilen haben '%s' im Feld '%s'", len(rows), key.oldValue, key.field),
				Description:  key.description,
				Action:       "replace_all",
				AffectedRows: rows,
				Field:        key.field,
				OldValue:     key.oldValue,
				NewValue:     key.newValue,
			})
		}
	}

	return bulkActions
}
