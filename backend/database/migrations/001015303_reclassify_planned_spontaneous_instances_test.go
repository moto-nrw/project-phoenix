package migrations

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #2299: only rows that are BOTH spontaneous and still 'planned' are
// reclassified — those are planning-module blocks that merely lacked an
// offering link. Ad-hoc rows (created at start, so never persisted as
// planned+spontaneous) keep their flag.
func TestReclassifyPlannedSpontaneousInstances(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	room := testpkg.CreateTestRoom(t, db, "Migration-2299-Room")
	date := timezone.NewDate(2026, 9, 1)
	plannedSpont := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		Status:        schedule.InstanceStatusPlanned,
		IsSpontaneous: true,
	})
	activeSpont := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		Status:        schedule.InstanceStatusActive,
		IsSpontaneous: true,
	})
	completedSpont := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		Status:        schedule.InstanceStatusCompleted,
		IsSpontaneous: true,
	})
	plannedNormal := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		Status: schedule.InstanceStatusPlanned,
	})

	require.NoError(t, reclassifyPlannedSpontaneousInstancesUp(ctx, db))

	flags := map[int64]bool{}
	var rows []struct {
		ID            int64 `bun:"id"`
		IsSpontaneous bool  `bun:"is_spontaneous"`
	}
	require.NoError(t, db.NewRaw(`
		SELECT id, is_spontaneous
		FROM schedule.activity_instances
		WHERE id IN (?, ?, ?, ?)
	`, plannedSpont.ID, activeSpont.ID, completedSpont.ID, plannedNormal.ID).Scan(ctx, &rows))
	require.Len(t, rows, 4)
	for _, r := range rows {
		flags[r.ID] = r.IsSpontaneous
	}

	assert.False(t, flags[plannedSpont.ID], "planned spontaneous row must be reclassified as a planned block")
	assert.True(t, flags[activeSpont.ID], "active ad-hoc row keeps its spontaneous origin")
	assert.True(t, flags[completedSpont.ID], "completed ad-hoc row keeps its spontaneous origin")
	assert.False(t, flags[plannedNormal.ID], "already-planned row stays planned")
}
