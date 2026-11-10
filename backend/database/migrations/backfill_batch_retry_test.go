//go:build testhooks

package migrations

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The batch loops of the four storage backfills retry and restart only on the
// direct result of the failed step (#6449). The lock sampler and a failed
// rollback are diagnostics, and a COMMIT with unknown outcome is never
// replayed or booked twice. The legacy sources are gone after the Contracts,
// so the tests replace only the copy statements; connection, sampler, BEGIN,
// SET LOCAL, checkpoint, COMMIT, rollback, flush and the retry loops run for
// real on a fault-injecting pool (testpkg.NewStatementFaultPool).

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
// then the main failure at its site. Each failure fires once, so a retried or
// restarted batch succeeds.
type faultPlan struct {
	monitor string // sampler failure kind, or none
	site    string // where the main failure fires
	main    string // main failure kind, or none
}

// Main failure sites.
const (
	siteBackendPID = "pg_backend_pid"
	siteBegin      = "BEGIN"
	siteLimits     = "limits"     // first SET LOCAL
	siteLimitsLast = "limitsLast" // last SET LOCAL (request-child only)
	siteBodyFirst  = "bodyFirst"  // first body statement
	siteCopy       = "copy"       // copy statement (request-child: first body statement)
	siteBodyLast   = "bodyLast"   // last body step before observing the sampler
	siteCheckpoint = "checkpoint" // checkpoint UPDATE
	siteCommit     = "commit"     // COMMIT rejected by the server (FaultCommitDiscarded with SQLSTATE)
)

type batchResult struct {
	err   error
	calls int // copy executions
}

type checkpointRow struct {
	HighWater, BatchesCompleted, RowsScanned, BatchesRetried, Deadlocks, Serialization, LockTimeouts int64
	Complete                                                                                         bool
}

// backfillDriver drives one batch of one backfill through its real loop.
type backfillDriver struct {
	name   string
	owner  bool
	prefix map[string]string // statement prefixes of the sites
	run    func(t *testing.T, pool *bun.DB, tenantID int64, s driverSeams) batchResult
	read   func(t *testing.T, db *bun.DB, tenantID int64) checkpointRow
}

// driverSeams are the test hooks of one batch: copy runs at the copy position
// (its error is the copy statement's result), last at the last body position.
type driverSeams struct {
	hooks batchAttemptHooks
	copy  func(ctx context.Context, tx bun.Tx, call int) error
	last  func(call int) error
}

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

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
		run: func(t *testing.T, pool *bun.DB, tenantID int64, s driverSeams) batchResult {
			var calls atomic.Int32
			copyRows := func(ctx context.Context, tx bun.Tx, dest ...any) error {
				call := int(calls.Add(1))
				if s.copy != nil {
					if err := s.copy(ctx, tx, call); err != nil {
						return err
					}
				}
				return tx.NewRaw(`SELECT 1::bigint, 10::bigint, 0::bigint, 1::bigint`).Scan(ctx, dest...)
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
			require.NoError(t, db.NewRaw(`SELECT high_water_id, batches_completed, rows_scanned, batches_retried,
				deadlocks, serialization_failures, lock_timeouts, stable
				FROM platform.storage_backfill_checkpoints WHERE backfill = ? AND tenant_id = ?`, backfill, tenantID).
				Scan(t.Context(), &row.HighWater, &row.BatchesCompleted, &row.RowsScanned, &row.BatchesRetried,
					&row.Deadlocks, &row.Serialization, &row.LockTimeouts, &row.Complete))
			return row
		},
	}
}

var staffDriver = ownerDriver("staff", StaffOwnerBackfillName,
	func(pool *bun.DB, tenantID int64, copyRows func(context.Context, bun.Tx, ...any) error,
		inject func(context.Context, bun.Tx, int64, int) error, hooks batchAttemptHooks,
	) *staffOwnerTenantRun {
		opts := StaffOwnerBackfillOptions{MaxAttempts: 3, Logger: discardLogger(), copyRows: copyRows,
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
		opts := GuardianOwnerBackfillOptions{MaxAttempts: 3, Logger: discardLogger(), copyRows: copyRows,
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
		opts := StudentOwnerBackfillOptions{MaxAttempts: 3, Logger: discardLogger(), copyRows: copyRows,
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
	prefix: map[string]string{
		siteLimits:     "SET LOCAL statement_timeout = '60s'",
		siteLimitsLast: "SET LOCAL lock_timeout",
		siteCheckpoint: "UPDATE enrollment.request_child_storage_backfill_checkpoints SET",
	},
	run: func(t *testing.T, pool *bun.DB, tenantID int64, s driverSeams) batchResult {
		var calls atomic.Int32
		options := RequestChildStorageBackfillOptions{MaxRetries: 3, Logger: discardLogger(), attemptHooks: s.hooks,
			applySource: func(ctx context.Context, tx bun.Tx, _ *RequestChildStorageBatch) (requestChildSourceCounts, error) {
				call := int(calls.Add(1))
				if s.copy != nil {
					if err := s.copy(ctx, tx, call); err != nil {
						return requestChildSourceCounts{}, err
					}
				}
				var scanned int64
				err := tx.NewRaw(`SELECT 1::bigint`).Scan(ctx, &scanned)
				return requestChildSourceCounts{scannedSource: scanned, insertedBookings: 1}, err
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
		err := run.commitBatch(t.Context(), &RequestChildStorageBatch{IDs: []int64{10}, HighWaterMark: 10})
		return batchResult{err: err, calls: int(calls.Load())}
	},
	read: func(t *testing.T, db *bun.DB, tenantID int64) checkpointRow {
		var row checkpointRow
		require.NoError(t, db.NewRaw(`SELECT high_water_mark, batches_completed, rows_scanned, batches_retried,
			deadlocks, serialization_failures, lock_timeouts, complete
			FROM enrollment.request_child_storage_backfill_checkpoints WHERE tenant_id = ?`, tenantID).
			Scan(t.Context(), &row.HighWater, &row.BatchesCompleted, &row.RowsScanned, &row.BatchesRetried,
				&row.Deadlocks, &row.Serialization, &row.LockTimeouts, &row.Complete))
		return row
	},
}

// backfillTestDB is an isolated clone with the empty legacy tables the owner
// rewinds read, and a deferred probe whose rows fail COMMIT with their code.
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
		CREATE FUNCTION public.commit_probe_fail() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'commit probe %', NEW.code USING ERRCODE = NEW.code; END $$;
		CREATE CONSTRAINT TRIGGER commit_probe_fail AFTER INSERT ON public.commit_probe
			DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.commit_probe_fail();`)
	require.NoError(t, err)
	return db
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
	mainKinds    = []string{none, "", "23514", "57014", "40P01", "40001", "55P03", "23505", "bad_conn"}
	monitorKinds = []string{none, "", "23514", "57014", "40P01", "40001", "55P03", "23505", "bad_conn"}
	minimumKinds = []string{"", "57014", "40P01"}
)

// runPlan arms plan on a fresh pool and tenant, runs one batch and checks the
// decision and the checkpoint it left.
func runPlan(t *testing.T, super *bun.DB, d backfillDriver, plan faultPlan, want outcome) {
	t.Helper()
	pool, faults := testpkg.NewStatementFaultPool(t)
	tenantID, _ := testpkg.CreateTestTenant(t, super)
	armMain := func() {
		if plan.main == none {
			return
		}
		failure := testpkg.InjectedFailure(plan.main)
		switch plan.site {
		case siteBegin:
			faults.AlsoFail(testpkg.FaultBegin, failure)
		case siteCommit:
			faults.AlsoFail(testpkg.FaultCommitDiscarded, failure)
		case siteBodyLast, siteCopy:
		default:
			prefix := d.prefix[plan.site]
			if plan.site == siteBackendPID {
				prefix = "SELECT pg_backend_pid()"
			}
			require.NotEmpty(t, prefix, plan.site)
			faults.AlsoFailStatement(prefix, failure)
		}
	}
	seams := driverSeams{}
	if plan.site == siteCopy && plan.main != none {
		seams.copy = func(_ context.Context, _ bun.Tx, call int) error {
			if call == 1 {
				return testpkg.InjectedFailure(plan.main)
			}
			return nil
		}
	}
	if plan.site == siteBodyLast && plan.main != none {
		seams.last = func(call int) error {
			if call == 1 {
				return testpkg.InjectedFailure(plan.main)
			}
			return nil
		}
	}
	if plan.monitor != none {
		faults.Arm(testpkg.FaultNone, nil)
		faults.AlsoFailStatement(monitorSamplePrefix, testpkg.InjectedFailure(plan.monitor))
		var armed atomic.Bool
		seams.hooks.afterMonitorStart = func() {
			if armed.Swap(true) {
				return
			}
			waitFired(t, faults)
			armMain()
		}
	} else {
		armMain()
	}
	result := d.run(t, pool, tenantID, seams)
	row := d.read(t, super, tenantID)
	assertOutcome(t, d, plan, want, result, row)
}

func waitFired(t *testing.T, faults *testpkg.StatementFaults) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !faults.Fired() {
		if time.Now().After(deadline) {
			t.Fatal("lock sampler did not run within 2s")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func assertOutcome(t *testing.T, d backfillDriver, plan faultPlan, want outcome, result batchResult, row checkpointRow) {
	t.Helper()
	label := fmt.Sprintf("%s %+v", d.name, plan)
	// Failures before the copy statement never reach it.
	beforeCopy := plan.main != none && (plan.site == siteBackendPID || plan.site == siteBegin || plan.site == siteLimits ||
		plan.site == siteLimitsLast || (d.owner && plan.site == siteBodyFirst))
	switch want {
	case outcomeSuccess:
		require.NoError(t, result.err, label)
		require.Equal(t, 1, result.calls, label)
		require.Equal(t, checkpointRow{HighWater: 10, BatchesCompleted: 1, RowsScanned: 1}, row, label)
	case outcomeRetry:
		require.NoError(t, result.err, label)
		wantCalls := 2
		if beforeCopy {
			wantCalls = 1
		}
		require.Equal(t, wantCalls, result.calls, label)
		want := checkpointRow{HighWater: 10, BatchesCompleted: 1, RowsScanned: 1, BatchesRetried: 1}
		switch plan.main {
		case "40P01":
			want.Deadlocks = 1
		case "40001":
			want.Serialization = 1
		case "55P03":
			want.LockTimeouts = 1
		}
		require.Equal(t, want, row, label)
	case outcomeRestart:
		require.NoError(t, result.err, label)
		// A restart rewinds the pass (high_water_id 0, committed) and counts
		// as a retried batch, not as an attempt or a contention failure.
		wantCalls := 2
		if beforeCopy {
			wantCalls = 1
		}
		require.Equal(t, wantCalls, result.calls, label)
		require.Equal(t, checkpointRow{HighWater: 10, BatchesCompleted: 1, RowsScanned: 1, BatchesRetried: 1}, row, label)
	case outcomeFail:
		require.Error(t, result.err, label)
		wantCalls := 1
		if beforeCopy {
			wantCalls = 0
		}
		require.Equal(t, wantCalls, result.calls, label)
		require.Equal(t, checkpointRow{}, row, label)
	}
}

// The full cross of the first and last representatives (Klasse 12): main
// failure × sampler failure where the sampler can fail before the step.
func checkRetryDecidesOnDirectResult(t *testing.T, super *bun.DB) {
	for _, d := range []backfillDriver{staffDriver, requestChildDriver} {
		sites := []string{siteBegin, siteLimits, siteBodyLast}
		if d.owner {
			sites = append(sites, siteBodyFirst)
		} else {
			sites = append(sites, siteLimitsLast, siteCopy)
		}
		for _, site := range sites {
			for _, main := range mainKinds {
				for _, monitor := range monitorKinds {
					plan := faultPlan{monitor: monitor, site: site, main: main}
					t.Run(fmt.Sprintf("%s/%s/main=%s/monitor=%s", d.name, site, label(main), label(monitor)), func(t *testing.T) {
						t.Parallel()
						want := expectedDecision(d, main)
						if main == none && monitor != none {
							want = outcomeFail // Variante A: sampler failure aborts without retry
						}
						runPlan(t, super, d, plan, want)
					})
				}
			}
		}
	}
}

// Steps after the sampler stopped cannot meet a sampler failure: it aborts the
// attempt before the checkpoint UPDATE and COMMIT. They cross the main axis.
func checkRetryAfterObservation(t *testing.T, super *bun.DB) {
	for _, d := range []backfillDriver{staffDriver, requestChildDriver} {
		for _, site := range []string{siteBackendPID, siteCheckpoint, siteCommit} {
			for _, main := range mainKinds {
				if site == siteCommit && main == "bad_conn" {
					continue // not a server rejection: checkUnknownCommit
				}
				t.Run(fmt.Sprintf("%s/%s/main=%s", d.name, site, label(main)), func(t *testing.T) {
					t.Parallel()
					want := expectedDecision(d, main)
					if site == siteCommit && main == "" {
						want = outcomeFail
					}
					runPlan(t, super, d, faultPlan{monitor: none, site: site, main: main}, want)
				})
			}
		}
	}
}

// Guardian and student run the same loop through their own code: the
// minimum per class plus every sampler-only failure.
func checkRetryOwnerBindings(t *testing.T, super *bun.DB) {
	for _, d := range []backfillDriver{guardianDriver, studentDriver} {
		for _, site := range []string{siteBackendPID, siteBegin, siteLimits, siteBodyFirst, siteCopy, siteBodyLast, siteCheckpoint, siteCommit} {
			kinds := append([]string{"40001", "55P03", "23505"}, minimumKinds...)
			for _, main := range kinds {
				t.Run(fmt.Sprintf("%s/%s/main=%s", d.name, site, label(main)), func(t *testing.T) {
					t.Parallel()
					want := expectedDecision(d, main)
					runPlan(t, super, d, faultPlan{monitor: none, site: site, main: main}, want)
				})
			}
			t.Run(fmt.Sprintf("%s/%s/main=none/monitor=40P01", d.name, site), func(t *testing.T) {
				t.Parallel()
				runPlan(t, super, d, faultPlan{monitor: "40P01", site: site, main: none}, outcomeFail)
			})
			if site == siteBegin || site == siteBodyLast {
				for _, monitor := range []string{"40P01", "23505"} {
					t.Run(fmt.Sprintf("%s/%s/main=nosqlstate/monitor=%s", d.name, site, monitor), func(t *testing.T) {
						t.Parallel()
						runPlan(t, super, d, faultPlan{monitor: monitor, site: site, main: ""}, outcomeFail)
					})
				}
			}
		}
	}
}

func label(kind string) string {
	if kind == "" {
		return "nosqlstate"
	}
	return kind
}

// A real deferred check fails COMMIT with a server ERROR: nothing persisted,
// and the existing decision on that direct result stays.
func checkServerRejectedCommit(t *testing.T, super *bun.DB) {
	for _, d := range []backfillDriver{staffDriver, guardianDriver, studentDriver, requestChildDriver} {
		for _, code := range []string{"40P01", "40001", "55P03", "23505", "23514"} {
			t.Run(d.name+"/"+code, func(t *testing.T) {
				t.Parallel()
				pool, _ := testpkg.NewStatementFaultPool(t)
				tenantID, _ := testpkg.CreateTestTenant(t, super)
				result := d.run(t, pool, tenantID, driverSeams{copy: func(ctx context.Context, tx bun.Tx, call int) error {
					if call > 1 {
						return nil
					}
					_, err := tx.ExecContext(ctx, `INSERT INTO public.commit_probe (code) VALUES (?)`, code)
					return err
				}})
				want := expectedDecision(d, code)
				row := d.read(t, super, tenantID)
				plan := faultPlan{monitor: none, site: siteCommit, main: code}
				if want == outcomeRetry || want == outcomeRestart {
					require.Equal(t, 2, result.calls)
				}
				assertOutcome(t, d, plan, want, result, row)
			})
		}
	}
}

// A COMMIT with unknown outcome is never replayed and never books the
// pending counters again, whether it committed or not. The batch first
// retries once on a real deadlock, so the pending counters are not zero.
func checkUnknownCommit(t *testing.T, super *bun.DB) {
	for _, d := range []backfillDriver{staffDriver, guardianDriver, studentDriver, requestChildDriver} {
		for _, applied := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/applied=%v", d.name, applied), func(t *testing.T) {
				t.Parallel()
				pool, faults := testpkg.NewStatementFaultPool(t)
				tenantID, _ := testpkg.CreateTestTenant(t, super)
				result := d.run(t, pool, tenantID, driverSeams{copy: func(ctx context.Context, tx bun.Tx, call int) error {
					if call == 1 {
						return testpkg.RaiseSQLState(ctx, tx, "40P01")
					}
					site := testpkg.FaultCommitDiscarded
					if applied {
						site = testpkg.FaultCommitApplied
					}
					faults.Arm(site, testpkg.InjectedFailure("bad_conn"))
					return nil
				}})
				require.Error(t, result.err)
				require.Equal(t, 2, result.calls, "no replay after an unknown commit")
				row := d.read(t, super, tenantID)
				if applied {
					require.Equal(t, checkpointRow{HighWater: 10, BatchesCompleted: 1, RowsScanned: 1, BatchesRetried: 1, Deadlocks: 1}, row)
				} else {
					require.Equal(t, checkpointRow{}, row)
				}
				var unknown interface{ CommitOutcomeUnknown() }
				require.ErrorAs(t, result.err, &unknown)
			})
		}
	}
}

// A failed rollback is a diagnostic: the decision stays on the body failure.
func checkRollbackDiagnostic(t *testing.T, super *bun.DB) {
	for _, d := range []backfillDriver{staffDriver, guardianDriver, studentDriver, requestChildDriver} {
		for _, main := range []string{"57014", "40P01"} {
			for _, rollback := range []string{"40P01", "bad_conn"} {
				t.Run(fmt.Sprintf("%s/main=%s/rollback=%s", d.name, main, rollback), func(t *testing.T) {
					t.Parallel()
					pool, faults := testpkg.NewStatementFaultPool(t)
					tenantID, _ := testpkg.CreateTestTenant(t, super)
					result := d.run(t, pool, tenantID, driverSeams{last: func(call int) error {
						if call > 1 {
							return nil
						}
						faults.Arm(testpkg.FaultRollback, testpkg.InjectedFailure(rollback))
						return testpkg.InjectedFailure(main)
					}})
					require.True(t, faults.Fired())
					plan := faultPlan{monitor: none, site: siteBodyLast, main: main}
					assertOutcome(t, d, plan, expectedDecision(d, main), result, d.read(t, super, tenantID))
				})
			}
		}
	}
}

// The sampler stops once, before the checkpoint UPDATE: after a confirmed
// commit nothing reads it, so a later sampler failure has no effect.
func checkSamplerStopsBeforeCommit(t *testing.T, super *bun.DB) {
	for _, d := range []backfillDriver{staffDriver, guardianDriver, studentDriver, requestChildDriver} {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			pool, faults := testpkg.NewStatementFaultPool(t)
			tenantID, _ := testpkg.CreateTestTenant(t, super)
			result := d.run(t, pool, tenantID, driverSeams{})
			faults.Arm(testpkg.FaultNone, nil)
			faults.AlsoFailStatement(monitorSamplePrefix, testpkg.InjectedFailure("40P01"))
			time.Sleep(50 * time.Millisecond) // five sampler ticks; no sampler may still run
			require.False(t, faults.Fired(), "the sampler kept running after the batch")
			assertOutcome(t, d, faultPlan{monitor: none, main: none}, outcomeSuccess, result, d.read(t, super, tenantID))
		})
	}
}

// The remaining body statement of each owner, the copy statement, gets the
// minimum (Klasse 12), with and without a sampler failure.
func checkRetryCopyStatement(t *testing.T, super *bun.DB) {
	for _, d := range []backfillDriver{staffDriver} {
		for _, main := range minimumKinds {
			for _, monitor := range []string{none, "40P01"} {
				t.Run(fmt.Sprintf("%s/main=%s/monitor=%s", d.name, label(main), label(monitor)), func(t *testing.T) {
					t.Parallel()
					runPlan(t, super, d, faultPlan{monitor: monitor, site: siteCopy, main: main}, expectedDecision(d, main))
				})
			}
		}
	}
}

// A panic must roll back before conn.Close, not wait for context cancellation.
func checkPanicRollsBack(t *testing.T, super *bun.DB) {
	t.Parallel()
	pool, _ := testpkg.NewStatementFaultPool(t)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	const panicValue = "batch body panic"
	require.PanicsWithValue(t, panicValue, func() {
		_ = runMonitoredBatch(ctx, pool, nil, batchAttemptHooks{}, func(time.Duration) {},
			func(ctx context.Context, tx bun.Tx, _ func() error) error {
				_, err := tx.ExecContext(ctx, `INSERT INTO public.commit_probe (code) VALUES (?)`, panicValue)
				require.NoError(t, err)
				panic(panicValue)
			})
	})
	require.NoError(t, ctx.Err(), "panic propagation must not wait for transaction context expiry")
	var rows int
	require.NoError(t, super.NewRaw(`SELECT count(*) FROM public.commit_probe WHERE code = ?`, panicValue).
		Scan(t.Context(), &rows))
	require.Zero(t, rows, "the panicking batch must not persist its writes")
}

// One isolated clone serves every group: the legacy tables and the commit
// probe are database-global, and the leaves run in parallel on their own
// pools and tenants.
func TestBackfillBatchRetry(t *testing.T) {
	t.Parallel()
	super := backfillTestDB(t)
	t.Run("RetryDecidesOnDirectResult", func(t *testing.T) { checkRetryDecidesOnDirectResult(t, super) })
	t.Run("RetryAfterObservation", func(t *testing.T) { checkRetryAfterObservation(t, super) })
	t.Run("RetryOwnerBindings", func(t *testing.T) { checkRetryOwnerBindings(t, super) })
	t.Run("ServerRejectedCommit", func(t *testing.T) { checkServerRejectedCommit(t, super) })
	t.Run("UnknownCommit", func(t *testing.T) { checkUnknownCommit(t, super) })
	t.Run("RollbackDiagnostic", func(t *testing.T) { checkRollbackDiagnostic(t, super) })
	t.Run("PanicRollsBack", func(t *testing.T) { checkPanicRollsBack(t, super) })
	t.Run("SamplerStopsBeforeCommit", func(t *testing.T) { checkSamplerStopsBeforeCommit(t, super) })
	t.Run("RetryCopyStatement", func(t *testing.T) { checkRetryCopyStatement(t, super) })
}
