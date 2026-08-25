package importpkg

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// importerUserIDKey is a context key for the importing user's ID
type importerUserIDKey struct{}
type importModeKey struct{}

// ContextWithImporterID stores the importer's user ID in the context
func ContextWithImporterID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, importerUserIDKey{}, userID)
}

// ImporterIDFromContext retrieves the importer's user ID from the context
func ImporterIDFromContext(ctx context.Context) int64 {
	if id, ok := ctx.Value(importerUserIDKey{}).(int64); ok {
		return id
	}
	return 0
}

func importModeFromContext(ctx context.Context) importModels.ImportMode {
	mode, _ := ctx.Value(importModeKey{}).(importModels.ImportMode)
	return mode
}

// ImportService handles generic import logic for any entity type
type ImportService[T any] struct {
	config    importModels.ImportConfig[T]
	batchSize int
	auditRepo audit.DataImportRepository
	// config keeps request-specific reference data after PreloadReferenceData.
	// Serialize processing while that mutable configuration is shared.
	importMu sync.Mutex
}

type requestScopedConfig[T any] interface {
	NewRequestScoped() importModels.ImportConfig[T]
}

type importConfigLocker interface{ ImportLock() *sync.Mutex }

// NewImportService creates a new import service
func NewImportService[T any](config importModels.ImportConfig[T]) *ImportService[T] {
	return &ImportService[T]{
		config:    config,
		batchSize: 100, // Default batch size
	}
}

// SetAuditRepository wires the GDPR import-audit repository (issue #584:
// audit writes moved out of api/import). Optional at construction time, but
// production wiring must set it — RecordAuditInTransaction fails when unset.
func (s *ImportService[T]) SetAuditRepository(repo audit.DataImportRepository) {
	s.auditRepo = repo
}

// RecordAuditInTransaction writes a GDPR import audit record using the caller's
// tenant-scoped transaction, so the import is only acknowledged once its
// required audit record has been persisted. The write MUST run inside a tenant
// transaction (TenantTxMiddleware or tenant.WithTenantTx): a detached context
// leaves the connection on the NOINHERIT phoenix_auth role, which has no
// grants on audit.data_imports, and the INSERT fails with 42501 (#2141).
func (s *ImportService[T]) RecordAuditInTransaction(ctx context.Context, entityType, filename string, result *importModels.ImportResult[T], userID int64, dryRun bool, tenantID int64) error {
	if s.auditRepo == nil {
		return fmt.Errorf("import audit repository not wired")
	}

	auditRecord := &audit.DataImport{
		EntityType:   entityType,
		Filename:     filename,
		TotalRows:    result.TotalRows,
		CreatedCount: result.CreatedCount,
		UpdatedCount: result.UpdatedCount,
		SkippedCount: 0, // Not tracked separately
		ErrorCount:   result.ErrorCount,
		WarningCount: result.WarningCount,
		DryRun:       dryRun,
		ImportedBy:   userID,
		StartedAt:    result.StartedAt,
		CompletedAt:  &result.CompletedAt,
		Metadata:     audit.JSONBMap{},
	}
	auditRecord.SetTenantID(tenantID)
	if err := s.auditRepo.Create(ctx, auditRecord); err != nil {
		return fmt.Errorf("create import audit record: %w", err)
	}
	return nil
}

// Import executes the import operation
func (s *ImportService[T]) Import(ctx context.Context, request importModels.ImportRequest[T]) (*importModels.ImportResult[T], error) {
	if scoped, ok := s.config.(requestScopedConfig[T]); ok {
		clone := *s
		clone.config = scoped.NewRequestScoped()
		if locker, ok := clone.config.(importConfigLocker); ok {
			locker.ImportLock().Lock()
			defer locker.ImportLock().Unlock()
		}
		return clone.importWithConfig(ctx, request)
	}
	s.importMu.Lock()
	defer s.importMu.Unlock()
	return s.importWithConfig(ctx, request)
}

func (s *ImportService[T]) importWithConfig(ctx context.Context, request importModels.ImportRequest[T]) (*importModels.ImportResult[T], error) {
	result := &importModels.ImportResult[T]{
		StartedAt: time.Now(),
		TotalRows: len(request.Rows),
		DryRun:    request.DryRun,
	}

	// Store importer's user ID in context for entity creation (e.g. pickup schedules)
	ctx = ContextWithImporterID(ctx, request.UserID)
	ctx = context.WithValue(ctx, importModeKey{}, request.Mode)

	if err := s.config.PreloadReferenceData(ctx); err != nil {
		return nil, fmt.Errorf("preload reference data: %w", err)
	}

	batchErrors := s.validateBatch(ctx, request.Rows)

	// Process all rows (may terminate early if StopOnError is set)
	_ = s.processAllRows(ctx, request, result, batchErrors)

	result.CompletedAt = time.Now()
	result.BulkActions = s.generateBulkActions(result.Errors)

	return result, nil
}

func (s *ImportService[T]) validateBatch(ctx context.Context, rows []T) map[int][]importModels.ValidationError {
	validator, ok := s.config.(importModels.BatchValidator[T])
	if !ok {
		return nil
	}
	return validator.ValidateBatch(ctx, rows)
}

// processAllRows processes all rows in the import request
func (s *ImportService[T]) processAllRows(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T], batchErrors map[int][]importModels.ValidationError) bool {
	order := make([]int, len(request.Rows))
	for i := range request.Rows {
		order[i] = i
	}
	if !request.DryRun {
		if orderer, ok := s.config.(importModels.ProcessingOrderer[T]); ok {
			if configuredOrder := orderer.ProcessingOrder(request.Rows); validProcessingOrder(configuredOrder, len(request.Rows)) {
				order = configuredOrder
			}
		}
	}

	for _, i := range order {
		row := &request.Rows[i]
		rowNum := i + 2

		if s.processImportRow(ctx, request, result, row, rowNum, batchErrors[i]) {
			return true
		}
	}
	return false
}

// validProcessingOrder verifies an optional config's order before using it so
// a malformed implementation cannot skip a row or panic the generic importer.
func validProcessingOrder(order []int, rowCount int) bool {
	if len(order) != rowCount {
		return false
	}
	seen := make([]bool, rowCount)
	for _, index := range order {
		if index < 0 || index >= rowCount || seen[index] {
			return false
		}
		seen[index] = true
	}
	return true
}

// processImportRow processes a single row
func (s *ImportService[T]) processImportRow(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int, batchErrors []importModels.ValidationError) bool {
	validationErrors := s.config.Validate(ctx, row)
	validationErrors = append(validationErrors, batchErrors...)
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
		return s.processDryRunRow(ctx, request, result, row, rowNum)
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

// appendRowErrors adds messages to the result entry of a row, creating the
// entry on first use. One entry per row: the preview renders entries as
// cards keyed by row number, and a row that collects a warning during
// validation and a duplicate note afterwards must not appear twice.
func appendRowErrors[T any](result *importModels.ImportResult[T], rowNum int, row *T, errs []importModels.ValidationError) {
	for i := range result.Errors {
		if result.Errors[i].RowNumber == rowNum {
			result.Errors[i].Errors = append(result.Errors[i].Errors, errs...)
			result.Errors[i].Timestamp = time.Now()
			return
		}
	}
	result.Errors = append(result.Errors, importModels.ImportError[T]{
		RowNumber: rowNum,
		Data:      *row,
		Errors:    errs,
		Timestamp: time.Now(),
	})
}

// recordWarnings records warnings in the result
func recordWarnings[T any](result *importModels.ImportResult[T], rowNum int, row *T, warnings []importModels.ValidationError) {
	appendRowErrors(result, rowNum, row, warnings)
}

// recordBlockingErrors records blocking errors in the result
func recordBlockingErrors[T any](result *importModels.ImportResult[T], rowNum int, row *T, blockingErrors, warnings []importModels.ValidationError) {
	appendRowErrors(result, rowNum, row, append(blockingErrors, warnings...))
	result.ErrorCount++
}

// processDryRunRow processes a row in dry run mode
func (s *ImportService[T]) processDryRunRow(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int) bool {
	existingID, err := s.config.FindExisting(ctx, *row)
	if err != nil {
		recordDuplicateCheckError(result, rowNum, row, err)
		return false
	}

	if existingID != nil {
		if request.Mode == importModels.ImportModeCreate {
			recordAlreadyExistsError(s, result, rowNum, row)
			return false
		}
		recordWillUpdateInfo(s, result, rowNum, row)
		result.UpdatedCount++
	} else if request.Mode == importModels.ImportModeUpdate {
		recordNotFoundError(s, result, rowNum, row)
	} else {
		result.CreatedCount++
	}

	return false
}

// recordWillUpdateInfo marks a preview row that resolves to an existing record
// in update/upsert mode, so the UI can show which rows will be changed rather
// than created.
func recordWillUpdateInfo[T any](s *ImportService[T], result *importModels.ImportResult[T], rowNum int, row *T) {
	appendRowErrors(result, rowNum, row, []importModels.ValidationError{{
		Field:    "update",
		Message:  "Schon vorhanden: Die Zeile aktualisiert diesen Datensatz. Leere Zellen ändern nichts.",
		Code:     "will_update",
		Severity: importModels.ErrorSeverityInfo,
	}})
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
	appendRowErrors(result, rowNum, row, []importModels.ValidationError{{
		Field:    "duplicate_check",
		Message:  fmt.Sprintf("Fehler bei Duplikatprüfung: %s", err.Error()),
		Code:     "duplicate_check_failed",
		Severity: importModels.ErrorSeverityError,
	}})
	result.ErrorCount++
}

// recordAlreadyExistsError records an error when entity already exists
func recordAlreadyExistsError[T any](s *ImportService[T], result *importModels.ImportResult[T], rowNum int, row *T) {
	appendRowErrors(result, rowNum, row, []importModels.ValidationError{{
		Field:    "duplicate",
		Message:  fmt.Sprintf("%s existiert bereits", s.config.EntityName()),
		Code:     "already_exists",
		Severity: importModels.ErrorSeverityError,
	}})
	result.ErrorCount++
}

// recordNotFoundError records an error when entity is not found
func recordNotFoundError[T any](s *ImportService[T], result *importModels.ImportResult[T], rowNum int, row *T) {
	appendRowErrors(result, rowNum, row, []importModels.ValidationError{{
		Field:    "not_found",
		Message:  fmt.Sprintf("%s nicht gefunden", s.config.EntityName()),
		Code:     "not_found",
		Severity: importModels.ErrorSeverityError,
	}})
	result.ErrorCount++
}

// recordCreationError records a creation error
func recordCreationError[T any](result *importModels.ImportResult[T], rowNum int, row *T, err error) {
	appendRowErrors(result, rowNum, row, []importModels.ValidationError{{
		Field:    "creation",
		Message:  fmt.Sprintf("Fehler beim Erstellen: %s", err.Error()),
		Code:     "creation_failed",
		Severity: importModels.ErrorSeverityError,
	}})
	result.ErrorCount++
}

// recordUpdateError records an update error
func recordUpdateError[T any](result *importModels.ImportResult[T], rowNum int, row *T, err error) {
	appendRowErrors(result, rowNum, row, []importModels.ValidationError{{
		Field:    "update",
		Message:  fmt.Sprintf("Fehler beim Aktualisieren: %s", err.Error()),
		Code:     "update_failed",
		Severity: importModels.ErrorSeverityError,
	}})
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
