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

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Measures owner reads with fixed settings, independently of HTTP and settings
// resolution. The same workload runs against the pre-cutover reader.
func TestEmergencySnapshotRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Runtime", "Supervisor")
	device := testpkg.CreateTestDevice(t, db, "Emergency-runtime")
	room := testpkg.CreateTestRoom(t, db, "Runtime-room")
	activity := testpkg.CreateTestActivityGroup(t, db, "Runtime-activity")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	people, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, Observe: func(peopleCompose.Observation) {}})
	require.NoError(t, err)
	rooms, err := facilitiesCompose.New(facilitiesCompose.Dependencies{DB: db, DeletionLock: func(context.Context) error { return nil }, DeletionGuard: func(context.Context, int64) error { return nil }, Observe: func(facilitiesCompose.Observation) {}})
	require.NoError(t, err)
	query, err := New(Sources{Presence: newPresenceModule(t, db), PresenceMode: fakeMode{mode: "detailed"},
		Students: usersRepo.NewStudentRepository(db), Persons: people, Contacts: usersRepo.NewStudentGuardianRepository(db),
		Rooms: rooms, Settings: &fakeSettings{enabled: true}, Renderer: listexport.NewService()})
	require.NoError(t, err)
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
		name     string
		children int
		pdf      bool
	}{{"empty", 0, false}, {"document-32", 32, false}, {"document-128", 128, false}, {"pdf-128", 128, true}} {
		t.Run(scenario.name, func(t *testing.T) {
			for seeded < scenario.children {
				child := testpkg.CreateTestStudent(t, db, "Runtime", fmt.Sprintf("Child%03d", seeded), "3a")
				testpkg.CreateTestAttendance(t, db, child.ID, teacher.Staff.ID, device.ID, time.Now().Add(-time.Hour), nil)
				testpkg.CreateTestVisit(t, db, child.ID, group.ID, time.Now().Add(-time.Hour), nil)
				guardian := testpkg.CreateTestGuardianProfileNamed(t, db, "Runtime", "Guardian", fmt.Sprintf("runtime-guardian-%03d", seeded))
				testpkg.CreateTestStudentGuardianLink(t, db, child.ID, guardian.ID, "parent")
				seeded++
			}
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
				rows := 0
				var pdf []byte
				err := testpkg.WithTenantTx(t, counter.Context(ctx), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
					if scenario.pdf {
						file, err := query.Export(txCtx)
						pdf = file.Data
						return err
					}
					doc, err := query.Document(txCtx, time.Now())
					rows = len(doc.Rows)
					return err
				})
				elapsed := float64(time.Since(start)) / float64(time.Millisecond)
				after := db.Stats()
				require.NoError(t, err)
				if scenario.pdf {
					require.True(t, strings.HasPrefix(string(pdf), "%PDF-"))
				} else {
					require.Equal(t, scenario.children, rows)
				}
				require.Zero(t, counter.WriteRows(), "projection and export must not persist authoritative state")
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
			report, err := json.Marshal(map[string]any{"scenario": scenario.name, "children": scenario.children, "samples": samples, "p50_ms": durations[14], "p95_ms": durations[28], "max_ms": durations[29], "go": runtime.Version(), "postgres": postgres, "warmup": 5, "concurrency": 1, "unexpected_errors": 0, "lock_samples": locks, "deadlocks": deadlocks() - beforeDeadlocks, "cache": "no projection cache; fixed settings ports"})
			require.NoError(t, err)
			t.Logf("emergency-runtime: %s", report)
			if scenario.name == "document-128" {
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
				t.Logf("emergency-plans: %s", encoded)
			}
		})
	}
}
