package repositories_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Worker's overdue tick reads the day's blocks through this repository
// and warns only about blocks that are still planned (#2746). A block whose
// session started must therefore read as active, not with its planning
// status.
func TestActivityInstancesOfTheDayCarryTheirSessionStatus(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Overdue Room %d", time.Now().UnixNano()))
	day := testpkg.ScheduleDate(2026, time.April, 20)
	planned := testpkg.CreateTestActivityInstance(t, db, day, room.ID, testpkg.ActivityInstanceOpts{
		Title: "Overdue planned", IsSpontaneous: true, StartHHMM: "10:00",
	})
	started := testpkg.CreateTestActivityInstance(t, db, day, room.ID, testpkg.ActivityInstanceOpts{
		Title: "Overdue started", IsSpontaneous: true, StartHHMM: "10:15",
	})
	testpkg.SetActivityInstanceLifecycle(t, ctx, db, started.ID, "active")
	repos, err := repositories.NewTimetableTestRepositories(db)
	require.NoError(t, err)

	rows, err := repos.ActivityInstance.FindByTenantAndDate(ctx, day)

	require.NoError(t, err)
	statuses := make(map[int64]string, len(rows))
	for _, row := range rows {
		statuses[row.ID] = row.Status
		if row.ID == planned.ID {
			assert.Equal(t, room.ID, row.RoomID)
			assert.Equal(t, 10, row.StartTime.Hour())
			assert.Equal(t, day.String(), row.Date.String())
		}
	}
	assert.Equal(t, "planned", statuses[planned.ID])
	assert.Equal(t, "active", statuses[started.ID])
}
