//go:build testhooks

package test_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// This is an accounting regression, not a latency benchmark: run the real
// checkpoint helper and compare its published samples with an independent
// capture of each SAME materialization callback. Never mark the operation
// here: applying the scoped counter's context is the checkpoint's job.
func TestTimetableCheckpointReportsOnlyItsWorkerQueries(t *testing.T) {
	t.Parallel()
	fixtureDB := testpkg.SetupIsolatedTestDB(t)
	// A private pool on this test's same isolated clone keeps fixture resets
	// out of the unscoped reference capture. Live lock sampling is evidence-only.
	runtimeDB := testpkg.SetupClosableTestDB(t)
	t.Cleanup(func() { require.NoError(t, runtimeDB.Close()) })
	database := testpkg.NewDatabase(t, runtimeDB)
	module, err := api.NewTimetableHTTPTestModule(database, api.TimetableHTTPTestOptions{})
	require.NoError(t, err)
	room := testpkg.CreateTestRoom(t, fixtureDB, "Checkpoint accounting")
	templateID := testpkg.CreateRuntimeCheckpointTimetable(t, fixtureDB, room.ID)

	counter := testpkg.CaptureQueriesForContext(t, runtimeDB)
	counter.Stop()
	reference := testpkg.CaptureQueries(t, runtimeDB)
	reference.Stop()
	type operationEvidence struct {
		queries            int
		rowsAffected       int64
		statementsWithRows int
		sql                []string
	}
	var operations []operationEvidence
	run := func(ctx context.Context) (int, int, error) {
		// A real, unmarked observer query on the SAME pool while the helper's
		// counter is enabled must not leak into the operation's scoped sample.
		var observer int
		if err := runtimeDB.NewRaw("SELECT 41 /* checkpoint-unmarked-observer */").Scan(context.Background(), &observer); err != nil {
			return 0, 0, err
		}
		reference.Reset()
		reference.Start()
		result, runErr := module.Materialization.MaterializeForTenant(ctx,
			calendar.NewDate(2026, 9, 7), calendar.NewDate(2026, 9, 7), "scheduler")
		reference.Stop()
		rows, statements := reference.Rows()
		operations = append(operations, operationEvidence{
			queries: reference.Total(), rowsAffected: rows,
			statementsWithRows: statements, sql: reference.Queries(),
		})
		if runErr != nil {
			return 0, 0, runErr
		}
		if result == nil {
			return 0, 0, fmt.Errorf("materialization returned no result")
		}
		return result.InstancesCreated, result.CandidatesSkippedExisting, nil
	}
	metrics := func() string {
		var metric int
		require.NoError(t, runtimeDB.NewRaw("SELECT 42 /* checkpoint-unmarked-metrics */").Scan(context.Background(), &metric))
		return fmt.Sprintf("checkpoint_scope_metric %d\n", metric)
	}

	results := testpkg.MeasureTimetableCheckpoint(t, runtimeDB, fixtureDB, database.UnitOfWork(), counter, templateID, run, metrics, false)
	counter.Stop()
	reference.Stop()
	require.Len(t, results, 2)
	// Each mode has five warmups followed by thirty reported samples. The
	// reference measures the same callbacks, not a separately replayed state.
	require.Len(t, operations, 70)
	for mode, result := range results {
		require.Len(t, result.Samples, 30, result.Name)
		require.Equal(t, "checkpoint_scope_metric 42\n", result.MetricsBefore, result.Name)
		require.Equal(t, "checkpoint_scope_metric 42\n", result.MetricsAfter, result.Name)
		for index, sample := range result.Samples {
			actual := operations[mode*35+index+5]
			require.Positive(t, actual.queries, "real materialization must execute SQL")
			require.Positive(t, actual.rowsAffected, "the reference must see actual driver row results")
			require.Positive(t, actual.statementsWithRows, "the reference must see statements reporting rows")
			if index == 0 {
				t.Logf("%s: same-callback reference queries=%d rows=%d row_statements=%d; published queries=%d rows=%d row_statements=%d",
					result.Name, actual.queries, actual.rowsAffected, actual.statementsWithRows,
					sample.Queries, sample.RowsAffected, sample.StatementsWithRows)
			}
			require.Equal(t, actual.queries, sample.Queries, "%s sample %d must report the callback's real SQL", result.Name, index)
			require.Equal(t, actual.rowsAffected, sample.RowsAffected, "%s sample %d must report the callback's real driver rows", result.Name, index)
			require.Equal(t, actual.statementsWithRows, sample.StatementsWithRows, "%s sample %d must report the callback's row-bearing statements", result.Name, index)
		}
	}
	// The final capture also retains complete SQL, so equal totals cannot
	// accidentally conceal a replaced observer/fixture query.
	require.Equal(t, operations[len(operations)-1].sql, counter.Queries())
	for _, query := range counter.Queries() {
		upper := strings.ToUpper(query)
		require.NotContains(t, upper, "CHECKPOINT-UNMARKED-OBSERVER")
		require.NotContains(t, upper, "CHECKPOINT-UNMARKED-METRICS")
		require.NotContains(t, upper, "PG_STAT_ACTIVITY")
		require.NotContains(t, upper, "PG_STAT_DATABASE")
		require.NotContains(t, upper, "DELETE FROM SCHEDULE.ACTIVITY_INSTANCES")
	}
}
