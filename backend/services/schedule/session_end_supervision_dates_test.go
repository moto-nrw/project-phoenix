package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Preserve the old active-supervision selection for both entrypoints:
// start <= closing day and (end is NULL or end > closing day).
func TestSessionEndPreservesActiveSupervisionDateSelection(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	for _, bulk := range []bool{false, true} {
		name := "single"
		if bulk {
			name = "bulk"
		}
		t.Run(name, func(t *testing.T) {
			ctx := testpkg.OwnCtx(t)
			day := timezone.NewDate(2026, time.September, 9)
			at := day.BerlinMidnight().Add(30 * time.Minute)
			activity := testpkg.CreateTestActivityGroup(t, db, name)
			room := testpkg.CreateTestRoom(t, db, name)
			group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			_, err := db.NewUpdate().Table("active.groups").Set("start_time = ?", at.Add(-time.Hour)).
				Where("id = ?", group.ID).Exec(ctx)
			require.NoError(t, err)
			zero, tomorrow, yesterday := 0, 1, -1
			cases := []struct {
				name        string
				startOffset int
				endOffset   *int
				active      bool
			}{
				{"open", 0, nil, true},
				{"bounded active", 0, &tomorrow, true},
				{"future", 1, nil, false},
				{"already ended", -1, &zero, false},
				{"history", -2, &yesterday, false},
			}
			var activeIDs []int64
			unchanged := make(map[int64]string)
			for _, scenario := range cases {
				staff := testpkg.CreateTestStaff(t, db, name, scenario.name)
				supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
				var endDate *timezone.Date
				if scenario.endOffset != nil {
					date := day.AddDays(*scenario.endOffset)
					endDate = &date
				}
				_, err := db.NewUpdate().Table("active.group_supervisors").
					Set("start_date = ?", day.AddDays(scenario.startOffset)).Set("end_date = ?", endDate).
					Where("id = ?", supervisor.ID).Exec(ctx)
				require.NoError(t, err)
				if scenario.active {
					activeIDs = append(activeIDs, supervisor.ID)
				} else {
					var snapshot string
					require.NoError(t, db.NewRaw("SELECT row_to_json(s)::text FROM active.group_supervisors AS s WHERE id = ?", supervisor.ID).Scan(ctx, &snapshot))
					unchanged[supervisor.ID] = snapshot
				}
			}
			require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				_, err := module.LockGroup(txCtx, group.ID)
				require.NoError(t, err)
				if bulk {
					result, err := module.EndGroupSessions(txCtx, []int64{group.ID}, at)
					require.NoError(t, err)
					assert.EqualValues(t, len(activeIDs), result.SupervisorsEnded)
				} else {
					result, err := module.EndGroupSession(txCtx, group.ID, at)
					require.NoError(t, err)
					assert.ElementsMatch(t, activeIDs, result.EndedSupervisorIDs)
				}
				return nil
			}))
			for _, id := range activeIDs {
				var endDate timezone.Date
				require.NoError(t, db.NewRaw("SELECT end_date FROM active.group_supervisors WHERE id = ?", id).Scan(ctx, &endDate))
				assert.Equal(t, day, endDate, "all currently active supervisors must end")
			}
			for id, before := range unchanged {
				var after string
				require.NoError(t, db.NewRaw("SELECT row_to_json(s)::text FROM active.group_supervisors AS s WHERE id = ?", id).Scan(ctx, &after))
				assert.Equal(t, before, after, "future and historical supervision rows must not change")
			}
		})
	}
}
