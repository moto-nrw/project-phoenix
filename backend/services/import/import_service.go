package importpkg

import (
	"context"
	"fmt"
	"time"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/uptrace/bun"
)

// ImportService handles generic import logic for any entity type
type ImportService[T any] struct {
	config    importModels.ImportConfig[T]
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

// Import executes the import operation
func (s *ImportService[T]) Import(ctx context.Context, request importModels.ImportRequest[T]) (*importModels.ImportResult[T], error) {
	startTime := time.Now()

	result := &importModels.ImportResult[T]{
		StartedAt: startTime,
		TotalRows: len(request.Rows),
		DryRun:    request.DryRun,
	}

	// Pre-load reference data (groups, rooms, etc.)
	if err := s.config.PreloadReferenceData(ctx); err != nil {
		return nil, fmt.Errorf("preload reference data: %w", err)
	}

	// Process rows (validate and create/update)
	for i := range request.Rows {
		row := &request.Rows[i] // Use pointer to avoid copy
		rowNum := i + 2         // CSV row number (skip header, 1-indexed)

		// Validate the row
		validationErrors := s.config.Validate(ctx, row)

		// Separate errors by severity
		blockingErrors := []importModels.ValidationError{}
		warnings := []importModels.ValidationError{}

		for _, err := range validationErrors {
			switch err.Severity {
			case importModels.ErrorSeverityError:
				blockingErrors = append(blockingErrors, err)
			case importModels.ErrorSeverityWarning:
				warnings = append(warnings, err)
			case importModels.ErrorSeverityInfo:
				// Info-level errors are tracked but not counted
			}
		}

		// Count warnings
		result.WarningCount += len(warnings)

		// Record warnings even if no blocking errors (for display in preview)
		if len(blockingErrors) == 0 && len(warnings) > 0 {
			result.Errors = append(result.Errors, importModels.ImportError[T]{
				RowNumber: rowNum,
				Data:      *row,
				Errors:    warnings,
				Timestamp: time.Now(),
			})
			// Note: Don't increment ErrorCount since these are just warnings
			// Don't continue - let the row proceed with creation
		}

		// If there are blocking errors, record them and skip
		if len(blockingErrors) > 0 {
			result.Errors = append(result.Errors, importModels.ImportError[T]{
				RowNumber: rowNum,
				Data:      *row,
				Errors:    append(blockingErrors, warnings...), // Include warnings
				Timestamp: time.Now(),
			})
			result.ErrorCount++

			if request.StopOnError {
				break // Stop processing on first error
			}
			continue // Skip this row but continue processing
		}

		// If dry run, skip actual creation
		if request.DryRun {
			// Check if exists for preview
			existingID, err := s.config.FindExisting(ctx, *row)
			if err != nil {
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
				continue
			}

			if existingID != nil {
				result.UpdatedCount++ // Would be updated
			} else {
				result.CreatedCount++ // Would be created
			}
			continue
		}

		// Actual import: Create or Update
		existingID, err := s.config.FindExisting(ctx, *row)
		if err != nil {
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
			continue
		}

		// Determine action based on mode
		var action string
		if existingID != nil {
			// Entity exists
			switch request.Mode {
			case importModels.ImportModeCreate:
				// Error: trying to create but already exists
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
				continue
			case importModels.ImportModeUpdate, importModels.ImportModeUpsert:
				action = "update"
			}
		} else {
			// Entity doesn't exist
			switch request.Mode {
			case importModels.ImportModeUpdate:
				// Error: trying to update but doesn't exist
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
				continue
			case importModels.ImportModeCreate, importModels.ImportModeUpsert:
				action = "create"
			}
		}

		// Perform create or update
		if action == "create" {
			_, err := s.config.Create(ctx, *row)
			if err != nil {
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

				if request.StopOnError {
					break
				}
				continue
			}
			result.CreatedCount++
		} else if action == "update" {
			err := s.config.Update(ctx, *existingID, *row)
			if err != nil {
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

				if request.StopOnError {
					break
				}
				continue
			}
			result.UpdatedCount++
		}
	}

	result.CompletedAt = time.Now()

	// Generate bulk actions (for user-friendly corrections)
	result.BulkActions = s.generateBulkActions(result.Errors)

	return result, nil
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
