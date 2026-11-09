package importpkg

import (
	"context"
	"errors"
	"fmt"
	"time"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

// DefaultBatchStatementLimit bounds every statement of a batch attempt
// (decision E13 of 09.11.2026, ADR 0067 addendum of 2026-11-08). pgdriver
// gives up a read after 10 s (its default ReadTimeout, overridden neither in
// database/db.go nor in the production DSN), and PostgreSQL sends nothing
// while a statement waits for a lock. A bundled write that waits for several
// conflicting transactions in one statement adds those waits up in one read;
// the row path waits for each in a statement of its own. 4 s stays well below
// the 10 s, with room for the statement's own work. Past it the attempt is
// cancelled (57014) and the chunk replays row by row without a bound.
const DefaultBatchStatementLimit = 4 * time.Second

// errBatchBoundExceeded ends a chunk's transaction after the bound cancelled
// a batch statement (decision E13′ of 09.11.2026). pgdriver treats 57014 as a
// broken connection and closes it (pgdriver v1.3.0, error.go:55–82), so
// neither ROLLBACK TO SAVEPOINT nor any other statement can follow in that
// transaction. Nothing of it commits; the chunk replays in a new one.
var errBatchBoundExceeded = errors.New("data import: batch attempt exceeded its bound")

// errBatchCommand marks a failure of the batch command itself (CreateChunk)
// and nothing else: not the statements that set or reset its bound, not the
// chunk savepoint, not a row written on the row path. Only such a failure
// replays the chunk; every other failure behaves as the same failure at the
// row path's first write, with the same retry count and no fallback (#5827
// review R1, R2; fault table in the PR).
var errBatchCommand = errors.New("data import: batch command failed")

// errChunkRefused is the refusal RefusesChunk reported.
var errChunkRefused = errors.New("data import: the row path would refuse a row of the chunk")

func batchCommandFailed(err error) bool { return errors.Is(err, errBatchCommand) }

// applyChunk writes one chunk (ADR 0067 addendum, point 2). A config with
// batch commands runs a create chunk of two or more accepted rows as a batch
// attempt inside SAVEPOINT chunk. A failed batch command, including
// 40P01/40001, rolls back to the savepoint and replays the chunk row by row in
// the same transaction attempt, so the replay keeps the whole retry budget; a
// cancelled batch command replays in a new transaction (E13′); a broken
// connection is returned. Every other failure is returned as the row path
// returns it. Any other chunk is the row path from the start.
func (a *batchImportAttempt[T]) applyChunk(ctx context.Context, batch *importModels.ImportResult[T]) (*importModels.ImportResult[T], error) {
	service := a.service()
	writer, ok := service.config.(ports.ChunkWriter[T])
	if !ok || !writer.ChunkWriterReady() || service.boundStatements == nil || a.rowPath ||
		a.run.request.Mode != importModels.ImportModeCreate || len(a.acceptedRows()) < 2 {
		return batch, a.applyAcceptedRows(ctx, service, batch)
	}
	entered := false
	err := service.transactions.Savepoint(ctx, func(ctx context.Context) error {
		entered = true
		return a.applySegments(ctx, writer, batch)
	})
	if err == nil {
		return batch, nil
	}
	if !entered {
		return batch, err
	}
	if batchCommandFailed(err) && boundExceeded(err) {
		// E13′: pgdriver closes the connection on 57014, so the chunk's
		// transaction is lost; writeBatches replays the chunk row by row in
		// a new one.
		a.rowPath = true
		a.run.observation.BatchFallbacks++
		return batch, fmt.Errorf("%w: %w", errBatchBoundExceeded, err)
	}
	if errors.Is(err, importModels.ErrSavepointControl) || (service.connectionLost != nil && service.connectionLost(err)) {
		// The savepoint itself failed or the connection broke: nothing can
		// replay in this transaction, and the failure is neither replayed nor
		// retried (#5827, lesson of Q6).
		return batch, err
	}
	if !batchCommandFailed(err) {
		return batch, err
	}
	a.rowPath = true
	a.run.observation.BatchFallbacks++
	var state interface{ Field(byte) string }
	if errors.As(err, &state) && state.Field('C') == "40P01" {
		a.run.observation.Deadlocks++
	}
	// The chunk result is rebuilt from the validation results, as a new
	// transaction attempt does; resetting lengths and counters would keep row
	// errors appended in place to a validation entry (ADR 0067 addendum,
	// Invariante Chunk-Rollback).
	replay := importValidationSlice(a.run.validated, a.run.request.Rows, a.run.order[a.run.position:a.next])
	replay.StartedAt = batch.StartedAt
	a.failed = replay
	// The replay runs over a newly loaded config, so nothing the first preload
	// or the batch attempt left behind reaches it.
	fresh := service.renewed()
	if err := fresh.config.PreloadReferenceData(ctx); err != nil {
		return replay, err
	}
	return replay, a.applyAcceptedRows(ctx, fresh, replay)
}

// applySegments splits the accepted rows of the chunk into segments of
// independent rows and writes them one after the other; each segment is
// resolved against the state its predecessors left.
func (a *batchImportAttempt[T]) applySegments(ctx context.Context, writer ports.ChunkWriter[T], batch *importModels.ImportResult[T]) error {
	accepted := a.acceptedRows()
	for start := 0; start < len(accepted); {
		end := segmentEnd(writer, a.run.request.Rows, accepted, start)
		if err := a.applySegment(ctx, writer, batch, accepted[start:end]); err != nil {
			return err
		}
		start = end
	}
	return nil
}

// acceptedRows lists the chunk's accepted rows in processing order.
func (a *batchImportAttempt[T]) acceptedRows() []int {
	var accepted []int
	for _, index := range a.run.order[a.run.position:a.next] {
		if a.run.accepted[index] {
			accepted = append(accepted, index)
		}
	}
	return accepted
}

// segmentEnd extends a segment until a row repeats a key already in it.
func segmentEnd[T any](writer ports.ChunkWriter[T], rows []T, indices []int, start int) int {
	seen := make(map[string]bool)
	end := start
	for end < len(indices) {
		key := writer.ChunkKey(rows[indices[end]])
		if seen[key] {
			break
		}
		seen[key] = true
		end++
	}
	return end
}

// applySegment resolves the segment in one protected read, decides each
// row's action in processing order exactly as the row path does, and creates
// the remaining rows through one bounded batch command.
func (a *batchImportAttempt[T]) applySegment(ctx context.Context, writer ports.ChunkWriter[T], batch *importModels.ImportResult[T], segment []int) error {
	service, request := a.service(), a.run.request
	if len(segment) < 2 {
		return service.applyBatchRow(ctx, request, batch, &request.Rows[segment[0]], segment[0]+2)
	}
	rows := make([]T, len(segment))
	for i, index := range segment {
		rows[i] = request.Rows[index]
	}
	resolved, err := writer.ResolveChunk(ctx, rows)
	if err != nil {
		resolved = service.config
	}
	creates := make([]T, 0, len(segment))
	for _, index := range segment {
		row := &request.Rows[index]
		id, err := resolved.FindExisting(ctx, *row)
		if err != nil {
			recordDuplicateCheckError(batch, index+2, row, err)
			continue
		}
		action, skip := service.determineImportAction(request, batch, row, index+2, id)
		if skip {
			continue
		}
		if action != "create" {
			// Unreachable: the batch path runs create imports only.
			return fmt.Errorf("data import: chunk executor got action %q in a create import", action)
		}
		creates = append(creates, *row)
	}
	if len(creates) == 0 {
		return nil
	}
	if err := writeSegment(ctx, service, writer, creates); err != nil {
		return err
	}
	batch.CreatedCount += len(creates)
	return nil
}

// writeSegment reads the student guard, then runs the batch command under
// its bound. Only the batch command's own failure and the guard's refusal
// carry errBatchCommand (fault table in the PR).
func writeSegment[T any](ctx context.Context, service *ImportService[T], writer ports.ChunkWriter[T], creates []T) error {
	refused, err := writer.RefusesChunk(ctx, creates)
	if err != nil {
		return err
	}
	if refused {
		// The row path would refuse a row (ADR F5): the batch command is
		// refused, and the replay names the row as the row path does.
		return fmt.Errorf("%w: %w", errBatchCommand, errChunkRefused)
	}
	return service.boundStatements(ctx, service.batchLimit, func(ctx context.Context) error {
		if err := writer.CreateChunk(ctx, creates); err != nil {
			return fmt.Errorf("%w: %w", errBatchCommand, err)
		}
		return nil
	})
}

// boundExceeded reports a batch statement the bound cancelled (57014).
func boundExceeded(err error) bool {
	var state interface{ Field(byte) string }
	return errors.As(err, &state) && state.Field('C') == "57014"
}
