package timetableblockdisplay

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type planningTrackRow struct {
	ID        int64  `bun:"id,pk,autoincrement"`
	TenantID  int64  `bun:"tenant_id"`
	Name      string `bun:"name"`
	Color     string `bun:"color"`
	SortOrder int    `bun:"sort_order"`
}

func TestListResolvesPlanningTrackThroughTemplate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, scope.TenantID, "Track metadata")
	room := testpkg.CreateTestRoomForTenant(t, db, scope.TenantID, "Track room")
	track := &planningTrackRow{TenantID: scope.TenantID, Name: "Mittag", Color: "#F78C10", SortOrder: 3}
	err := db.NewInsert().Model(track).ModelTableExpr("schedule.planning_tracks").Scan(scope.Context())
	require.NoError(t, err)
	_, err = db.NewUpdate().Table("activities.groups").
		Set("planning_track_id = ?", track.ID).
		Where("tenant_id = ?", scope.TenantID).
		Where("id = ?", group.ID).
		Exec(scope.Context())
	require.NoError(t, err)

	instance := testpkg.CreateTestActivityInstanceForTenant(t, db, scope.TenantID, testpkg.TodayDate().AddDays(1), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &group.ID,
	})
	metadata, err := List(scope.Context(), db, scope.TenantID, []int64{instance.ID})
	require.NoError(t, err)
	meta := metadata[instance.ID]

	require.NotNil(t, meta.PlanningTrackID)
	assert.Equal(t, track.ID, *meta.PlanningTrackID)
	assert.Equal(t, track.Name, meta.PlanningTrackName)
	assert.Equal(t, track.Color, meta.PlanningTrackColor)
	require.NotNil(t, meta.PlanningTrackSortOrder)
	assert.Equal(t, track.SortOrder, *meta.PlanningTrackSortOrder)
}
