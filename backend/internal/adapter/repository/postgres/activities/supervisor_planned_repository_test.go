package activities_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// createSupervisor is a helper to create a supervisor planned record
func createSupervisor(t *testing.T, db *bun.DB, staffID, groupID int64, isPrimary bool) *activities.SupervisorPlanned {
	t.Helper()

	ctx := context.Background()
	supervisor := &activities.SupervisorPlanned{
		StaffID:   staffID,
		GroupID:   groupID,
		IsPrimary: isPrimary,
	}

	err := db.NewInsert().
		Model(supervisor).
		ModelTableExpr(`activities.supervisors AS "supervisor"`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test supervisor")

	return supervisor
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestSupervisorPlannedRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("creates supervisor with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "SupervisorGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		supervisor := &activities.SupervisorPlanned{
			StaffID:   staff.ID,
			GroupID:   group.ID,
			IsPrimary: false,
		}

		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		assert.NotZero(t, supervisor.ID)

		testpkg.CleanupTableRecords(t, db, "activities.supervisors", supervisor.ID)
	})

	t.Run("creates primary supervisor", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Primary", "Supervisor")
		group := testpkg.CreateTestActivityGroup(t, db, "PrimaryGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		supervisor := &activities.SupervisorPlanned{
			StaffID:   staff.ID,
			GroupID:   group.ID,
			IsPrimary: true,
		}

		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		assert.True(t, supervisor.IsPrimary)

		testpkg.CleanupTableRecords(t, db, "activities.supervisors", supervisor.ID)
	})
}

func TestSupervisorPlannedRepository_Create_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("returns error when supervisor is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestSupervisorPlannedRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("finds existing supervisor", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Find", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "FindGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		supervisor := createSupervisor(t, db, staff.ID, group.ID, false)
		defer testpkg.CleanupTableRecords(t, db, "activities.supervisors", supervisor.ID)

		found, err := repo.FindByID(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.Equal(t, supervisor.ID, found.ID)
		assert.Equal(t, staff.ID, found.StaffID)
	})

	t.Run("returns error for non-existent supervisor", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestSupervisorPlannedRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("updates supervisor primary status", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Update", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "UpdateGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		supervisor := createSupervisor(t, db, staff.ID, group.ID, false)
		defer testpkg.CleanupTableRecords(t, db, "activities.supervisors", supervisor.ID)

		supervisor.IsPrimary = true
		err := repo.Update(ctx, supervisor)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.True(t, found.IsPrimary)
	})
}

func TestSupervisorPlannedRepository_Update_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("returns error when supervisor is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestSupervisorPlannedRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("deletes existing supervisor", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Delete", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "DeleteGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		supervisor := createSupervisor(t, db, staff.ID, group.ID, false)

		err := repo.Delete(ctx, supervisor.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, supervisor.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestSupervisorPlannedRepository_List(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("lists all supervisors", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "List", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "ListGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		supervisor := createSupervisor(t, db, staff.ID, group.ID, false)
		defer testpkg.CleanupTableRecords(t, db, "activities.supervisors", supervisor.ID)

		supervisors, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, supervisors)
	})
}

func TestSupervisorPlannedRepository_FindByStaffID(t *testing.T) {
	// Skip: FindByStaffID method tries to load Group relation which doesn't exist or has schema issues

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("finds supervisors by staff ID", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Staff", "MultiGroup")
		group1 := testpkg.CreateTestActivityGroup(t, db, "Group1")
		group2 := testpkg.CreateTestActivityGroup(t, db, "Group2")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, group1.CategoryID, 0)
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group2.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group1.ID, group2.ID)

		supervisor1 := createSupervisor(t, db, staff.ID, group1.ID, true)
		supervisor2 := createSupervisor(t, db, staff.ID, group2.ID, false)
		defer testpkg.CleanupTableRecords(t, db, "activities.supervisors", supervisor1.ID, supervisor2.ID)

		supervisors, err := repo.FindByStaffID(ctx, staff.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(supervisors), 2)

		// Verify our supervisors are in the results
		var foundIDs []int64
		for _, s := range supervisors {
			if s.ID == supervisor1.ID || s.ID == supervisor2.ID {
				foundIDs = append(foundIDs, s.ID)
			}
		}
		assert.Len(t, foundIDs, 2)
	})

	t.Run("returns empty for staff with no groups", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "NoGroups", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, 0, 0)

		supervisors, err := repo.FindByStaffID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, supervisors)
	})
}

func TestSupervisorPlannedRepository_FindByGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("finds supervisors by group ID with loaded relations", func(t *testing.T) {
		staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
		staff2 := testpkg.CreateTestStaff(t, db, "Staff", "Two")
		group := testpkg.CreateTestActivityGroup(t, db, "MultiSupervisor")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff1.ID, 0, group.CategoryID, 0)
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff2.ID, 0, 0, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		supervisor1 := createSupervisor(t, db, staff1.ID, group.ID, true)
		supervisor2 := createSupervisor(t, db, staff2.ID, group.ID, false)
		defer testpkg.CleanupTableRecords(t, db, "activities.supervisors", supervisor1.ID, supervisor2.ID)

		supervisors, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(supervisors), 2)

		// Verify our supervisors are in the results and have loaded relations
		var foundPrimary, foundSecondary bool
		for _, s := range supervisors {
			if s.ID == supervisor1.ID {
				foundPrimary = true
				assert.True(t, s.IsPrimary)
				// Check that staff and person are loaded
				assert.NotNil(t, s.Staff)
				if s.Staff != nil {
					assert.NotNil(t, s.Staff.Person)
				}
			}
			if s.ID == supervisor2.ID {
				foundSecondary = true
				assert.False(t, s.IsPrimary)
			}
		}
		assert.True(t, foundPrimary)
		assert.True(t, foundSecondary)
	})

	t.Run("returns empty for group with no supervisors", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "NoSupervisors")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		supervisors, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, supervisors)
	})
}

func TestSupervisorPlannedRepository_FindPrimaryByGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("finds primary supervisor for a group", func(t *testing.T) {
		staff1 := testpkg.CreateTestStaff(t, db, "Primary", "Supervisor")
		staff2 := testpkg.CreateTestStaff(t, db, "Secondary", "Supervisor")
		group := testpkg.CreateTestActivityGroup(t, db, "PrimarySupervisor")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff1.ID, 0, group.CategoryID, 0)
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff2.ID, 0, 0, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		primary := createSupervisor(t, db, staff1.ID, group.ID, true)
		secondary := createSupervisor(t, db, staff2.ID, group.ID, false)
		defer testpkg.CleanupTableRecords(t, db, "activities.supervisors", primary.ID, secondary.ID)

		found, err := repo.FindPrimaryByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, primary.ID, found.ID)
		assert.True(t, found.IsPrimary)
		// Check that staff and person are loaded
		assert.NotNil(t, found.Staff)
		if found.Staff != nil {
			assert.NotNil(t, found.Staff.Person)
		}
	})

	t.Run("returns error when no primary supervisor exists", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "NoPrimary", "Staff")
		group := testpkg.CreateTestActivityGroup(t, db, "NoPrimaryGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		supervisor := createSupervisor(t, db, staff.ID, group.ID, false)
		defer testpkg.CleanupTableRecords(t, db, "activities.supervisors", supervisor.ID)

		_, err := repo.FindPrimaryByGroupID(ctx, group.ID)
		require.Error(t, err)
	})
}

func TestSupervisorPlannedRepository_SetPrimary(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("sets supervisor as primary", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "SetPrimary", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "SetPrimaryGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		supervisor := createSupervisor(t, db, staff.ID, group.ID, false)
		defer testpkg.CleanupTableRecords(t, db, "activities.supervisors", supervisor.ID)

		err := repo.SetPrimary(ctx, supervisor.ID)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.True(t, found.IsPrimary)
	})
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestSupervisorPlannedRepository_Delete_NonExistent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := context.Background()

	t.Run("does not error when deleting non-existent supervisor", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}
