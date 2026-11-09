package importpkg

import (
	"context"
	"errors"
	"fmt"
	"time"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

// DefaultBatchStatementLimit bounds the bundled statements of a batch attempt
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

// errChunkRollbackFailed ends the upload when the chunk savepoint could not
// be rolled back after a failed attempt.
var errChunkRollbackFailed = errors.New("data import: chunk rollback failed")

// errSavepointLost ends the upload when a savepoint statement of an owner
// command failed inside a write: the transaction can no longer be trusted
// (#5552 review of 1bb4bde258, the rule of #5827 review R3).
var errSavepointLost = errors.New("data import: a savepoint of the write failed")

// errChunkRefused is the refusal RefusesChunk reported.
var errChunkRefused = errors.New("data import: the row path would refuse a row of the chunk")

// batchFailure is the failure of the batch command itself (CreateChunk) and
// nothing else: not the statements that set or reset its bound, not the
// chunk savepoint, not a row written on the row path. Only such a failure
// replays the chunk; every other failure behaves as the same failure at the
// row path's first write (#5827 review R1, R2). writeSegment keeps it on the
// attempt where the command fails, and nothing runs in the attempt after it,
// so the executor decides on that field, never by searching an error chain
// (#5552 review of 1bb4bde258).
type batchFailure struct {
	err error
	// cancelled: the bounded statements themselves ended with 57014, so the
	// bound cancelled them (E13′). Taken from what they returned directly.
	cancelled bool
}

// applyChunk writes one chunk (ADR 0067 addendum, point 2). A config with
// batch commands runs a create chunk of at least its minimum of accepted rows
// as a batch attempt inside SAVEPOINT chunk. A failed batch command,
// including 40P01/40001, rolls back to the savepoint and replays the chunk
// row by row in the same transaction attempt, so the replay keeps the whole
// retry budget; bounded statements the bound cancelled replay in a new
// transaction (E13′); a broken connection is returned. Every other failure is
// returned as the row path returns it. Any other chunk is the row path from
// the start. Each decision rests on a result where it arises, never on an
// error found somewhere in a chain (#5552 review of 1bb4bde258).
func (a *batchImportAttempt[T]) applyChunk(ctx context.Context, batch *importModels.ImportResult[T]) (*importModels.ImportResult[T], error) {
	service := a.service()
	writer, ok := service.config.(ports.ChunkWriter[T])
	if !ok || !writer.ChunkWriterReady() || service.boundStatements == nil || a.rowPath ||
		a.run.request.Mode != importModels.ImportModeCreate || len(a.acceptedRows()) < chunkMinRows(writer) {
		return batch, a.applyAcceptedRows(ctx, service, batch)
	}
	// The attempt's failure reaches the savepoint as a marker of this call;
	// the savepoint returns that very marker only when its own rollback
	// succeeded.
	failed := errors.New("data import: chunk attempt failed")
	var attemptErr error
	a.batchFailure = nil
	err := service.transactions.Savepoint(ctx, func(ctx context.Context) error {
		if attemptErr = a.applySegments(ctx, writer, batch); attemptErr != nil {
			return failed
		}
		return nil
	})
	if err == nil {
		return batch, nil
	}
	if attemptErr == nil {
		// The savepoint's own statements failed before or after a successful
		// attempt: as the row path's first write.
		return batch, err
	}
	failure := a.batchFailure
	if failure != nil && failure.cancelled {
		// E13′: pgdriver closes the connection on 57014, so the chunk's
		// transaction is lost, its rollback included; writeBatches replays the
		// chunk row by row in a new one. The cause stays in the text only.
		a.rowPath = true
		a.run.observation.BatchFallbacks++
		return batch, fmt.Errorf("%w: %v", errBatchBoundExceeded, errors.Join(attemptErr, err))
	}
	if err != failed { //nolint:errorlint // identity: the savepoint returned our marker, so it rolled back
		// ROLLBACK TO SAVEPOINT itself failed, so the transaction is lost:
		// 500, no retry, no replay (#5827 review R3). The cause stays in the
		// text only, so no SQLSTATE in it can start a retry.
		return batch, fmt.Errorf("%w: %v", errChunkRollbackFailed, errors.Join(attemptErr, err))
	}
	if failure == nil {
		return batch, attemptErr
	}
	if service.connectionLost != nil && service.connectionLost(failure.err) {
		// The connection broke: nothing can replay in this transaction, and
		// the failure is neither replayed nor retried (#5827, lesson of Q6).
		return batch, failure.err
	}
	return a.replayChunk(ctx, service, batch, failure)
}

// replayChunk replays the chunk row by row in the same transaction after its
// batch command failed and the savepoint rolled back.
func (a *batchImportAttempt[T]) replayChunk(ctx context.Context, service *ImportService[T], batch *importModels.ImportResult[T], cause *batchFailure) (*importModels.ImportResult[T], error) {
	a.rowPath = true
	a.run.observation.BatchFallbacks++
	if directSQLState(cause.err) == "40P01" {
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

// segmentEnd extends a segment until a row shares a key with a row already
// in it.
func segmentEnd[T any](writer ports.ChunkWriter[T], rows []T, indices []int, start int) int {
	seen := make(map[string]bool)
	end := start
	for end < len(indices) {
		keys := writer.ChunkKeys(rows[indices[end]])
		for _, key := range keys {
			if seen[key] {
				return end
			}
		}
		for _, key := range keys {
			seen[key] = true
		}
		end++
	}
	return end
}

// applySegment resolves the segment in one protected read, decides each
// row's action in processing order exactly as the row path does, and creates
// the remaining rows through one bounded batch command.
func (a *batchImportAttempt[T]) applySegment(ctx context.Context, writer ports.ChunkWriter[T], batch *importModels.ImportResult[T], segment []int) error {
	service, request := a.service(), a.run.request
	if len(segment) < chunkMinRows(writer) {
		return a.applySegmentRows(ctx, batch, segment)
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
	if err := a.writeSegment(ctx, service, writer, creates); err != nil {
		return err
	}
	batch.CreatedCount += len(creates)
	return nil
}

// applySegmentRows writes a segment too small for the batch command on the
// row path.
func (a *batchImportAttempt[T]) applySegmentRows(ctx context.Context, batch *importModels.ImportResult[T], segment []int) error {
	service, request := a.service(), a.run.request
	for _, index := range segment {
		if err := service.applyBatchRow(ctx, request, batch, &request.Rows[index], index+2); err != nil {
			return err
		}
	}
	return nil
}

// writeSegment reads the student guard, then runs the batch command; its
// bundled statements run under the bound. Only the batch command's own
// failure and the guard's refusal are kept as the attempt's batchFailure
// (fault table in the PR).
func (a *batchImportAttempt[T]) writeSegment(ctx context.Context, service *ImportService[T], writer ports.ChunkWriter[T], creates []T) error {
	refused, err := writer.RefusesChunk(ctx, creates)
	if err != nil {
		return err
	}
	if refused {
		// The row path would refuse a row (ADR F5): the batch command is
		// refused, and the replay names the row as the row path does.
		a.batchFailure = &batchFailure{err: errChunkRefused}
		return errChunkRefused
	}
	statements := &boundedStatements[T]{service: service}
	err = writer.CreateChunk(ctx, creates, statements.run)
	switch {
	case err == nil:
		return nil
	case statements.control != nil:
		// A statement setting or resetting the bound failed, not the batch
		// command: as the row path's first write.
		return statements.control
	case service.lostTransaction(err):
		return fmt.Errorf("%w: %v", errSavepointLost, err)
	}
	a.batchFailure = &batchFailure{err: err, cancelled: statements.cancelled}
	return err
}

// boundedStatements runs the bundled statements of a batch command under the
// bound of the attempt (E13) and keeps, where they run, whether the bound
// cancelled them: E13′ rests on what they returned directly.
type boundedStatements[T any] struct {
	service   *ImportService[T]
	control   error // a statement setting or resetting the bound failed
	cancelled bool  // the bounded statements themselves ended with 57014
}

func (b *boundedStatements[T]) run(ctx context.Context, fn func(context.Context) error) error {
	ran := false
	var direct error
	err := b.service.boundStatements(ctx, b.service.batchLimit, func(ctx context.Context) error {
		ran = true
		direct = fn(ctx)
		return direct
	})
	switch {
	case err == nil:
	case !ran || direct == nil:
		b.control = err
	default:
		b.cancelled = directSQLState(direct) == "57014"
	}
	return err
}

// chunkMinRows is the writer's minimum, never below the two rows of the
// addendum.
func chunkMinRows[T any](writer ports.ChunkWriter[T]) int { return max(2, writer.ChunkMinRows()) }

// directSQLState is the SQLSTATE of err itself or of an error it wraps one by
// one (%w), never of an error joined into it: a joined error comes from
// another statement, such as a rollback after the failure.
func directSQLState(err error) string {
	for err != nil {
		if state, ok := err.(interface{ Field(byte) string }); ok {
			return state.Field('C')
		}
		err = errors.Unwrap(err)
	}
	return ""
}
