package api

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	transactionFailures "github.com/moto-ogs/project-phoenix/modules/transactionruntime"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The connectivity retry of DecideAndInvite on the real tenant runtime
// (#6316, PR #6380 tables A, B, C, D): it decides on the operation's own
// error, never on a failed savepoint control or an attached ROLLBACK failure,
// and never replays a decision whose body ran.

type decideProbe struct {
	probe *testpkg.FaultProbe
	ctx   context.Context
	calls int
}

func newDecideProbe(t *testing.T) *decideProbe {
	t.Helper()
	probe := testpkg.NewFaultProbe(t)
	id, err := tenant.NewTenantID(testpkg.Tenant(t))
	require.NoError(t, err)
	ctx := tenant.WithTenant(tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, probe.Pool)), id)
	return &decideProbe{probe: probe, ctx: ctx}
}

// decisions composes the decision command as the root does: the tenant
// runtime binding of Enrollment's composition and the root's transient
// classifier (roots/owners/root_enrollment_services.go).
func (p *decideProbe) decisions() *enrollmentCompose.Decisions {
	return enrollmentCompose.NewDecisions(enrollmentCompose.DecisionDependencies{
		IsTransient:         transactionFailures.IsTransientDatabaseError,
		IsSavepointRollback: func(err error) bool { return errors.Is(err, tenant.ErrSavepointRollback) },
		Settings:            unreadDecisionSettings{},
		Logger:              slog.Default(),
	})
}

// unreadDecisionSettings satisfies the composition's settings requirement;
// the unit and retry under test read no setting, so any call panics.
type unreadDecisionSettings struct {
	enrollmentCompose.DecisionSettings
}

// run decides once through decideAndInvite; the body writes a probe row and
// then runs script in its first call only.
func (p *decideProbe) run(t *testing.T, script func(ctx context.Context, tx bun.Tx) error) (int, int, error) {
	t.Helper()
	p.probe.Reset(t)
	p.calls = 0
	ctx, cancel := context.WithTimeout(p.ctx, 20*time.Second)
	defer cancel()
	decide := func(ctx context.Context, _ enrollmentNative.DecideInput) (*enrollmentNative.DecideOutcome, error) {
		p.calls++
		raw, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("no transaction")
		}
		tx := raw.(bun.Tx)
		if err := p.probe.Write(ctx, tx, p.calls); err != nil {
			return nil, err
		}
		if p.calls == 1 && script != nil {
			if err := script(ctx, tx); err != nil {
				return nil, err
			}
		}
		return &enrollmentNative.DecideOutcome{Child: &enrollmentNative.RequestChild{ID: 99, Status: enrollmentNative.ChildStatusApproved}}, nil
	}
	input := enrollmentNative.DecideInput{RequestID: 1234, ChildID: 99, Status: enrollmentNative.DecisionApproved, ReviewedBy: 7}
	_, err := p.decisions().DecideAndInviteWith(ctx, input, decide)
	require.NoError(t, ctx.Err(), "the decision ended within its deadline")
	return p.calls, p.probe.Rows(t), err
}

func TestDecideAndInviteRetriesOnTheOperationErrorOnly(t *testing.T) {
	t.Parallel()
	p := newDecideProbe(t)
	inSavepoint := func(step func(ctx context.Context, tx bun.Tx) error) func(context.Context, bun.Tx) error {
		return func(ctx context.Context, tx bun.Tx) error {
			return tenant.WithSavepoint(ctx, func(ctx context.Context) error { return step(ctx, tx) })
		}
	}
	terminate := func(ctx context.Context, tx bun.Tx) error { return p.probe.TerminateBackend(ctx, tx) }
	brokenStatement := func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, testpkg.ProbeStatement)
		return err
	}
	failsPlainly := func(context.Context, bun.Tx) error { return errors.New("operation failed without SQLSTATE") }

	cases := []struct {
		name          string
		site          testpkg.FaultSite
		kind          string
		script        func(context.Context, bun.Tx) error
		calls, rows   int
		failed        bool
		savepointLost bool
	}{
		// B1: the connection dies inside a savepoint: E19, no replay.
		{name: "B1_connection_lost_in_savepoint", script: inSavepoint(terminate), calls: 1, failed: true, savepointLost: true},
		// B2: the driver reports a broken connection outside a savepoint
		// before the body completed: one replay, unchanged.
		{name: "B2_bad_conn_outside_savepoint", site: testpkg.FaultProbeStatement, kind: "bad_conn", script: brokenStatement, calls: 2, rows: 1},
		// B2b: the backend dies outside a savepoint; the statement fails
		// with FATAL 57P01, only the attached ROLLBACK reports ErrBadConn.
		{name: "B2b_backend_terminated_outside_savepoint", script: terminate, calls: 1, failed: true},
		// A3: an operation error without SQLSTATE, the outer ROLLBACK fails
		// with ErrBadConn: diagnostics only.
		{name: "A3_outer_rollback_bad_conn", site: testpkg.FaultRollback, kind: "bad_conn", script: failsPlainly, calls: 1, failed: true},
		// A1: the savepoint rollback fails with ErrBadConn: E19.
		{name: "A1_savepoint_rollback_bad_conn", site: testpkg.FaultSavepointRestore, kind: "bad_conn", script: inSavepoint(failsPlainly), calls: 1, failed: true, savepointLost: true},
		// D2: BEGIN fails with ErrBadConn: the body never ran, one replay,
		// unchanged (outside Befundklasse 4).
		{name: "D2_begin_bad_conn", site: testpkg.FaultBegin, kind: "bad_conn", calls: 1, rows: 1},
		// C3: COMMIT applied but reported ErrBadConn: the body ran, no replay.
		{name: "C3_commit_applied_bad_conn", site: testpkg.FaultCommitApplied, kind: "bad_conn", calls: 1, rows: 1, failed: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p.probe.Faults.Arm(c.site, testpkg.InjectedFailure(c.kind))
			calls, rows, err := p.run(t, c.script)
			t.Logf("CELL %s calls=%d rows=%d err=%v", c.name, calls, rows, err)
			if c.site != testpkg.FaultNone {
				require.True(t, p.probe.Faults.Fired(), "the fault fired")
			}
			require.Equal(t, c.failed, err != nil, "error: %v", err)
			require.Equal(t, c.calls, calls, "decision bodies run")
			require.Equal(t, c.rows, rows, "committed rows")
			require.Equal(t, c.savepointLost, errors.Is(err, tenant.ErrSavepointRollback))
		})
	}
}
