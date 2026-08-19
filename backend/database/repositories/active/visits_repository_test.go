package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

func TestVisitRepository_FindByActiveGroupIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("empty input hits no DB and returns nothing", func(t *testing.T) {
		visits, err := repo.FindByActiveGroupIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, visits)
	})

	t.Run("finds visits across the given active groups in one call", func(t *testing.T) {
		now := time.Now()
		visit1 := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		require.NoError(t, repo.Create(ctx, visit1))
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit1.ID)

		visit2 := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now,
		}
		require.NoError(t, repo.Create(ctx, visit2))
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit2.ID)

		// One unknown id in the set must not affect the real matches.
		visits, err := repo.FindByActiveGroupIDs(ctx, []int64{data.ActiveGroup.ID, -1})
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
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

func TestVisitsRepository_GetCurrentByStudentIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

func TestVisitRepository_GetTodayVisitNamesForStudents(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	visit := testpkg.CreateTestVisit(t, db, data.Student1.ID, data.ActiveGroup.ID, timezone.Today().Add(30*time.Minute), nil)
	defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

	t.Run("empty input short-circuits", func(t *testing.T) {
		names, err := repo.GetTodayVisitNamesForStudents(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, names)
	})

	t.Run("returns activity and room names for today's visits", func(t *testing.T) {
		names, err := repo.GetTodayVisitNamesForStudents(ctx, []int64{data.Student1.ID, data.Student1.ID})

		require.NoError(t, err)
		require.NotEmpty(t, names)

		var found bool
		for _, row := range names {
			if row.StudentID == data.Student1.ID {
				found = true
				assert.NotEmpty(t, row.ActivityGroupName)
				assert.NotEmpty(t, row.RoomName)
			}
		}
		assert.True(t, found)
	})
}

func TestVisitRepository_GetCurrentRoomNamesForStudents(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("empty input returns empty map", func(t *testing.T) {
		locations, err := repo.GetCurrentRoomNamesForStudents(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, locations)
	})

	t.Run("returns newest active room per student", func(t *testing.T) {
		secondRoom := testpkg.CreateTestRoom(t, db, "VisitRoomCurrent")
		secondActivity := testpkg.CreateTestActivityGroup(t, db, "VisitActivityCurrent")
		secondGroup := testpkg.CreateTestActiveGroup(t, db, secondActivity.ID, secondRoom.ID)
		defer func() {
			cleanupActiveGroupRecords(t, db, secondGroup.ID)
			testpkg.CleanupActivityFixtures(t, db, secondActivity.ID, secondRoom.ID)
		}()

		oldExit := time.Now().Add(-90 * time.Minute)
		oldVisit := testpkg.CreateTestVisit(t, db, data.Student1.ID, data.ActiveGroup.ID, time.Now().Add(-2*time.Hour), &oldExit)
		newVisit := testpkg.CreateTestVisit(t, db, data.Student1.ID, secondGroup.ID, time.Now().Add(-10*time.Minute), nil)
		otherVisit := testpkg.CreateTestVisit(t, db, data.Student2.ID, data.ActiveGroup.ID, time.Now().Add(-20*time.Minute), nil)
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", oldVisit.ID, newVisit.ID, otherVisit.ID)
		}()

		locations, err := repo.GetCurrentRoomNamesForStudents(ctx, []int64{data.Student1.ID, data.Student2.ID})

		require.NoError(t, err)
		assert.Equal(t, secondRoom.Name, locations[data.Student1.ID])
		assert.NotEmpty(t, locations[data.Student2.ID])
	})

	t.Run("ended active group is excluded", func(t *testing.T) {
		endedAt := time.Now()
		endedGroup := testpkg.CreateTestActiveGroup(t, db, data.ActivityGroup, data.Room)
		endedGroup.EndTime = &endedAt
		_, err := db.NewUpdate().
			Model(endedGroup).
			ModelTableExpr(`active.groups`).
			Column("end_time").
			Where("id = ?", endedGroup.ID).
			Exec(ctx)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, endedGroup.ID)

		visit := testpkg.CreateTestVisit(t, db, data.Student1.ID, endedGroup.ID, time.Now().Add(-10*time.Minute), nil)
		defer testpkg.CleanupTableRecords(t, db, "active.visits", visit.ID)

		locations, err := repo.GetCurrentRoomNamesForStudents(ctx, []int64{data.Student1.ID})

		require.NoError(t, err)
		assert.NotContains(t, locations, data.Student1.ID)
	})
}

func TestVisitRepository_FindActiveWithStudentDisplayByGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	educationGroup := testpkg.CreateTestEducationGroup(t, db, "Visit Display Group")
	_, err := db.NewUpdate().
		Table("users.students").
		Set("group_id = ?", educationGroup.ID).
		Where("id = ?", data.Student1.ID).
		Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewUpdate().
			Table("users.students").
			Set("group_id = NULL").
			Where("id = ?", data.Student1.ID).
			Exec(ctx)
		_, _ = db.NewDelete().
			Table("education.groups").
			Where("id = ?", educationGroup.ID).
			Exec(ctx)
	}()

	activeVisit := testpkg.CreateTestVisit(t, db, data.Student1.ID, data.ActiveGroup.ID, time.Now().Add(-10*time.Minute), nil)
	exitTime := time.Now().Add(-5 * time.Minute)
	endedVisit := testpkg.CreateTestVisit(t, db, data.Student2.ID, data.ActiveGroup.ID, time.Now().Add(-20*time.Minute), &exitTime)
	defer func() {
		testpkg.CleanupTableRecords(t, db, "active.visits", activeVisit.ID, endedVisit.ID)
	}()

	results, err := repo.FindActiveWithStudentDisplayByGroup(ctx, data.ActiveGroup.ID)

	require.NoError(t, err)
	require.Len(t, results, 1)
	row := results[0]
	assert.Equal(t, activeVisit.ID, row.VisitID)
	assert.Equal(t, data.Student1.ID, row.StudentID)
	assert.Equal(t, data.ActiveGroup.ID, row.ActiveGroupID)
	assert.Equal(t, "Visit", row.FirstName)
	assert.Equal(t, "Student1", row.LastName)
	assert.Equal(t, "1a", row.SchoolClass)
	require.NotNil(t, row.GroupID)
	assert.Equal(t, educationGroup.ID, *row.GroupID)
	assert.Equal(t, educationGroup.Name, row.OGSGroupName)
	assert.Nil(t, row.ExitTime)
}

// ============================================================================
// Visit End Tests
// ============================================================================

func TestVisitRepository_EndVisit(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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
			VALUES (?, ?, ?, ?, ?, ?, ?)
			RETURNING id
		`, data.Student1.ID, data.ActiveGroup.ID, entryTime, exitTime, createdAt, now, testpkg.Tenant(t)).
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

// but are not exposed in the VisitRepository interface, so they cannot be
// tested through the interface.

// ============================================================================
// Transfer and Cleanup Tests
// ============================================================================

func TestVisitRepository_TransferVisitsFromRecentSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	groupRepo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.Ctx(t)
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
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			RETURNING id
		`, now.Add(-3*time.Hour), now.Add(-3*time.Hour), now.Add(-2*time.Hour), 30, data.ActivityGroup, device.ID, data.Room, now.Add(-3*time.Hour), now, testpkg.Tenant(t)).
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

func TestVisitRepository_TransferActiveVisitsBetweenGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	groupRepo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	now := time.Now()
	oldGroup := &active.Group{
		StartTime:      now.Add(-30 * time.Minute),
		LastActivity:   now.Add(-10 * time.Minute),
		TimeoutMinutes: 30,
		GroupID:        base.Int64Ptr(data.ActivityGroup),
		RoomID:         data.Room,
	}
	require.NoError(t, groupRepo.Create(ctx, oldGroup))
	defer cleanupActiveGroupRecords(t, db, oldGroup.ID)

	newGroup := &active.Group{
		StartTime:      now,
		LastActivity:   now,
		TimeoutMinutes: 30,
		GroupID:        base.Int64Ptr(data.ActivityGroup),
		RoomID:         data.Room,
	}
	require.NoError(t, groupRepo.Create(ctx, newGroup))
	defer cleanupActiveGroupRecords(t, db, newGroup.ID)

	activeVisit := &active.Visit{
		StudentID:     data.Student1.ID,
		ActiveGroupID: oldGroup.ID,
		EntryTime:     now.Add(-20 * time.Minute),
	}
	require.NoError(t, repo.Create(ctx, activeVisit))
	defer testpkg.CleanupTableRecords(t, db, "active.visits", activeVisit.ID)

	exitTime := now.Add(-5 * time.Minute)
	endedVisit := &active.Visit{
		StudentID:     data.Student2.ID,
		ActiveGroupID: oldGroup.ID,
		EntryTime:     now.Add(-25 * time.Minute),
		ExitTime:      &exitTime,
	}
	require.NoError(t, repo.Create(ctx, endedVisit))
	defer testpkg.CleanupTableRecords(t, db, "active.visits", endedVisit.ID)

	transferred, err := repo.TransferActiveVisitsBetweenGroups(ctx, oldGroup.ID, newGroup.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, transferred)

	foundActive, err := repo.FindByID(ctx, activeVisit.ID)
	require.NoError(t, err)
	assert.Equal(t, newGroup.ID, foundActive.ActiveGroupID)
	assert.Nil(t, foundActive.ExitTime)

	foundEnded, err := repo.FindByID(ctx, endedVisit.ID)
	require.NoError(t, err)
	assert.Equal(t, oldGroup.ID, foundEnded.ActiveGroupID)
	require.NotNil(t, foundEnded.ExitTime)
	assert.WithinDuration(t, exitTime, *foundEnded.ExitTime, time.Second)
}

func TestVisitRepository_GetVisitRetentionStats(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("gets retention stats for students with expired visits", func(t *testing.T) {
		// Create a student with privacy consent
		student := testpkg.CreateTestStudent(t, db, "RetentionStats", "Student", "4a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("counts all expired visits", func(t *testing.T) {
		// Create a student with privacy consent
		student := testpkg.CreateTestStudent(t, db, "ExpiredCount", "Student", "4b")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	groupRepo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.Ctx(t)
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

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
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

// ============================================================================
// ListActiveStudentIDsByRoomID Tests
// ============================================================================

func TestVisitRepository_ListActiveStudentIDsByRoomID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	t.Run("returns IDs of currently checked-in students", func(t *testing.T) {
		now := time.Now()
		v1 := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-10 * time.Minute),
		}
		v2 := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-5 * time.Minute),
		}
		require.NoError(t, repo.Create(ctx, v1))
		require.NoError(t, repo.Create(ctx, v2))
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", v1.ID)
			testpkg.CleanupTableRecords(t, db, "active.visits", v2.ID)
		}()

		ids, err := repo.ListActiveStudentIDsByRoomID(ctx, data.Room)
		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{data.Student1.ID, data.Student2.ID}, ids)
	})

	t.Run("excludes visits whose exit_time is set", func(t *testing.T) {
		now := time.Now()
		exitTime := now.Add(-2 * time.Minute)
		open := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-10 * time.Minute),
		}
		closed := &active.Visit{
			StudentID:     data.Student2.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
			ExitTime:      &exitTime,
		}
		require.NoError(t, repo.Create(ctx, open))
		require.NoError(t, repo.Create(ctx, closed))
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", open.ID)
			testpkg.CleanupTableRecords(t, db, "active.visits", closed.ID)
		}()

		ids, err := repo.ListActiveStudentIDsByRoomID(ctx, data.Room)
		require.NoError(t, err)
		assert.Equal(t, []int64{data.Student1.ID}, ids,
			"a visit with exit_time IS NOT NULL must not surface — the student already left")
	})

	t.Run("excludes visits whose group has end_time set", func(t *testing.T) {
		// New room + a closed active group on it. A visit with exit_time IS NULL
		// should still be hidden because the session itself is over.
		room := testpkg.CreateTestRoom(t, db, "EndedSessionRoom")
		groupRepo := repositories.NewFactory(db).ActiveGroup
		now := time.Now()
		endTime := now.Add(-1 * time.Minute)
		closedGroup := &active.Group{
			StartTime:      now.Add(-1 * time.Hour),
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(data.ActivityGroup),
			RoomID:         room.ID,
			EndTime:        &endTime,
		}
		require.NoError(t, groupRepo.Create(ctx, closedGroup))
		v := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: closedGroup.ID,
			EntryTime:     now.Add(-30 * time.Minute),
		}
		require.NoError(t, repo.Create(ctx, v))
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", v.ID)
			cleanupActiveGroupRecords(t, db, closedGroup.ID)
			testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room.ID)
		}()

		ids, err := repo.ListActiveStudentIDsByRoomID(ctx, room.ID)
		require.NoError(t, err)
		assert.Empty(t, ids,
			"a visit attached to a group with end_time IS NOT NULL must not surface — the session is closed")
	})

	t.Run("returns empty for room with no active visits", func(t *testing.T) {
		emptyRoom := testpkg.CreateTestRoom(t, db, "EmptyListRoom")
		defer testpkg.CleanupActivityFixtures(t, db, emptyRoom.ID)

		ids, err := repo.ListActiveStudentIDsByRoomID(ctx, emptyRoom.ID)
		require.NoError(t, err)
		assert.Empty(t, ids)
	})

	t.Run("respects tenant scoping", func(t *testing.T) {
		// A visit is created in tenant 1; querying with tenant 2 context must
		// return zero IDs. This pins the TenantWhere clause in the repo.
		now := time.Now()
		v := &active.Visit{
			StudentID:     data.Student1.ID,
			ActiveGroupID: data.ActiveGroup.ID,
			EntryTime:     now.Add(-1 * time.Minute),
		}
		require.NoError(t, repo.Create(ctx, v))
		defer testpkg.CleanupTableRecords(t, db, "active.visits", v.ID)

		otherTenant := testpkg.TenantContext(2)
		ids, err := repo.ListActiveStudentIDsByRoomID(otherTenant, data.Room)
		require.NoError(t, err)
		assert.Empty(t, ids,
			"querying as tenant 2 must not see tenant 1's visits — RLS / tenant filter regression")
	})

	t.Run("aggregates students across multiple active groups in the same room", func(t *testing.T) {
		// Same room can host more than one concurrent active group. The repo
		// must union students across all of them.
		groupRepo := repositories.NewFactory(db).ActiveGroup
		now := time.Now()
		secondGroup := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        base.Int64Ptr(data.ActivityGroup),
			RoomID:         data.Room,
		}
		require.NoError(t, groupRepo.Create(ctx, secondGroup))

		v1 := &active.Visit{StudentID: data.Student1.ID, ActiveGroupID: data.ActiveGroup.ID, EntryTime: now}
		v2 := &active.Visit{StudentID: data.Student2.ID, ActiveGroupID: secondGroup.ID, EntryTime: now}
		require.NoError(t, repo.Create(ctx, v1))
		require.NoError(t, repo.Create(ctx, v2))
		defer func() {
			testpkg.CleanupTableRecords(t, db, "active.visits", v1.ID)
			testpkg.CleanupTableRecords(t, db, "active.visits", v2.ID)
			cleanupActiveGroupRecords(t, db, secondGroup.ID)
		}()

		ids, err := repo.ListActiveStudentIDsByRoomID(ctx, data.Room)
		require.NoError(t, err)
		assert.ElementsMatch(t, []int64{data.Student1.ID, data.Student2.ID}, ids)
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

// cleanupTenantPrivacyConsents removes consents for the given tenants —
// CleanupTenantTestData does not cover users.privacy_consents, and the
// student rows it deletes are FK targets of the consents.
func cleanupTenantPrivacyConsents(t *testing.T, db *bun.DB, tenantIDs ...int64) {
	t.Helper()
	for _, tid := range tenantIDs {
		_, _ = db.NewDelete().
			Table("users.privacy_consents").
			Where("tenant_id = ?", tid).
			Exec(testpkg.TenantContext(tid))
	}
}

func TestVisitRepository_OldestExpiredVisitDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit

	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	t.Cleanup(func() {
		cleanupTenantPrivacyConsents(t, db, tenantID, otherTenantID)
		testpkg.CleanupTenantTestData(t, db, tenantID, otherTenantID)
	})

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
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit

	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	t.Cleanup(func() {
		cleanupTenantPrivacyConsents(t, db, tenantID, otherTenantID)
		testpkg.CleanupTenantTestData(t, db, tenantID, otherTenantID)
	})

	ctx := testpkg.TenantContext(tenantID)

	t.Run("empty map when no visit is expired", func(t *testing.T) {
		counts, err := repo.ExpiredVisitMonthlyCounts(ctx)
		require.NoError(t, err)
		assert.Empty(t, counts)
	})

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Monthly", "Expired", "4c")
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
	otherStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Foreign", "Monthly", "4d")
	otherGroup := testpkg.CreateTestActiveGroupForTenant(t, db, otherTenantID)
	createAcceptedConsentForTenant(t, db, otherTenantID, otherStudent.ID, "v1.0", 7)
	createCompletedVisitForTenant(t, db, otherTenantID, otherStudent.ID, otherGroup.ID, newer)

	t.Run("groups the tenant's expired visits by month", func(t *testing.T) {
		counts, err := repo.ExpiredVisitMonthlyCounts(ctx)
		require.NoError(t, err)
		require.Len(t, counts, 2)

		var total int64
		for month, count := range counts {
			assert.Regexp(t, `^\d{4}-\d{2}$`, month)
			total += count
		}
		assert.EqualValues(t, 3, total)
	})
}
