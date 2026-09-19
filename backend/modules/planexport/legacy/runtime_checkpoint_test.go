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
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Measures the Betreuungsplan export over the real retained instance,
// instance-staff and room sources, independently of HTTP. The same workload
// runs against the pre-cutover services/planexport with the same sources.
func TestPlanExportRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "Runtime-room")
	members := map[int64]*usersModel.Staff{}
	for i := range 4 {
		member := testpkg.CreateTestStaff(t, db, "Runtime", fmt.Sprintf("Staff%d", i))
		members[member.ID] = member
	}
	staffIDs := make([]int64, 0, len(members))
	for id := range members {
		staffIDs = append(staffIDs, id)
	}
	slices.Sort(staffIDs)
	rooms := facadeRooms{facade: newFacilities(t, db)}
	sources := func(renderer planexport.Renderer) Sources {
		return Sources{
			Instances:     scheduleRepo.NewActivityInstanceRepository(db),
			InstanceStaff: scheduleRepo.NewInstanceStaffRepository(db),
			Rooms:         rooms,
			Staff:         fakeStaff{members: members},
			Renderer:      renderer,
		}
	}
	pdfService := New(sources(listexport.NewService()))
	documentService := New(sources(documentProbe{}))
	var postgres string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &postgres))
	counter := testpkg.CaptureQueriesForContext(t, db)
	deadlocks := func() int64 {
		var n int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &n))
		return n
	}
	seeded := 0
	for _, scenario := range []struct {
		name   string
		blocks int
		weeks  int
		pdf    bool
	}{{"empty-week", 0, 1, false}, {"week-20", 20, 1, false}, {"weeks-4x20", 80, 4, false}, {"pdf-4x20", 80, 4, true}} {
		t.Run(scenario.name, func(t *testing.T) {
			for seeded < scenario.blocks {
				// Four spontaneous blocks per weekday with twenty distinct titles
				// per week, so every week prints twenty offering rows.
				date := monday.AddDays(7*(seeded/20) + (seeded%20)/4)
				instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
					Title:     fmt.Sprintf("Block %02d", seeded%20),
					StartHHMM: fmt.Sprintf("%02d:00", 12+seeded%4), EndHHMM: fmt.Sprintf("%02d:00", 13+seeded%4),
				})
				testpkg.CreateTestInstanceStaff(t, db, instance.ID, staffIDs[seeded%len(staffIDs)], testpkg.InstanceStaffOpts{})
				seeded++
			}
			params, err := planexport.ParseParams(monday.String(), monday.AddDays(7*scenario.weeks-3).String(), string(planexport.TemplateByOffering), "", "")
			require.NoError(t, err)
			beforeDeadlocks := deadlocks()
			stopLocks := testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
				var n int
				err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &n)
				return n, err
			})
			var samples []testpkg.RuntimeCheckpointSample
			var durations []float64
			var selected []string
			for i := 0; i < 35; i++ {
				counter.Reset()
				before := db.Stats()
				start := time.Now()
				var file listexport.File
				err := testpkg.WithTenantTx(t, counter.Context(ctx), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
					var err error
					if scenario.pdf {
						file, err = pdfService.ExportBetreuungsplan(txCtx, params)
						return err
					}
					file, err = documentService.ExportBetreuungsplan(txCtx, params)
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
					require.Equal(t, expectedRows(scenario.blocks, scenario.weeks), rows)
				}
				require.Zero(t, counter.WriteRows(), "the plan export must not persist anything")
				if i >= 5 {
					affected, statements := counter.Rows()
					zero := counter.WriteRows()
					samples = append(samples, testpkg.RuntimeCheckpointSample{DurationMS: elapsed, Queries: counter.Total(), RowsAffected: affected, StatementsWithRows: statements, WriteRowsAffected: &zero, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
					durations = append(durations, elapsed)
				}
				selected = counter.Operation("SELECT")
			}
			locks := stopLocks()
			require.Empty(t, locks.Error)
			slices.Sort(durations)
			report, err := json.Marshal(map[string]any{"scenario": scenario.name, "blocks": scenario.blocks, "weeks": scenario.weeks, "samples": samples, "p50_ms": durations[14], "p95_ms": durations[28], "max_ms": durations[29], "go": runtime.Version(), "postgres": postgres, "warmup": 5, "concurrency": 1, "unexpected_errors": 0, "lock_samples": locks, "deadlocks": deadlocks() - beforeDeadlocks, "cache": "no projection cache; fixture-backed staff names"})
			require.NoError(t, err)
			t.Logf("planexport-runtime: %s", report)
			if scenario.name == "weeks-4x20" {
				var plans []json.RawMessage
				require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
					for _, statement := range selected {
						if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement)), "SELECT") {
							continue
						}
						var plan string
						if err := tx.NewRaw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+statement).Scan(txCtx, &plan); err != nil {
							return err
						}
						plans = append(plans, json.RawMessage(plan))
					}
					return nil
				}))
				encoded, err := json.Marshal(plans)
				require.NoError(t, err)
				t.Logf("planexport-plans: %s", encoded)
			}
		})
	}
}

// expectedRows is the printed row count: one group heading per week when
// several weeks print, plus one offering row per distinct block title (20
// per week, the same titles every week) or the explicit empty-week row.
func expectedRows(blocks, weeks int) int {
	if blocks == 0 {
		return 1
	}
	titles := blocks
	if titles > 20 {
		titles = 20
	}
	if weeks == 1 {
		return titles
	}
	return weeks + weeks*titles
}

// documentProbe reports the row count instead of rendering, so the
// document scenarios measure the reads and the layout without the PDF.
type documentProbe struct{}

func (documentProbe) Render(doc listexport.Document, _ listexport.Format, filenameBase string) (listexport.File, error) {
	data, err := json.Marshal(len(doc.Rows))
	if err != nil {
		return listexport.File{}, err
	}
	return listexport.File{Data: data, ContentType: "application/json", Filename: filenameBase + ".json"}, nil
}
