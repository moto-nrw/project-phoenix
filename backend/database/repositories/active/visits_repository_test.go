package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
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
	groupRepo := repositories.NewFactory(db).ActiveGroup
	now := time.Now()
	activeGroup := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        base.Int64Ptr(activityGroup.ID),
		RoomID:         room.ID,
	}
	err := groupRepo.Create(testpkg.TenantContext(1), activeGroup)
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

// cleanupVisitTestData removes test data
func cleanupVisitTestData(t *testing.T, db *bun.DB, data *visitTestData) {
	cleanupActiveGroupRecords(t, db, data.ActiveGroup.ID)
	testpkg.CleanupActivityFixtures(t, db, data.Student1.ID, data.Student2.ID)
	testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, data.CategoryID, data.Room)
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestVisitRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("creates visit with valid data", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}

		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		assert.NotZero(t, visit.ID)

		testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)
	})

	t.Run("creates visit with exit time", func(t *testing.T) {
		now := time.Now()
		exitTime := now.Add(1 * time.Hour)
		visit := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
			ExitTime:      &exitTime,
		}

		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		assert.NotZero(t, visit.ID)
		assert.NotNil(t, visit.ExitTime)

		testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)
	})

	t.Run("create with nil visit should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestVisitRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("finds existing visit", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		found, err := repo.FindByID(ctx, visit.ID)
		require.NoError(t, err)
		assert.Equal(t, visit.ID, found.ID)
		assert.Equal(t, data.Student1.ID, found.StudentID)
	})

	t.Run("returns error for non-existent visit", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestVisitRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("updates visit exit time", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		exitTime := now.Add(2 * time.Hour)
		visit.ExitTime = &exitTime
		err = repo.Update(ctx, visit)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, visit.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.ExitTime)
	})
}

func TestVisitRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("deletes existing visit", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)

		err = repo.Delete(ctx, visit.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, visit.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestVisitRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("lists all visits", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		visits, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, visits)
	})
}

func TestVisitRepository_FindActiveVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("finds only active visits (no exit_time)", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		visits, err := repo.FindActiveVisits(ctx)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("finds active visits for student", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		visits, err := repo.FindActiveByStudentID(ctx, data.Student1.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, visits)

		// All visits should be for this student and active
		for _, v := range visits {
			assert.Equal(t, data.Student1.ID, v.StudentID)
			assert.Nil(t, v.ExitTime)
		}
	})

	t.Run("returns empty for student with no active visits", func(t *testing.T) {
		// Student2 has no visits
		visits, err := repo.FindActiveByStudentID(ctx, data.Student2.ID)
		require.NoError(t, err)
		assert.Empty(t, visits)
	})
}

func TestVisitRepository_FindByActiveGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("finds visits for active group", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		visits, err := repo.FindByActiveGroupID(ctx, data.ActiveGroup.ID)
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

func TestVisitRepository_FindByTimeRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("finds visits in time range", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		start := now.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)

		visits, err := repo.FindByTimeRange(ctx, start, end)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("gets current active visit for student", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		current, err := repo.GetCurrentByStudentID(ctx, data.Student1.ID)
		require.NoError(t, err)
		assert.Equal(t, visit.ID, current.ID)
		assert.Nil(t, current.ExitTime)
	})

	t.Run("returns error for student with no current visit", func(t *testing.T) {
		_, err := repo.GetCurrentByStudentID(ctx, data.Student2.ID)
		require.Error(t, err)
	})
}

func TestVisitRepository_FindWithRelations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := activeRepo.NewVisitRepository(db).(*activeRepo.VisitRepository)
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("find with student loads student relation", func(t *testing.T) {
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     time.Now(),
		}
		require.NoError(t, repo.Create(ctx, visit))
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		found, err := repo.FindWithStudent(ctx, visit.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, data.Student1.ID, found.Student.ID)
	})

	t.Run("find with active group loads active group relation", func(t *testing.T) {
		visit := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     time.Now(),
		}
		require.NoError(t, repo.Create(ctx, visit))
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		found, err := repo.FindWithActiveGroup(ctx, visit.ID)
		require.NoError(t, err)
		require.NotNil(t, found.ActiveGroup)
		assert.Equal(t, data.ActiveGroup.ID, found.ActiveGroup.ID)
	})
}

func TestVisitsRepository_GetCurrentByStudentIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("gets current visits for multiple students", func(t *testing.T) {
		now := time.Now()
		visit1 := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit2 := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}

		err := repo.Create(ctx, visit1)
		require.NoError(t, err)
		err = repo.Create(ctx, visit2)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", visit1.ID)
			testpkg.CleanupTableRecords(t, db, "active.visits", visit2.ID)
		}()

		visitMap, err := repo.GetCurrentByStudentIDs(ctx, []int64{data.Student1.ID, data.Student2.ID})
		require.NoError(t, err)
		assert.Len(t, visitMap, 2)
		assert.Contains(t, visitMap, data.Student1.ID)
		assert.Contains(t, visitMap, data.Student2.ID)
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		visitMap, err := repo.GetCurrentByStudentIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, visitMap)
	})
}

// ============================================================================
// Visit End Tests
// ============================================================================

func TestVisitRepository_EndVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("ends active visit", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		err = repo.EndVisit(ctx, visit.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, visit.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.ExitTime)
	})
}

// ============================================================================
// Cleanup Tests
// ============================================================================

func TestVisitRepository_DeleteExpiredVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("deletes expired visits for student", func(t *testing.T) {
		// Create an old completed visit using raw SQL to bypass created_at auto-setting
		now := time.Now()
		exitTime := now.Add(-90 * 24 * time.Hour) // 90 days ago
		entryTime := exitTime.Add(-1 * time.Hour)
		createdAt := exitTime.Add(-1 * time.Hour)

		var visitID int64
		err := db.NewRaw(`
			INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at, tenant_id)
			VALUES (?, ?, ?, ?, ?, ?, 1)
			RETURNING id
		`, data.Student1.ID, data.ActiveGroup.ID, entryTime, exitTime, createdAt, now).
			Scan(ctx, &visitID)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visitID)

		// Delete visits older than 30 days
		deleted, err := repo.DeleteExpiredVisits(ctx, data.Student1.ID, 30)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, int64(1))
	})

	t.Run("does not delete active visits", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-60 * 24 * time.Hour), // 60 days ago
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		// Try to delete - should not delete active visits
		deleted, err := repo.DeleteExpiredVisits(ctx, data.Student1.ID, 30)
		require.NoError(t, err)

		// Visit should still exist
		_, err = repo.FindByID(ctx, visit.ID)
		require.NoError(t, err, "Active visit should not be deleted even if old")
		_ = deleted // Count may vary based on other test data
	})
}

func TestVisitRepository_DeleteVisitsBeforeDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("deletes visits before specified date", func(t *testing.T) {
		now := time.Now()
		exitTime := now.Add(-60 * 24 * time.Hour)
		entryTime := exitTime.Add(-1 * time.Hour)
		createdAt := exitTime.Add(-1 * time.Hour)

		var visitID int64
		err := db.NewRaw(`
			INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at, tenant_id)
			VALUES (?, ?, ?, ?, ?, ?, 1)
			RETURNING id
		`, data.Student1.ID, data.ActiveGroup.ID, entryTime, exitTime, createdAt, now).
			Scan(ctx, &visitID)
		require.NoError(t, err)

		cutoffDate := now.Add(-30 * 24 * time.Hour)
		deleted, err := repo.DeleteVisitsBeforeDate(ctx, data.Student1.ID, cutoffDate)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, int64(1))
	})
}

// NOTE: FindWithStudent and FindWithActiveGroup methods exist in implementation
// but are not exposed in the VisitRepository interface, so they cannot be
// tested through the interface.

// ============================================================================
// Transfer and Cleanup Tests
// ============================================================================

func TestVisitRepository_TransferVisitsFromRecentSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	groupRepo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("transfers visits from recently ended session", func(t *testing.T) {
		// Create device for this test
		device := testpkg.CreateTestDevice(t, db, "transfer-test-device")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		// Create old active group with device and end it recently
		now := time.Now()
		oldGroup := &active.Group{
			StartTime:      now.Add(-2 * time.Hour),
			LastActivity:   now.Add(-1 * time.Hour),
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(data.ActivityGroup),
			DeviceID:       &device.ID,
			RoomID:         data.Room,
		}
		err := groupRepo.Create(ctx, oldGroup)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, oldGroup.ID)

		// Create visit in old group (still active)
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: oldGroup.ID,
			EntryTime:     now.Add(-1 * time.Hour),
		}
		err = repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		// End the old group within the last hour
		err = groupRepo.EndSession(ctx, oldGroup.ID)
		require.NoError(t, err)

		// Create new active group with same device
		newGroup := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(data.ActivityGroup),
			DeviceID:       &device.ID,
			RoomID:         data.Room,
		}
		err = groupRepo.Create(ctx, newGroup)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, newGroup.ID)

		// Transfer visits
		transferred, err := repo.TransferVisitsFromRecentSessions(ctx, newGroup.ID, device.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, transferred)

		// Verify visit was transferred
		found, err := repo.FindByID(ctx, visit.ID)
		require.NoError(t, err)
		assert.Equal(t, newGroup.ID, found.ActiveGroupID)
	})

	t.Run("does not transfer from sessions ended more than 1 hour ago", func(t *testing.T) {
		// Create device for this test
		device := testpkg.CreateTestDevice(t, db, "no-transfer-device")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		// Create old group and end it more than 1 hour ago using raw SQL
		now := time.Now()
		var oldGroupID int64
		err := db.NewRaw(`
			INSERT INTO active.groups (start_time, last_activity, end_time, timeout_minutes, group_id, device_id, room_id, created_at, updated_at, tenant_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
			RETURNING id
		`, now.Add(-3*time.Hour), now.Add(-3*time.Hour), now.Add(-2*time.Hour), 30, data.ActivityGroup, device.ID, data.Room, now.Add(-3*time.Hour), now).
			Scan(ctx, &oldGroupID)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, oldGroupID)

		// Create visit in that old group
		visit := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: oldGroupID,
			EntryTime:     now.Add(-3 * time.Hour),
		}
		err = repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		// Create new active group with same device
		newGroup := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(data.ActivityGroup),
			DeviceID:       &device.ID,
			RoomID:         data.Room,
		}
		err = groupRepo.Create(ctx, newGroup)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, newGroup.ID)

		// Try to transfer - should transfer 0 because old session ended >1h ago
		transferred, err := repo.TransferVisitsFromRecentSessions(ctx, newGroup.ID, device.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, transferred)
	})
}

func TestVisitRepository_GetVisitRetentionStats(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("gets retention stats for students with expired visits", func(t *testing.T) {
		// Create a student with privacy consent
		student := testpkg.CreateTestStudent(t, db, "RetentionStats", "Student", "4a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Create privacy consent with short retention using raw SQL
		_, err := db.NewRaw(`
			INSERT INTO users.privacy_consents (student_id, policy_version, accepted, renewal_required, data_retention_days, tenant_id, created_at, updated_at)
			VALUES (?, 'v1.0', true, false, 7, 1, NOW(), NOW())
		`, student.ID).Exec(ctx)
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
			VALUES (?, ?, ?, ?, ?, ?, 1)
			RETURNING id
		`, student.ID, data.ActiveGroup.ID, entryTime, exitTime, createdAt, now).
			Scan(ctx, &visitID)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visitID)

		// Get stats
		stats, err := repo.GetVisitRetentionStats(ctx)
		require.NoError(t, err)

		// Should have stats for our student
		count, exists := stats[student.ID]
		if exists {
			assert.GreaterOrEqual(t, count, 1)
		}
	})
}

func TestVisitRepository_CountExpiredVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("counts all expired visits", func(t *testing.T) {
		// Create a student with privacy consent
		student := testpkg.CreateTestStudent(t, db, "ExpiredCount", "Student", "4b")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Create privacy consent with short retention using raw SQL
		_, err := db.NewRaw(`
			INSERT INTO users.privacy_consents (student_id, policy_version, accepted, renewal_required, data_retention_days, tenant_id, created_at, updated_at)
			VALUES (?, 'v1.0', true, false, 7, 1, NOW(), NOW())
		`, student.ID).Exec(ctx)
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
			VALUES (?, ?, ?, ?, ?, ?, 1)
			RETURNING id
		`, student.ID, data.ActiveGroup.ID, entryTime, exitTime, createdAt, now).
			Scan(ctx, &visitID)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visitID)

		// Count expired visits
		count, err := repo.CountExpiredVisits(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(1))
	})
}

// ============================================================================
// GetCurrentByStudentIDWithRoom Tests
// ============================================================================

func TestVisitRepository_GetCurrentByStudentIDWithRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("returns visit with active group and room", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		result, err := repo.GetCurrentByStudentIDWithRoom(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, visit.ID, result.ID)
		assert.Nil(t, result.ExitTime)

		// ActiveGroup should be loaded
		require.NotNil(t, result.ActiveGroup, "ActiveGroup should be loaded")
		assert.Equal(t, data.ActiveGroup.ID, result.ActiveGroup.ID)

		// Room should be loaded on active group
		require.NotNil(t, result.ActiveGroup.Room, "Room should be loaded on ActiveGroup")
		assert.Equal(t, data.Room, result.ActiveGroup.Room.ID)
	})

	t.Run("returns visit when active group timeout_minutes is null", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		_, err = db.NewUpdate().
			Table("active.groups").
			Set("timeout_minutes = NULL").
			Where("id = ?", data.ActiveGroup.ID).
			Exec(ctx)
		require.NoError(t, err)

		result, err := repo.GetCurrentByStudentIDWithRoom(ctx, data.Student1.ID)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.ActiveGroup)
		assert.Equal(t, 0, result.ActiveGroup.TimeoutMinutes)
		require.NotNil(t, result.ActiveGroup.Room)
		assert.Equal(t, data.Room, result.ActiveGroup.Room.ID)
	})

	t.Run("returns error for student with no active visit", func(t *testing.T) {
		_, err := repo.GetCurrentByStudentIDWithRoom(ctx, data.Student2.ID)
		require.Error(t, err)
	})

	t.Run("ignores exited visits", func(t *testing.T) {
		now := time.Now()
		exitTime := now.Add(-10 * time.Minute)
		visit := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
			ExitTime:      &exitTime,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		// Student2 should have no current visit (only exited one)
		_, err = repo.GetCurrentByStudentIDWithRoom(ctx, data.Student2.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// CountActiveByRoomID Tests
// ============================================================================

func TestVisitRepository_CountActiveByRoomID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("counts active visits in room", func(t *testing.T) {
		now := time.Now()
		visit1 := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		visit2 := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit1)
		require.NoError(t, err)
		err = repo.Create(ctx, visit2)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", visit1.ID)
			testpkg.CleanupTableRecords(t, db, "active.visits", visit2.ID)
		}()

		count, err := repo.CountActiveByRoomID(ctx, data.Room)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("excludes exited visits", func(t *testing.T) {
		now := time.Now()
		exitTime := now.Add(-5 * time.Minute)

		activeVisit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		exitedVisit := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
			ExitTime:      &exitTime,
		}
		err := repo.Create(ctx, activeVisit)
		require.NoError(t, err)
		err = repo.Create(ctx, exitedVisit)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", activeVisit.ID)
			testpkg.CleanupTableRecords(t, db, "active.visits", exitedVisit.ID)
		}()

		count, err := repo.CountActiveByRoomID(ctx, data.Room)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("returns zero for room with no visits", func(t *testing.T) {
		emptyRoom := testpkg.CreateTestRoom(t, db, "EmptyCountRoom")
		defer testpkg.CleanupActivityFixtures(t, db, emptyRoom.ID)

		count, err := repo.CountActiveByRoomID(ctx, emptyRoom.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

// ============================================================================
// CountActiveByGroupID Tests
// ============================================================================

func TestVisitRepository_CountActiveByGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("counts active visits in group", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		count, err := repo.CountActiveByGroupID(ctx, data.ActiveGroup.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("returns zero for group with no visits", func(t *testing.T) {
		count, err := repo.CountActiveByGroupID(ctx, data.ActiveGroup.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("excludes exited visits", func(t *testing.T) {
		now := time.Now()
		exitTime := now.Add(-5 * time.Minute)

		activeVisit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		exitedVisit := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-20 * time.Minute),
			ExitTime:      &exitTime,
		}
		err := repo.Create(ctx, activeVisit)
		require.NoError(t, err)
		err = repo.Create(ctx, exitedVisit)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", activeVisit.ID)
			testpkg.CleanupTableRecords(t, db, "active.visits", exitedVisit.ID)
		}()

		count, err := repo.CountActiveByGroupID(ctx, data.ActiveGroup.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

// ============================================================================
// EndVisitsByActiveGroupIDs Tests
// ============================================================================

func TestVisitRepository_EndVisitsByActiveGroupIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	groupRepo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("ends all active visits for group IDs", func(t *testing.T) {
		now := time.Now()

		// Create a second active group
		secondGroup := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(data.ActivityGroup),
			RoomID:         data.Room,
		}
		err := groupRepo.Create(ctx, secondGroup)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, secondGroup.ID)

		// Create visits in both groups (entry_time in past to avoid chk_entry_before_exit with DB now())
		visit1 := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-1 * time.Minute),
		}
		visit2 := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: secondGroup.ID,
			EntryTime:     now.Add(-1 * time.Minute),
		}
		err = repo.Create(ctx, visit1)
		require.NoError(t, err)
		err = repo.Create(ctx, visit2)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", visit1.ID)
			testpkg.CleanupTableRecords(t, db, "active.visits", visit2.ID)
		}()

		// End visits for both groups — expect count of 2 affected
		ended, err := repo.EndVisitsByActiveGroupIDs(ctx, []int64{data.ActiveGroup.ID, secondGroup.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(2), ended) // affected rows count

		// Verify visits are ended
		found1, err := repo.FindByID(ctx, visit1.ID)
		require.NoError(t, err)
		assert.NotNil(t, found1.ExitTime)

		found2, err := repo.FindByID(ctx, visit2.ID)
		require.NoError(t, err)
		assert.NotNil(t, found2.ExitTime)
	})

	t.Run("returns zero for empty group IDs", func(t *testing.T) {
		ended, err := repo.EndVisitsByActiveGroupIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Equal(t, int64(0), ended)
	})

	t.Run("does not end already exited visits", func(t *testing.T) {
		now := time.Now()
		exitTime := now.Add(-5 * time.Minute)

		exitedVisit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
			ExitTime:      &exitTime, // exit_time after entry_time
		}
		activeVisit := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-1 * time.Minute), // slightly in the past to avoid clock skew with DB now()
		}
		err := repo.Create(ctx, exitedVisit)
		require.NoError(t, err)
		err = repo.Create(ctx, activeVisit)
		require.NoError(t, err)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", exitedVisit.ID)
			testpkg.CleanupTableRecords(t, db, "active.visits", activeVisit.ID)
		}()

		// Should only end the active visit
		ended, err := repo.EndVisitsByActiveGroupIDs(ctx, []int64{data.ActiveGroup.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(1), ended) // affected rows count
	})

	t.Run("returns zero when no active visits exist", func(t *testing.T) {
		ended, err := repo.EndVisitsByActiveGroupIDs(ctx, []int64{data.ActiveGroup.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(0), ended)
	})
}

// ============================================================================
// GetCurrentByStudentIDs Deduplication Tests
// ============================================================================

func TestVisitsRepository_GetCurrentByStudentIDs_Deduplication(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.TenantContext(1)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("deduplicates student IDs in input", func(t *testing.T) {
		now := time.Now()
		visit := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		err := repo.Create(ctx, visit)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		// Pass duplicate IDs
		visitMap, err := repo.GetCurrentByStudentIDs(ctx, []int64{data.Student1.ID, data.Student1.ID, data.Student1.ID})
		require.NoError(t, err)
		assert.Len(t, visitMap, 1)
		assert.Contains(t, visitMap, data.Student1.ID)
	})
}
