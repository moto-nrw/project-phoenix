package importpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/services/import/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// BatchAudit identifies the upload for its GDPR records. Options binds
// request-scoped settings not contained in Rows (for example an opening
// balance's cutoff and note). It is hashed, never used as a metric label.
type BatchAudit struct {
	EntityType string
	Filename   string
	AccountID  int64
	Options    string
}

type batchRowError struct {
	RowNumber int
	Errors    []importModels.ValidationError
	Timestamp time.Time
}

// batchReceipt retains outcomes, not copies of the uploaded rows. Row data in
// a replay response comes from the identical, hash-bound request.
type batchReceipt struct {
	FirstRow    int
	OrderHash   string
	Created     int
	Updated     int
	Rejected    int
	Warnings    int
	StartedAt   time.Time
	CompletedAt time.Time
	Errors      []batchRowError
}

// inputSnapshotter preserves parser-only values that the API's JSON shape
// intentionally omits. The snapshot supplies both detached rows and their
// full semantic identity for durable replay.
type inputSnapshotter[T any] interface {
	SnapshotImportRows([]T) ([]T, json.RawMessage, error)
}

func snapshotImportRows[T any](rows []T) ([]T, json.RawMessage, error) {
	encoded, err := json.Marshal(rows)
	if err != nil {
		return nil, nil, err
	}
	var detached []T
	if err := json.Unmarshal(encoded, &detached); err != nil {
		return nil, nil, err
	}
	return detached, encoded, nil
}

// ImportBatches validates the whole upload before writes, then owns bounded
// tenant transactions. A write or audit failure rolls back the current batch;
// retrying the identical request resumes after the last durable receipt.
// Callers must not wrap this entry point in another transaction.
func (s *ImportService[T]) ImportBatches(ctx context.Context, request importModels.ImportRequest[T], audit BatchAudit) (*importModels.ImportResult[T], error) {
	if _, active := tenant.TransactionFromContext(ctx); active {
		return nil, fmt.Errorf("batched import must own its transactions")
	}
	if tenant.FromContext(ctx) <= 0 || audit.AccountID <= 0 || audit.EntityType == "" {
		return nil, fmt.Errorf("batched import requires tenant, account and entity")
	}
	if s.batchSize <= 0 {
		return nil, fmt.Errorf("import batch size must be positive")
	}
	if scoped, ok := s.config.(requestScopedConfig[T]); ok {
		clone := &ImportService[T]{config: scoped.NewRequestScoped(), batchSize: s.batchSize, audit: s.audit, observe: s.observe, fingerprint: s.fingerprint, checkpoints: s.checkpoints}
		if locker, ok := clone.config.(importConfigLocker); ok {
			locker.ImportLock().Lock()
			defer locker.ImportLock().Unlock()
		}
		return clone.importBatches(ctx, request, audit)
	}
	s.importMu.Lock()
	defer s.importMu.Unlock()
	return s.importBatches(ctx, request, audit)
}

func (s *ImportService[T]) importBatches(ctx context.Context, request importModels.ImportRequest[T], audit BatchAudit) (*importModels.ImportResult[T], error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if request.DryRun {
		var result *importModels.ImportResult[T]
		err := tenant.WithinCurrentTenant(ctx, func(ctx context.Context) error {
			var err error
			result, err = s.importWithConfig(ctx, request)
			if err != nil {
				return err
			}
			return s.RecordAuditInTransaction(ctx, audit.EntityType, audit.Filename, result, audit.AccountID, true, tenant.FromContext(ctx))
		})
		return result, err
	}
	reader := s.checkpoints
	if reader == nil {
		return nil, fmt.Errorf("import checkpoint reader not wired")
	}
	result := &importModels.ImportResult[T]{StartedAt: time.Now(), TotalRows: len(request.Rows)}
	observation := ImportObservation{Entity: s.config.EntityName(), Rows: len(request.Rows), CheckpointLag: len(request.Rows)}
	started := time.Now()
	ctx = ports.WithCommandObserver(ctx, func(command ports.CommandObservation) {
		observation.Commands = append(observation.Commands, command)
	})
	defer func() {
		observation.Duration = time.Since(started)
		observation.Accepted = result.CreatedCount + result.UpdatedCount
		observation.Rejected = result.ErrorCount
		if s.observe != nil {
			s.observe(observation)
		}
	}()
	ctx = tenant.WithAdditionalUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) {
		switch event.Kind {
		case tenant.UnitOfWorkPoolWait:
			observation.PoolWait += event.Duration
		case tenant.UnitOfWorkLockWait:
			observation.LockWait += event.Duration
		case tenant.UnitOfWorkTransaction:
			observation.BatchesRetried += event.Retries
		}
	})
	if s.fingerprint == nil {
		return nil, fmt.Errorf("import fingerprint capability not wired")
	}
	// Validation normalizes nested row fields. Keep the caller's upload intact
	// so retrying the same request value also retains its checkpoint identity.
	snapshot := snapshotImportRows[T]
	if config, ok := s.config.(inputSnapshotter[T]); ok {
		snapshot = config.SnapshotImportRows
	}
	rows, identity, err := snapshot(request.Rows)
	if err != nil {
		return nil, err
	}
	key, err := importBatchKey(request, identity, audit, s.fingerprint)
	if err != nil {
		return nil, err
	}
	request.Rows = rows
	ctx = ContextWithImporterID(ctx, request.UserID)
	ctx = context.WithValue(ctx, importModeKey{}, request.Mode)
	validated := &importModels.ImportResult[T]{TotalRows: len(request.Rows)}
	accepted := make([]bool, len(request.Rows))
	order := make([]int, len(request.Rows))
	for i := range order {
		order[i] = i
	}
	var receipts []auditModels.ImportCheckpoint
	err = tenant.WithinCurrentTenant(ctx, func(ctx context.Context) error {
		if authorizer, ok := s.config.(importModeAuthorizer); ok {
			if err := authorizer.AuthorizeImportMode(ctx, request.Mode); err != nil {
				return err
			}
		}
		var err error
		receipts, err = reader.ListImportCheckpoints(ctx, key)
		if err != nil {
			return err
		}
		if err := s.config.PreloadReferenceData(ctx); err != nil {
			return err
		}
		batchErrors := s.validateBatch(ctx, request.Rows)
		for i := range request.Rows {
			accepted[i], _ = s.validateRow(ctx, request, validated, &request.Rows[i], i+2, batchErrors[i])
		}
		if orderer, ok := s.config.(importModels.ProcessingOrderer[T]); ok {
			if candidate := orderer.ProcessingOrder(request.Rows); validProcessingOrder(candidate, len(order)) {
				order = candidate
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	orderBytes, err := json.Marshal(order)
	if err != nil {
		return nil, err
	}
	orderHash := s.fingerprint(orderBytes)
	position, err := restoreImportReceipts(result, request.Rows, receipts, orderHash)
	if err != nil {
		return nil, err
	}
	observation.CheckpointLag = len(order) - position
	if request.StopOnError && validated.ErrorCount > 0 && position == 0 {
		validated.StartedAt, validated.CompletedAt = result.StartedAt, time.Now()
		result = validated
		err := tenant.WithinCurrentTenant(ctx, func(ctx context.Context) error {
			return s.RecordAuditInTransaction(ctx, audit.EntityType, audit.Filename, result, audit.AccountID, false, tenant.FromContext(ctx))
		})
		return result, err
	}
	for position < len(order) {
		var committed *importModels.ImportResult[T]
		var failed *importModels.ImportResult[T]
		var appended *importModels.ImportResult[T]
		next := position
		err = tenant.WithinTenantRetry(ctx, tenantID, func(ctx context.Context) (attemptErr error) {
			appended = nil
			defer func() {
				var state interface{ Field(byte) string }
				if errors.As(attemptErr, &state) && state.Field('C') == "40P01" {
					observation.Deadlocks++
				}
			}()
			// Serializes identical uploads across processes, not just goroutines.
			if err := tenant.AcquireLock(ctx, "data-import:"+key, false); err != nil {
				return err
			}
			current, err := reader.ListImportCheckpoints(ctx, key)
			if err != nil {
				return err
			}
			// Another request may have committed while this request waited.
			catchup := &importModels.ImportResult[T]{TotalRows: len(request.Rows), StartedAt: result.StartedAt}
			cursor, err := restoreImportReceipts(catchup, request.Rows, current, orderHash)
			if err != nil {
				return err
			}
			if cursor != position {
				if cursor < position {
					return fmt.Errorf("import checkpoint moved backwards")
				}
				committed, next = catchup, cursor
				return nil
			}
			// Refresh mutable match caches for each attempt. Failed transactions
			// must never leave IDs in memory that a retry treats as persisted.
			if err := s.config.PreloadReferenceData(ctx); err != nil {
				return err
			}
			next = min(position+s.batchSize, len(order))
			batch := importValidationSlice(validated, request.Rows, order[position:next])
			batch.StartedAt = time.Now()
			failed = batch
			for _, index := range order[position:next] {
				if accepted[index] {
					if err := s.applyBatchRow(ctx, request, batch, &request.Rows[index], index+2); err != nil {
						return err
					}
				}
			}
			batch.CompletedAt = time.Now()
			payload, err := encodeBatchReceipt(batch, position, orderHash)
			if err != nil {
				return err
			}
			record := &auditModels.DataImport{
				EntityType: audit.EntityType, Filename: audit.Filename, TotalRows: batch.TotalRows,
				CreatedCount: batch.CreatedCount, UpdatedCount: batch.UpdatedCount,
				ErrorCount: batch.ErrorCount, WarningCount: batch.WarningCount,
				ImportedBy: audit.AccountID, StartedAt: batch.StartedAt, CompletedAt: &batch.CompletedAt,
			}
			if err := record.AttachImportCheckpoint(auditModels.ImportCheckpoint{ImportKey: key, LastRow: next, TotalRows: len(order), Payload: payload}); err != nil {
				return err
			}
			if err := s.audit.Append(ctx, record); err != nil {
				return err
			}
			mergeImportBatch(catchup, batch)
			committed = catchup
			appended = batch
			return nil
		})
		if err != nil {
			if failed != nil {
				// Counters for writes rolled back with this batch must not leak
				// into the acknowledged result. Keep its row errors for callers.
				failed.CreatedCount, failed.UpdatedCount = 0, 0
				mergeImportBatch(result, failed)
			}
			result.CompletedAt = time.Now()
			return result, err
		}
		result, position = committed, next
		if appended != nil {
			observation.BatchesCommitted++
			observation.Created += appended.CreatedCount
			observation.Updated += appended.UpdatedCount
		}
		observation.CheckpointLag = len(order) - position
	}
	result.CompletedAt = time.Now()
	result.BulkActions = s.generateBulkActions(result.Errors)
	return result, nil
}

func importBatchKey[T any](request importModels.ImportRequest[T], rows json.RawMessage, audit BatchAudit, fingerprint func([]byte) string) (string, error) {
	// Hash before validation resolves IDs or normalizes input. Equivalent
	// re-uploads carry the same parsed input even after prior batches commit.
	request.Rows = nil
	data, err := json.Marshal(struct {
		Version int
		Request importModels.ImportRequest[T]
		Rows    json.RawMessage
		Audit   BatchAudit
	}{1, request, rows, audit})
	if err != nil {
		return "", fmt.Errorf("hash import request: %w", err)
	}
	return fingerprint(data), nil
}

func (s *ImportService[T]) applyBatchRow(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int) error {
	id, err := s.config.FindExisting(ctx, *row)
	if err != nil {
		recordDuplicateCheckError(result, rowNum, row, err)
		return err
	}
	action, skip := s.determineImportAction(request, result, row, rowNum, id)
	if skip {
		return nil
	}
	if action == "create" {
		if _, err := s.config.Create(ctx, *row); err != nil {
			recordCreationError(result, rowNum, row, err)
			return err
		}
		result.CreatedCount++
		return nil
	}
	if err := s.config.Update(ctx, *id, *row); err != nil {
		recordUpdateError(result, rowNum, row, err)
		return err
	}
	result.UpdatedCount++
	return nil
}

func importValidationSlice[T any](validated *importModels.ImportResult[T], rows []T, indices []int) *importModels.ImportResult[T] {
	result := &importModels.ImportResult[T]{TotalRows: len(indices)}
	wanted := make(map[int]bool, len(indices))
	for _, index := range indices {
		wanted[index+2] = true
	}
	for _, row := range validated.Errors {
		if !wanted[row.RowNumber] {
			continue
		}
		blocking, warnings := categorizeValidationErrors(row.Errors)
		if len(blocking) > 0 {
			result.ErrorCount++
		}
		result.WarningCount += len(warnings)
		result.Errors = append(result.Errors, importModels.ImportError[T]{RowNumber: row.RowNumber, Data: rows[row.RowNumber-2], Errors: append([]importModels.ValidationError(nil), row.Errors...), Timestamp: row.Timestamp})
	}
	return result
}

func encodeBatchReceipt[T any](result *importModels.ImportResult[T], first int, orderHash string) ([]byte, error) {
	receipt := batchReceipt{FirstRow: first, OrderHash: orderHash, Created: result.CreatedCount, Updated: result.UpdatedCount, Rejected: result.ErrorCount, Warnings: result.WarningCount, StartedAt: result.StartedAt, CompletedAt: result.CompletedAt}
	for _, row := range result.Errors {
		receipt.Errors = append(receipt.Errors, batchRowError{RowNumber: row.RowNumber, Errors: row.Errors, Timestamp: row.Timestamp})
	}
	return json.Marshal(receipt)
}

func restoreImportReceipts[T any](result *importModels.ImportResult[T], rows []T, checkpoints []auditModels.ImportCheckpoint, orderHash string) (int, error) {
	position := 0
	for _, checkpoint := range checkpoints {
		var receipt batchReceipt
		if err := json.Unmarshal(checkpoint.Payload, &receipt); err != nil {
			return 0, fmt.Errorf("decode import batch outcomes: %w", err)
		}
		if checkpoint.TotalRows != len(rows) || checkpoint.LastRow > len(rows) || checkpoint.LastRow <= position || receipt.FirstRow != position || receipt.OrderHash != orderHash {
			return 0, fmt.Errorf("import checkpoint does not match the processing order")
		}
		batch := &importModels.ImportResult[T]{CreatedCount: receipt.Created, UpdatedCount: receipt.Updated, ErrorCount: receipt.Rejected, WarningCount: receipt.Warnings, StartedAt: receipt.StartedAt, CompletedAt: receipt.CompletedAt}
		for _, row := range receipt.Errors {
			if row.RowNumber < 2 || row.RowNumber-2 >= len(rows) {
				return 0, fmt.Errorf("import checkpoint has an invalid error row")
			}
			batch.Errors = append(batch.Errors, importModels.ImportError[T]{RowNumber: row.RowNumber, Data: rows[row.RowNumber-2], Errors: row.Errors, Timestamp: row.Timestamp})
		}
		mergeImportBatch(result, batch)
		position = checkpoint.LastRow
	}
	return position, nil
}

func mergeImportBatch[T any](result, batch *importModels.ImportResult[T]) {
	result.CreatedCount += batch.CreatedCount
	result.UpdatedCount += batch.UpdatedCount
	result.ErrorCount += batch.ErrorCount
	result.WarningCount += batch.WarningCount
	result.Errors = append(result.Errors, batch.Errors...)
	if !batch.StartedAt.IsZero() && (result.StartedAt.IsZero() || batch.StartedAt.Before(result.StartedAt)) {
		result.StartedAt = batch.StartedAt
	}
	result.CompletedAt = batch.CompletedAt
}
