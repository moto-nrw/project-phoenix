package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoomSessionHistory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	setActiveGroupTimes := func(groupID int64, start time.Time, end *time.Time) {
		t.Helper()
		_, err := db.NewUpdate().
			TableExpr("active.groups").
			Set("start_time = ?", start).
			Set("last_activity = ?", start).
			Set("end_time = ?", end).
			Where("id = ?", groupID).
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Exec(testpkg.Ctx(t))
		require.NoError(t, err)
	}

	repo, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)

	room := testpkg.CreateTestRoom(t, db, "AggregateRoomSessions")
	otherRoom := testpkg.CreateTestRoom(t, db, "AggregateRoomSessionsOther")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Aggregate Activity")
	staffA := testpkg.CreateTestStaff(t, db, "Ada", "Supervisor")
	staffB := testpkg.CreateTestStaff(t, db, "Bert", "Supervisor")
	studentA := testpkg.CreateTestStudent(t, db, "Aggregate", "StudentA", "1a")
	studentB := testpkg.CreateTestStudent(t, db, "Aggregate", "StudentB", "1a")

	baseTime := time.Date(2026, time.May, 16, 10, 0, 0, 0, time.UTC)
	windowStart := baseTime
	windowEnd := baseTime.Add(2 * time.Hour)

	overlapping := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	running := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	endedBeforeWindow := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, room.ID)
	otherRoomSession := testpkg.CreateTestActiveGroup(t, db, activityGroup.ID, otherRoom.ID)

	overlapStart := baseTime.Add(-2 * time.Hour)
	overlapEnd := baseTime.Add(time.Hour)
	runningStart := baseTime.Add(30 * time.Minute)
	oldStart := baseTime.Add(-4 * time.Hour)
	oldEnd := baseTime.Add(-3 * time.Hour)
	otherRoomStart := baseTime.Add(45 * time.Minute)
	otherRoomEnd := baseTime.Add(90 * time.Minute)

	setActiveGroupTimes(overlapping.ID, overlapStart, &overlapEnd)
	setActiveGroupTimes(running.ID, runningStart, nil)
	setActiveGroupTimes(endedBeforeWindow.ID, oldStart, &oldEnd)
	setActiveGroupTimes(otherRoomSession.ID, otherRoomStart, &otherRoomEnd)

	_ = testpkg.CreateTestGroupSupervisor(t, db, staffA.ID, overlapping.ID, "lead")
	_ = testpkg.CreateTestGroupSupervisor(t, db, staffB.ID, overlapping.ID, "support")

	visitAEnd := windowStart.Add(30 * time.Minute)
	visitBEnd := windowStart.Add(45 * time.Minute)
	duplicateAEnd := overlapStart.Add(55 * time.Minute)
	_ = testpkg.CreateTestVisit(t, db, studentA.ID, overlapping.ID, overlapStart.Add(5*time.Minute), &visitAEnd)
	_ = testpkg.CreateTestVisit(t, db, studentB.ID, overlapping.ID, overlapStart.Add(10*time.Minute), &visitBEnd)
	_ = testpkg.CreateTestVisit(t, db, studentA.ID, overlapping.ID, overlapStart.Add(40*time.Minute), &duplicateAEnd)

	t.Run("returns aggregated sessions active inside the window", func(t *testing.T) {
		rows, err := repo.ListRoomSessionHistory(ctx, room.ID, windowStart, windowEnd, nil)
		require.NoError(t, err)
		require.Len(t, rows, 2)

		assert.Equal(t, running.ID, rows[0].SessionID)
		require.NotNil(t, rows[0].ActivityGroupID)
		assert.Equal(t, activityGroup.ID, *rows[0].ActivityGroupID)
		assert.Nil(t, rows[0].EndedAt)
		assert.Nil(t, rows[0].DurationMinutes)
		assert.Empty(t, rows[0].SupervisorStaffIDs)
		assert.Equal(t, 0, rows[0].StudentCount)

		assert.Equal(t, overlapping.ID, rows[1].SessionID)
		require.NotNil(t, rows[1].ActivityGroupID)
		assert.Equal(t, activityGroup.ID, *rows[1].ActivityGroupID)
		require.NotNil(t, rows[1].EndedAt)
		assert.True(t, rows[1].EndedAt.Equal(overlapEnd))
		require.NotNil(t, rows[1].DurationMinutes)
		assert.Equal(t, 180, *rows[1].DurationMinutes)
		assert.Equal(t, 2, rows[1].StudentCount)
		assert.ElementsMatch(t, []int64{staffA.ID, staffB.ID}, rows[1].SupervisorStaffIDs)
	})

	t.Run("filters to sessions supervised by the supplied staff member", func(t *testing.T) {
		rows, err := repo.ListRoomSessionHistory(ctx, room.ID, windowStart, windowEnd, &staffA.ID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, overlapping.ID, rows[0].SessionID)

		rows, err = repo.ListRoomSessionHistory(ctx, room.ID, windowStart, windowEnd, &staffB.ID)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, overlapping.ID, rows[0].SessionID)
	})

	t.Run("rejects a missing tenant context", func(t *testing.T) {
		_, err := repo.ListRoomSessionHistory(context.Background(), room.ID, windowStart, windowEnd, &staffA.ID)
		require.Error(t, err)
	})
}

func TestRoomSessionHistory_ReturnsDatabaseError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupClosableTestDB(t)
	repo, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = repo.ListRoomSessionHistory(
		testpkg.Ctx(t),
		time.Now().UnixNano(),
		time.Now().Add(-time.Hour),
		time.Now(),
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "room session history")
}
