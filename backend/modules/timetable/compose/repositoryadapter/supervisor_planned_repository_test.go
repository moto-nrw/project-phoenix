package repositoryadapter_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/activities"
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

	ctx := testpkg.Ctx(t)
	supervisor := &activities.SupervisorPlanned{
		StaffID:   staffID,
		GroupID:   groupID,
		IsPrimary: isPrimary,
	}
	supervisor.SetTenantID(testpkg.Tenant(t))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("creates supervisor with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "SupervisorGroup")

		supervisor := &activities.SupervisorPlanned{
			StaffID:   staff.ID,
			GroupID:   group.ID,
			IsPrimary: false,
		}

		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		assert.NotZero(t, supervisor.ID)

	})

	t.Run("creates primary supervisor", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Primary", "Supervisor")
		group := testpkg.CreateTestActivityGroup(t, db, "PrimaryGroup")

		supervisor := &activities.SupervisorPlanned{
			StaffID:   staff.ID,
			GroupID:   group.ID,
			IsPrimary: true,
		}

		err := repo.Create(ctx, supervisor)
		require.NoError(t, err)
		assert.True(t, supervisor.IsPrimary)

	})
}

func TestSupervisorPlannedRepository_Create_WithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("returns error when supervisor is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestSupervisorPlannedRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("finds existing supervisor", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Find", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "FindGroup")

		supervisor := createSupervisor(t, db, staff.ID, group.ID, false)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("updates supervisor primary status", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Update", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "UpdateGroup")

		supervisor := createSupervisor(t, db, staff.ID, group.ID, false)

		supervisor.IsPrimary = true
		err := repo.Update(ctx, supervisor)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, supervisor.ID)
		require.NoError(t, err)
		assert.True(t, found.IsPrimary)
	})
}

func TestSupervisorPlannedRepository_Update_WithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("returns error when supervisor is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestSupervisorPlannedRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing supervisor", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Delete", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "DeleteGroup")

		supervisor := createSupervisor(t, db, staff.ID, group.ID, false)

		err := repo.Delete(ctx, supervisor.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, supervisor.ID)
		require.Error(t, err)
	})
}

func TestSupervisorPlannedRepository_DeleteByStaffID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Offboarded", "Supervisor")
	otherStaff := testpkg.CreateTestStaff(t, db, "Other", "Supervisor")
	groupA := testpkg.CreateTestActivityGroup(t, db, "DelByStaffA")
	groupB := testpkg.CreateTestActivityGroup(t, db, "DelByStaffB")

	createSupervisor(t, db, staff.ID, groupA.ID, true)
	createSupervisor(t, db, staff.ID, groupB.ID, false)
	createSupervisor(t, db, otherStaff.ID, groupA.ID, false)

	affected, err := repo.DeleteByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	remaining, err := repo.FindByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	assert.Empty(t, remaining)

	otherRemaining, err := repo.FindByStaffID(ctx, otherStaff.ID)
	require.NoError(t, err)
	assert.Len(t, otherRemaining, 1)
}

// ============================================================================
// Query Tests
// ============================================================================

func TestSupervisorPlannedRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("lists all supervisors", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "List", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "ListGroup")

		createSupervisor(t, db, staff.ID, group.ID, false)

		supervisors, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, supervisors)
	})
}

func TestSupervisorPlannedRepository_FindByStaffID(t *testing.T) {
	t.Parallel()

	// Skip: FindByStaffID method tries to load Group relation which doesn't exist or has schema issues

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("finds supervisors by staff ID", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Staff", "MultiGroup")
		group1 := testpkg.CreateTestActivityGroup(t, db, "Group1")
		group2 := testpkg.CreateTestActivityGroup(t, db, "Group2")

		supervisor1 := createSupervisor(t, db, staff.ID, group1.ID, true)
		supervisor2 := createSupervisor(t, db, staff.ID, group2.ID, false)

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

		supervisors, err := repo.FindByStaffID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, supervisors)
	})
}

func TestSupervisorPlannedRepository_FindByGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	// Person relations come from the People Directory composition (#2661),
	// so this test drives the composed repository the service graph uses.
	factory, err := repositories.NewFactoryWithPeopleDirectory(db)
	require.NoError(t, err)
	repo := factory.ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("finds supervisors by group ID with loaded relations", func(t *testing.T) {
		staff1 := testpkg.CreateTestStaff(t, db, "Staff", "One")
		staff2 := testpkg.CreateTestStaff(t, db, "Staff", "Two")
		group := testpkg.CreateTestActivityGroup(t, db, "MultiSupervisor")

		supervisor1 := createSupervisor(t, db, staff1.ID, group.ID, true)
		supervisor2 := createSupervisor(t, db, staff2.ID, group.ID, false)

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

		supervisors, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, supervisors)
	})
}

func TestSupervisorPlannedRepository_FindByGroupIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	// Person relations come from the People Directory composition (#2661),
	// so this test drives the composed repository the service graph uses.
	factory, err := repositories.NewFactoryWithPeopleDirectory(db)
	require.NoError(t, err)
	repo := factory.ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("finds supervisors for multiple groups", func(t *testing.T) {
		staff1 := testpkg.CreateTestStaff(t, db, "Multi1", "Staff")
		staff2 := testpkg.CreateTestStaff(t, db, "Multi2", "Staff")
		group1 := testpkg.CreateTestActivityGroup(t, db, "MultiGroup1")
		group2 := testpkg.CreateTestActivityGroup(t, db, "MultiGroup2")

		supervisor1 := createSupervisor(t, db, staff1.ID, group1.ID, true)
		supervisor2 := createSupervisor(t, db, staff2.ID, group2.ID, true)

		supervisors, err := repo.FindByGroupIDs(ctx, []int64{group1.ID, group2.ID})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(supervisors), 2)

		// Verify our supervisors are in the results with loaded relations
		var foundIDs []int64
		for _, s := range supervisors {
			if s.ID == supervisor1.ID || s.ID == supervisor2.ID {
				foundIDs = append(foundIDs, s.ID)
				// Check that staff and person are loaded
				assert.NotNil(t, s.Staff)
				if s.Staff != nil {
					assert.NotNil(t, s.Staff.Person)
				}
			}
		}
		assert.Len(t, foundIDs, 2)
	})

	t.Run("returns empty slice for empty group IDs", func(t *testing.T) {
		supervisors, err := repo.FindByGroupIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, supervisors)
	})

	t.Run("returns empty slice for non-existent group IDs", func(t *testing.T) {
		supervisors, err := repo.FindByGroupIDs(ctx, []int64{999998, 999999})
		require.NoError(t, err)
		assert.Empty(t, supervisors)
	})
}

func TestSupervisorPlannedRepository_SetPrimary(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("sets supervisor as primary", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "SetPrimary", "Test")
		group := testpkg.CreateTestActivityGroup(t, db, "SetPrimaryGroup")

		supervisor := createSupervisor(t, db, staff.ID, group.ID, false)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActivitySupervisor
	ctx := testpkg.Ctx(t)

	t.Run("does not error when deleting non-existent supervisor", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}
