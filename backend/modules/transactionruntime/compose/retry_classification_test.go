package compose_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// #6316: the runtime decides a retry on the operation's own error only. A
// failed savepoint control (ROLLBACK TO, RELEASE after rollback) is fatal:
// no retry, nothing written (E19). A failure of the outer ROLLBACK is
// diagnostics attached to the operation's error and decides nothing. A
// transaction that never started is not replayed either. The tables are in
// PR #6380, section "Invarianten"; HTTP statuses are measured where a route
// maps these errors (modules/enrollment/http).

var mainKinds = []string{"", "23514", "57014", "40P01", "40001"}
var cleanupKinds = []string{"", "23514", "57014", "40P01", "40001", "bad_conn"}

func kindName(kind string) string {
	if kind == "" {
		return "no_sqlstate"
	}
	return kind
}

// retryOutcome is what a cell observes: whether the unit failed, how often
// the callback ran and how many rows committed.
type retryOutcome struct {
	failed   bool
	attempts int
	rows     int
}

func (o retryOutcome) String() string {
	return fmt.Sprintf("failed=%t attempts=%d rows=%d", o.failed, o.attempts, o.rows)
}

type retryRig struct {
	super  *bun.DB
	faults *testpkg.StatementFaults
	ctx    context.Context
	tenant tenant.TenantID
}

func newRetryRig(t *testing.T) *retryRig {
	t.Helper()
	super := testpkg.SetupIsolatedTestDB(t)
	_, err := super.ExecContext(context.Background(), `CREATE TABLE public.retry_probe (attempt int NOT NULL);
GRANT SELECT, INSERT ON public.retry_probe TO phoenix_tenant, phoenix_admin`)
	require.NoError(t, err)
	pool, faults := testpkg.NewStatementFaultPool(t)
	runtime, err := compose.NewTenantRuntime(pool)
	require.NoError(t, err)
	return &retryRig{
		super:  super,
		faults: faults,
		ctx:    tenant.WithUnitOfWork(context.Background(), runtime),
		tenant: school(t, testpkg.Tenant(t)),
	}
}

func unitTx(ctx context.Context) (bun.Tx, error) {
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return bun.Tx{}, errors.New("no transaction")
	}
	return raw.(bun.Tx), nil
}

// step runs inside the attempt's savepoint; attempt counts from 1.
type step func(ctx context.Context, tx bun.Tx, attempt int) error

// run executes one retrying unit: the callback writes one row, then runs
// inside in a savepoint. The unit's context is ctx (derived from the rig's).
func (r *retryRig) run(t *testing.T, ctx context.Context, admin bool, inside step) (retryOutcome, error) {
	t.Helper()
	_, err := r.super.ExecContext(context.Background(), `TRUNCATE public.retry_probe`)
	require.NoError(t, err)

	var attempts atomic.Int32
	body := func(ctx context.Context) error {
		attempt := int(attempts.Add(1))
		tx, err := unitTx(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO public.retry_probe (attempt) VALUES (?)`, attempt); err != nil {
			return err
		}
		return tenant.WithSavepoint(ctx, func(ctx context.Context) error {
			return inside(ctx, tx, attempt)
		})
	}
	if admin {
		err = tenant.WithinAdminRetry(ctx, body)
	} else {
		err = tenant.WithinTenantRetry(ctx, r.tenant, body)
	}
	var rows int
	require.NoError(t, r.super.NewRaw(`SELECT count(*) FROM public.retry_probe`).Scan(context.Background(), &rows))
	return retryOutcome{failed: err != nil, attempts: int(attempts.Load()), rows: rows}, err
}

// failFirst fails the first failing attempts with a real server error of
// mainKind ("" is an operation error without SQLSTATE).
func failFirst(mainKind string, failing int) step {
	return func(ctx context.Context, tx bun.Tx, attempt int) error {
		if attempt > failing {
			return nil
		}
		if mainKind == "" {
			return errors.New("operation failed without SQLSTATE")
		}
		return testpkg.RaiseSQLState(ctx, tx, mainKind)
	}
}

func deadline(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

var (
	failedOnce   = retryOutcome{failed: true, attempts: 1}
	retriedOnce  = retryOutcome{attempts: 2, rows: 1}
	retriedFully = retryOutcome{failed: true, attempts: 4}
	neverRan     = retryOutcome{failed: true}
)

// expectedCell is table A for a main failure that occurs once.
func expectedCell(mainKind string, site testpkg.FaultSite) retryOutcome {
	switch {
	case site == testpkg.FaultSavepointRestore || site == testpkg.FaultSavepointRelease:
		return failedOnce // A1/A2: savepoint control failed (E19)
	case mainKind == "57014":
		return failedOnce // pgdriver closes the connection: ROLLBACK TO fails for real
	case mainKind == "40P01" || mainKind == "40001":
		return retriedOnce
	default:
		return failedOnce
	}
}

var units = []struct {
	name  string
	admin bool
}{{"tenant", false}, {"admin", true}}

// Table A: operation error once in a savepoint × failed cleanup step.
func TestRetryDecidesOnTheOperationErrorOnly(t *testing.T) {
	t.Parallel()
	rig := newRetryRig(t)
	type cell struct {
		site testpkg.FaultSite
		kind string
	}
	cells := []cell{{site: testpkg.FaultNone}}
	for _, site := range []testpkg.FaultSite{testpkg.FaultSavepointRestore, testpkg.FaultSavepointRelease, testpkg.FaultRollback} {
		for _, kind := range cleanupKinds {
			cells = append(cells, cell{site: site, kind: kind})
		}
	}
	for _, unit := range units {
		for _, mainKind := range mainKinds {
			for _, c := range cells {
				name := fmt.Sprintf("%s/main_%s/%s_%s", unit.name, kindName(mainKind), strings.ReplaceAll(string(c.site), " ", "_"), kindName(c.kind))
				if c.site == testpkg.FaultNone {
					name = fmt.Sprintf("%s/main_%s/cleanup_ok", unit.name, kindName(mainKind))
				}
				t.Run(name, func(t *testing.T) {
					rig.faults.Arm(c.site, testpkg.InjectedFailure(c.kind))
					got, err := rig.run(t, deadline(t, rig.ctx), unit.admin, failFirst(mainKind, 1))
					t.Logf("CELL %s %s", name, got)
					require.Equal(t, expectedCell(mainKind, c.site), got, "error: %v", err)
					if c.site != testpkg.FaultNone && mainKind != "57014" {
						require.True(t, rig.faults.Fired(), "the cleanup fault fired")
					}
				})
			}
		}
	}
}

// A real deadlock or serialization failure with an intact savepoint keeps
// the full retry budget, also when the outer ROLLBACK of a failed attempt
// reports an error of its own.
func TestRetryKeepsItsBudgetDespiteOuterRollbackDiagnostics(t *testing.T) {
	t.Parallel()
	rig := newRetryRig(t)
	for _, unit := range units {
		for _, mainKind := range []string{"40P01", "40001"} {
			for _, kind := range append([]string{"ok"}, cleanupKinds...) {
				name := fmt.Sprintf("%s/main_%s/outer_rollback_%s", unit.name, mainKind, kindName(kind))
				t.Run(name, func(t *testing.T) {
					if kind == "ok" {
						rig.faults.Arm(testpkg.FaultNone, nil)
					} else {
						rig.faults.Arm(testpkg.FaultRollback, testpkg.InjectedFailure(kind))
					}
					got, err := rig.run(t, deadline(t, rig.ctx), unit.admin, failFirst(mainKind, 4))
					t.Logf("CELL %s %s", name, got)
					require.Equal(t, retriedFully, got, "error: %v", err)
				})
			}
		}
	}
}

// Table B: the connection breaks or the request context ends inside a
// savepoint. Nothing is replayed and nothing commits.
func TestRetryAfterConnectionLossOrContextEnd(t *testing.T) {
	t.Parallel()
	rig := newRetryRig(t)
	for _, unit := range units {
		t.Run(unit.name+"/B1_connection_lost_in_savepoint", func(t *testing.T) {
			got, err := rig.run(t, deadline(t, rig.ctx), unit.admin, rig.terminateOnce())
			t.Logf("CELL B1 %s %s", unit.name, got)
			require.Equal(t, failedOnce, got, "error: %v", err)
			require.ErrorIs(t, err, tenant.ErrSavepointRollback, "ROLLBACK TO failed on the dead connection")
		})
		t.Run(unit.name+"/B3_request_cancelled_in_savepoint", func(t *testing.T) {
			ctx, cancel := context.WithCancel(deadline(t, rig.ctx))
			defer cancel()
			got, err := rig.run(t, ctx, unit.admin, func(ctx context.Context, tx bun.Tx, _ int) error {
				cancel()
				_, err := tx.ExecContext(ctx, `SELECT 1`)
				return err
			})
			t.Logf("CELL B3 %s %s", unit.name, got)
			require.Equal(t, failedOnce, got, "error: %v", err)
			require.ErrorIs(t, err, context.Canceled)
		})
		t.Run(unit.name+"/B4_request_deadline_in_savepoint", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(rig.ctx, 300*time.Millisecond)
			defer cancel()
			got, err := rig.run(t, ctx, unit.admin, func(ctx context.Context, tx bun.Tx, _ int) error {
				_, err := tx.ExecContext(ctx, `SELECT pg_sleep(5)`)
				return err
			})
			t.Logf("CELL B4 %s %s", unit.name, got)
			require.Equal(t, failedOnce, got, "error: %v", err)
			// pgdriver reports the expired deadline either as the context's
			// error or as the network read timeout it derived from it.
			require.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded), "deadline in chain: %v", err)
		})
	}
}

// terminateOnce ends the attempt's own backend from the superuser pool in
// the first attempt and then uses the dead connection.
func (r *retryRig) terminateOnce() step {
	return func(ctx context.Context, tx bun.Tx, attempt int) error {
		if attempt > 1 {
			return nil
		}
		var pid int
		if err := tx.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid); err != nil {
			return err
		}
		var terminated bool
		if err := r.super.NewRaw(`SELECT pg_terminate_backend(?, 5000)`, pid).Scan(ctx, &terminated); err != nil {
			return err
		}
		if !terminated {
			return errors.New("backend was not terminated")
		}
		_, err := tx.ExecContext(ctx, `SELECT 1`)
		if err == nil {
			return errors.New("statement on a terminated backend succeeded")
		}
		return err
	}
}

// Table C: COMMIT fails. Only a COMMIT the server reports as a deadlock or
// serialization failure (rolled back) is replayed; a COMMIT with unknown
// outcome is never replayed, whether or not it was applied.
func TestRetryAtCommit(t *testing.T) {
	t.Parallel()
	rig := newRetryRig(t)
	ok := func(context.Context, bun.Tx, int) error { return nil }
	cases := []struct {
		name    string
		site    testpkg.FaultSite
		kind    string
		want    retryOutcome
		unknown bool
	}{
		{"C1_40001_rolled_back", testpkg.FaultCommitDiscarded, "40001", retriedOnce, false},
		{"C1_40P01_rolled_back", testpkg.FaultCommitDiscarded, "40P01", retriedOnce, false},
		{"C2_23505", testpkg.FaultCommitDiscarded, "23505", failedOnce, true},
		{"C3_bad_conn_applied", testpkg.FaultCommitApplied, "bad_conn", retryOutcome{failed: true, attempts: 1, rows: 1}, true},
		{"C4_bad_conn_discarded", testpkg.FaultCommitDiscarded, "bad_conn", failedOnce, true},
	}
	for _, unit := range units {
		for _, c := range cases {
			t.Run(unit.name+"/"+c.name, func(t *testing.T) {
				rig.faults.Arm(c.site, testpkg.InjectedFailure(c.kind))
				got, err := rig.run(t, deadline(t, rig.ctx), unit.admin, ok)
				t.Logf("CELL %s %s %s", c.name, unit.name, got)
				require.True(t, rig.faults.Fired())
				require.Equal(t, c.want, got, "error: %v", err)
				var outcome interface{ CommitOutcomeUnknown() }
				require.Equal(t, c.unknown, errors.As(err, &outcome), "commit outcome unknown: %v", err)
			})
		}
		t.Run(unit.name+"/C5_request_cancelled_before_commit", func(t *testing.T) {
			ctx, cancel := context.WithCancel(deadline(t, rig.ctx))
			defer cancel()
			rig.faults.Arm(testpkg.FaultNone, nil)
			got, err := rig.runThen(t, ctx, unit.admin, cancel)
			t.Logf("CELL C5 %s %s", unit.name, got)
			require.Equal(t, failedOnce, got, "error: %v", err)
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

// runThen runs a unit whose callback writes one row and then calls after.
func (r *retryRig) runThen(t *testing.T, ctx context.Context, admin bool, after func()) (retryOutcome, error) {
	t.Helper()
	return r.run(t, ctx, admin, func(context.Context, bun.Tx, int) error {
		after()
		return nil
	})
}

// Table D: BEGIN fails once. A transaction that never started runs no
// callback and is not replayed, whatever SQLSTATE the start failure carries
// (D1 changed with #6316; D2/D3 unchanged).
func TestRetryNeverReplaysAFailedStart(t *testing.T) {
	t.Parallel()
	rig := newRetryRig(t)
	ok := func(context.Context, bun.Tx, int) error { return nil }
	for _, unit := range units {
		for _, kind := range []string{"40P01", "40001", "bad_conn", "23514", ""} {
			name := fmt.Sprintf("%s/begin_%s", unit.name, kindName(kind))
			t.Run(name, func(t *testing.T) {
				rig.faults.Arm(testpkg.FaultBegin, testpkg.InjectedFailure(kind))
				got, err := rig.run(t, deadline(t, rig.ctx), unit.admin, ok)
				t.Logf("CELL D %s %s", name, got)
				require.True(t, rig.faults.Fired())
				require.Equal(t, neverRan, got, "error: %v", err)
				var notStarted interface{ TransactionNotStarted() }
				require.ErrorAs(t, err, &notStarted)
			})
		}
	}
}

// A failure to create a savepoint or to release it after success is a
// savepoint-control failure, not a failed rollback: the runtime still replays
// the whole transaction after a deadlock or serialization failure there
// (#5827 fault table, unchanged by #6316).
func TestRetryReplaysAFailedSavepointCreateOrRelease(t *testing.T) {
	t.Parallel()
	rig := newRetryRig(t)
	ok := func(context.Context, bun.Tx, int) error { return nil }
	for _, unit := range units {
		for _, kind := range []string{"40P01", "40001"} {
			t.Run(unit.name+"/create_"+kind, func(t *testing.T) {
				rig.faults.Arm(testpkg.FaultNone, nil)
				rig.faults.AlsoFailStatement("SAVEPOINT phoenix_operation", testpkg.InjectedFailure(kind))
				got, err := rig.run(t, deadline(t, rig.ctx), unit.admin, ok)
				require.True(t, rig.faults.Fired())
				require.Equal(t, retriedOnce, got, "error: %v", err)
			})
			t.Run(unit.name+"/release_after_success_"+kind, func(t *testing.T) {
				rig.faults.Arm(testpkg.FaultSavepointRelease, testpkg.InjectedFailure(kind))
				got, err := rig.run(t, deadline(t, rig.ctx), unit.admin, ok)
				require.True(t, rig.faults.Fired())
				require.Equal(t, retriedOnce, got, "error: %v", err)
			})
		}
	}
}
