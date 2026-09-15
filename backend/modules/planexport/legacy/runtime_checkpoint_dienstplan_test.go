package legacy

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Measures the Dienstplan export over the retained staff schedule overview,
// independently of HTTP. The overview's shift and staff reads are Workforce
// adapters composed only by the legacy repository factory, so they are
// reconstructed over the tenant transaction (txOverviewReads); the instance,
// assignment and room reads are the real retained sources.
func TestDienstplanRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "Runtime-room")
	staffIDs := make([]int64, 0, 4)
	for i := range 4 {
		staffIDs = append(staffIDs, testpkg.CreateTestStaff(t, db, "Runtime", fmt.Sprintf("Staff%d", i)).ID)
	}
	rooms := facadeRooms{facade: newFacilities(t, db)}
	export := func(txCtx context.Context, tx bun.Tx, renderer planexport.Renderer, params planexport.Params) (listexport.File, error) {
		reads := txOverviewReads{tx: tx}
		service := New(Sources{
			Overview: scheduleSvc.NewStaffScheduleOverviewService(scheduleSvc.StaffScheduleOverviewDependencies{
				Shifts:        reads,
				Instances:     scheduleRepo.NewActivityInstanceRepository(db),
				InstanceStaff: scheduleRepo.NewInstanceStaffRepository(db),
				Rooms:         rooms,
				Staff:         reads,
			}),
			Renderer: renderer,
		})
		return service.ExportDienstplan(txCtx, params)
	}
	var postgres string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &postgres))
	counter := testpkg.CaptureQueriesForContext(t, db)
	deadlocks := func() int64 {
		var n int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &n))
		return n
	}
	seededWeeks := 0
	for _, scenario := range []struct {
		name  string
		weeks int
		pdf   bool
	}{{"empty-week", 0, false}, {"week", 1, false}, {"weeks-4", 4, false}, {"pdf-4", 4, true}} {
		t.Run(scenario.name, func(t *testing.T) {
			for seededWeeks < scenario.weeks {
				// Every staff member works every weekday and supervises one
				// block per day: 20 shifts and 20 blocks per printed week.
				for weekday := range 5 {
					date := monday.AddDays(7*seededWeeks + weekday)
					for i, staffID := range staffIDs {
						testpkg.CreateTestStaffShift(t, db, staffID, date, testpkg.StaffShiftOpts{})
						instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
							Title:     fmt.Sprintf("Block %d", i),
							StartHHMM: fmt.Sprintf("%02d:00", 12+i), EndHHMM: fmt.Sprintf("%02d:00", 13+i),
						})
						testpkg.CreateTestInstanceStaff(t, db, instance.ID, staffID, testpkg.InstanceStaffOpts{})
					}
				}
				seededWeeks++
			}
			weeks := max(scenario.weeks, 1)
			params, err := planexport.ParseParams(monday.String(), monday.AddDays(7*weeks-3).String(), string(planexport.TemplateByPerson), "", "")
			require.NoError(t, err)
			beforeDeadlocks := deadlocks()
			stopLocks := testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
				var n int
				err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &n)
				return n, err
			})
			var samples []testpkg.RuntimeCheckpointSample
			var durations []float64
			expectedRows := -1
			for i := range 35 {
				counter.Reset()
				before := db.Stats()
				start := time.Now()
				var file listexport.File
				err := testpkg.WithTenantTx(t, counter.Context(ctx), db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
					var err error
					if scenario.pdf {
						file, err = export(txCtx, tx, listexport.NewService(), params)
						return err
					}
					file, err = export(txCtx, tx, documentProbe{}, params)
					return err
				})
				elapsed := float64(time.Since(start)) / float64(time.Millisecond)
				after := db.Stats()
				require.NoError(t, err)
				if scenario.pdf {
					require.True(t, strings.HasPrefix(string(file.Data), "%PDF-"))
				} else {
					var rows int
					require.NoError(t, json.Unmarshal(file.Data, &rows))
					if expectedRows < 0 {
						expectedRows = rows
						if scenario.weeks == 0 {
							require.Equal(t, 1, rows, "an empty week prints its explicit empty row")
						} else {
							require.GreaterOrEqual(t, rows, len(staffIDs)*scenario.weeks, "every staff member prints in every week")
						}
					}
					require.Equal(t, expectedRows, rows, "the row count is stable across samples")
				}
				require.Zero(t, counter.WriteRows(), "the plan export must not persist anything")
				if i >= 5 {
					affected, statements := counter.Rows()
					zero := counter.WriteRows()
					samples = append(samples, testpkg.RuntimeCheckpointSample{DurationMS: elapsed, Queries: counter.Total(), RowsAffected: affected, StatementsWithRows: statements, WriteRowsAffected: &zero, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
					durations = append(durations, elapsed)
				}
			}
			locks := stopLocks()
			require.Empty(t, locks.Error)
			slices.Sort(durations)
			report, err := json.Marshal(map[string]any{"scenario": scenario.name, "weeks": scenario.weeks, "shifts": 20 * scenario.weeks, "blocks": 20 * scenario.weeks, "samples": samples, "p50_ms": durations[14], "p95_ms": durations[28], "max_ms": durations[29], "go": runtime.Version(), "postgres": postgres, "warmup": 5, "concurrency": 1, "unexpected_errors": 0, "lock_samples": locks, "deadlocks": deadlocks() - beforeDeadlocks, "cache": "no projection cache; shift and staff reads reconstructed over the tenant transaction"})
			require.NoError(t, err)
			t.Logf("dienstplan-runtime: %s", report)
		})
	}
}
