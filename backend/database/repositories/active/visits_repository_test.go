package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupVisitRepo(t *testing.T, db *bun.DB) active.VisitRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.ActiveVisit
}

func setupVisitGroupRepo(t *testing.T, db *bun.DB) active.GroupRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.ActiveGroup
}

// cleanupVisitRecords removes visits directly
func cleanupVisitRecords(t *testing.T, db *bun.DB, visitIDs ...int64) {
	t.Helper()
	if len(visitIDs) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("active.visits").
		Where("id IN (?)", bun.In(visitIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup visits: %v", err)
	}
}

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
	groupRepo := setupVisitGroupRepo(t, db)
	now := time.Now()
	activeGroup := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        activityGroup.ID,
		RoomID:         room.ID,
	}
	err := groupRepo.Create(context.Background(), activeGroup)
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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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

		cleanupVisitRecords(t, db, visit.ID)
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

		cleanupVisitRecords(t, db, visit.ID)
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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
		defer cleanupVisitRecords(t, db, visit.ID)

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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
		defer cleanupVisitRecords(t, db, visit.ID)

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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
		defer cleanupVisitRecords(t, db, visit.ID)

		visits, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, visits)
	})
}

func TestVisitRepository_FindActiveVisits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
		defer cleanupVisitRecords(t, db, visit.ID)

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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
		defer cleanupVisitRecords(t, db, visit.ID)

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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
		defer cleanupVisitRecords(t, db, visit.ID)

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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
		defer cleanupVisitRecords(t, db, visit.ID)

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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
		defer cleanupVisitRecords(t, db, visit.ID)

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

func TestVisitsRepository_GetCurrentByStudentIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
		defer cleanupVisitRecords(t, db, visit1.ID, visit2.ID)

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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
		defer cleanupVisitRecords(t, db, visit.ID)

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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
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
			INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			RETURNING id
		`, data.Student1.ID, data.ActiveGroup.ID, entryTime, exitTime, createdAt, now).
			Scan(ctx, &visitID)
		require.NoError(t, err)
		defer cleanupVisitRecords(t, db, visitID)

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
		defer cleanupVisitRecords(t, db, visit.ID)

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

	repo := setupVisitRepo(t, db)
	ctx := context.Background()
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("deletes visits before specified date", func(t *testing.T) {
		now := time.Now()
		exitTime := now.Add(-60 * 24 * time.Hour)
		entryTime := exitTime.Add(-1 * time.Hour)
		createdAt := exitTime.Add(-1 * time.Hour)

		var visitID int64
		err := db.NewRaw(`
			INSERT INTO active.visits (student_id, active_group_id, entry_time, exit_time, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
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
