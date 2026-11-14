//go:build testhooks

package migrations

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The batch loops of the four storage backfills retry and restart only on the
// direct result of the failed step (#6449). The lock sampler and a failed
// rollback are diagnostics, and a COMMIT with unknown outcome is never
// replayed or booked twice. The legacy sources are gone after the Contracts,
// so the tests replace only the copy statements with real statements of the
// same shape; connection, sampler, BEGIN, SET LOCAL, checkpoint, COMMIT,
// rollback, flush, rewind and the retry loops run for real on a
// fault-injecting pool: a cell of the clone's shared fault pool
// (testpkg.NewStatementFaultCells), or a pool of its own
// (testpkg.NewStatementFaultPool) for a cell that breaks a connection.

const (
	monitorSamplePrefix = "SELECT coalesce(wait_event_type"
	none                = "none" // no failure armed
)

// outcome is the decision of a batch loop: R retries the batch, N restarts
// the pass, F fails the tenant, S succeeds on the first attempt.
type outcome string

const (
	outcomeSuccess outcome = "S"
	outcomeRetry   outcome = "R"
	outcomeRestart outcome = "N"
	outcomeFail    outcome = "F"
)

// faultPlan arms one batch: an optional sampler failure observed before BEGIN,
// the main failure at its site and an optional rollback failure. Each failure
// fires once, so a retried or restarted batch succeeds.
type faultPlan struct {
	monitor  string
	site     string
	main     string
	rollback string
}

// breaksConnection reports whether a failure of the plan is
// driver.ErrBadConn, on which database/sql discards the connection.
func (p faultPlan) breaksConnection() bool {
	return p.monitor == "bad_conn" || p.main == "bad_conn" || p.rollback == "bad_conn"
}

// Failure sites, grouped by the classes of the PR's Klasse-12 plan.
const (
	siteBackendPID = "pg_backend_pid" // K1 first, before the sampler starts
	siteBegin      = "BEGIN"          // K1 last
	siteLimits     = "limits"         // K2 first
	siteLimitsLast = "limitsLast"     // K2 last (request-child)
	siteBodyFirst  = "bodyFirst"      // K3 first (owner: checkpoint FOR UPDATE; request-child: source count)
	siteCopy       = "copy"           // K3 other (owner copy statement)
	siteBodyLast   = "bodyLast"       // K3 last (injectFault / BeforeCommit)
	siteCheckpoint = "checkpoint"     // K5
	siteCommit     = "commit"         // K6, rejected by the server
)

// requestChildSources stand in for the five legacy source statements of a
// request-child batch, in their order; each has its own prefix.
var requestChildSources = []string{
	"SELECT 'rc_count_source', 3::bigint",
	"SELECT 'rc_stale_bookings', 1::bigint",
	"SELECT 'rc_missing_bookings', 1::bigint",
	"SELECT 'rc_stale_selections', 1::bigint",
	"SELECT 'rc_missing_selections', 1::bigint",
}

func requestChildSourceSite(i int) string { return fmt.Sprintf("source%d", i+1) }

type batchResult struct {
	err   error
	calls int // copy executions (owner) or source runs (request-child)
}

// checkpointRow holds every column the batch loops persist. Rejected is
// rows_rejected for owners and rows_deleted for request-child; Removed and
// PassWrites exist for owners only, Complete for request-child only.
type checkpointRow struct {
	HighWater, Scanned, Copied, Skipped, Rejected, Removed, PassWrites int64
	Completed, Retried, Deadlocks, Serialization, LockTimeouts         int64
	LockWaitMs                                                         float64
	Stable, Complete                                                   bool
}

// backfillDriver drives one batch of one backfill through its real loop.
type backfillDriver struct {
	name   string
	owner  bool
	prefix map[string]string
	seed   func(t *testing.T, super *bun.DB, tenantID int64)
	run    func(t *testing.T, pool, super *bun.DB, tenantID int64, s driverSeams) batchResult
	read   func(t *testing.T, db *bun.DB, tenantID int64) checkpointRow
}

// driverSeams are the test hooks of one batch: copy runs inside the copy
// position before its statement, last at the last body position.
type driverSeams struct {
	hooks batchAttemptHooks
	copy  func(ctx context.Context, tx bun.Tx, call int) error
	last  func(call int) error
}

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// ownerStart is the checkpoint an owner batch starts from.
var ownerStart = checkpointRow{HighWater: 5, Scanned: 100, Copied: 50, Skipped: 30, Rejected: 20, Removed: 4,
	PassWrites: 7, Completed: 10, Retried: 3, Deadlocks: 2, Serialization: 2, LockTimeouts: 2, LockWaitMs: 12.5, Stable: true}

// requestChildStart is the checkpoint a request-child batch starts from;
// loadCheckpoint withdraws acceptance, the driver grants it again.
var requestChildStart = checkpointRow{HighWater: 5, Scanned: 100, Copied: 50, Skipped: 30, Rejected: 20,
	Completed: 10, Retried: 3, Deadlocks: 2, Serialization: 2, LockTimeouts: 2, LockWaitMs: 12.5, Stable: true, Complete: true}

func ownerDriver[R any, C any](name, backfill string,
	newRun func(pool *bun.DB, tenantID int64, copyRows func(context.Context, bun.Tx, ...any) error,
		inject func(context.Context, bun.Tx, int64, int) error, hooks batchAttemptHooks) R,
	load func(R, context.Context) (*C, error), copyBatch func(R, context.Context, *C) error,
) backfillDriver {
	return backfillDriver{
		name:  name,
		owner: true,
		prefix: map[string]string{
			siteLimits:     "SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'",
			siteBodyFirst:  "SELECT tenant_id FROM platform.storage_backfill_checkpoints",
			siteCheckpoint: "UPDATE platform.storage_backfill_checkpoints SET",
		},
		seed: func(t *testing.T, super *bun.DB, tenantID int64) {
			s := ownerStart
			_, err := super.ExecContext(t.Context(), `INSERT INTO platform.storage_backfill_checkpoints
				(backfill, tenant_id, high_water_id, rows_scanned, rows_copied, rows_skipped, rows_rejected, rows_removed,
				 pass_writes, batches_completed, batches_retried, deadlocks, serialization_failures, lock_timeouts, lock_wait_ms, stable)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, backfill, tenantID, s.HighWater, s.Scanned, s.Copied,
				s.Skipped, s.Rejected, s.Removed, s.PassWrites, s.Completed, s.Retried, s.Deadlocks, s.Serialization,
				s.LockTimeouts, s.LockWaitMs, s.Stable)
			require.NoError(t, err)
		},
		run: func(t *testing.T, pool, _ *bun.DB, tenantID int64, s driverSeams) batchResult {
			var calls atomic.Int32
			// The copy reports 3 scanned, 1 rejected, 1 copied and the row
			// after the high-water mark the attempt read under its lock.
			copyRows := func(ctx context.Context, tx bun.Tx, dest ...any) error {
				call := int(calls.Add(1))
				var highWater int64
				if err := tx.NewRaw(`SELECT high_water_id FROM platform.storage_backfill_checkpoints
					WHERE backfill = ? AND tenant_id = ?`, backfill, tenantID).Scan(ctx, &highWater); err != nil {
					return err
				}
				if s.copy != nil {
					if err := s.copy(ctx, tx, call); err != nil {
						return err
					}
				}
				return tx.NewRaw(`SELECT 3::bigint, ?::bigint, 1::bigint, 1::bigint`, highWater+1).Scan(ctx, dest...)
			}
			inject := func(context.Context, bun.Tx, int64, int) error {
				if s.last != nil {
					return s.last(int(calls.Load()))
				}
				return nil
			}
			run := newRun(pool, tenantID, copyRows, inject, s.hooks)
			cp, err := load(run, t.Context())
			require.NoError(t, err)
			err = copyBatch(run, t.Context(), cp)
			return batchResult{err: err, calls: int(calls.Load())}
		},
		read: func(t *testing.T, db *bun.DB, tenantID int64) checkpointRow {
			var row checkpointRow
			require.NoError(t, db.NewRaw(`SELECT high_water_id, rows_scanned, rows_copied, rows_skipped, rows_rejected,
				rows_removed, pass_writes, batches_completed, batches_retried, deadlocks, serialization_failures,
				lock_timeouts, lock_wait_ms, stable
				FROM platform.storage_backfill_checkpoints WHERE backfill = ? AND tenant_id = ?`, backfill, tenantID).
				Scan(t.Context(), &row.HighWater, &row.Scanned, &row.Copied, &row.Skipped, &row.Rejected, &row.Removed,
					&row.PassWrites, &row.Completed, &row.Retried, &row.Deadlocks, &row.Serialization, &row.LockTimeouts,
					&row.LockWaitMs, &row.Stable))
			return row
		},
	}
}

var staffDriver = ownerDriver("staff", StaffOwnerBackfillName,
	func(pool *bun.DB, tenantID int64, copyRows func(context.Context, bun.Tx, ...any) error,
		inject func(context.Context, bun.Tx, int64, int) error, hooks batchAttemptHooks,
	) *staffOwnerTenantRun {
		opts := StaffOwnerBackfillOptions{MaxAttempts: 5, Logger: discardLogger(), copyRows: copyRows,
			injectFault: inject, attemptHooks: hooks}.withDefaults()
		return &staffOwnerTenantRun{db: pool, opts: opts, tenantID: tenantID}
	},
	func(r *staffOwnerTenantRun, ctx context.Context) (*StaffOwnerBackfillCheckpoint, error) {
		return r.loadCheckpoint(ctx)
	},
	func(r *staffOwnerTenantRun, ctx context.Context, cp *StaffOwnerBackfillCheckpoint) error {
		_, err := r.copyBatch(ctx, cp)
		return err
	})

var guardianDriver = ownerDriver("guardian", GuardianOwnerBackfillName,
	func(pool *bun.DB, tenantID int64, copyRows func(context.Context, bun.Tx, ...any) error,
		inject func(context.Context, bun.Tx, int64, int) error, hooks batchAttemptHooks,
	) *guardianOwnerTenantRun {
		opts := GuardianOwnerBackfillOptions{MaxAttempts: 5, Logger: discardLogger(), copyRows: copyRows,
			injectFault: inject, attemptHooks: hooks}.withDefaults()
		return &guardianOwnerTenantRun{db: pool, opts: opts, tenantID: tenantID}
	},
	func(r *guardianOwnerTenantRun, ctx context.Context) (*GuardianOwnerBackfillCheckpoint, error) {
		return r.loadCheckpoint(ctx)
	},
	func(r *guardianOwnerTenantRun, ctx context.Context, cp *GuardianOwnerBackfillCheckpoint) error {
		_, err := r.copyBatch(ctx, cp)
		return err
	})

var studentDriver = ownerDriver("student", StudentOwnerBackfillName,
	func(pool *bun.DB, tenantID int64, copyRows func(context.Context, bun.Tx, ...any) error,
		inject func(context.Context, bun.Tx, int64, int) error, hooks batchAttemptHooks,
	) *studentOwnerTenantRun {
		opts := StudentOwnerBackfillOptions{MaxAttempts: 5, Logger: discardLogger(), copyRows: copyRows,
			injectFault: inject, attemptHooks: hooks}.withDefaults()
		return &studentOwnerTenantRun{db: pool, opts: opts, tenantID: tenantID}
	},
	func(r *studentOwnerTenantRun, ctx context.Context) (*StudentOwnerBackfillCheckpoint, error) {
		return r.loadCheckpoint(ctx)
	},
	func(r *studentOwnerTenantRun, ctx context.Context, cp *StudentOwnerBackfillCheckpoint) error {
		_, err := r.copyBatch(ctx, cp)
		return err
	})

var requestChildDriver = backfillDriver{
	name: "request-child",
	prefix: func() map[string]string {
		prefix := map[string]string{
			siteLimits:     "SET LOCAL statement_timeout = '60s'",
			siteLimitsLast: "SET LOCAL lock_timeout",
			siteCheckpoint: "UPDATE enrollment.request_child_storage_backfill_checkpoints SET",
			siteBodyFirst:  requestChildSources[0],
		}
		for i, statement := range requestChildSources {
			prefix[requestChildSourceSite(i)] = statement
		}
		return prefix
	}(),
	seed: func(t *testing.T, super *bun.DB, tenantID int64) {
		s := requestChildStart
		_, err := super.ExecContext(t.Context(), `INSERT INTO enrollment.request_child_storage_backfill_checkpoints
			(tenant_id, high_water_mark, rows_scanned, rows_copied, rows_skipped, rows_deleted, batches_completed,
			 batches_retried, deadlocks, serialization_failures, lock_timeouts, lock_wait_ms)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, tenantID, s.HighWater, s.Scanned, s.Copied, s.Skipped, s.Rejected,
			s.Completed, s.Retried, s.Deadlocks, s.Serialization, s.LockTimeouts, s.LockWaitMs)
		require.NoError(t, err)
	},
	run: func(t *testing.T, pool, super *bun.DB, tenantID int64, s driverSeams) batchResult {
		var calls atomic.Int32
		options := RequestChildStorageBackfillOptions{MaxRetries: 5, Logger: discardLogger(), attemptHooks: s.hooks,
			applySource: func(ctx context.Context, tx bun.Tx, _ *RequestChildStorageBatch) (requestChildSourceCounts, error) {
				call := int(calls.Add(1))
				if s.copy != nil {
					if err := s.copy(ctx, tx, call); err != nil {
						return requestChildSourceCounts{}, err
					}
				}
				var values [5]int64
				for i, statement := range requestChildSources {
					var label string
					if err := tx.NewRaw(statement).Scan(ctx, &label, &values[i]); err != nil {
						return requestChildSourceCounts{}, err
					}
				}
				return requestChildSourceCounts{scannedSource: values[0], deletedBookings: values[1],
					insertedBookings: values[2], deletedSelections: values[3], insertedSelections: values[4]}, nil
			},
			BeforeCommit: func(int64, int) error {
				if s.last != nil {
					return s.last(int(calls.Load()))
				}
				return nil
			},
		}.withDefaults()
		run := &tenantBackfillRun{db: pool, tenantID: tenantID, options: options, logger: options.Logger}
		require.NoError(t, run.loadCheckpoint(t.Context()))
		_, err := super.ExecContext(t.Context(), `UPDATE enrollment.request_child_storage_backfill_checkpoints
			SET complete = TRUE, stable = TRUE WHERE tenant_id = ?`, tenantID)
		require.NoError(t, err)
		err = run.commitBatch(t.Context(), &RequestChildStorageBatch{IDs: []int64{10}, HighWaterMark: 10})
		return batchResult{err: err, calls: int(calls.Load())}
	},
	read: func(t *testing.T, db *bun.DB, tenantID int64) checkpointRow {
		var row checkpointRow
		require.NoError(t, db.NewRaw(`SELECT high_water_mark, rows_scanned, rows_copied, rows_skipped, rows_deleted,
			batches_completed, batches_retried, deadlocks, serialization_failures, lock_timeouts, lock_wait_ms,
			stable, complete
			FROM enrollment.request_child_storage_backfill_checkpoints WHERE tenant_id = ?`, tenantID).
			Scan(t.Context(), &row.HighWater, &row.Scanned, &row.Copied, &row.Skipped, &row.Rejected, &row.Completed,
				&row.Retried, &row.Deadlocks, &row.Serialization, &row.LockTimeouts, &row.LockWaitMs, &row.Stable, &row.Complete))
		return row
	},
}

var allDrivers = []backfillDriver{staffDriver, guardianDriver, studentDriver, requestChildDriver}

// backfillTestDB is an isolated clone with the empty legacy tables the owner
// rewinds read, a deferred probe whose rows fail COMMIT with their code, and
// a row-lock probe.
func backfillTestDB(t *testing.T) *bun.DB {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	_, err := db.ExecContext(t.Context(), `
		CREATE TABLE users.staff (id bigint, tenant_id bigint, person_id bigint, deleted_at timestamptz,
			created_at timestamptz, updated_at timestamptz, work_time_model_id bigint);
		CREATE TABLE users.students_guardians (id bigint, tenant_id bigint, student_id bigint,
			guardian_profile_id bigint, is_primary boolean, is_payer boolean);
		CREATE TABLE users.students (id bigint, tenant_id bigint, person_id bigint);
		CREATE TABLE public.commit_probe (code text NOT NULL);
		CREATE TABLE public.lock_probe (id bigint PRIMARY KEY);
		CREATE FUNCTION public.commit_probe_fail() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'commit probe %', NEW.code USING ERRCODE = NEW.code; END $$;
		CREATE CONSTRAINT TRIGGER commit_probe_fail AFTER INSERT ON public.commit_probe
			DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.commit_probe_fail();`)
	require.NoError(t, err)
	return db
}

// faultCells holds the shared fault pool of each clone TestBackfillBatchRetry
// opened: *bun.DB of the clone -> *testpkg.StatementFaultCells.
var faultCells sync.Map

// newBatch opens a cell of the clone's shared fault pool and a seeded tenant
// of its own. A cell whose plan breaks a connection (newConnection) keeps a
// fault pool of its own with fresh connections, as every cell had before the
// shared pool.
func newBatch(t *testing.T, super *bun.DB, d backfillDriver, newConnection bool) (*bun.DB, *testpkg.StatementFaults, int64) {
	t.Helper()
	var pool *bun.DB
	var faults *testpkg.StatementFaults
	if cells, ok := faultCells.Load(super); ok && !newConnection {
		pool, faults = testpkg.NewStatementFaultCell(t, cells.(*testpkg.StatementFaultCells))
	} else {
		pool, faults = testpkg.NewStatementFaultPool(t)
	}
	tenantID, _ := testpkg.CreateTestTenant(t, super)
	d.seed(t, super, tenantID)
	return pool, faults, tenantID
}

// expectedDecision is the decision on the direct result of a failed step.
func expectedDecision(d backfillDriver, kind string) outcome {
	switch kind {
	case none:
		return outcomeSuccess
	case "40P01", "40001", "55P03":
		return outcomeRetry
	case "23505":
		if d.owner {
			return outcomeRestart
		}
	}
	return outcomeFail
}

var (
	failureKinds = []string{"", "23514", "57014", "40P01", "40001", "55P03", "23505", "bad_conn"}
	mainKinds    = append([]string{none}, failureKinds...)
	monitorKinds = mainKinds
	minimumKinds = []string{"", "57014", "40P01"}
)

// runPlan arms plan on a cell and a fresh tenant, runs one batch and checks
// the decision and every checkpoint value it left.
func runPlan(t *testing.T, super *bun.DB, d backfillDriver, plan faultPlan, want outcome) {
	t.Helper()
	pool, faults, tenantID := newBatch(t, super, d, plan.breaksConnection())
	seams := driverSeams{}
	failOnFirstCall := func(call int) error {
		if call == 1 {
			return testpkg.InjectedFailure(plan.main)
		}
		return nil
	}
	// armRest arms the main and rollback failures; with a sampler failure it
	// runs only after the sampler failed, so Fired observes that alone.
	armRest := func() {
		if plan.rollback != "" {
			faults.AlsoFail(testpkg.FaultRollback, testpkg.InjectedFailure(plan.rollback))
		}
		if plan.main == none {
			return
		}
		failure := testpkg.InjectedFailure(plan.main)
		switch plan.site {
		case siteBegin:
			faults.AlsoFail(testpkg.FaultBegin, failure)
		case siteCommit:
			faults.AlsoFail(testpkg.FaultCommitDiscarded, failure)
		case siteBackendPID:
			faults.AlsoFailStatement("SELECT pg_backend_pid()", failure)
		case siteCopy, siteBodyLast:
		default:
			prefix := d.prefix[plan.site]
			require.NotEmpty(t, prefix, plan.site)
			faults.AlsoFailStatement(prefix, failure)
		}
	}
	if plan.main != none && plan.site == siteCopy {
		seams.copy = func(_ context.Context, _ bun.Tx, call int) error { return failOnFirstCall(call) }
	}
	if plan.main != none && plan.site == siteBodyLast {
		seams.last = failOnFirstCall
	}
	faults.Arm(testpkg.FaultNone, nil)
	if plan.monitor != none {
		faults.AlsoFailStatement(monitorSamplePrefix, testpkg.InjectedFailure(plan.monitor))
		var armed atomic.Bool
		seams.hooks.afterMonitorStart = func() {
			if armed.Swap(true) {
				return
			}
			waitSamplerFailed(t, faults)
			armRest()
		}
	} else {
		armRest()
	}
	result := d.run(t, pool, super, tenantID, seams)
	if plan.rollback != "" {
		require.True(t, faults.Fired(), "the rollback failure fired")
	}
	assertOutcome(t, d, plan, want, result, d.read(t, super, tenantID))
}

// waitSamplerFailed waits, with a hard deadline, until the sampler failure,
// the only failure armed so far, fired.
func waitSamplerFailed(t *testing.T, faults *testpkg.StatementFaults) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !faults.Fired() {
		if time.Now().After(deadline) {
			t.Fatal("lock sampler did not run within 2s")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// successRow is the checkpoint after one committed batch; restart rewound the
// pass to 0 before it, retried counts the failed attempts.
func successRow(d backfillDriver, restart bool, retried int64, code string) checkpointRow {
	if !d.owner {
		want := requestChildStart
		want.HighWater = 10
		want.Scanned++
		want.Copied += 2
		want.Skipped += 2
		want.Rejected += 2
		want.Completed++
		want.Retried += retried
		want.Stable, want.Complete = false, false
		addCode(&want, code)
		return want
	}
	want := ownerStart
	want.HighWater++
	if restart {
		want.HighWater = 1
	}
	want.Scanned += 3
	want.Copied++
	want.Skipped++
	want.Rejected++
	want.PassWrites++
	want.Completed++
	want.Retried += retried
	want.Stable = false
	addCode(&want, code)
	return want
}

func addCode(row *checkpointRow, code string) {
	switch code {
	case "40P01":
		row.Deadlocks++
	case "40001":
		row.Serialization++
	case "55P03":
		row.LockTimeouts++
	}
}

// failedRow is the checkpoint after a failed tenant: no batch, the flush
// books the pending counters (none here) and withdraws request-child acceptance.
func failedRow(d backfillDriver) checkpointRow {
	if !d.owner {
		want := requestChildStart
		want.Stable, want.Complete = false, false
		return want
	}
	return ownerStart
}

// assertRow compares everything but lock_wait_ms, which grows by the measured
// wait only; no batch of these tests waits on a lock, so it stays within a
// few sampler ticks.
func assertRow(t *testing.T, want, got checkpointRow, label string) {
	t.Helper()
	wait := got.LockWaitMs - want.LockWaitMs
	require.GreaterOrEqual(t, wait, 0.0, label)
	require.Less(t, wait, 100.0, label)
	got.LockWaitMs = want.LockWaitMs
	require.Equal(t, want, got, label)
}

func assertOutcome(t *testing.T, d backfillDriver, plan faultPlan, want outcome, result batchResult, row checkpointRow) {
	t.Helper()
	label := fmt.Sprintf("%s %+v", d.name, plan)
	// Failures before the copy never reach it.
	beforeCopy := plan.main != none && (plan.site == siteBackendPID || plan.site == siteBegin ||
		plan.site == siteLimits || plan.site == siteLimitsLast || (d.owner && plan.site == siteBodyFirst))
	attempts := 2
	if beforeCopy {
		attempts = 1
	}
	switch want {
	case outcomeSuccess:
		require.NoError(t, result.err, label)
		require.Equal(t, 1, result.calls, label)
		assertRow(t, successRow(d, false, 0, ""), row, label)
	case outcomeRetry:
		require.NoError(t, result.err, label)
		require.Equal(t, attempts, result.calls, label)
		assertRow(t, successRow(d, false, 1, plan.main), row, label)
	case outcomeRestart:
		require.NoError(t, result.err, label)
		require.Equal(t, attempts, result.calls, label)
		// The restart rewinds the pass and counts as a retried batch, not as
		// an attempt or a contention failure.
		assertRow(t, successRow(d, true, 1, ""), row, label)
	case outcomeFail:
		require.Error(t, result.err, label)
		require.Equal(t, attempts-1, result.calls, label)
		assertRow(t, failedRow(d), row, label)
	}
}

func label(kind string) string {
	if kind == "" {
		return "nosqlstate"
	}
	return kind
}

// fullCrossSites are the first and last representatives of K1 (after the
// sampler started), K2 and K3: main failure × sampler failure.
func fullCrossSites(d backfillDriver) []string {
	if d.owner {
		return []string{siteBegin, siteLimits, siteBodyFirst, siteBodyLast}
	}
	return []string{siteBegin, siteLimits, siteLimitsLast, siteBodyFirst, siteBodyLast}
}

// Main failure × sampler failure. A sampler failure never decides: with a
// failing main step the main result decides, otherwise the tenant fails.
func checkFullCross(t *testing.T, super *bun.DB) {
	for _, d := range allDrivers {
		for _, site := range fullCrossSites(d) {
			for _, main := range mainKinds {
				for _, monitor := range monitorKinds {
					plan := faultPlan{monitor: monitor, site: site, main: main}
					t.Run(fmt.Sprintf("%s/%s/main=%s/monitor=%s", d.name, site, label(main), label(monitor)), func(t *testing.T) {
						t.Parallel()
						want := expectedDecision(d, main)
						if main == none && monitor != none {
							want = outcomeFail // Variante A
						}
						runPlan(t, super, d, plan, want)
					})
				}
			}
		}
	}
}

// Steps before the sampler starts (K1 first) or after it stopped (K5, K6)
// cannot meet a sampler failure: a sampler failure aborts the attempt before
// the checkpoint UPDATE. They cross the main axis.
func checkMainAxis(t *testing.T, super *bun.DB) {
	for _, d := range allDrivers {
		for _, site := range []string{siteBackendPID, siteCheckpoint, siteCommit} {
			for _, main := range failureKinds {
				if site == siteCommit && main == "bad_conn" {
					continue // not a server rejection: checkUnknownCommit
				}
				t.Run(fmt.Sprintf("%s/%s/main=%s", d.name, site, label(main)), func(t *testing.T) {
					t.Parallel()
					want := expectedDecision(d, main)
					if site == siteCommit && main == "" {
						want = outcomeFail // unknown outcome, never replayed
					}
					runPlan(t, super, d, faultPlan{monitor: none, site: site, main: main}, want)
				})
			}
		}
	}
}

// The remaining body statements get the minimum (Klasse 12), with and
// without a sampler failure: the owners' copy statement and the four
// request-child source statements after the first.
func checkMinimum(t *testing.T, super *bun.DB) {
	for _, d := range allDrivers {
		sites := []string{siteCopy}
		if !d.owner {
			sites = nil
			for i := 1; i < len(requestChildSources); i++ {
				sites = append(sites, requestChildSourceSite(i))
			}
		}
		for _, site := range sites {
			for _, main := range minimumKinds {
				for _, monitor := range []string{none, "40P01"} {
					t.Run(fmt.Sprintf("%s/%s/main=%s/monitor=%s", d.name, site, label(main), label(monitor)), func(t *testing.T) {
						t.Parallel()
						runPlan(t, super, d, faultPlan{monitor: monitor, site: site, main: main}, expectedDecision(d, main))
					})
				}
			}
		}
	}
}

// A failed rollback is a diagnostic: the decision stays on the body failure.
// K3 first and last × every main failure × rollback failure.
func checkRollbackCross(t *testing.T, super *bun.DB) {
	for _, d := range allDrivers {
		for _, site := range []string{siteBodyFirst, siteBodyLast} {
			for _, main := range failureKinds {
				for _, rollback := range []string{"40P01", "bad_conn"} {
					t.Run(fmt.Sprintf("%s/%s/main=%s/rollback=%s", d.name, site, label(main), rollback), func(t *testing.T) {
						t.Parallel()
						plan := faultPlan{monitor: none, site: site, main: main, rollback: rollback}
						runPlan(t, super, d, plan, expectedDecision(d, main))
					})
				}
			}
		}
	}
}

// A real deferred check fails COMMIT with a server ERROR: nothing persisted,
// and the existing decision on that direct result stays.
func checkServerRejectedCommit(t *testing.T, super *bun.DB) {
	for _, d := range allDrivers {
		for _, code := range []string{"40P01", "40001", "55P03", "23505", "23514"} {
			t.Run(d.name+"/"+code, func(t *testing.T) {
				t.Parallel()
				pool, _, tenantID := newBatch(t, super, d, false)
				result := d.run(t, pool, super, tenantID, driverSeams{copy: func(ctx context.Context, tx bun.Tx, call int) error {
					if call > 1 {
						return nil
					}
					_, err := tx.ExecContext(ctx, `INSERT INTO public.commit_probe (code) VALUES (?)`, code)
					return err
				}})
				plan := faultPlan{monitor: none, site: siteCommit, main: code}
				assertOutcome(t, d, plan, expectedDecision(d, code), result, d.read(t, super, tenantID))
			})
		}
	}
}

// holdLockWhileWaiting holds the lock probe row of tenantID until the worker
// that reports its backend pid on pids waits on it (pg_blocking_pids of that
// pid), then for hold more: the wait the sampler measures.
func holdLockWhileWaiting(t *testing.T, super *bun.DB, tenantID int64, pids <-chan int, hold time.Duration) <-chan error {
	t.Helper()
	ctx := t.Context()
	_, err := super.ExecContext(ctx, `INSERT INTO public.lock_probe (id) VALUES (?)`, tenantID)
	require.NoError(t, err)
	tx, err := super.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SELECT id FROM public.lock_probe WHERE id = ? FOR UPDATE`, tenantID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		defer func() { _ = tx.Rollback() }()
		var pid int
		select {
		case pid = <-pids:
		case <-time.After(5 * time.Second):
			done <- fmt.Errorf("worker never reported its pid")
			return
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			var blocked bool
			if err := super.NewRaw(`SELECT cardinality(pg_blocking_pids(?)) > 0`, pid).Scan(ctx, &blocked); err != nil {
				done <- err
				return
			}
			if blocked {
				break
			}
			if time.Now().After(deadline) {
				done <- fmt.Errorf("worker %d never waited on the lock probe", pid)
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(hold) // the measured wait
		done <- nil
	}()
	return done
}

// A COMMIT with unknown outcome is never replayed and never books the
// pending counters again, whether it committed or not. Before it, the batch
// waits 300 ms on a row lock and fails on a real deadlock, serialization
// failure and lock timeout, so every pending counter is non-zero.
func checkUnknownCommit(t *testing.T, super *bun.DB) {
	const hold = 300 * time.Millisecond
	for _, d := range allDrivers {
		for _, applied := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/applied=%v", d.name, applied), func(t *testing.T) {
				t.Parallel()
				// The unknown COMMIT is driver.ErrBadConn: a pool of its own.
				pool, faults, tenantID := newBatch(t, super, d, true)
				pids := make(chan int, 1)
				held := holdLockWhileWaiting(t, super, tenantID, pids, hold)
				codes := []string{"40P01", "40001", "55P03"}
				result := d.run(t, pool, super, tenantID, driverSeams{copy: func(ctx context.Context, tx bun.Tx, call int) error {
					if call == 1 {
						var pid int
						if err := tx.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid); err != nil {
							return err
						}
						pids <- pid
						if _, err := tx.ExecContext(ctx, `SELECT id FROM public.lock_probe WHERE id = ? FOR UPDATE`, tenantID); err != nil {
							return err
						}
					}
					if call <= len(codes) {
						return testpkg.RaiseSQLState(ctx, tx, codes[call-1])
					}
					site := testpkg.FaultCommitDiscarded
					if applied {
						site = testpkg.FaultCommitApplied
					}
					faults.Arm(site, testpkg.InjectedFailure("bad_conn"))
					return nil
				}})
				require.NoError(t, <-held)
				require.Error(t, result.err)
				require.Equal(t, len(codes)+1, result.calls, "no replay after an unknown commit")
				row := d.read(t, super, tenantID)
				want := failedRow(d)
				if applied {
					want = successRow(d, false, int64(len(codes)), "")
					want.Deadlocks++
					want.Serialization++
					want.LockTimeouts++
				}
				wait := row.LockWaitMs - want.LockWaitMs
				if applied {
					// Booked once with the committed batch: about the hold, not twice.
					require.InDelta(t, float64(hold/time.Millisecond), wait, 120, "lock_wait_ms delta")
				} else {
					require.Zero(t, wait, "no lock wait booked without a known commit")
				}
				row.LockWaitMs = want.LockWaitMs
				require.Equal(t, want, row)
				var unknown interface{ CommitOutcomeUnknown() }
				require.ErrorAs(t, result.err, &unknown)
			})
		}
	}
}

// targetRows counts the target rows of an owner backfill in tenantID.
func targetRows(t *testing.T, super *bun.DB, d backfillDriver, tenantID int64) int {
	t.Helper()
	table := map[string]string{
		"staff":    "users.staff_school_memberships",
		"guardian": "users.student_guardian_relationships",
		"student":  "users.student_profiles",
	}[d.name]
	var n int
	require.NoError(t, super.NewRaw(`SELECT count(*) FROM `+table+` WHERE tenant_id = ?`, tenantID).Scan(t.Context(), &n))
	return n
}

// createOrphanTarget creates a target row of tenant whose legacy source row is
// gone, which the rewind of a pass restart must remove.
func createOrphanTarget(t *testing.T, super *bun.DB, d backfillDriver, tenantID int64) {
	t.Helper()
	switch d.name {
	case "staff":
		testpkg.CreateTestStaffForTenant(t, super, tenantID, "Orphan", "Staff")
	case "student":
		testpkg.CreateTestStudentForTenant(t, super, tenantID, "Orphan", "Student", "1a")
	case "guardian":
		student := testpkg.CreateTestStudentForTenant(t, super, tenantID, "Orphan", "Child", "1a")
		guardian := testpkg.CreateTestGuardianProfileForTenant(t, super, tenantID, "Orphan", "Guardian", "")
		testpkg.CreateTestStudentGuardianLinkForTenant(t, super, tenantID, student.ID, guardian.ID, "primary_guardian")
	}
}

// A unique conflict of the copy statement restarts the pass: the rewind
// commits high_water_id 0 and removes the orphaned target row before the
// next attempt, which then copies from the start of the pass.
func checkRewind(t *testing.T, super *bun.DB) {
	for _, d := range allDrivers[:3] {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			pool, _, tenantID := newBatch(t, super, d, false)
			createOrphanTarget(t, super, d, tenantID)
			require.Equal(t, 1, targetRows(t, super, d, tenantID))
			var beforeRetry checkpointRow
			targetsBeforeRetry := -1
			result := d.run(t, pool, super, tenantID, driverSeams{copy: func(_ context.Context, _ bun.Tx, call int) error {
				if call == 1 {
					return testpkg.InjectedFailure("23505")
				}
				beforeRetry = d.read(t, super, tenantID)
				targetsBeforeRetry = targetRows(t, super, d, tenantID)
				return nil
			}})
			require.NoError(t, result.err)
			require.Equal(t, 2, result.calls)
			rewound := ownerStart
			rewound.HighWater = 0
			rewound.Removed++
			rewound.PassWrites++
			assertRow(t, rewound, beforeRetry, "committed rewind before the next attempt")
			require.Zero(t, targetsBeforeRetry, "the rewind removed the orphaned target")
			final := successRow(d, true, 1, "")
			final.Removed++
			final.PassWrites++
			assertRow(t, final, d.read(t, super, tenantID), "after the restarted batch")
		})
	}
}

// The sampler stops once, before the checkpoint UPDATE: every sampler query
// is issued before it and none after; after a sampler failure neither the
// checkpoint UPDATE nor COMMIT runs.
func checkSamplerOrder(t *testing.T, super *bun.DB) {
	for _, d := range allDrivers {
		for _, failing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/samplerFails=%v", d.name, failing), func(t *testing.T) {
				t.Parallel()
				pool, faults, tenantID := newBatch(t, super, d, false)
				counter := testpkg.CaptureQueries(t, pool)
				seams := driverSeams{}
				if failing {
					faults.Arm(testpkg.FaultNone, nil)
					faults.AlsoFailStatement(monitorSamplePrefix, testpkg.InjectedFailure("40P01"))
					seams.hooks.afterMonitorStart = func() { waitSamplerFailed(t, faults) }
				} else {
					seams.hooks.afterMonitorStart = func() { waitSampled(t, counter) }
				}
				result := d.run(t, pool, super, tenantID, seams)
				lastSample, update, commit := -1, -1, -1
				for i, q := range counter.Queries() {
					q = strings.TrimSpace(q)
					switch {
					case strings.HasPrefix(q, monitorSamplePrefix):
						lastSample = i
					case strings.HasPrefix(q, d.prefix[siteCheckpoint]) &&
						(strings.Contains(q, "high_water_id = ") || strings.Contains(q, "high_water_mark = ")):
						update = i // the batch's checkpoint UPDATE, not the failure flush
					case q == "COMMIT":
						commit = i
					}
				}
				require.GreaterOrEqual(t, lastSample, 0, "the sampler ran")
				if failing {
					require.Error(t, result.err)
					require.Equal(t, -1, update, "no checkpoint UPDATE after a sampler failure")
					require.Equal(t, -1, commit, "no COMMIT after a sampler failure")
					assertRow(t, failedRow(d), d.read(t, super, tenantID), d.name)
					return
				}
				require.NoError(t, result.err)
				require.Greater(t, update, lastSample, "the sampler stopped before the checkpoint UPDATE")
				require.Greater(t, commit, update)
				assertRow(t, successRow(d, false, 0, ""), d.read(t, super, tenantID), d.name)
			})
		}
	}
}

// waitSampled waits until the sampler issued a query, with a hard deadline.
func waitSampled(t *testing.T, counter *testpkg.QueryCounter) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		for _, q := range counter.Queries() {
			if strings.HasPrefix(strings.TrimSpace(q), monitorSamplePrefix) {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("lock sampler did not run within 2s")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// backfillAttemptChecks are further groups on the same clone (other files).
var backfillAttemptChecks = map[string]func(*testing.T, *bun.DB){}

// One isolated clone serves every group: the legacy tables and the probes are
// database-global, and the leaves run in parallel on their own tenants and
// cells of one shared fault pool.
func TestBackfillBatchRetry(t *testing.T) {
	t.Parallel()
	super := backfillTestDB(t)
	faultCells.Store(super, testpkg.NewStatementFaultCells(t))
	t.Cleanup(func() { faultCells.Delete(super) })
	t.Run("FullCross", func(t *testing.T) { checkFullCross(t, super) })
	t.Run("MainAxis", func(t *testing.T) { checkMainAxis(t, super) })
	t.Run("Minimum", func(t *testing.T) { checkMinimum(t, super) })
	t.Run("RollbackCross", func(t *testing.T) { checkRollbackCross(t, super) })
	t.Run("ServerRejectedCommit", func(t *testing.T) { checkServerRejectedCommit(t, super) })
	t.Run("UnknownCommit", func(t *testing.T) { checkUnknownCommit(t, super) })
	t.Run("Rewind", func(t *testing.T) { checkRewind(t, super) })
	t.Run("SamplerOrder", func(t *testing.T) { checkSamplerOrder(t, super) })
	for name, check := range backfillAttemptChecks {
		t.Run(name, func(t *testing.T) { check(t, super) })
	}
}
