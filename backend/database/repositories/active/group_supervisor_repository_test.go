package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

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
	groupRepo := repositories.NewFactory(db).ActiveGroup
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

	return &supervisorTestData{
		Staff1:        staff1,
		Staff2:        staff2,
		ActivityGroup: activityGroup.ID,
		CategoryID:    activityGroup.CategoryID,
		Room:          room.ID,
		ActiveGroup:   activeGroup,
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestGroupSupervisorRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("creates group supervisor with valid data", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}

		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		assert.NotZero(t, supervisor.ID)

	})

	t.Run("creates supervisor with end date", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		endDate := today.AddDays(7) // One week
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

	})

	t.Run("create with nil supervisor should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGroupSupervisorRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("finds existing group supervisor", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.NewDate(2026, 8, 24)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("updates supervisor role", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.NewDate(2026, 8, 24)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		supervisor.Role = "lead"
		err = repo.Update(ctx, supervisor)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.Equal(t, "lead", found.Role)
	})
}

func TestGroupSupervisorRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing supervisor", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.NewDate(2026, 8, 24)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("lists all group supervisors", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.NewDate(2026, 8, 24)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		supervisors, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, supervisors)
	})

	t.Run("filters active_only supervisors", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.NewDate(2026, 8, 24)
		yesterday := today.AddDays(-1)

		// Create an active supervisor (no end_date)
		activeSupervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, activeSupervisor)
		require.NoError(t, err)

		// Create an ended supervisor (end_date in past)
		endedSupervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff2.ID,
			StartDate: yesterday.AddDays(-7),
			EndDate:   &yesterday,
			Role:      "supervisor",
		}
		err = repo.Create(ctx, endedSupervisor)
		require.NoError(t, err)

		// Test active_only=true filter
		options := modelBase.NewQueryOptions()
		options.Filter.Equal("active_only", true)

		supervisors, err := repo.List(ctx, options)
		require.NoError(t, err)

		// Should contain active supervisor
		var foundActive, foundEnded bool
		for _, s := range supervisors {
			if s.ID == activeSupervisor.ID {
				foundActive = true
			}
			if s.ID == endedSupervisor.ID {
				foundEnded = true
			}
		}
		assert.True(t, foundActive, "active supervisor should be in results")
		assert.False(t, foundEnded, "ended supervisor should not be in results")
	})
}

func TestGroupSupervisorRepository_FindActiveByStaffID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("finds active supervisions for staff", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.NewDate(2026, 8, 24)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

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
		data := createSupervisorTestData(t, db)
		supervisions, err := repo.FindActiveByStaffID(ctx, data.Staff2.ID)
		require.NoError(t, err)
		assert.Empty(t, supervisions)
	})
}

func TestGroupSupervisorRepository_FindByActiveGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("finds all supervisors for active group", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
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

		// Get all supervisors (active or not)
		supervisions, err := repo.FindByActiveGroupID(ctx, data.ActiveGroup.ID, false)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(supervisions), 2)
	})

	t.Run("finds only active supervisors when activeOnly is true", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		endDate := today.AddDays(-1) // Yesterday (ended)

		activeSupervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		endedSupervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff2.ID,
			StartDate: today.AddDays(-7),
			EndDate:   &endDate,
			Role:      "assistant",
		}

		err := repo.Create(ctx, activeSupervisor)
		require.NoError(t, err)
		err = repo.Create(ctx, endedSupervisor)
		require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	groupRepo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.Ctx(t)

	t.Run("finds supervisors for multiple active groups", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		// Create a second active group
		room2 := testpkg.CreateTestRoom(t, db, "SupervisorRoom2")

		now := time.Now()
		activeGroup2 := &active.Group{
			StartTime:      now,
			LastActivity:   now,
			TimeoutMinutes: 30,
			GroupID:        ptrtest.Ptr(data.ActivityGroup),
			RoomID:         room2.ID,
		}
		err := groupRepo.Create(ctx, activeGroup2)
		require.NoError(t, err)

		today := timezone.TodayDate()
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("ends active supervision", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.NewDate(2026, 8, 24)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		err = repo.EndSupervision(ctx, supervisor.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.EndDate)
	})
}

// ============================================================================
// Presence Tracking Tests
// ============================================================================

func TestGroupSupervisorRepository_GetStaffIDsWithSupervisionToday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("returns staff with supervision starting today", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		staffIDs, err := repo.GetStaffIDsWithSupervisionToday(ctx)
		require.NoError(t, err)
		assert.Contains(t, staffIDs, data.Staff1.ID)
	})

	t.Run("returns staff with supervision ending today", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		yesterday := today.AddDays(-1)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: yesterday,
			EndDate:   &today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		staffIDs, err := repo.GetStaffIDsWithSupervisionToday(ctx)
		require.NoError(t, err)
		assert.Contains(t, staffIDs, data.Staff1.ID)
	})

	t.Run("returns staff with supervision spanning today", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		yesterday := today.AddDays(-1)
		tomorrow := today.AddDays(1)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: yesterday,
			EndDate:   &tomorrow,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		staffIDs, err := repo.GetStaffIDsWithSupervisionToday(ctx)
		require.NoError(t, err)
		assert.Contains(t, staffIDs, data.Staff1.ID)
	})

	t.Run("returns staff with ongoing supervision (no end date)", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		yesterday := today.AddDays(-1)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: yesterday,
			EndDate:   nil, // Still active
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		staffIDs, err := repo.GetStaffIDsWithSupervisionToday(ctx)
		require.NoError(t, err)
		assert.Contains(t, staffIDs, data.Staff1.ID)
	})

	t.Run("excludes staff with supervision ended before today", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		twoDaysAgo := today.AddDays(-2)
		yesterday := today.AddDays(-1)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff2.ID,
			StartDate: twoDaysAgo,
			EndDate:   &yesterday, // Ended yesterday
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		staffIDs, err := repo.GetStaffIDsWithSupervisionToday(ctx)
		require.NoError(t, err)
		assert.NotContains(t, staffIDs, data.Staff2.ID)
	})

	t.Run("returns distinct staff IDs for multiple supervisions", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		// Create multiple supervisions for same staff on same day
		supervisor1 := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		supervisor2 := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "assistant",
		}
		err := repo.Create(ctx, supervisor1)
		require.NoError(t, err)
		err = repo.Create(ctx, supervisor2)
		require.NoError(t, err)

		staffIDs, err := repo.GetStaffIDsWithSupervisionToday(ctx)
		require.NoError(t, err)

		// Count occurrences of Staff1 ID
		count := 0
		for _, id := range staffIDs {
			if id == data.Staff1.ID {
				count++
			}
		}
		assert.Equal(t, 1, count, "Staff ID should appear only once due to DISTINCT")
	})
}

// ============================================================================
// Nil and Error Path Tests
// ============================================================================

func TestGroupSupervisorRepository_Update_NilSupervision(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("update with nil supervision should fail", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGroupSupervisorRepository_Update_ValidationFailure(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("update with invalid supervision should fail validation", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		// Create a valid supervisor first
		today := timezone.TodayDate()
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		// Now make it invalid by setting StaffID to 0
		supervisor.StaffID = 0
		err = repo.Update(ctx, supervisor)
		require.Error(t, err)
	})
}

func TestGroupSupervisorRepository_Create_ValidationFailure(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("create with invalid supervision should fail validation", func(t *testing.T) {
		// Missing required StaffID
		supervisor := &active.GroupSupervisor{
			GroupID:   1,
			StaffID:   0, // Invalid - required
			StartDate: timezone.TodayDate(),
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.Error(t, err)
	})

	t.Run("create with missing GroupID should fail validation", func(t *testing.T) {
		supervisor := &active.GroupSupervisor{
			GroupID:   0, // Invalid - required
			StaffID:   1,
			StartDate: timezone.TodayDate(),
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.Error(t, err)
	})
}

func TestGroupSupervisorRepository_List_WithQueryOptions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("lists with filter options applied", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "lead",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		// Test with query options (using nil filter is already tested, so test with options)
		supervisors, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, supervisors)
	})
}

func TestGroupSupervisorRepository_EndSupervision_AlreadyEnded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("ending already ended supervision is no-op", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.TodayDate()
		endDate := today.AddDays(-1)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today.AddDays(-7),
			EndDate:   &endDate, // Already ended
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		// Try to end again - should not fail (idempotent)
		err = repo.EndSupervision(ctx, supervisor.ID)
		require.NoError(t, err)
	})
}

func TestGroupSupervisorRepository_EndSupervision_NonExistent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("ending non-existent supervision is no-op", func(t *testing.T) {
		// This should not fail - just won't update anything
		err := repo.EndSupervision(ctx, 999999)
		require.NoError(t, err)
	})
}

// ============================================================================
// EndAllActiveByStaffID Tests
// ============================================================================

func TestGroupSupervisorRepository_EndAllActiveByStaffID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	factory := repositories.NewFactory(db)
	repo := factory.GroupSupervisor
	ctx := testpkg.Ctx(t)

	t.Run("ends all active supervisions for staff", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		// End the active session up front. Tests in other packages run
		// EndDailySessions concurrently against the shared test DB, which
		// bulk-ends supervisors of every group with end_time IS NULL — racing
		// the count assertions below. EndAllActiveByStaffID filters only on
		// staff_id + end_date IS NULL, so an ended session changes nothing
		// about what this test exercises.
		require.NoError(t, factory.ActiveGroup.EndSession(ctx, data.ActiveGroup.ID))
		today := timezone.NewDate(2026, 8, 24)
		// Create multiple active supervisions for same staff
		supervisor1 := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "supervisor",
		}
		supervisor2 := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today,
			Role:      "assistant",
		}
		err := repo.Create(ctx, supervisor1)
		require.NoError(t, err)
		err = repo.Create(ctx, supervisor2)
		require.NoError(t, err)

		// End all active supervisions
		count, err := repo.EndAllActiveByStaffID(ctx, data.Staff1.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		// Verify both are ended
		found1, err := repo.FindByID(ctx, supervisor1.ID)
		require.NoError(t, err)
		assert.NotNil(t, found1.EndDate)

		found2, err := repo.FindByID(ctx, supervisor2.ID)
		require.NoError(t, err)
		assert.NotNil(t, found2.EndDate)
	})

	t.Run("returns zero for staff with no active supervisions", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		count, err := repo.EndAllActiveByStaffID(ctx, data.Staff2.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("does not affect already ended supervisions", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.NewDate(2026, 8, 24)
		endDate := today.AddDays(-1)
		supervisor := &active.GroupSupervisor{
			GroupID:   data.ActiveGroup.ID,
			StaffID:   data.Staff1.ID,
			StartDate: today.AddDays(-7),
			EndDate:   &endDate,
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)

		// Try to end - should not affect already ended supervision
		count, err := repo.EndAllActiveByStaffID(ctx, data.Staff1.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count) // Should not count already-ended supervisions

		// Verify end date hasn't changed
		found, err := repo.FindByID(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.Equal(t, endDate, *found.EndDate)
	})

	t.Run("ends only active supervisions for specific staff", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		today := timezone.NewDate(2026, 8, 24)
		// Create active supervisions for two different staff members
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
			Role:      "supervisor",
		}
		err := repo.Create(ctx, supervisor1)
		require.NoError(t, err)
		err = repo.Create(ctx, supervisor2)
		require.NoError(t, err)

		// End only Staff1's supervisions
		count, err := repo.EndAllActiveByStaffID(ctx, data.Staff1.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Verify Staff1's supervision is ended
		found1, err := repo.FindByID(ctx, supervisor1.ID)
		require.NoError(t, err)
		assert.NotNil(t, found1.EndDate)

		// Verify Staff2's supervision is still active
		found2, err := repo.FindByID(ctx, supervisor2.ID)
		require.NoError(t, err)
		assert.Nil(t, found2.EndDate)
	})
}

// ============================================================================
// FindAllActive Tests
// ============================================================================

// but are not exposed in the GroupSupervisorRepository interface, so they
// cannot be tested through the interface. These methods are unused in the
// codebase and account for the 6 uncovered lines (62.5% → ~68% coverage).

func TestGroupSupervisorRepository_EndByActiveGroupAndStaffID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor
	ctx := testpkg.Ctx(t)

	createSup := func(t *testing.T, groupID, staffID int64) *active.GroupSupervisor {
		sup := &active.GroupSupervisor{
			GroupID:   groupID,
			StaffID:   staffID,
			StartDate: timezone.NewDate(2026, 8, 24),
			Role:      "supervisor",
		}
		require.NoError(t, repo.Create(ctx, sup))
		return sup
	}

	t.Run("EndsActiveRowForMatchingPair", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		sup := createSup(t, data.ActiveGroup.ID, data.Staff1.ID)

		n, err := repo.EndByActiveGroupAndStaffID(ctx, data.ActiveGroup.ID, data.Staff1.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		// Verify end_date is now set.
		reloaded, err := repo.FindByID(ctx, sup.ID)
		require.NoError(t, err)
		assert.NotNil(t, reloaded.EndDate)
	})

	t.Run("Idempotent_ZeroRowsOnAlreadyEnded", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		createSup(t, data.ActiveGroup.ID, data.Staff1.ID)

		// First call ends it.
		n1, err := repo.EndByActiveGroupAndStaffID(ctx, data.ActiveGroup.ID, data.Staff1.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, n1)

		// Second call is idempotent: zero rows, no error.
		n2, err := repo.EndByActiveGroupAndStaffID(ctx, data.ActiveGroup.ID, data.Staff1.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, n2)
	})

	t.Run("LeavesOtherStaffUntouched", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		createSup(t, data.ActiveGroup.ID, data.Staff1.ID)
		sup2 := createSup(t, data.ActiveGroup.ID, data.Staff2.ID)

		n, err := repo.EndByActiveGroupAndStaffID(ctx, data.ActiveGroup.ID, data.Staff1.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		// Staff2 must remain active.
		reloaded, err := repo.FindByID(ctx, sup2.ID)
		require.NoError(t, err)
		assert.Nil(t, reloaded.EndDate, "other staff's supervision must remain open")
	})

	t.Run("TenantIsolation_DoesNotTouchOtherTenantRow", func(t *testing.T) {
		data := createSupervisorTestData(t, db)
		sup := createSup(t, data.ActiveGroup.ID, data.Staff1.ID)

		// Call from a different tenant context: must match zero rows.
		otherTenantCtx := testpkg.TenantContext(999)
		n, err := repo.EndByActiveGroupAndStaffID(otherTenantCtx, data.ActiveGroup.ID, data.Staff1.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, n)

		// Confirm the original row is still open from the correct tenant.
		reloaded, err := repo.FindByID(ctx, sup.ID)
		require.NoError(t, err)
		assert.Nil(t, reloaded.EndDate)
	})
}

// ============================================================================
// FindStaleOpen Tests
// ============================================================================

// createSupervisorRowForTenant inserts a supervisor row with explicit start
// and end dates under the supplied tenant.
func createSupervisorRowForTenant(t *testing.T, db *bun.DB, tenantID, staffID, groupID int64, startDate timezone.Date, endDate *timezone.Date) int64 {
	t.Helper()
	var id int64
	err := db.NewRaw(`
		INSERT INTO active.group_supervisors (staff_id, group_id, role, start_date, end_date, tenant_id)
		VALUES (?, ?, 'supervisor', ?, ?, ?)
		RETURNING id
	`, staffID, groupID, startDate, endDate, tenantID).Scan(testpkg.TenantContext(tenantID), &id)
	require.NoError(t, err)
	return id
}

func TestGroupSupervisorRepository_FindStaleOpen(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).GroupSupervisor

	tenantID := testpkg.UniqueTestTenantID(t)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureTestTenant(t, db, otherTenantID)

	ctx := testpkg.TenantContext(tenantID)
	today := timezone.TodayDate()
	yesterday := today.AddDays(-1)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Stale", "Finder")
	// Second staff: the partial unique index allows only one open supervision
	// per (tenant, staff, group, role).
	todayStaff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Fresh", "Finder")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, tenantID)

	staleID := createSupervisorRowForTenant(t, db, tenantID, staff.ID, group.ID, yesterday, nil)
	// Started today — not stale.
	createSupervisorRowForTenant(t, db, tenantID, todayStaff.ID, group.ID, today, nil)
	// Already closed — not stale.
	createSupervisorRowForTenant(t, db, tenantID, staff.ID, group.ID, yesterday, &yesterday)

	// Another tenant's stale row must stay invisible.
	otherStaff := testpkg.CreateTestStaffForTenant(t, db, otherTenantID, "Foreign", "Stale")
	otherGroup := testpkg.CreateTestActiveGroupForTenant(t, db, otherTenantID)
	createSupervisorRowForTenant(t, db, otherTenantID, otherStaff.ID, otherGroup.ID, yesterday, nil)

	stale, err := repo.FindStaleOpen(ctx, today)
	require.NoError(t, err)
	require.Len(t, stale, 1)
	assert.Equal(t, staleID, stale[0].ID)
	assert.Equal(t, staff.ID, stale[0].StaffID)
	assert.Nil(t, stale[0].EndDate)
}
