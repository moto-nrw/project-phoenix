package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

// Request-child storage backfill (#2713).
//
// enrollment.request_child_offerings stays the only production authority until
// Cutover (#2714). Effective bookings copy independently of submission proof:
//
//   - enrollment.care_offering_bookings receives one row per legacy row and
//     keeps the legacy id, so both sides join on id and every batch is
//     idempotent. It owns the effective care state (manual/automatic days,
//     half-open validity).
//   - No immutable, full-field submission authority is established for legacy
//     pairs. Every nonempty pair remains unresolved; no selection is guessed
//     from current intervals, old target rows or partial audit snapshots.
//     Copy runs remove untrusted target selections; source history is untouched.
//
// Every batch commits on its own and persists a per-tenant checkpoint
// (high-water mark plus counters) in
// enrollment.request_child_storage_backfill_checkpoints, so an interrupted run
// resumes at the next batch and a completed batch can be replayed without
// effect. Legacy rows are only read; they are never locked, mutated, or
// deleted.

const (
	requestChildOriginPolicy              = "immutable-submission-v1-unresolved-legacy"
	requestChildOriginReason              = "missing-authoritative-submission"
	defaultRequestChildStorageBatchSize   = 500
	defaultRequestChildStorageMaxPasses   = 5
	defaultRequestChildStorageMaxRetries  = 5
	defaultRequestChildStorageLockTimeout = 5 * time.Second
	requestChildStorageRetryBackoff       = 25 * time.Millisecond
)

// RequestChildStorageBackfillOptions tunes one run. Zero values select the
// defaults documented on each field.
type RequestChildStorageBackfillOptions struct {
	// BatchSize is the number of legacy ids per committed batch (default 500).
	BatchSize int
	// TenantIDs limits the run; empty means every school in platform.schools.
	TenantIDs []int64
	// MaxPasses bounds the reconcile loop that re-reads changed legacy rows
	// until a pass finds no difference (default 5).
	MaxPasses int
	// MaxRetries bounds replays of one batch after a deadlock, serialization
	// failure, or lock timeout (default 5).
	MaxRetries int
	// LockTimeout is applied per batch transaction so the backfill never waits
	// longer than this on application locks (default 5s).
	LockTimeout time.Duration
	// VerifyOnly leaves source/target data untouched, but refreshes coherent
	// verification and invalidates acceptance when history or copies disagree.
	VerifyOnly bool
	// Restart truncates the target tables and resets the checkpoints before
	// copying from zero. Legacy rows are untouched. Only valid before Cutover.
	Restart bool
	// Logger defaults to slog.Default().
	Logger *slog.Logger

	// AfterBatch runs after each committed batch. Returning an error stops the
	// run at that batch boundary; the checkpoint already holds the batch.
	AfterBatch func(tenantID int64, batch RequestChildStorageBatch) error
	// BeforeCommit runs inside the batch transaction after every statement and
	// before the checkpoint update. Returning an error aborts that attempt.
	BeforeCommit func(tenantID int64, attempt int) error
	// Test seam for a source commit between verification statements.
	afterVerificationCounts func() error
}

func (o RequestChildStorageBackfillOptions) withDefaults() RequestChildStorageBackfillOptions {
	if o.BatchSize <= 0 {
		o.BatchSize = defaultRequestChildStorageBatchSize
	}
	if o.MaxPasses <= 0 {
		o.MaxPasses = defaultRequestChildStorageMaxPasses
	}
	if o.MaxRetries < 0 {
		o.MaxRetries = 0
	} else if o.MaxRetries == 0 {
		o.MaxRetries = defaultRequestChildStorageMaxRetries
	}
	if o.LockTimeout <= 0 {
		o.LockTimeout = defaultRequestChildStorageLockTimeout
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return o
}

// RequestChildStorageBatch describes one committed batch.
type RequestChildStorageBatch struct {
	// Pass is 0 for the id sweep and 1..MaxPasses for reconcile passes.
	Pass int
	// IDs are the legacy ids the batch examined; Pairs are explicit
	// (request_child_id, care_offering_id) pairs from reconcile passes.
	IDs   []int64
	Pairs [][2]int64
	// HighWaterMark is the checkpoint after this batch.
	HighWaterMark int64
	Scanned       int64
	Copied        int64
	Skipped       int64
	Deleted       int64
	Attempts      int
	Duration      time.Duration
}

// RequestChildStorageVerification compares both targets against the legacy
// table for one tenant. Checksums are SHA-256 digests over a canonical,
// session-independent row rendering ordered by id (bookings) or by pair
// (selections); they are comparable across runs and hosts.
type RequestChildStorageVerification struct {
	SourceBookings           int64
	TargetBookings           int64
	SourceSelections         int64
	TargetSelections         int64
	SourceBookingsChecksum   string
	TargetBookingsChecksum   string
	SourceSelectionsChecksum string
	TargetSelectionsChecksum string
	// MissingOrDifferent and Orphans measure copy differences separately from
	// unresolved historical submission origins.
	MissingOrDifferent int64
	Orphans            int64
	// OldestUnmigrated is the age of the oldest legacy row that still lacks an
	// equal target row; zero when everything matches.
	OldestUnmigrated  time.Duration
	VerifiedAt        time.Time
	Snapshot          string
	ProvenancePolicy  string
	UnresolvedOrigins int64
	UnresolvedReasons map[string]int64
}

// Mismatches is the total the exit criterion requires to be zero.
func (v RequestChildStorageVerification) Mismatches() int64 {
	return v.MissingOrDifferent + v.Orphans
}

// Equal reports equal counts and checksums with no mismatch.
func (v RequestChildStorageVerification) Equal() bool {
	return v.ProvenancePolicy == requestChildOriginPolicy && v.Snapshot != "" && !v.VerifiedAt.IsZero() && v.UnresolvedOrigins == 0 && v.SourceBookings == v.TargetBookings && v.SourceSelections == v.TargetSelections &&
		v.SourceBookingsChecksum == v.TargetBookingsChecksum &&
		v.SourceSelectionsChecksum == v.TargetSelectionsChecksum && v.Mismatches() == 0
}

// RequestChildStorageTenantReport is the persisted checkpoint plus the runtime
// evidence of the current run for one tenant.
type RequestChildStorageTenantReport struct {
	TenantID      int64
	HighWaterMark int64
	// Cumulative counters across every run since the last restart.
	RowsScanned           int64
	RowsCopied            int64
	RowsSkipped           int64
	RowsDeleted           int64
	BatchesCompleted      int64
	BatchesRetried        int64
	Deadlocks             int64
	SerializationFailures int64
	LockTimeouts          int64
	// BatchP95 and PoolWait describe the current run; BatchMax is the maximum
	// across runs.
	BatchP95 time.Duration
	BatchMax time.Duration
	PoolWait time.Duration
	LockWait time.Duration
	// Passes is the number of reconcile passes of the current run; Stable
	// means the last pass found nothing to change.
	Passes       int
	Stable       bool
	Verification RequestChildStorageVerification
	// Complete is the checkpoint state Cutover reads: the sweep and a stable
	// reconcile finished and verification found equal counts and checksums.
	Complete bool
}

// RequestChildStorageReport is the outcome of one run.
type RequestChildStorageReport struct {
	Tenants []RequestChildStorageTenantReport
	// DatabaseDeadlocks is the pg_stat_database.deadlocks delta for the whole
	// database during the run; it also counts other sessions' deadlocks.
	DatabaseDeadlocks      int64
	DatabaseDeadlocksKnown bool
	Duration               time.Duration
}

// IncompleteTenants lists tenants whose checkpoint is not complete. An empty
// result is the run-level exit criterion.
func (r RequestChildStorageReport) IncompleteTenants() []int64 {
	var ids []int64
	for _, tenant := range r.Tenants {
		if !tenant.Complete {
			ids = append(ids, tenant.TenantID)
		}
	}
	return ids
}

// RunRequestChildStorageBackfill copies and verifies request-child storage
// for the selected tenants. It returns an error only for failures it could
// not retry; an incomplete or unstable tenant is reported, not returned.
func RunRequestChildStorageBackfill(ctx context.Context, db *bun.DB, options RequestChildStorageBackfillOptions) (RequestChildStorageReport, error) {
	if db == nil {
		return RequestChildStorageReport{}, fmt.Errorf("request child storage backfill: database is required")
	}
	options = options.withDefaults()
	release, err := lockRequestChildStorageBackfill(ctx, db)
	if err != nil {
		return RequestChildStorageReport{}, err
	}
	defer release()
	if err := assertRequestChildStorageSource(ctx, db); err != nil {
		return RequestChildStorageReport{}, err
	}
	started := time.Now()
	if options.Restart && options.VerifyOnly {
		return RequestChildStorageReport{}, fmt.Errorf("request child storage backfill: restart and verify-only are mutually exclusive")
	}
	tenants, err := requestChildStorageTenants(ctx, db, options.TenantIDs)
	if err != nil {
		return RequestChildStorageReport{}, err
	}
	if options.Restart {
		if err := restartRequestChildStorageBackfill(ctx, db, options.TenantIDs); err != nil {
			return RequestChildStorageReport{}, err
		}
	}
	deadlocksBefore, err := databaseDeadlocks(ctx, db)
	if err != nil {
		return RequestChildStorageReport{}, err
	}
	report := RequestChildStorageReport{}
	var failures []error
	for _, tenantID := range tenants {
		tenantReport, err := backfillRequestChildStorageTenant(ctx, db, tenantID, options)
		report.Tenants = append(report.Tenants, tenantReport)
		if err != nil {
			failures = append(failures, err)
			if ctx.Err() != nil {
				break
			}
		}
	}
	if !options.VerifyOnly {
		if err := alignCareOfferingBookingSequence(ctx, db); err != nil {
			report.Duration = time.Since(started)
			return report, errors.Join(errors.Join(failures...), err)
		}
	}
	deadlocksAfter, err := databaseDeadlocks(ctx, db)
	if err != nil {
		report.Duration = time.Since(started)
		return report, errors.Join(errors.Join(failures...), err)
	}
	report.DatabaseDeadlocks = deadlocksAfter - deadlocksBefore
	report.DatabaseDeadlocksKnown = true
	report.Duration = time.Since(started)
	return report, errors.Join(failures...)
}

func requestChildStorageTenants(ctx context.Context, db *bun.DB, requested []int64) ([]int64, error) {
	var tenants []int64
	query := `SELECT id FROM platform.schools ORDER BY id`
	var err error
	if len(requested) > 0 {
		err = db.NewRaw(`SELECT id FROM platform.schools WHERE id = ANY(?) ORDER BY id`, sqlBigintArray(requested)).Scan(ctx, &tenants)
	} else {
		err = db.NewRaw(query).Scan(ctx, &tenants)
	}
	if err != nil {
		return nil, fmt.Errorf("request child storage backfill: list tenants: %w", err)
	}
	if len(requested) > 0 && len(tenants) != len(requested) {
		return nil, fmt.Errorf("request child storage backfill: unknown tenant in %v", requested)
	}
	return tenants, nil
}

// sqlBigintArray renders ids as an inline PostgreSQL bigint[] literal. Values
// are integers, so the literal contains no user-controlled text.
func sqlBigintArray(ids []int64) bun.Safe {
	rendered := make([]string, len(ids))
	for i, id := range ids {
		rendered[i] = strconv.FormatInt(id, 10)
	}
	return bun.Safe("ARRAY[" + strings.Join(rendered, ",") + "]::bigint[]")
}

func pairArrays(pairs [][2]int64) (childIDs, offeringIDs []int64) {
	childIDs = make([]int64, 0, len(pairs))
	offeringIDs = make([]int64, 0, len(pairs))
	for _, pair := range pairs {
		childIDs = append(childIDs, pair[0])
		offeringIDs = append(offeringIDs, pair[1])
	}
	return childIDs, offeringIDs
}

func databaseDeadlocks(ctx context.Context, db *bun.DB) (int64, error) {
	var deadlocks int64
	if err := db.NewRaw(`SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()`).Scan(ctx, &deadlocks); err != nil {
		return 0, fmt.Errorf("request child storage backfill: read database deadlocks: %w", err)
	}
	return deadlocks, nil
}

// restartRequestChildStorageBackfill is the pre-Cutover rollback: it empties
// only the target rows of the selected tenants and resets their checkpoints.
func restartRequestChildStorageBackfill(ctx context.Context, db *bun.DB, tenantIDs []int64) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if len(tenantIDs) == 0 {
			_, err := tx.ExecContext(ctx, `
				TRUNCATE enrollment.care_offering_bookings, enrollment.request_child_offering_selections;
				DELETE FROM enrollment.request_child_storage_backfill_checkpoints;
			`)
			if err != nil {
				return fmt.Errorf("request child storage backfill: restart: %w", err)
			}
			return nil
		}
		_, err := tx.NewRaw(`
			DELETE FROM enrollment.care_offering_bookings WHERE tenant_id = ANY(?0);
			DELETE FROM enrollment.request_child_offering_selections WHERE tenant_id = ANY(?0);
			DELETE FROM enrollment.request_child_storage_backfill_checkpoints WHERE tenant_id = ANY(?0);
		`, sqlBigintArray(tenantIDs)).Exec(ctx)
		if err != nil {
			return fmt.Errorf("request child storage backfill: restart tenants %v: %w", tenantIDs, err)
		}
		return nil
	})
}

// alignCareOfferingBookingSequence moves the bookings sequence past the copied
// legacy ids so a later application insert cannot collide with them.
func alignCareOfferingBookingSequence(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `SELECT setval('enrollment.care_offering_bookings_id_seq',
		GREATEST((SELECT COALESCE(MAX(id), 0) FROM enrollment.care_offering_bookings) + 1,
			(SELECT last_value FROM enrollment.care_offering_bookings_id_seq)), false)`)
	if err != nil {
		return fmt.Errorf("request child storage backfill: align booking sequence: %w", err)
	}
	return nil
}

type requestChildStorageCheckpoint struct {
	TenantID              int64   `bun:"tenant_id"`
	HighWaterMark         int64   `bun:"high_water_mark"`
	RowsScanned           int64   `bun:"rows_scanned"`
	RowsCopied            int64   `bun:"rows_copied"`
	RowsSkipped           int64   `bun:"rows_skipped"`
	RowsDeleted           int64   `bun:"rows_deleted"`
	BatchesCompleted      int64   `bun:"batches_completed"`
	BatchesRetried        int64   `bun:"batches_retried"`
	Deadlocks             int64   `bun:"deadlocks"`
	SerializationFailures int64   `bun:"serialization_failures"`
	LockTimeouts          int64   `bun:"lock_timeouts"`
	LockWaitMs            float64 `bun:"lock_wait_ms"`
	BatchMaxMs            int64   `bun:"batch_max_ms"`
}

type tenantBackfillRun struct {
	db       *bun.DB
	tenantID int64
	options  RequestChildStorageBackfillOptions
	logger   *slog.Logger

	checkpoint requestChildStorageCheckpoint
	durations  []time.Duration
	// Failures of the current run, persisted with the next successful batch.
	retried, deadlocks, serializationFailures, lockTimeouts int64
	lockWait                                                time.Duration
}

func backfillRequestChildStorageTenant(ctx context.Context, db *bun.DB, tenantID int64, options RequestChildStorageBackfillOptions) (RequestChildStorageTenantReport, error) {
	run := &tenantBackfillRun{db: db, tenantID: tenantID, options: options, logger: options.Logger.With("tenant_id", tenantID)}
	poolWaitBefore := db.DB.Stats().WaitDuration
	if err := run.loadCheckpoint(ctx); err != nil {
		return RequestChildStorageTenantReport{TenantID: tenantID}, err
	}
	var passes int
	var stable bool
	if !options.VerifyOnly {
		if err := run.sweep(ctx); err != nil {
			return run.report(passes, stable, RequestChildStorageVerification{}, poolWaitBefore), err
		}
		var err error
		passes, stable, err = run.reconcile(ctx)
		if err != nil {
			return run.report(passes, stable, RequestChildStorageVerification{}, poolWaitBefore), err
		}
	}
	var report RequestChildStorageTenantReport
	err := db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL TIME ZONE 'UTC'; SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		verification, err := verifyRequestChildStorageTenant(ctx, tx, tenantID, options.afterVerificationCounts)
		if err != nil {
			return err
		}
		report = run.report(passes, stable, verification, poolWaitBefore)
		report.Complete = verification.Equal() && (stable || options.VerifyOnly)
		return run.persistVerification(ctx, tx, report)
	})
	if err != nil {
		return run.report(passes, false, RequestChildStorageVerification{}, poolWaitBefore), err
	}
	verification := report.Verification
	run.logger.Info("request child storage backfill tenant finished",
		"complete", report.Complete,
		"provenance_policy", verification.ProvenancePolicy,
		"unresolved_origins", verification.UnresolvedOrigins,
		"unresolved_reasons", verification.UnresolvedReasons,
		"lock_wait_ms", float64(report.LockWait)/float64(time.Millisecond),
		"stable", report.Stable,
		"passes", report.Passes,
		"high_water_mark", report.HighWaterMark,
		"rows_scanned", report.RowsScanned,
		"rows_copied", report.RowsCopied,
		"rows_skipped", report.RowsSkipped,
		"rows_deleted", report.RowsDeleted,
		"batches_completed", report.BatchesCompleted,
		"batches_retried", report.BatchesRetried,
		"deadlocks", report.Deadlocks,
		"serialization_failures", report.SerializationFailures,
		"lock_timeouts", report.LockTimeouts,
		"batch_p95_ms", report.BatchP95.Milliseconds(),
		"batch_max_ms", report.BatchMax.Milliseconds(),
		"pool_wait_ms", report.PoolWait.Milliseconds(),
		"mismatch_count", verification.Mismatches(),
		"orphan_count", verification.Orphans,
		"oldest_unmigrated_seconds", int64(verification.OldestUnmigrated.Seconds()),
		"source_bookings", verification.SourceBookings,
		"target_bookings", verification.TargetBookings,
		"source_selections", verification.SourceSelections,
		"target_selections", verification.TargetSelections,
	)
	return report, nil
}

func (run *tenantBackfillRun) loadCheckpoint(ctx context.Context) error {
	_, err := run.db.NewRaw(`INSERT INTO enrollment.request_child_storage_backfill_checkpoints (tenant_id)
		VALUES (?) ON CONFLICT (tenant_id) DO UPDATE SET complete = FALSE, stable = FALSE`, run.tenantID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("request child storage backfill: create checkpoint for tenant %d: %w", run.tenantID, err)
	}
	if !run.options.VerifyOnly {
		// Cumulative counters span every copy run; runs and last_run_started_at
		// tell per-run columns (batch p95, pool wait, passes) apart.
		_, err = run.db.NewRaw(`UPDATE enrollment.request_child_storage_backfill_checkpoints
			SET runs = runs + 1, last_run_started_at = NOW(), complete = FALSE, stable = FALSE, updated_at = NOW() WHERE tenant_id = ?`, run.tenantID).Exec(ctx)
		if err != nil {
			return fmt.Errorf("request child storage backfill: start run for tenant %d: %w", run.tenantID, err)
		}
	}
	err = run.db.NewRaw(`SELECT tenant_id, high_water_mark, rows_scanned, rows_copied, rows_skipped, rows_deleted,
			batches_completed, batches_retried, deadlocks, serialization_failures, lock_timeouts, batch_max_ms, lock_wait_ms
		FROM enrollment.request_child_storage_backfill_checkpoints WHERE tenant_id = ?`, run.tenantID).Scan(ctx, &run.checkpoint)
	if err != nil {
		return fmt.Errorf("request child storage backfill: load checkpoint for tenant %d: %w", run.tenantID, err)
	}
	return nil
}

// ErrRequestChildStorageBackfillStopped reports a stop at a batch boundary
// after the context was cancelled. Every committed batch is checkpointed and
// the next run resumes there.
var ErrRequestChildStorageBackfillStopped = errors.New("request child storage backfill stopped; checkpoint retained")

func stopRequested(ctx context.Context) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %w", ErrRequestChildStorageBackfillStopped, ctx.Err())
	}
	return nil
}

// sweep copies legacy rows above the high-water mark in ascending id batches.
func (run *tenantBackfillRun) sweep(ctx context.Context) error {
	for {
		if err := stopRequested(ctx); err != nil {
			return err
		}
		var ids []int64
		err := run.db.NewRaw(`SELECT id FROM enrollment.request_child_offerings
			WHERE tenant_id = ? AND id > ? ORDER BY id LIMIT ?`, run.tenantID, run.checkpoint.HighWaterMark, run.options.BatchSize).Scan(ctx, &ids)
		if err != nil {
			return fmt.Errorf("request child storage backfill: select batch for tenant %d: %w", run.tenantID, err)
		}
		if len(ids) == 0 {
			return nil
		}
		batch := RequestChildStorageBatch{Pass: 0, IDs: ids, HighWaterMark: ids[len(ids)-1]}
		if err := run.commitBatch(ctx, &batch); err != nil {
			return err
		}
		if run.options.AfterBatch != nil {
			if err := run.options.AfterBatch(run.tenantID, batch); err != nil {
				return err
			}
		}
	}
}

// reconcile re-reads legacy rows that differ from their targets until a pass
// finds none, bounded by MaxPasses. It returns the number of passes and
// whether the last pass was empty.
func (run *tenantBackfillRun) reconcile(ctx context.Context) (int, bool, error) {
	for pass := 1; pass <= run.options.MaxPasses; pass++ {
		ids, pairs, err := run.mismatches(ctx)
		if err != nil {
			return pass, false, err
		}
		if len(ids) == 0 && len(pairs) == 0 {
			return pass, true, nil
		}
		run.logger.Info("request child storage backfill reconcile pass",
			"pass", pass,
			"changed_rows", len(ids),
			"changed_pairs", len(pairs))
		for len(ids) > 0 || len(pairs) > 0 {
			if err := stopRequested(ctx); err != nil {
				return pass, false, err
			}
			batch := RequestChildStorageBatch{Pass: pass, HighWaterMark: run.checkpoint.HighWaterMark}
			batch.IDs, ids = splitBatch(ids, run.options.BatchSize)
			batch.Pairs, pairs = splitBatch(pairs, run.options.BatchSize)
			if err := run.commitBatch(ctx, &batch); err != nil {
				return pass, false, err
			}
			if run.options.AfterBatch != nil {
				if err := run.options.AfterBatch(run.tenantID, batch); err != nil {
					return pass, false, err
				}
			}
		}
	}
	return run.options.MaxPasses, false, nil
}

func splitBatch[T any](items []T, size int) (batch, rest []T) {
	if len(items) <= size {
		return items, nil
	}
	return items[:size], items[size:]
}

// mismatches lists legacy ids whose booking is missing, different, or orphaned
// and pairs whose selection is missing, different, or orphaned.
func (run *tenantBackfillRun) mismatches(ctx context.Context) ([]int64, [][2]int64, error) {
	ids, _, err := bookingMismatches(ctx, run.db, run.tenantID)
	if err != nil {
		return nil, nil, err
	}
	pairs, _, err := selectionMismatches(ctx, run.db, run.tenantID)
	if err != nil {
		return nil, nil, err
	}
	return ids, pairs, nil
}

const bookingMismatchQuery = `
	SELECT o.id, CASE WHEN b.id IS NULL THEN 'missing' ELSE 'different' END AS kind
	FROM enrollment.request_child_offerings AS o
	LEFT JOIN enrollment.care_offering_bookings AS b ON b.id = o.id AND b.tenant_id = o.tenant_id
	WHERE o.tenant_id = ?0 AND (b.id IS NULL OR NOT (` + bookingEqualsSource + `))
	UNION ALL
	SELECT b.id, 'orphan'
	FROM enrollment.care_offering_bookings AS b
	WHERE b.tenant_id = ?0 AND NOT EXISTS (
		SELECT 1 FROM enrollment.request_child_offerings AS o WHERE o.id = b.id AND o.tenant_id = b.tenant_id)
	ORDER BY 1`

// bookingEqualsSource compares a booking alias b with a legacy alias o.
const bookingEqualsSource = `b.request_child_id = o.request_child_id AND b.care_offering_id = o.care_offering_id
	AND b.manual_selected_days IS NOT DISTINCT FROM o.manual_selected_days
	AND b.automatic_selected_days IS NOT DISTINCT FROM o.automatic_selected_days
	AND b.valid_from IS NOT DISTINCT FROM o.valid_from AND b.valid_until IS NOT DISTINCT FROM o.valid_until
	AND b.created_at = o.created_at AND b.updated_at = o.updated_at`

// trustedSelections deliberately resolves no legacy pair: no immutable,
// tenant-linked full submission authority has been established. Interval order,
// current targets, and partial audit payloads are not provenance.
const trustedSelections = `
	SELECT tenant_id, request_child_id, care_offering_id, selected_days, notes, created_at
	FROM enrollment.request_child_offerings WHERE tenant_id = ?0 AND FALSE`

const selectionMismatchQuery = `
	WITH origin AS (` + trustedSelections + `),
	target AS (
		SELECT s.request_child_id, s.care_offering_id, s.selected_days, s.notes, s.created_at
		FROM enrollment.request_child_offering_selections AS s WHERE s.tenant_id = ?0)
	SELECT COALESCE(origin.request_child_id, target.request_child_id) AS request_child_id,
		COALESCE(origin.care_offering_id, target.care_offering_id) AS care_offering_id,
		CASE WHEN target.request_child_id IS NULL THEN 'missing'
			WHEN origin.request_child_id IS NULL THEN 'orphan' ELSE 'different' END AS kind
	FROM origin
	FULL JOIN target ON target.request_child_id = origin.request_child_id AND target.care_offering_id = origin.care_offering_id
	WHERE origin.request_child_id IS NULL OR target.request_child_id IS NULL
		OR (origin.selected_days, origin.notes, origin.created_at) IS DISTINCT FROM (target.selected_days, target.notes, target.created_at)
	ORDER BY 1, 2`

type mismatchCounts struct {
	missingOrDifferent, orphans int64
}

func bookingMismatches(ctx context.Context, db bun.IDB, tenantID int64) ([]int64, mismatchCounts, error) {
	var rows []struct {
		ID   int64  `bun:"id"`
		Kind string `bun:"kind"`
	}
	if err := db.NewRaw(bookingMismatchQuery, tenantID).Scan(ctx, &rows); err != nil {
		return nil, mismatchCounts{}, fmt.Errorf("request child storage backfill: find booking mismatches for tenant %d: %w", tenantID, err)
	}
	ids := make([]int64, 0, len(rows))
	var counts mismatchCounts
	for _, row := range rows {
		ids = append(ids, row.ID)
		if row.Kind == "orphan" {
			counts.orphans++
		} else {
			counts.missingOrDifferent++
		}
	}
	return ids, counts, nil
}

func selectionMismatches(ctx context.Context, db bun.IDB, tenantID int64) ([][2]int64, mismatchCounts, error) {
	var rows []struct {
		RequestChildID int64  `bun:"request_child_id"`
		CareOfferingID int64  `bun:"care_offering_id"`
		Kind           string `bun:"kind"`
	}
	if err := db.NewRaw(selectionMismatchQuery, tenantID).Scan(ctx, &rows); err != nil {
		return nil, mismatchCounts{}, fmt.Errorf("request child storage backfill: find selection mismatches for tenant %d: %w", tenantID, err)
	}
	pairs := make([][2]int64, 0, len(rows))
	var counts mismatchCounts
	for _, row := range rows {
		pairs = append(pairs, [2]int64{row.RequestChildID, row.CareOfferingID})
		if row.Kind == "orphan" {
			counts.orphans++
		} else {
			counts.missingOrDifferent++
		}
	}
	return pairs, counts, nil
}

// commitBatch applies one batch in its own transaction, replaying it after a
// deadlock, serialization failure, or lock timeout. A cancelled context stops
// between batches, never inside one: the running transaction finishes on a
// cancellation-free context (bounded by batch size and lock_timeout) so the
// checkpoint always lands on a batch boundary.
func (run *tenantBackfillRun) commitBatch(ctx context.Context, batch *RequestChildStorageBatch) (resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, run.flushFailureTelemetry(ctx))
		}
	}()
	started := time.Now()
	batchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	defer cancel()
	for attempt := 1; ; attempt++ {
		batch.Attempts = attempt
		err := run.monitoredBatch(batchCtx, batch, attempt)
		if err == nil {
			batch.Duration = time.Since(started)
			run.durations = append(run.durations, batch.Duration)
			run.checkpoint.HighWaterMark = batch.HighWaterMark
			run.checkpoint.RowsScanned += batch.Scanned
			run.checkpoint.RowsCopied += batch.Copied
			run.checkpoint.RowsSkipped += batch.Skipped
			run.checkpoint.RowsDeleted += batch.Deleted
			run.checkpoint.BatchesCompleted++
			run.checkpoint.BatchesRetried += run.retried
			run.checkpoint.Deadlocks += run.deadlocks
			run.checkpoint.SerializationFailures += run.serializationFailures
			run.checkpoint.LockTimeouts += run.lockTimeouts
			run.checkpoint.LockWaitMs += float64(run.lockWait) / float64(time.Millisecond)
			run.lockWait = 0
			run.checkpoint.BatchMaxMs = max(run.checkpoint.BatchMaxMs, batch.Duration.Milliseconds())
			run.retried, run.deadlocks, run.serializationFailures, run.lockTimeouts = 0, 0, 0, 0
			return nil
		}
		kind, retryable := classifyRetryableSQLError(err)
		switch kind {
		case "40P01":
			run.deadlocks++
		case "40001":
			run.serializationFailures++
		case "55P03":
			run.lockTimeouts++
		}
		if !retryable || attempt > run.options.MaxRetries {
			return fmt.Errorf("request child storage backfill: tenant %d pass %d batch ending at id %d after %d attempts: %w", run.tenantID, batch.Pass, batch.HighWaterMark, attempt, err)
		}
		run.logger.Warn("request child storage backfill batch retried",
			"pass", batch.Pass,
			"attempt", attempt,
			"sqlstate", kind,
			"error", err.Error())
		select {
		case <-ctx.Done():
			return stopRequested(ctx)
		case <-time.After(requestChildStorageRetryBackoff * time.Duration(attempt)):
		}
		run.retried++
	}
}

// classifyRetryableSQLError reports the SQLSTATE of a deadlock (40P01),
// serialization failure (40001), or lock timeout (55P03) anywhere in the chain.
func classifyRetryableSQLError(err error) (string, bool) {
	var state interface{ Field(byte) string }
	if !errors.As(err, &state) {
		return "", false
	}
	switch code := state.Field('C'); code {
	case "40P01", "40001", "55P03":
		return code, true
	default:
		return code, false
	}
}

// touchedPairs lists the (request_child_id, care_offering_id) pairs a batch
// touches: the pairs of its legacy ids (?1) plus explicit pairs (?2, ?3) from a
// reconcile pass. Every pair statement of a batch works on this set, so stale
// rows of a pair are gone before a new interval is inserted.
const touchedPairs = `
	WITH touched AS (
		SELECT o.request_child_id, o.care_offering_id FROM enrollment.request_child_offerings AS o
		WHERE o.tenant_id = ?0 AND o.id = ANY(?1)
		UNION SELECT * FROM unnest(?2, ?3) AS pair(request_child_id, care_offering_id))`

const staleBookingDelete = touchedPairs + `
	DELETE FROM enrollment.care_offering_bookings AS b
	WHERE b.tenant_id = ?0
		AND (b.id = ANY(?1) OR (b.request_child_id, b.care_offering_id) IN (SELECT request_child_id, care_offering_id FROM touched))
		AND NOT EXISTS (
			SELECT 1 FROM enrollment.request_child_offerings AS o
			WHERE o.id = b.id AND o.tenant_id = b.tenant_id AND ` + bookingEqualsSource + `)`

const missingBookingInsert = `
	INSERT INTO enrollment.care_offering_bookings
		(id, tenant_id, request_child_id, care_offering_id, manual_selected_days, automatic_selected_days,
		 valid_from, valid_until, created_at, updated_at)
	SELECT o.id, o.tenant_id, o.request_child_id, o.care_offering_id, o.manual_selected_days, o.automatic_selected_days,
		o.valid_from, o.valid_until, o.created_at, o.updated_at
	FROM enrollment.request_child_offerings AS o
	WHERE o.tenant_id = ?0 AND o.id = ANY(?1)
	ORDER BY o.id
	ON CONFLICT (id) DO NOTHING`

const orphanSelectionDelete = touchedPairs + `
	DELETE FROM enrollment.request_child_offering_selections AS s
	WHERE s.tenant_id = ?0
		AND (s.request_child_id, s.care_offering_id) IN (SELECT request_child_id, care_offering_id FROM touched)`

const checkpointBatchUpdate = `
	UPDATE enrollment.request_child_storage_backfill_checkpoints SET
		high_water_mark = ?1, rows_scanned = rows_scanned + ?2, rows_copied = rows_copied + ?3,
		rows_skipped = rows_skipped + ?4, rows_deleted = rows_deleted + ?5,
		batches_completed = batches_completed + 1, batches_retried = batches_retried + ?6,
		deadlocks = deadlocks + ?7, serialization_failures = serialization_failures + ?8,
		lock_timeouts = lock_timeouts + ?9, batch_max_ms = GREATEST(batch_max_ms, ?10), lock_wait_ms = lock_wait_ms + ?11,
		stable = FALSE, complete = FALSE, updated_at = NOW()
	WHERE tenant_id = ?0`

// applyBatch runs the idempotent statements of one batch. It never touches
// legacy rows: they are read once per statement and compared in SQL.
func (run *tenantBackfillRun) applyBatch(ctx context.Context, tx bun.Tx, batch *RequestChildStorageBatch, attempt int, finishObservation func() error) error {
	started := time.Now()
	// lock_timeout takes integer milliseconds; SET cannot bind parameters, so
	// bun inlines the integer.
	if _, err := tx.NewRaw("SET LOCAL lock_timeout = ?", run.options.LockTimeout.Milliseconds()).Exec(ctx); err != nil {
		return fmt.Errorf("set lock timeout: %w", err)
	}
	ids := sqlBigintArray(batch.IDs)
	childIDs, offeringIDs := pairArrays(batch.Pairs)
	children, offerings := sqlBigintArray(childIDs), sqlBigintArray(offeringIDs)

	var scannedSource int64
	if err := tx.NewRaw(`SELECT count(*) FROM enrollment.request_child_offerings WHERE tenant_id = ?0 AND id = ANY(?1)`, run.tenantID, ids).Scan(ctx, &scannedSource); err != nil {
		return fmt.Errorf("count batch source rows: %w", err)
	}
	deletedBookings, err := rowsAffected(tx.NewRaw(staleBookingDelete, run.tenantID, ids, children, offerings).Exec(ctx))
	if err != nil {
		return fmt.Errorf("delete stale bookings: %w", err)
	}
	insertedBookings, err := rowsAffected(tx.NewRaw(missingBookingInsert, run.tenantID, ids).Exec(ctx))
	if err != nil {
		return fmt.Errorf("insert bookings: %w", err)
	}
	deletedSelections, err := rowsAffected(tx.NewRaw(orphanSelectionDelete, run.tenantID, ids, children, offerings).Exec(ctx))
	if err != nil {
		return fmt.Errorf("delete orphan selections: %w", err)
	}
	batch.Scanned = int64(len(batch.IDs)) + int64(len(batch.Pairs))
	batch.Copied = insertedBookings
	batch.Skipped = scannedSource - insertedBookings
	batch.Deleted = deletedBookings + deletedSelections
	if run.options.BeforeCommit != nil {
		if err := run.options.BeforeCommit(run.tenantID, attempt); err != nil {
			return err
		}
	}
	if err := finishObservation(); err != nil {
		return err
	}
	_, err = tx.NewRaw(checkpointBatchUpdate, run.tenantID, batch.HighWaterMark, batch.Scanned, batch.Copied, batch.Skipped, batch.Deleted,
		run.retried, run.deadlocks, run.serializationFailures, run.lockTimeouts, time.Since(started).Milliseconds(), float64(run.lockWait)/float64(time.Millisecond)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("update checkpoint: %w", err)
	}
	return nil
}

func rowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Checksum rows render NULL distinctly from every real value and instants
// independent of the session time zone, so digests compare across hosts.
const bookingChecksumRow = `jsonb_build_array(id, request_child_id, care_offering_id,
	manual_selected_days, automatic_selected_days, valid_from, valid_until,
	to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US'),
	to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US'))::text`

const selectionChecksumRow = `jsonb_build_array(request_child_id, care_offering_id, selected_days, notes,
	to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US'))::text`

const verificationQuery = `
	WITH origin AS (` + trustedSelections + `)
	SELECT
		(SELECT count(*) FROM enrollment.request_child_offerings WHERE tenant_id = ?0) AS source_bookings,
		(SELECT count(*) FROM enrollment.care_offering_bookings WHERE tenant_id = ?0) AS target_bookings,
		(SELECT count(*) FROM origin) AS source_selections,
		(SELECT count(*) FROM enrollment.request_child_offering_selections WHERE tenant_id = ?0) AS target_selections,
		(SELECT encode(sha256(convert_to(COALESCE(string_agg(` + bookingChecksumRow + `, E'\n' ORDER BY id), ''), 'UTF8')), 'hex')
			FROM enrollment.request_child_offerings WHERE tenant_id = ?0) AS source_bookings_checksum,
		(SELECT encode(sha256(convert_to(COALESCE(string_agg(` + bookingChecksumRow + `, E'\n' ORDER BY id), ''), 'UTF8')), 'hex')
			FROM enrollment.care_offering_bookings WHERE tenant_id = ?0) AS target_bookings_checksum,
		(SELECT encode(sha256(convert_to(COALESCE(string_agg(` + selectionChecksumRow + `, E'\n' ORDER BY request_child_id, care_offering_id), ''), 'UTF8')), 'hex')
			FROM origin) AS source_selections_checksum,
		(SELECT encode(sha256(convert_to(COALESCE(string_agg(` + selectionChecksumRow + `, E'\n' ORDER BY request_child_id, care_offering_id), ''), 'UTF8')), 'hex')
			FROM enrollment.request_child_offering_selections WHERE tenant_id = ?0) AS target_selections_checksum,
		(SELECT count(*) FROM (SELECT DISTINCT request_child_id, care_offering_id FROM enrollment.request_child_offerings WHERE tenant_id = ?0) pairs) AS unresolved_origins,
		(SELECT COALESCE(GREATEST(0, EXTRACT(EPOCH FROM clock_timestamp() - MIN(created_at))), 0)::double precision
			FROM enrollment.request_child_offerings WHERE tenant_id = ?0) AS oldest_unresolved_seconds,
		pg_current_snapshot()::text AS snapshot,
		clock_timestamp() AS verified_at`

// verifyRequestChildStorageTenant compares counts, checksums, and row-level
// differences for one tenant without writing anything.
func verifyRequestChildStorageTenant(ctx context.Context, db bun.IDB, tenantID int64, afterCounts func() error) (RequestChildStorageVerification, error) {
	var row struct {
		SourceBookings           int64     `bun:"source_bookings"`
		TargetBookings           int64     `bun:"target_bookings"`
		SourceSelections         int64     `bun:"source_selections"`
		TargetSelections         int64     `bun:"target_selections"`
		SourceBookingsChecksum   string    `bun:"source_bookings_checksum"`
		TargetBookingsChecksum   string    `bun:"target_bookings_checksum"`
		SourceSelectionsChecksum string    `bun:"source_selections_checksum"`
		TargetSelectionsChecksum string    `bun:"target_selections_checksum"`
		VerifiedAt               time.Time `bun:"verified_at"`
		UnresolvedOrigins        int64     `bun:"unresolved_origins"`
		OldestUnresolvedSeconds  float64   `bun:"oldest_unresolved_seconds"`
		Snapshot                 string    `bun:"snapshot"`
	}
	if err := db.NewRaw(verificationQuery, tenantID).Scan(ctx, &row); err != nil {
		return RequestChildStorageVerification{}, fmt.Errorf("request child storage backfill: verify tenant %d: %w", tenantID, err)
	}
	if afterCounts != nil {
		if err := afterCounts(); err != nil {
			return RequestChildStorageVerification{}, err
		}
	}
	verification := RequestChildStorageVerification{
		SourceBookings: row.SourceBookings, TargetBookings: row.TargetBookings,
		SourceSelections: row.SourceSelections, TargetSelections: row.TargetSelections,
		SourceBookingsChecksum: row.SourceBookingsChecksum, TargetBookingsChecksum: row.TargetBookingsChecksum,
		SourceSelectionsChecksum: row.SourceSelectionsChecksum, TargetSelectionsChecksum: row.TargetSelectionsChecksum,
		VerifiedAt: row.VerifiedAt, Snapshot: row.Snapshot,
		ProvenancePolicy: requestChildOriginPolicy, UnresolvedOrigins: row.UnresolvedOrigins,
		UnresolvedReasons: map[string]int64{},
		OldestUnmigrated:  time.Duration(row.OldestUnresolvedSeconds * float64(time.Second)),
	}
	if row.UnresolvedOrigins > 0 {
		verification.UnresolvedReasons[requestChildOriginReason] = row.UnresolvedOrigins
	}
	ids, bookingCounts, err := bookingMismatches(ctx, db, tenantID)
	if err != nil {
		return verification, err
	}
	pairs, selectionCounts, err := selectionMismatches(ctx, db, tenantID)
	if err != nil {
		return verification, err
	}
	verification.MissingOrDifferent = bookingCounts.missingOrDifferent + selectionCounts.missingOrDifferent
	verification.Orphans = bookingCounts.orphans + selectionCounts.orphans
	if len(ids) > 0 || len(pairs) > 0 {
		childIDs, offeringIDs := pairArrays(pairs)
		var oldest sql.NullFloat64
		err := db.NewRaw(`SELECT EXTRACT(EPOCH FROM clock_timestamp() - MIN(o.created_at))
			FROM enrollment.request_child_offerings AS o
			WHERE o.tenant_id = ?0 AND (o.id = ANY(?1)
				OR (o.request_child_id, o.care_offering_id) IN (SELECT * FROM unnest(?2, ?3) AS pair(request_child_id, care_offering_id)))`,
			tenantID, sqlBigintArray(ids), sqlBigintArray(childIDs), sqlBigintArray(offeringIDs)).Scan(ctx, &oldest)
		if err != nil {
			return verification, fmt.Errorf("request child storage backfill: oldest unmigrated row for tenant %d: %w", tenantID, err)
		}
		if oldest.Valid {
			verification.OldestUnmigrated = max(verification.OldestUnmigrated, time.Duration(oldest.Float64*float64(time.Second)))
		}
	}
	return verification, nil
}

func (run *tenantBackfillRun) report(passes int, stable bool, verification RequestChildStorageVerification, poolWaitBefore time.Duration) RequestChildStorageTenantReport {
	checkpoint := run.checkpoint
	return RequestChildStorageTenantReport{
		TenantID:              run.tenantID,
		HighWaterMark:         checkpoint.HighWaterMark,
		RowsScanned:           checkpoint.RowsScanned,
		RowsCopied:            checkpoint.RowsCopied,
		RowsSkipped:           checkpoint.RowsSkipped,
		RowsDeleted:           checkpoint.RowsDeleted,
		BatchesCompleted:      checkpoint.BatchesCompleted,
		BatchesRetried:        checkpoint.BatchesRetried + run.retried,
		Deadlocks:             checkpoint.Deadlocks + run.deadlocks,
		SerializationFailures: checkpoint.SerializationFailures + run.serializationFailures,
		LockTimeouts:          checkpoint.LockTimeouts + run.lockTimeouts,
		BatchP95:              percentile95(run.durations),
		BatchMax:              max(time.Duration(checkpoint.BatchMaxMs)*time.Millisecond, slices.Max(append(run.durations, 0))),
		PoolWait:              run.db.DB.Stats().WaitDuration - poolWaitBefore,
		LockWait:              time.Duration(checkpoint.LockWaitMs*float64(time.Millisecond)) + run.lockWait,
		Passes:                passes,
		Stable:                stable,
		Verification:          verification,
	}
}

func percentile95(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	sorted := slices.Clone(durations)
	slices.Sort(sorted)
	index := max((len(sorted)*95+99)/100, 1)
	return sorted[index-1]
}

const checkpointVerificationUpdate = `
	UPDATE enrollment.request_child_storage_backfill_checkpoints SET
		batch_p95_ms = ?1, pool_wait_ms = ?2, passes = ?3, stable = ?4,
		source_bookings = ?5, target_bookings = ?6, source_selections = ?7, target_selections = ?8,
		source_bookings_checksum = ?9, target_bookings_checksum = ?10,
		source_selections_checksum = ?11, target_selections_checksum = ?12,
		mismatch_count = ?13, orphan_count = ?14, oldest_unmigrated_seconds = ?15,
		complete = ?16, verified_at = ?17, verification_snapshot = ?18,
		provenance_policy = ?19, unresolved_origins = ?20, unresolved_reasons = ?21::jsonb, updated_at = NOW()
	WHERE tenant_id = ?0`

// Verify-only refreshes all acceptance evidence in its snapshot, while keeping
// cumulative copy metrics. Old completion is never retained as current proof.
const checkpointVerifyOnlyUpdate = `
	UPDATE enrollment.request_child_storage_backfill_checkpoints SET
		source_bookings = ?5, target_bookings = ?6, source_selections = ?7, target_selections = ?8,
		source_bookings_checksum = ?9, target_bookings_checksum = ?10,
		source_selections_checksum = ?11, target_selections_checksum = ?12,
		mismatch_count = ?13, orphan_count = ?14, oldest_unmigrated_seconds = ?15,
		verified_at = ?17, stable = stable AND ?16,
		verify_only_mismatch_count = ?13, verify_only_at = ?17, complete = ?16,
		verification_snapshot = ?18, provenance_policy = ?19, unresolved_origins = ?20,
		unresolved_reasons = ?21::jsonb, updated_at = NOW()
	WHERE tenant_id = ?0`

func (run *tenantBackfillRun) persistVerification(ctx context.Context, tx bun.Tx, report RequestChildStorageTenantReport) error {
	v := report.Verification
	reasons, err := json.Marshal(v.UnresolvedReasons)
	if err != nil {
		return err
	}
	args := []any{run.tenantID,
		report.BatchP95.Milliseconds(), report.PoolWait.Milliseconds(), report.Passes, report.Stable,
		v.SourceBookings, v.TargetBookings, v.SourceSelections, v.TargetSelections,
		v.SourceBookingsChecksum, v.TargetBookingsChecksum, v.SourceSelectionsChecksum, v.TargetSelectionsChecksum,
		v.Mismatches(), v.Orphans, int64(v.OldestUnmigrated.Seconds()), report.Complete, v.VerifiedAt, v.Snapshot, v.ProvenancePolicy, v.UnresolvedOrigins, string(reasons)}
	if run.options.VerifyOnly {
		_, err = tx.NewRaw(checkpointVerifyOnlyUpdate, args...).Exec(ctx)
	} else {
		_, err = tx.NewRaw(checkpointVerificationUpdate, args...).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("request child storage backfill: persist verification for tenant %d: %w", run.tenantID, err)
	}
	return nil
}
