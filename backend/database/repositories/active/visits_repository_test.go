package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// visitTestData holds test entities created via hermetic fixtures
type visitTestData struct {
	Student1      *users.Student
	Student2      *users.Student
	ActivityGroup int64
	CategoryID    int64
	Room          int64
	ActiveGroup   *active.Group
}

// createVisitTestData creates test fixtures for visit tests
func createVisitTestData(t *testing.T, db *bun.DB) *visitTestData {
	student1 := testpkg.CreateTestStudent(t, db, "Visit", "Student1", "1a")
	student2 := testpkg.CreateTestStudent(t, db, "Visit", "Student2", "1b")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "VisitActivity")
	room := testpkg.CreateTestRoom(t, db, "VisitRoom")

	// Create an active group for visits
	groupRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActiveGroup
	now := time.Now()
	activeGroup := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        ptrtest.Ptr(activityGroup.ID),
		RoomID:         room.ID,
	}
	err := groupRepo.Create(testpkg.Ctx(t), activeGroup)
	require.NoError(t, err)

	return &visitTestData{
		Student1:      student1,
		Student2:      student2,
		ActivityGroup: activityGroup.ID,
		CategoryID:    activityGroup.CategoryID,
		Room:          room.ID,
		ActiveGroup:   activeGroup,
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestVisitRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates visit with valid data", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}

		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)
		assert.NotZero(t, visit.ID)

	})

	t.Run("creates visit with exit time", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		exitTime := now.Add(1 * time.Hour)
		visit := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
			ExitTime:      &exitTime,
		}

		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)
		assert.NotZero(t, visit.ID)
		assert.NotNil(t, visit.ExitTime)

	})

	t.Run("create with empty visit should fail", func(t *testing.T) {
		_, err := repo.RecordVisit(ctx, studentpresence.Visit{})
		assert.Error(t, err)
		assert.EqualError(t, err, "student ID is required")
	})
}

func TestVisitRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds existing visit", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		found, err := repo.FindVisit(ctx, visit.ID)
		require.NoError(t, err)
		assert.Equal(t, visit.ID, found.ID)
		assert.Equal(t, data.Student1.ID, found.StudentID)
	})

	t.Run("returns error for non-existent visit", func(t *testing.T) {
		_, err := repo.FindVisit(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestVisitRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates visit exit time", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		exitTime := now.Add(2 * time.Hour)
		visit.ExitTime = &exitTime
		visit, err = repo.ReviseVisit(ctx, visit)
		require.NoError(t, err)

		found, err := repo.FindVisit(ctx, visit.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.ExitTime)
	})
}

func TestVisitRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing visit", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		err = repo.DeleteVisit(ctx, visit.ID)
		require.NoError(t, err)

		_, err = repo.FindVisit(ctx, visit.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestVisitRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("lists all visits", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		_, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		visits, err := repo.ListVisits(ctx, studentpresence.VisitFilter{})
		require.NoError(t, err)
		assert.NotEmpty(t, visits)
	})
}

func TestVisitRepository_FindActiveVisits(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds only active visits (no exit_time)", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		visits, err := repo.ListVisits(ctx, studentpresence.VisitFilter{OpenOnly: true})
		require.NoError(t, err)

		// All returned visits should be active (no exit_time)
		for _, v := range visits {
			assert.Nil(t, v.ExitTime)
		}

		// Our visit should be in the results
		var found bool
		for _, v := range visits {
			if v.ID == visit.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestVisitRepository_FindActiveByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds active visits for student", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		_, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		visits, err := repo.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{data.Student1.ID}, OpenOnly: true})
		require.NoError(t, err)
		assert.NotEmpty(t, visits)

		// All visits should be for this student and active
		for _, v := range visits {
			assert.Equal(t, data.Student1.ID, v.StudentID)
			assert.Nil(t, v.ExitTime)
		}
	})

	t.Run("returns empty for student with no active visits", func(t *testing.T) {
		data := createVisitTestData(t, db)
		// Student2 has no visits
		visits, err := repo.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{data.Student2.ID}, OpenOnly: true})
		require.NoError(t, err)
		assert.Empty(t, visits)
	})
}

func TestVisitRepository_FindByActiveGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds visits for active group", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		visits, err := repo.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{data.ActiveGroup.ID}})
		require.NoError(t, err)
		assert.NotEmpty(t, visits)

		var found bool
		for _, v := range visits {
			if v.ID == visit.ID {
				found = true
				assert.Equal(t, data.ActiveGroup.ID, v.ActiveGroupID)
				break
			}
		}
		assert.True(t, found)
	})
}

func TestVisitRepository_FindByActiveGroupIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("empty input hits no DB and returns nothing", func(t *testing.T) {
		visits, err := repo.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{}})
		require.NoError(t, err)
		assert.Empty(t, visits)
	})

	t.Run("finds visits across the given active groups in one call", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit1 := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		_, err := repo.RecordVisit(ctx, visit1)
		require.NoError(t, err)

		visit2 := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		_, err = repo.RecordVisit(ctx, visit2)
		require.NoError(t, err)

		// One unknown id in the set must not affect the real matches.
		visits, err := repo.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{data.ActiveGroup.ID, -1}})
		require.NoError(t, err)

		foundStudents := map[int64]bool{}
		for _, v := range visits {
			assert.Equal(t, data.ActiveGroup.ID, v.ActiveGroupID)
			foundStudents[v.StudentID] = true
		}
		assert.True(t, foundStudents[data.Student1.ID])
		assert.True(t, foundStudents[data.Student2.ID])
	})
}

func TestVisitRepository_FindByTimeRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds visits in time range", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
		}
		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		start := now.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)

		visits, err := repo.ListVisits(ctx, studentpresence.VisitFilter{OverlapFrom: &start, OverlapUntil: &end})
		require.NoError(t, err)
		assert.NotEmpty(t, visits)

		var found bool
		for _, v := range visits {
			if v.ID == visit.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

// ============================================================================
// Current Visit Tests
// ============================================================================

func TestVisitRepository_GetCurrentByStudentID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("gets current active visit for student", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		current, err := repo.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{data.Student1.ID}, OpenOnly: true, NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Len(t, current, 1)
		assert.Equal(t, visit.ID, current[0].ID)
		assert.Nil(t, current[0].ExitTime)
	})

	t.Run("returns no rows for student with no current visit", func(t *testing.T) {
		data := createVisitTestData(t, db)
		current, err := repo.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{data.Student2.ID}, OpenOnly: true, NewestFirst: true, Limit: 1})
		require.NoError(t, err)
		require.Empty(t, current)
	})
}

func TestVisitsRepository_GetCurrentByStudentIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("gets current visits for multiple students", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit1 := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit2 := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}

		_, err := repo.RecordVisit(ctx, visit1)
		require.NoError(t, err)
		_, err = repo.RecordVisit(ctx, visit2)
		require.NoError(t, err)

		visits, err := repo.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{data.Student1.ID, data.Student2.ID}, OpenOnly: true, NewestFirst: true, StudentOrder: true})
		require.NoError(t, err)
		require.Len(t, visits, 2)
		assert.ElementsMatch(t, []int64{data.Student1.ID, data.Student2.ID}, []int64{visits[0].StudentID, visits[1].StudentID})
		for _, visit := range visits {
			assert.Nil(t, visit.ExitTime)
		}
	})

	t.Run("returns no rows for empty input", func(t *testing.T) {
		visitMap, err := repo.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{}, OpenOnly: true, NewestFirst: true, StudentOrder: true})
		require.NoError(t, err)
		assert.Empty(t, visitMap)
	})
}

// ============================================================================
// Visit End Tests
// ============================================================================

func TestVisitRepository_EndVisit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("ends active visit", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		closed, err := repo.CloseVisits(ctx, []int64{visit.ID}, time.Now())
		require.Len(t, closed, 1)
		require.NoError(t, err)

		found, err := repo.FindVisit(ctx, visit.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.ExitTime)
	})
}

// ============================================================================
// Cleanup Tests
// ============================================================================

func TestVisitRepository_DeleteExpiredVisits(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes expired visits for student", func(t *testing.T) {
		data := createVisitTestData(t, db)
		// Create an old completed visit using raw SQL to bypass created_at auto-setting
		now := time.Now()
		exitTime := now.Add(-90 * 24 * time.Hour) // 90 days ago
		entryTime := exitTime.Add(-1 * time.Hour)
		createdAt := exitTime.Add(-1 * time.Hour)

		var visitID int64
		err := db.NewRaw(`
			INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at, tenant_id)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			RETURNING id
		`, data.Student1.ID, data.ActiveGroup.ID, entryTime, exitTime, createdAt, now, testpkg.Tenant(t)).
			Scan(ctx, &visitID)
		require.NoError(t, err)

		// Delete visits older than 30 days
		deleted, err := repo.DeleteCompletedVisitsBefore(ctx, data.Student1.ID, now.AddDate(0, 0, -30))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, int64(1))
	})

	t.Run("does not delete active visits", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-60 * 24 * time.Hour), // 60 days ago
		}
		visit, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		// Try to delete - should not delete active visits
		_, err = repo.DeleteCompletedVisitsBefore(ctx, data.Student1.ID, now.AddDate(0, 0, -30))
		require.NoError(t, err)

		// Visit should still exist
		_, err = repo.FindVisit(ctx, visit.ID)
		require.NoError(t, err, "Active visit should not be deleted even if old")
	})
}

// but are not exposed in the VisitRepository interface, so they cannot be
// tested through the interface.

// ============================================================================
// Transfer and Cleanup Tests
// ============================================================================

func TestVisitRepository_TransferVisitsFromRecentSessions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	groupRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActiveGroup
	ctx := testpkg.Ctx(t)

	t.Run("transfers visits from recently ended session", func(t *testing.T) {
		data := createVisitTestData(t, db)
		// Create device for this test
		device := testpkg.CreateTestDevice(t, db, "transfer-test-device")

		// Create old active group with device and end it recently
		now := time.Now()
		oldGroup := &active.Group{
			StartTime:      now.Add(-2 * time.Hour),
			LastActivity:   now.Add(-1 * time.Hour),
			TimeoutMinutes: 30,
			GroupID:        ptrtest.Ptr(data.ActivityGroup),
			DeviceID:       &device.ID,
			RoomID:         data.Room,
		}
		err := groupRepo.Create(ctx, oldGroup)
		require.NoError(t, err)

		// Create visit in old group (still active)
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: oldGroup.ID,
			EntryTime:     now.Add(-1 * time.Hour),
		}
		visit, err = repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		// End the old group within the last hour
		testpkg.EndTestActiveGroup(t, db, testpkg.EndedActiveGroup{GroupID: oldGroup.ID})

		// Create new active group with same device
		newGroup := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        ptrtest.Ptr(data.ActivityGroup),
			DeviceID:       &device.ID,
			RoomID:         data.Room,
		}
		err = groupRepo.Create(ctx, newGroup)
		require.NoError(t, err)

		// Transfer visits
		transferred, err := repo.TransferRecentDeviceVisits(ctx, newGroup.ID, device.ID)
		require.NoError(t, err)
		assert.EqualValues(t, 1, transferred)

		// Verify visit was transferred
		found, err := repo.FindVisit(ctx, visit.ID)
		require.NoError(t, err)
		assert.Equal(t, newGroup.ID, found.ActiveGroupID)
	})

	t.Run("does not transfer from sessions ended more than 1 hour ago", func(t *testing.T) {
		data := createVisitTestData(t, db)
		// Create device for this test
		device := testpkg.CreateTestDevice(t, db, "no-transfer-device")

		// Create old group and end it more than 1 hour ago using raw SQL
		now := time.Now()
		var oldGroupID int64
		err := db.NewRaw(`
			INSERT INTO active.groups (start_time, last_activity, end_time, timeout_minutes, group_id, device_id, room_id, created_at, updated_at, tenant_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			RETURNING id
		`, now.Add(-3*time.Hour), now.Add(-3*time.Hour), now.Add(-2*time.Hour), 30, data.ActivityGroup, device.ID, data.Room, now.Add(-3*time.Hour), now, testpkg.Tenant(t)).
			Scan(ctx, &oldGroupID)
		require.NoError(t, err)

		// Create visit in that old group
		visit := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: oldGroupID,
			EntryTime:     now.Add(-3 * time.Hour),
		}
		_, err = repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		// Create new active group with same device
		newGroup := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        ptrtest.Ptr(data.ActivityGroup),
			DeviceID:       &device.ID,
			RoomID:         data.Room,
		}
		err = groupRepo.Create(ctx, newGroup)
		require.NoError(t, err)

		// Try to transfer - should transfer 0 because old session ended >1h ago
		transferred, err := repo.TransferRecentDeviceVisits(ctx, newGroup.ID, device.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), transferred)
	})
}

func TestVisitRepository_TransferActiveVisitsBetweenGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	groupRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActiveGroup
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)

	now := time.Now()
	oldGroup := &active.Group{
		StartTime:      now.Add(-30 * time.Minute),
		LastActivity:   now.Add(-10 * time.Minute),
		TimeoutMinutes: 30,
		GroupID:        ptrtest.Ptr(data.ActivityGroup),
		RoomID:         data.Room,
	}
	require.NoError(t, groupRepo.Create(ctx, oldGroup))

	newGroup := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        ptrtest.Ptr(data.ActivityGroup),
		RoomID:         data.Room,
	}
	require.NoError(t, groupRepo.Create(ctx, newGroup))

	activeVisit := studentpresence.Visit{
		StudentID:     data.Student1.ID,
		ActiveGroupID: oldGroup.ID,
		EntryTime:     now.Add(-20 * time.Minute),
	}
	activeVisit, err := repo.RecordVisit(ctx, activeVisit)
	require.NoError(t, err)

	exitTime := now.Add(-5 * time.Minute)
	endedVisit := studentpresence.Visit{
		StudentID:     data.Student2.ID,
		ActiveGroupID: oldGroup.ID,
		EntryTime:     now.Add(-25 * time.Minute),
		ExitTime:      &exitTime,
	}
	endedVisit, err = repo.RecordVisit(ctx, endedVisit)
	require.NoError(t, err)

	transferred, err := repo.TransferOpenVisits(ctx, oldGroup.ID, newGroup.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, transferred)

	foundActive, err := repo.FindVisit(ctx, activeVisit.ID)
	require.NoError(t, err)
	assert.Equal(t, newGroup.ID, foundActive.ActiveGroupID)
	assert.Nil(t, foundActive.ExitTime)

	foundEnded, err := repo.FindVisit(ctx, endedVisit.ID)
	require.NoError(t, err)
	assert.Equal(t, oldGroup.ID, foundEnded.ActiveGroupID)
	require.NotNil(t, foundEnded.ExitTime)
	assert.WithinDuration(t, exitTime, *foundEnded.ExitTime, time.Second)
}

func TestVisitRepository_GetVisitRetentionStats(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("gets retention stats for students with expired visits", func(t *testing.T) {
		data := createVisitTestData(t, db)
		// Create a student with privacy consent
		student := testpkg.CreateTestStudent(t, db, "RetentionStats", "Student", "4a")

		// Create privacy consent with short retention using raw SQL
		_, err := db.NewRaw(`
			INSERT INTO users.privacy_consents (student_id, policy_version, accepted, renewal_required, data_retention_days, tenant_id, created_at, updated_at)
			VALUES (?, 'v1.0', true, false, 7, ?, NOW(), NOW())
		`, student.ID, testpkg.Tenant(t)).Exec(ctx)
		require.NoError(t, err)
		defer func() {
			_, _ = db.NewDelete().Table("users.privacy_consents").Where("student_id = ?", student.ID).Exec(ctx)
		}()

		// Create old completed visit using raw SQL
		now := time.Now()
		exitTime := now.Add(-30 * 24 * time.Hour) // 30 days ago
		entryTime := exitTime.Add(-1 * time.Hour)
		createdAt := exitTime.Add(-1 * time.Hour)

		var visitID int64
		err = db.NewRaw(`
			INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at, tenant_id)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			RETURNING id
		`, student.ID, data.ActiveGroup.ID, entryTime, exitTime, createdAt, now, testpkg.Tenant(t)).
			Scan(ctx, &visitID)
		require.NoError(t, err)

		// Get stats
		stats, err := repo.ListVisitRetentionCounts(ctx)
		require.NoError(t, err)

		// Should have stats for our student
		var found bool
		for _, row := range stats {
			if row.StudentID == student.ID {
				found = true
				assert.GreaterOrEqual(t, row.Count, 1)
			}
		}
		require.True(t, found, "retention statistics must include the expired visit")
	})
}

func TestVisitRepository_CountExpiredVisits(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("counts all expired visits", func(t *testing.T) {
		data := createVisitTestData(t, db)
		// Create a student with privacy consent
		student := testpkg.CreateTestStudent(t, db, "ExpiredCount", "Student", "4b")

		// Create privacy consent with short retention using raw SQL
		_, err := db.NewRaw(`
			INSERT INTO users.privacy_consents (student_id, policy_version, accepted, renewal_required, data_retention_days, tenant_id, created_at, updated_at)
			VALUES (?, 'v1.0', true, false, 7, ?, NOW(), NOW())
		`, student.ID, testpkg.Tenant(t)).Exec(ctx)
		require.NoError(t, err)
		defer func() {
			_, _ = db.NewDelete().Table("users.privacy_consents").Where("student_id = ?", student.ID).Exec(ctx)
		}()

		// Create old completed visit
		now := time.Now()
		exitTime := now.Add(-30 * 24 * time.Hour)
		entryTime := exitTime.Add(-1 * time.Hour)
		createdAt := exitTime.Add(-1 * time.Hour)

		var visitID int64
		err = db.NewRaw(`
			INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at, tenant_id)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			RETURNING id
		`, student.ID, data.ActiveGroup.ID, entryTime, exitTime, createdAt, now, testpkg.Tenant(t)).
			Scan(ctx, &visitID)
		require.NoError(t, err)

		// Count expired visits
		count, err := repo.CountExpiredVisits(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(1))
	})
}

// ============================================================================
// GetCurrentByStudentIDWithRoom Tests
// ============================================================================

// ============================================================================
// CountActiveByRoomID Tests
// ============================================================================

func TestVisitRepository_CountActiveByRoomID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("counts active visits in room", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit1 := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit2 := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		_, err := repo.RecordVisit(ctx, visit1)
		require.NoError(t, err)
		_, err = repo.RecordVisit(ctx, visit2)
		require.NoError(t, err)

		count, err := repo.CountOpenVisitsInRoom(ctx, data.Room)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("excludes exited visits", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		exitTime := now.Add(-5 * time.Minute)

		activeVisit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		exitedVisit := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
			ExitTime:      &exitTime,
		}
		_, err := repo.RecordVisit(ctx, activeVisit)
		require.NoError(t, err)
		_, err = repo.RecordVisit(ctx, exitedVisit)
		require.NoError(t, err)

		count, err := repo.CountOpenVisitsInRoom(ctx, data.Room)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("returns zero for room with no visits", func(t *testing.T) {
		emptyRoom := testpkg.CreateTestRoom(t, db, "EmptyCountRoom")

		count, err := repo.CountOpenVisitsInRoom(ctx, emptyRoom.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

// ============================================================================
// CountActiveByGroupID Tests
// ============================================================================

func TestVisitRepository_CountActiveByGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("counts active visits in group", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		_, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		count, err := repo.CountOpenVisitsInGroup(ctx, data.ActiveGroup.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("returns zero for group with no visits", func(t *testing.T) {
		data := createVisitTestData(t, db)
		count, err := repo.CountOpenVisitsInGroup(ctx, data.ActiveGroup.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("excludes exited visits", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		exitTime := now.Add(-5 * time.Minute)

		activeVisit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		exitedVisit := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-20 * time.Minute),
			ExitTime:      &exitTime,
		}
		_, err := repo.RecordVisit(ctx, activeVisit)
		require.NoError(t, err)
		_, err = repo.RecordVisit(ctx, exitedVisit)
		require.NoError(t, err)

		count, err := repo.CountOpenVisitsInGroup(ctx, data.ActiveGroup.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

// ============================================================================
// EndVisitsByActiveGroupIDs Tests
// ============================================================================

func TestVisitRepository_EndVisitsByActiveGroupIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	groupRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActiveGroup
	ctx := testpkg.Ctx(t)

	t.Run("ends all active visits for group IDs", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()

		// Create a second active group
		secondGroup := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        ptrtest.Ptr(data.ActivityGroup),
			RoomID:         data.Room,
		}
		err := groupRepo.Create(ctx, secondGroup)
		require.NoError(t, err)

		// Create visits in both groups (entry_time in past to avoid chk_entry_before_exit with DB now())
		visit1 := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-1 * time.Minute),
		}
		visit2 := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: secondGroup.ID,
			EntryTime:     now.Add(-1 * time.Minute),
		}
		visit1, err = repo.RecordVisit(ctx, visit1)
		require.NoError(t, err)
		visit2, err = repo.RecordVisit(ctx, visit2)
		require.NoError(t, err)

		// End visits for both groups — expect count of 2 affected
		ended, err := repo.CloseGroupVisits(ctx, []int64{data.ActiveGroup.ID, secondGroup.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(2), ended) // affected rows count

		// Verify visits are ended
		found1, err := repo.FindVisit(ctx, visit1.ID)
		require.NoError(t, err)
		assert.NotNil(t, found1.ExitTime)

		found2, err := repo.FindVisit(ctx, visit2.ID)
		require.NoError(t, err)
		assert.NotNil(t, found2.ExitTime)
	})

	t.Run("returns zero for empty group IDs", func(t *testing.T) {
		ended, err := repo.CloseGroupVisits(ctx, []int64{})
		require.NoError(t, err)
		assert.Equal(t, int64(0), ended)
	})

	t.Run("does not end already exited visits", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		exitTime := now.Add(-5 * time.Minute)

		exitedVisit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
			ExitTime:      &exitTime, // exit_time after entry_time
		}
		activeVisit := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-1 * time.Minute), // slightly in the past to avoid clock skew with DB now()
		}
		_, err := repo.RecordVisit(ctx, exitedVisit)
		require.NoError(t, err)
		_, err = repo.RecordVisit(ctx, activeVisit)
		require.NoError(t, err)

		// Should only end the active visit
		ended, err := repo.CloseGroupVisits(ctx, []int64{data.ActiveGroup.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(1), ended) // affected rows count
	})

	t.Run("returns zero when no active visits exist", func(t *testing.T) {
		data := createVisitTestData(t, db)
		ended, err := repo.CloseGroupVisits(ctx, []int64{data.ActiveGroup.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(0), ended)
	})
}

// ============================================================================
// GetCurrentByStudentIDs Deduplication Tests
// ============================================================================

func TestVisitsRepository_GetCurrentByStudentIDs_Deduplication(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deduplicates student IDs in input", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		visit := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		_, err := repo.RecordVisit(ctx, visit)
		require.NoError(t, err)

		// Pass duplicate IDs
		visitMap, err := repo.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{data.Student1.ID, data.Student1.ID, data.Student1.ID}, OpenOnly: true, NewestFirst: true, StudentOrder: true})
		require.NoError(t, err)
		require.Len(t, visitMap, 1)
		assert.Equal(t, data.Student1.ID, visitMap[0].StudentID)
	})
}

// ============================================================================
// ListActiveStudentIDsByRoomID Tests
// ============================================================================

func TestVisitRepository_ListActiveStudentIDsByRoomID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns IDs of currently checked-in students", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		v1 := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-10 * time.Minute),
		}
		v2 := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-5 * time.Minute),
		}
		_, insertErr1 := repo.RecordVisit(ctx, v1)
		require.NoError(t, insertErr1)
		_, insertErr2 := repo.RecordVisit(ctx, v2)
		require.NoError(t, insertErr2)

		ids, err := repo.ListOpenVisitRooms(ctx, data.Room)
		require.NoError(t, err)
		assert.ElementsMatch(t, []studentpresence.OpenVisitRoom{{StudentID: data.Student1.ID, RoomID: data.Room}, {StudentID: data.Student2.ID, RoomID: data.Room}}, ids)
	})

	t.Run("excludes visits whose exit_time is set", func(t *testing.T) {
		data := createVisitTestData(t, db)
		now := time.Now()
		exitTime := now.Add(-2 * time.Minute)
		open := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-10 * time.Minute),
		}
		closed := studentpresence.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
			ExitTime:      &exitTime,
		}
		_, insertErr3 := repo.RecordVisit(ctx, open)
		require.NoError(t, insertErr3)
		_, insertErr4 := repo.RecordVisit(ctx, closed)
		require.NoError(t, insertErr4)

		ids, err := repo.ListOpenVisitRooms(ctx, data.Room)
		require.NoError(t, err)
		assert.Equal(t, []studentpresence.OpenVisitRoom{{StudentID: data.Student1.ID, RoomID: data.Room}}, ids,
			"a visit with exit_time IS NOT NULL must not surface — the student already left")
	})

	t.Run("excludes visits whose group has end_time set", func(t *testing.T) {
		data := createVisitTestData(t, db)
		// New room + a closed active group on it. A visit with exit_time IS NULL
		// should still be hidden because the session itself is over.
		room := testpkg.CreateTestRoom(t, db, "EndedSessionRoom")
		groupRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActiveGroup
		now := time.Now()
		endTime := now.Add(-1 * time.Minute)
		closedGroup := &active.Group{
			StartTime:      now.Add(-1 * time.Hour),
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        ptrtest.Ptr(data.ActivityGroup),
			RoomID:         room.ID,
			EndTime:        &endTime,
		}
		require.NoError(t, groupRepo.Create(ctx, closedGroup))
		v := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: closedGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
		}
		_, insertErr5 := repo.RecordVisit(ctx, v)
		require.NoError(t, insertErr5)

		ids, err := repo.ListOpenVisitRooms(ctx, room.ID)
		require.NoError(t, err)
		assert.Empty(t, ids,
			"a visit attached to a group with end_time IS NOT NULL must not surface — the session is closed")
	})

	t.Run("returns empty for room with no active visits", func(t *testing.T) {
		emptyRoom := testpkg.CreateTestRoom(t, db, "EmptyListRoom")

		ids, err := repo.ListOpenVisitRooms(ctx, emptyRoom.ID)
		require.NoError(t, err)
		assert.Empty(t, ids)
	})

	t.Run("respects tenant scoping", func(t *testing.T) {
		data := createVisitTestData(t, db)
		// A visit is created in tenant 1; querying with tenant 2 context must
		// return zero IDs. This pins the TenantWhere clause in the repo.
		now := time.Now()
		v := studentpresence.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-1 * time.Minute),
		}
		_, insertErr6 := repo.RecordVisit(ctx, v)
		require.NoError(t, insertErr6)

		otherTenant := testpkg.TenantContext(2)
		ids, err := repo.ListOpenVisitRooms(otherTenant, data.Room)
		require.NoError(t, err)
		assert.Empty(t, ids,
			"querying as tenant 2 must not see tenant 1's visits — RLS / tenant filter regression")
	})

	t.Run("aggregates students across multiple active groups in the same room", func(t *testing.T) {
		data := createVisitTestData(t, db)
		// Same room can host more than one concurrent active group. The repo
		// must union students across all of them.
		groupRepo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).ActiveGroup
		now := time.Now()
		secondGroup := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        ptrtest.Ptr(data.ActivityGroup),
			RoomID:         data.Room,
		}
		require.NoError(t, groupRepo.Create(ctx, secondGroup))

		v1 := studentpresence.Visit{StudentID: data.Student1.ID, ActiveGroupID: data.ActiveGroup.ID, EntryTime: now}
		v2 := studentpresence.Visit{StudentID: data.Student2.ID, ActiveGroupID: secondGroup.ID, EntryTime: now}
		_, insertErr7 := repo.RecordVisit(ctx, v1)
		require.NoError(t, insertErr7)
		_, insertErr8 := repo.RecordVisit(ctx, v2)
		require.NoError(t, insertErr8)

		ids, err := repo.ListOpenVisitRooms(ctx, data.Room)
		require.NoError(t, err)
		assert.ElementsMatch(t, []studentpresence.OpenVisitRoom{{StudentID: data.Student1.ID, RoomID: data.Room}, {StudentID: data.Student2.ID, RoomID: data.Room}}, ids)
	})
}

// ============================================================================
// OldestExpiredVisitDate / ExpiredVisitMonthlyCounts Tests
// ============================================================================

// createAcceptedConsentForTenant inserts an accepted privacy consent with the
// given retention window under the supplied tenant.
func createAcceptedConsentForTenant(t *testing.T, db *bun.DB, tenantID, studentID int64, policyVersion string, retentionDays int) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO users.privacy_consents (student_id, policy_version, accepted, renewal_required, data_retention_days, tenant_id, created_at, updated_at)
		VALUES (?, ?, true, false, ?, ?, NOW(), NOW())
	`, studentID, policyVersion, retentionDays, tenantID).Exec(testpkg.TenantContext(tenantID))
	require.NoError(t, err)
}

// createCompletedVisitForTenant inserts a completed visit with an explicit
// created_at so retention-window tests can backdate it past the consent's
// data_retention_days.
func createCompletedVisitForTenant(t *testing.T, db *bun.DB, tenantID, studentID, activeGroupID int64, createdAt time.Time) {
	t.Helper()
	var visitID int64
	err := db.NewRaw(`
		INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at, tenant_id)
		VALUES (?, ?, ?, ?, ?, NOW(), ?)
		RETURNING id
	`, studentID, activeGroupID, createdAt, createdAt.Add(time.Hour), createdAt, tenantID).Scan(testpkg.TenantContext(tenantID), &visitID)
	require.NoError(t, err)
}

func TestVisitRepository_OldestExpiredVisitDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)

	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)

	ctx := testpkg.TenantContext(tenantID)

	t.Run("returns nil when no visit is expired", func(t *testing.T) {
		oldest, err := repo.OldestExpiredVisitDate(ctx)
		require.NoError(t, err)
		assert.Nil(t, oldest)
	})

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Oldest", "Expired", "4a")
	activeGroup := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	createAcceptedConsentForTenant(t, db, tenantID, student.ID, "v1.0", 7)

	oldestCreatedAt := time.Now().Add(-90 * 24 * time.Hour)
	createCompletedVisitForTenant(t, db, tenantID, student.ID, activeGroup.ID, oldestCreatedAt)
	createCompletedVisitForTenant(t, db, tenantID, student.ID, activeGroup.ID, time.Now().Add(-30*24*time.Hour))

	// Another tenant holds an even older expired visit — must not leak in.
	otherStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Foreign", "Expired", "4b")
	otherGroup := testpkg.CreateTestActiveGroupForTenant(t, db, otherTenantID)
	createAcceptedConsentForTenant(t, db, otherTenantID, otherStudent.ID, "v1.0", 7)
	createCompletedVisitForTenant(t, db, otherTenantID, otherStudent.ID, otherGroup.ID, time.Now().Add(-365*24*time.Hour))

	t.Run("returns the tenant's oldest expired visit", func(t *testing.T) {
		oldest, err := repo.OldestExpiredVisitDate(ctx)
		require.NoError(t, err)
		require.NotNil(t, oldest)
		assert.WithinDuration(t, oldestCreatedAt, *oldest, time.Second)
	})
}

func TestVisitRepository_ExpiredVisitMonthlyCounts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPresence(t, db)

	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)

	ctx := testpkg.TenantContext(tenantID)

	t.Run("empty map when no visit is expired", func(t *testing.T) {
		counts, err := repo.ListExpiredVisitMonths(ctx)
		require.NoError(t, err)
		assert.Empty(t, counts)
	})

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Oldest", "Expired", "4a")
	activeGroup := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)
	createAcceptedConsentForTenant(t, db, tenantID, student.ID, "v1.0", 7)

	// Two visits share one month, the third lies in another (60 days apart
	// can never fall into the same calendar month).
	newer := time.Now().Add(-60 * 24 * time.Hour)
	older := time.Now().Add(-120 * 24 * time.Hour)
	createCompletedVisitForTenant(t, db, tenantID, student.ID, activeGroup.ID, newer)
	createCompletedVisitForTenant(t, db, tenantID, student.ID, activeGroup.ID, newer)
	createCompletedVisitForTenant(t, db, tenantID, student.ID, activeGroup.ID, older)

	// Another tenant's expired visit must not be counted.
	otherStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Foreign", "Expired", "4b")
	otherGroup := testpkg.CreateTestActiveGroupForTenant(t, db, otherTenantID)
	createAcceptedConsentForTenant(t, db, otherTenantID, otherStudent.ID, "v1.0", 7)
	createCompletedVisitForTenant(t, db, otherTenantID, otherStudent.ID, otherGroup.ID, newer)

	t.Run("groups the tenant's expired visits by month", func(t *testing.T) {
		counts, err := repo.ListExpiredVisitMonths(ctx)
		require.NoError(t, err)
		require.Len(t, counts, 2)

		var total int64
		for _, row := range counts {
			assert.Regexp(t, `^\d{4}-\d{2}$`, row.Month)
			total += row.Count
		}
		assert.EqualValues(t, 3, total)
	})
}
