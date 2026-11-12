//go:build testhooks

package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Contracts of runMonitoredBatch itself; they run on the clone that
// TestBackfillBatchRetry sets up with testpkg.SetupIsolatedTestDB
// (backfillTestDB). A separate file keeps the base-tree counter-check of the
// shared tests compilable without runMonitoredBatch.
func init() {
	backfillAttemptChecks["PanicRollsBack"] = checkPanicRollsBack
	backfillAttemptChecks["SamplerStopsOnce"] = checkSamplerStopsOnce
}

// A panicking body rolls back before the deferred conn.Close, as RunInTx did:
// sql.Conn.Close waits for an open transaction, which would otherwise end only
// with its context (never, for a migration context).
func checkPanicRollsBack(t *testing.T, super *bun.DB) {
	t.Parallel()
	pool, _ := testpkg.NewStatementFaultPool(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	started := time.Now()
	require.PanicsWithValue(t, "body panic", func() {
		_ = runMonitoredBatch(ctx, pool, nil, batchAttemptHooks{}, func(time.Duration) {},
			func(ctx context.Context, tx bun.Tx, _ func() error) error {
				if _, err := tx.ExecContext(ctx, `INSERT INTO public.commit_probe (code) VALUES ('panic')`); err != nil {
					return err
				}
				panic("body panic")
			})
	})
	require.Less(t, time.Since(started), 2*time.Second, "the panic waited for the transaction context")
	var rows int
	require.NoError(t, super.NewRaw(`SELECT count(*) FROM public.commit_probe WHERE code = 'panic'`).Scan(t.Context(), &rows))
	require.Zero(t, rows)
}

// observe stops the sampler and books its wait exactly once, however often
// the body and the runner call it; a failed sampler aborts every later call
// with the same error.
func checkSamplerStopsOnce(t *testing.T, _ *bun.DB) {
	t.Parallel()
	for _, failing := range []bool{false, true} {
		pool, faults := testpkg.NewStatementFaultPool(t)
		hooks := batchAttemptHooks{}
		if failing {
			faults.Arm(testpkg.FaultNone, nil)
			faults.AlsoFailStatement(monitorSamplePrefix, testpkg.InjectedFailure("40P01"))
			hooks.afterMonitorStart = func() { waitSamplerFailed(t, faults) }
		}
		var booked int
		var first, second error
		err := runMonitoredBatch(t.Context(), pool, nil, hooks, func(time.Duration) { booked++ },
			func(_ context.Context, _ bun.Tx, observe func() error) error {
				first, second = observe(), observe()
				return first
			})
		require.Equal(t, 1, booked, "lock wait booked once (failing=%v)", failing)
		require.Equal(t, first, second)
		if failing {
			require.Error(t, first)
			require.Empty(t, batchRetryState(err), "a sampler failure never steers a retry")
		} else {
			require.NoError(t, err)
		}
	}
}
