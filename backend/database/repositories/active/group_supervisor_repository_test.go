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

func setupGroupSupervisorRepo(_ *testing.T, db *bun.DB) active.GroupSupervisorRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.GroupSupervisor
}

func setupSupervisorGroupRepo(_ *testing.T, db *bun.DB) active.GroupRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.ActiveGroup
}

// cleanupGroupSupervisorRecords removes group supervisors directly
func cleanupGroupSupervisorRecords(t *testing.T, db *bun.DB, supervisorIDs ...int64) {
	t.Helper()
	if len(supervisorIDs) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("active.group_supervisors").
		Where("id IN (?)", bun.In(supervisorIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup group supervisors: %v", err)
	}
}

// supervisorTestData holds test entities for supervisor tests
type supervisorTestData struct {
	Staff1        *users.Staff
	Staff2        *users.Staff
	ActivityGroup int64
	CategoryID    int64
	Room          int64
	ActiveGroup   *active.Group
}

// createSupervisorTestData creates test fixtures for supervisor tests
func createSupervisorTestData(t *testing.T, db *bun.DB) *supervisorTestData {
	staff1 := testpkg.CreateTestStaff(t, db, "Supervisor", "One")
	staff2 := testpkg.CreateTestStaff(t, db, "Supervisor", "Two")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "SupervisorActivity")
	room := testpkg.CreateTestRoom(t, db, "SupervisorRoom")

	// Create an active group for supervisors
	groupRepo := setupSupervisorGroupRepo(t, db)
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

	return &supervisorTestData{
		Staff1:        staff1,
		Staff2:        staff2,
		ActivityGroup: activityGroup.ID,
		CategoryID:    activityGroup.CategoryID,
		Room:          room.ID,
		ActiveGroup:   activeGroup,
	}
}

// cleanupSupervisorTestData removes test data
func cleanupSupervisorTestData(t *testing.T, db *bun.DB, data *supervisorTestData) {
	cleanupActiveGroupRecords(t, db, data.ActiveGroup.ID)
	testpkg.CleanupActivityFixtures(t, db, 0, data.Staff1.ID)
	testpkg.CleanupActivityFixtures(t, db, 0, data.Staff2.ID)
	testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, data.CategoryID, data.Room)
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestGroupSupervisorRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupGroupSupervisorRepo(t, db)
	ctx := context.Background()
	data := createSupervisorTestData(t, db)
	defer cleanupSupervisorTestData(t, db, data)

	t.Run("creates group supervisor with valid data", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}

		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		assert.NotZero(t, supervisor.ID)

		cleanupGroupSupervisorRecords(t, db, supervisor.ID)
	})

	t.Run("creates supervisor with end date", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		endDate := today.AddDate(0, 0, 7) // One week
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff2.ID,
			StartDate: today,
			EndDate:   &endDate,
			Role:      "assistant",
		}

		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		assert.NotZero(t, supervisor.ID)
		assert.NotNil(t, supervisor.EndDate)

		cleanupGroupSupervisorRecords(t, db, supervisor.ID)
	})

	t.Run("create with nil supervisor should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGroupSupervisorRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupGroupSupervisorRepo(t, db)
	ctx := context.Background()
	data := createSupervisorTestData(t, db)
	defer cleanupSupervisorTestData(t, db, data)

	t.Run("finds existing group supervisor", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		defer cleanupGroupSupervisorRecords(t, db, supervisor.ID)

		found, err := repo.FindByID(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.Equal(t, supervisor.ID, found.ID)
		assert.Equal(t, data.Staff1.ID, found.StaffID)
	})

	t.Run("returns error for non-existent supervisor", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestGroupSupervisorRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupGroupSupervisorRepo(t, db)
	ctx := context.Background()
	data := createSupervisorTestData(t, db)
	defer cleanupSupervisorTestData(t, db, data)

	t.Run("updates supervisor role", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		defer cleanupGroupSupervisorRecords(t, db, supervisor.ID)

		supervisor.Role = "lead"
		err = repo.Update(ctx, supervisor)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.Equal(t, "lead", found.Role)
	})
}

func TestGroupSupervisorRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupGroupSupervisorRepo(t, db)
	ctx := context.Background()
	data := createSupervisorTestData(t, db)
	defer cleanupSupervisorTestData(t, db, data)

	t.Run("deletes existing supervisor", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		err = repo.Delete(ctx, supervisor.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, supervisor.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestGroupSupervisorRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupGroupSupervisorRepo(t, db)
	ctx := context.Background()
	data := createSupervisorTestData(t, db)
	defer cleanupSupervisorTestData(t, db, data)

	t.Run("lists all group supervisors", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		defer cleanupGroupSupervisorRecords(t, db, supervisor.ID)

		supervisors, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, supervisors)
	})
}

func TestGroupSupervisorRepository_FindActiveByStaffID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupGroupSupervisorRepo(t, db)
	ctx := context.Background()
	data := createSupervisorTestData(t, db)
	defer cleanupSupervisorTestData(t, db, data)

	t.Run("finds active supervisions for staff", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		defer cleanupGroupSupervisorRecords(t, db, supervisor.ID)

		supervisions, err := repo.FindActiveByStaffID(ctx, data.Staff1.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, supervisions)

		var found bool
		for _, s := range supervisions {
			if s.ID == supervisor.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for staff with no supervisions", func(t *testing.T) {
		supervisions, err := repo.FindActiveByStaffID(ctx, data.Staff2.ID)
		require.NoError(t, err)
		assert.Empty(t, supervisions)
	})
}

func TestGroupSupervisorRepository_FindByActiveGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupGroupSupervisorRepo(t, db)
	ctx := context.Background()
	data := createSupervisorTestData(t, db)
	defer cleanupSupervisorTestData(t, db, data)

	t.Run("finds all supervisors for active group", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		supervisor1 := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		supervisor2 := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff2.ID,
			StartDate: today,
			Role:      "assistant",
		}

		err := repo.Create(ctx, supervisor1)
		require.NoError(t, err)
		err = repo.Create(ctx, supervisor2)
		require.NoError(t, err)
		defer cleanupGroupSupervisorRecords(t, db, supervisor1.ID, supervisor2.ID)

		// Get all supervisors (active or not)
		supervisions, err := repo.FindByActiveGroupID(ctx, data.ActiveGroup.ID, false)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(supervisions), 2)
	})

	t.Run("finds only active supervisors when activeOnly is true", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		endDate := today.AddDate(0, 0, -1) // Yesterday (ended)

		activeSupervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		endedSupervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff2.ID,
			StartDate: today.AddDate(0, 0, -7),
			EndDate:   &endDate,
			Role:      "assistant",
		}

		err := repo.Create(ctx, activeSupervisor)
		require.NoError(t, err)
		err = repo.Create(ctx, endedSupervisor)
		require.NoError(t, err)
		defer cleanupGroupSupervisorRecords(t, db, activeSupervisor.ID, endedSupervisor.ID)

		// Get only active supervisors
		supervisions, err := repo.FindByActiveGroupID(ctx, data.ActiveGroup.ID, true)
		require.NoError(t, err)

		// All returned supervisions should not have an end_date
		for _, s := range supervisions {
			assert.Nil(t, s.EndDate)
		}
	})
}

func TestGroupSupervisorRepository_FindByActiveGroupIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupGroupSupervisorRepo(t, db)
	groupRepo := setupSupervisorGroupRepo(t, db)
	ctx := context.Background()
	data := createSupervisorTestData(t, db)
	defer cleanupSupervisorTestData(t, db, data)

	t.Run("finds supervisors for multiple active groups", func(t *testing.T) {
		// Create a second active group
		room2 := testpkg.CreateTestRoom(t, db, "SupervisorRoom2")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, 0, room2.ID)

		now := time.Now()
		activeGroup2 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        data.ActivityGroup,
			RoomID:         room2.ID,
		}
		err := groupRepo.Create(ctx, activeGroup2)
		require.NoError(t, err)
		defer cleanupActiveGroupRecords(t, db, activeGroup2.ID)

		today := time.Now().Truncate(24 * time.Hour)
		supervisor1 := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		supervisor2 := &active.GroupSupervisor{
			GroupID:   activeGroup2.ID,
			StaffID:   data.Staff2.ID,
			StartDate: today,
			Role:      "supervisor",
		}

		err = repo.Create(ctx, supervisor1)
		require.NoError(t, err)
		err = repo.Create(ctx, supervisor2)
		require.NoError(t, err)
		defer cleanupGroupSupervisorRecords(t, db, supervisor1.ID, supervisor2.ID)

		supervisions, err := repo.FindByActiveGroupIDs(ctx, []int64{data.ActiveGroup.ID, activeGroup2.ID}, false)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(supervisions), 2)
	})

	t.Run("returns empty for empty input", func(t *testing.T) {
		supervisions, err := repo.FindByActiveGroupIDs(ctx, []int64{}, false)
		require.NoError(t, err)
		assert.Empty(t, supervisions)
	})
}

// ============================================================================
// End Supervision Tests
// ============================================================================

func TestGroupSupervisorRepository_EndSupervision(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupGroupSupervisorRepo(t, db)
	ctx := context.Background()
	data := createSupervisorTestData(t, db)
	defer cleanupSupervisorTestData(t, db, data)

	t.Run("ends active supervision", func(t *testing.T) {
		today := time.Now().Truncate(24 * time.Hour)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		defer cleanupGroupSupervisorRecords(t, db, supervisor.ID)

		err = repo.EndSupervision(ctx, supervisor.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.EndDate)
	})
}

// NOTE: FindWithStaff and FindWithActiveGroup methods exist in implementation
// but are not exposed in the GroupSupervisorRepository interface, so they
// cannot be tested through the interface.
