package compose_test

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Bounded local reader evidence; the same three-child fixture also runs on
// the pre-cutover reader for comparison. This does not claim staging parity.
func TestSlotListRuntime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f := buildMensaFixtureOn(t, db)
	ctx := testpkg.Ctx(t)
	counter := testpkg.CaptureQueriesForContext(t, db)
	var version string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &version))
	t.Logf("slot-runtime environment: go=%s postgres=%s concurrency=1 warmup=5 samples=30 cache=none", runtime.Version(), version)
	samples := make(map[string][]testpkg.RuntimeCheckpointSample)
	plans := make(map[string][]json.RawMessage)
	percentiles := make(map[string]map[string]float64)
	for _, rows := range []int{3, 8} {
		if rows == 8 {
			f.addPlannedPresent(t, 5)
		}
		name := fmt.Sprintf("reconciliation_%d", rows)
		for iteration := range 35 {
			counter.Reset()
			before, started := db.Stats(), time.Now()
			err := testpkg.WithinCurrentTenant(counter.Context(ctx), func(txCtx context.Context) error {
				result, err := f.svc.BuildList(txCtx, classday.Params{Date: classday.Date(listDate.String()), Target: classday.TargetSlots, Source: classday.SourceReconciliation})
				if err != nil {
					return err
				}
				require.Len(t, result.Rows, rows)
				require.Len(t, result.Slots, 1)
				return nil
			})
			elapsed, after := time.Since(started), db.Stats()
			require.NoError(t, err)
			require.Zero(t, counter.WriteRows(), "the projection must never write")
			for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
				require.Empty(t, counter.Operation(operation), "the projection must not issue DML")
			}
			if iteration >= 5 {
				returned, statements := counter.Rows()
				samples[name] = append(samples[name], testpkg.RuntimeCheckpointSample{DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(), RowsAffected: returned, StatementsWithRows: statements, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
			}
		}
		// Explain every captured read on the same restricted tenant role. The
		// plans include actual rows/loops, buffers, scan types and timings; do
		// not infer rows scanned from the driver's rows-returned counter.
		queries := counter.Operation("SELECT")
		require.NotEmpty(t, queries)
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			var bypass bool
			require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
			require.False(t, bypass)
			for _, query := range queries {
				var plan string
				require.NoError(t, tx.NewRaw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+query).Scan(txCtx, &plan))
				plans[name] = append(plans[name], json.RawMessage(plan))
			}
			return nil
		}))
		durations := make([]float64, 0, 30)
		for _, sample := range samples[name] {
			durations = append(durations, sample.DurationMS)
		}
		slices.Sort(durations)
		percentiles[name] = map[string]float64{"p50_ms": durations[14], "p95_ms": durations[28]}
	}
	report, err := json.Marshal(map[string]any{"samples": samples, "percentiles": percentiles, "plans": plans, "cache": "none", "unexpected_errors": 0})
	require.NoError(t, err)
	t.Logf("slot-runtime: %s", report)
}
