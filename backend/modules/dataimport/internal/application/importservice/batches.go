package importpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/imports"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

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
func (s *ImportService[T]) ImportBatches(ctx context.Context, request importModels.ImportRequest[T], audit importModels.BatchAudit) (*importModels.ImportResult[T], error) {
	if s.transactions == nil {
		return nil, fmt.Errorf("import transaction capability not wired")
	}
	if s.transactions.HasTransaction(ctx) {
		return nil, fmt.Errorf("batched import must own its transactions")
	}
	if s.transactions.TenantID(ctx) <= 0 || audit.AccountID <= 0 || audit.EntityType == "" {
		return nil, fmt.Errorf("batched import requires tenant, account and entity")
	}
	if s.batchSize <= 0 {
		return nil, fmt.Errorf("import batch size must be positive")
	}
	if scoped, ok := s.config.(requestScopedConfig[T]); ok {
		clone := &ImportService[T]{config: scoped.NewRequestScoped(), batchSize: s.batchSize, audit: s.audit, observe: s.observe, fingerprint: s.fingerprint, checkpoints: s.checkpoints, transactions: s.transactions}
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

func (s *ImportService[T]) importBatches(ctx context.Context, request importModels.ImportRequest[T], audit importModels.BatchAudit) (*importModels.ImportResult[T], error) {
	if request.DryRun {
		return s.importBatchesDryRun(ctx, request, audit)
	}
	if s.checkpoints == nil {
		return nil, fmt.Errorf("import checkpoint reader not wired")
	}
	run := &batchImportRun[T]{
		service: s, request: request, audit: audit,
		result:      &importModels.ImportResult[T]{StartedAt: time.Now(), TotalRows: len(request.Rows)},
		observation: importModels.ImportObservation{Entity: s.config.EntityName(), Rows: len(request.Rows), CheckpointLag: len(request.Rows)},
		started:     time.Now(),
	}
	ctx = run.observe(ctx)
	defer run.emitObservation()
	ctx, err := run.prepare(ctx)
	if err != nil {
		return nil, err
	}
	if request.StopOnError && run.validated.ErrorCount > 0 && run.position == 0 {
		return run.stopBeforeWrites(ctx)
	}
	return run.writeBatches(ctx)
}

// batchImportRun holds the receipt-bound progress of one upload, never shared configuration.
type batchImportRun[T any] struct {
	service        *ImportService[T]
	request        importModels.ImportRequest[T]
	audit          importModels.BatchAudit
	result         *importModels.ImportResult[T]
	validated      *importModels.ImportResult[T]
	observation    importModels.ImportObservation
	started        time.Time
	accepted       []bool
	order          []int
	key, orderHash string
	position       int
}

func (s *ImportService[T]) importBatchesDryRun(ctx context.Context, request importModels.ImportRequest[T], audit importModels.BatchAudit) (*importModels.ImportResult[T], error) {
	var result *importModels.ImportResult[T]
	err := s.transactions.WithinCurrent(ctx, func(ctx context.Context) error {
		var err error
		result, err = s.importWithConfig(ctx, request)
		if err != nil {
			return err
		}
		return s.RecordAuditInTransaction(ctx, audit.EntityType, audit.Filename, result, audit.AccountID, true, s.transactions.TenantID(ctx))
	})
	return result, err
}

func (r *batchImportRun[T]) observe(ctx context.Context) context.Context {
	ctx = ports.WithCommandObserver(ctx, func(command importModels.CommandObservation) {
		r.observation.Commands = append(r.observation.Commands, command)
	})
	return r.service.transactions.Observe(ctx, func(event importModels.TransactionObservation) {
		r.observation.PoolWait += event.PoolWait
		r.observation.LockWait += event.LockWait
		r.observation.BatchesRetried += event.Retries
	})
}

func (r *batchImportRun[T]) emitObservation() {
	r.observation.Duration = time.Since(r.started)
	r.observation.Accepted = r.result.CreatedCount + r.result.UpdatedCount
	r.observation.Rejected = r.result.ErrorCount
	if r.service.observe != nil {
		r.service.observe(r.observation)
	}
}

func (r *batchImportRun[T]) prepare(ctx context.Context) (context.Context, error) {
	if r.service.fingerprint == nil {
		return ctx, fmt.Errorf("import fingerprint capability not wired")
	}
	// Detach before validation normalizes parser-only values and nested row fields.
	snapshot := snapshotImportRows[T]
	if config, ok := r.service.config.(inputSnapshotter[T]); ok {
		snapshot = config.SnapshotImportRows
	}
	rows, identity, err := snapshot(r.request.Rows)
	if err != nil {
		return ctx, err
	}
	r.key, err = importBatchKey(r.request, identity, r.audit, r.service.fingerprint)
	if err != nil {
		return ctx, err
	}
	r.request.Rows = rows
	ctx = ContextWithImporterID(ctx, r.request.UserID)
	ctx = context.WithValue(ctx, importModeKey{}, r.request.Mode)
	r.validated = &importModels.ImportResult[T]{TotalRows: len(rows)}
	r.accepted = make([]bool, len(rows))
	r.order = make([]int, len(rows))
	for index := range r.order {
		r.order[index] = index
	}
	var receipts []auditModels.ImportCheckpoint
	err = r.service.transactions.WithinCurrent(ctx, func(ctx context.Context) error {
		var err error
		receipts, err = r.validateUpload(ctx)
		return err
	})
	if err != nil {
		return ctx, err
	}
	return ctx, r.restoreProgress(receipts)
}

func (r *batchImportRun[T]) validateUpload(ctx context.Context) ([]auditModels.ImportCheckpoint, error) {
	if authorizer, ok := r.service.config.(importModeAuthorizer); ok {
		if err := authorizer.AuthorizeImportMode(ctx, r.request.Mode); err != nil {
			return nil, err
		}
	}
	receipts, err := r.service.checkpoints.ListImportCheckpoints(ctx, r.key)
	if err != nil {
		return nil, err
	}
	if err := r.service.config.PreloadReferenceData(ctx); err != nil {
		return nil, err
	}
	batchErrors := r.service.validateBatch(ctx, r.request.Rows)
	for index := range r.request.Rows {
		r.accepted[index], _ = r.service.validateRow(ctx, r.request, r.validated, &r.request.Rows[index], index+2, batchErrors[index])
	}
	if orderer, ok := r.service.config.(ports.ProcessingOrderer[T]); ok {
		if candidate := orderer.ProcessingOrder(r.request.Rows); validProcessingOrder(candidate, len(r.order)) {
			r.order = candidate
		}
	}
	return receipts, nil
}

func (r *batchImportRun[T]) restoreProgress(receipts []auditModels.ImportCheckpoint) error {
	orderBytes, err := json.Marshal(r.order)
	if err != nil {
		return err
	}
	r.orderHash = r.service.fingerprint(orderBytes)
	r.position, err = restoreImportReceipts(r.result, r.request.Rows, receipts, r.orderHash)
	if err != nil {
		return err
	}
	r.observation.CheckpointLag = len(r.order) - r.position
	return nil
}

func (r *batchImportRun[T]) stopBeforeWrites(ctx context.Context) (*importModels.ImportResult[T], error) {
	r.validated.StartedAt, r.validated.CompletedAt = r.result.StartedAt, time.Now()
	r.result = r.validated
	err := r.service.transactions.WithinCurrent(ctx, func(ctx context.Context) error {
		return r.service.RecordAuditInTransaction(ctx, r.audit.EntityType, r.audit.Filename, r.result, r.audit.AccountID, false, r.service.transactions.TenantID(ctx))
	})
	return r.result, err
}

func (r *batchImportRun[T]) writeBatches(ctx context.Context) (*importModels.ImportResult[T], error) {
	for r.position < len(r.order) {
		attempt := &batchImportAttempt[T]{run: r, next: r.position}
		err := r.service.transactions.WithinRetry(ctx, attempt.execute)
		if err != nil {
			if attempt.failed != nil {
				// Rolled-back write counters are not acknowledged; row errors remain visible.
				attempt.failed.CreatedCount, attempt.failed.UpdatedCount = 0, 0
				mergeImportBatch(r.result, attempt.failed)
			}
			r.result.CompletedAt = time.Now()
			return r.result, err
		}
		r.result, r.position = attempt.committed, attempt.next
		if attempt.appended != nil {
			r.observation.BatchesCommitted++
			r.observation.Created += attempt.appended.CreatedCount
			r.observation.Updated += attempt.appended.UpdatedCount
		}
		r.observation.CheckpointLag = len(r.order) - r.position
	}
	r.result.CompletedAt = time.Now()
	r.result.BulkActions = r.service.generateBulkActions(r.result.Errors)
	return r.result, nil
}

// One attempt is reused by the transaction runner's retries, as the original local receipts were.
type batchImportAttempt[T any] struct {
	run                         *batchImportRun[T]
	committed, failed, appended *importModels.ImportResult[T]
	next                        int
}

func (a *batchImportAttempt[T]) execute(ctx context.Context) (attemptErr error) {
	a.appended = nil
	defer func() {
		var state interface{ Field(byte) string }
		if errors.As(attemptErr, &state) && state.Field('C') == "40P01" {
			a.run.observation.Deadlocks++
		}
	}()
	catchup, cursor, err := a.readCheckpoint(ctx)
	if err != nil {
		return err
	}
	if cursor != a.run.position {
		if cursor < a.run.position {
			return fmt.Errorf("import checkpoint moved backwards")
		}
		a.committed, a.next = catchup, cursor
		return nil
	}
	// Refresh mutable match caches after a failed attempt before resolving any row again.
	if err := a.run.service.config.PreloadReferenceData(ctx); err != nil {
		return err
	}
	a.next = min(a.run.position+a.run.service.batchSize, len(a.run.order))
	batch := importValidationSlice(a.run.validated, a.run.request.Rows, a.run.order[a.run.position:a.next])
	batch.StartedAt = time.Now()
	a.failed = batch
	if err := a.applyAcceptedRows(ctx, batch); err != nil {
		return err
	}
	batch.CompletedAt = time.Now()
	if err := a.appendCheckpoint(ctx, batch); err != nil {
		return err
	}
	mergeImportBatch(catchup, batch)
	a.committed, a.appended = catchup, batch
	return nil
}

func (a *batchImportAttempt[T]) readCheckpoint(ctx context.Context) (*importModels.ImportResult[T], int, error) {
	// This lock serializes identical uploads across processes, not only goroutines.
	if err := a.run.service.transactions.AcquireLock(ctx, "data-import:"+a.run.key); err != nil {
		return nil, 0, err
	}
	current, err := a.run.service.checkpoints.ListImportCheckpoints(ctx, a.run.key)
	if err != nil {
		return nil, 0, err
	}
	catchup := &importModels.ImportResult[T]{TotalRows: len(a.run.request.Rows), StartedAt: a.run.result.StartedAt}
	cursor, err := restoreImportReceipts(catchup, a.run.request.Rows, current, a.run.orderHash)
	return catchup, cursor, err
}

func (a *batchImportAttempt[T]) applyAcceptedRows(ctx context.Context, batch *importModels.ImportResult[T]) error {
	for _, index := range a.run.order[a.run.position:a.next] {
		if a.run.accepted[index] {
			if err := a.run.service.applyBatchRow(ctx, a.run.request, batch, &a.run.request.Rows[index], index+2); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *batchImportAttempt[T]) appendCheckpoint(ctx context.Context, batch *importModels.ImportResult[T]) error {
	payload, err := encodeBatchReceipt(batch, a.run.position, a.run.orderHash)
	if err != nil {
		return err
	}
	record := &auditModels.DataImport{
		EntityType: a.run.audit.EntityType, Filename: a.run.audit.Filename, TotalRows: batch.TotalRows,
		CreatedCount: batch.CreatedCount, UpdatedCount: batch.UpdatedCount, ErrorCount: batch.ErrorCount, WarningCount: batch.WarningCount,
		ImportedBy: a.run.audit.AccountID, StartedAt: batch.StartedAt, CompletedAt: &batch.CompletedAt,
	}
	if err := record.AttachImportCheckpoint(auditModels.ImportCheckpoint{ImportKey: a.run.key, LastRow: a.next, TotalRows: len(a.run.order), Payload: payload}); err != nil {
		return err
	}
	return a.run.service.audit.Append(ctx, record)
}

func importBatchKey[T any](request importModels.ImportRequest[T], rows json.RawMessage, audit importModels.BatchAudit, fingerprint func([]byte) string) (string, error) {
	// Hash before validation resolves IDs or normalizes input. Equivalent
	// re-uploads carry the same parsed input even after prior batches commit.
	request.Rows = nil
	data, err := json.Marshal(struct {
		Version int
		Request importModels.ImportRequest[T]
		Rows    json.RawMessage
		Audit   importModels.BatchAudit
	}{1, request, rows, audit})
	if err != nil {
		return "", fmt.Errorf("hash import request: %w", err)
	}
	return fingerprint(data), nil
}

func (s *ImportService[T]) applyBatchRow(ctx context.Context, request importModels.ImportRequest[T], result *importModels.ImportResult[T], row *T, rowNum int) error {
	id, err := s.config.FindExisting(ctx, *row)
	if err != nil {
		// Match errors are per-row, not write failures. Preview and the
		// retained importer skip the row; returning here would roll back the
		// whole unit of work and block resume of an identical upload.
		recordDuplicateCheckError(result, rowNum, row, err)
		return nil
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
