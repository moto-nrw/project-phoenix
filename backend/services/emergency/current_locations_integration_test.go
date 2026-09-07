package emergency

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmergencyCurrentLocationsSelectLatestRunningSession(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	firstRoom := testpkg.CreateTestRoom(t, db, "VisitRoom")
	secondRoom := testpkg.CreateTestRoom(t, db, "VisitRoomCurrent")
	activity := testpkg.CreateTestActivityGroup(t, db, "VisitActivity")
	firstGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, firstRoom.ID)
	secondGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, secondRoom.ID)
	first := testpkg.CreateTestStudent(t, db, "Visit", "First", "1a")
	second := testpkg.CreateTestStudent(t, db, "Visit", "Second", "1b")
	svc := NewService(Dependencies{Visits: newVisitPresence(t, db), RoomNames: func(_ context.Context, ids []int64) (map[int64]string, error) {
		names := map[int64]string{firstRoom.ID: firstRoom.Name, secondRoom.ID: secondRoom.Name}
		result := make(map[int64]string)
		for _, id := range ids {
			if name, ok := names[id]; ok {
				result[id] = name
			}
		}
		return result, nil
	}})
	locations, err := svc.loadVisitLocations(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, locations)

	oldExit := time.Now().Add(-90 * time.Minute)
	testpkg.CreateTestVisit(t, db, first.ID, firstGroup.ID, time.Now().Add(-2*time.Hour), &oldExit)
	testpkg.CreateTestVisit(t, db, first.ID, secondGroup.ID, time.Now().Add(-10*time.Minute), nil)
	testpkg.CreateTestVisit(t, db, second.ID, firstGroup.ID, time.Now().Add(-20*time.Minute), nil)
	locations, err = svc.loadVisitLocations(ctx, []int64{first.ID, second.ID})
	require.NoError(t, err)
	assert.Equal(t, secondRoom.Name, locations[first.ID])
	assert.Equal(t, firstRoom.Name, locations[second.ID])

	_, err = db.NewUpdate().Table("active.groups").Set("end_time = ?", time.Now()).Where("id = ?", secondGroup.ID).Exec(ctx)
	require.NoError(t, err)
	locations, err = svc.loadVisitLocations(ctx, []int64{first.ID})
	require.NoError(t, err)
	assert.NotContains(t, locations, first.ID, "ended sessions cannot supply a current room")
}
