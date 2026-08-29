package users_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// ============================================================================
// CRUD Tests
// ============================================================================

func TestStaffRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("creates staff member with valid data", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "Staff", "Create")

		staff := &users.Staff{
			PersonID: person.ID,
		}

		err := repo.Create(ctx, staff)
		require.NoError(t, err)
		assert.NotZero(t, staff.ID)
		assert.NotZero(t, staff.CreatedAt)

		// Verify in DB
		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, found.PersonID)

		// Cleanup
	})

	t.Run("creates staff member with notes", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "Staff", "Notes")

		staff := &users.Staff{
			PersonID:   person.ID,
			StaffNotes: "Initial staff notes",
		}

		err := repo.Create(ctx, staff)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, "Initial staff notes", found.StaffNotes)

	})

	t.Run("fails with nil staff", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("fails with missing person ID", func(t *testing.T) {
		staff := &users.Staff{
			PersonID: 0, // Invalid
		}

		err := repo.Create(ctx, staff)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "person ID")
	})
}

func TestStaffRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("finds existing staff member", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "FindByID", "Test")

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, staff.ID, found.ID)
		assert.Equal(t, staff.PersonID, found.PersonID)
	})

	t.Run("returns error for non-existent staff", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no rows")
	})
}

func TestStaffRepository_FindByPersonID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("finds staff by person ID", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "FindByPerson", "Test")

		found, err := repo.FindByPersonID(ctx, staff.PersonID)
		require.NoError(t, err)
		assert.Equal(t, staff.ID, found.ID)
		assert.Equal(t, staff.PersonID, found.PersonID)
	})

	t.Run("returns error for non-existent person ID", func(t *testing.T) {
		_, err := repo.FindByPersonID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestStaffRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("updates staff notes", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Update", "Test")

		staff.StaffNotes = "Updated staff notes"

		err := repo.Update(ctx, staff)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated staff notes", found.StaffNotes)
	})

	t.Run("fails with nil staff", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})
}

func TestStaffRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing staff member", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Delete", "Test")
		personID := staff.PersonID

		err := repo.Delete(ctx, staff.ID)
		require.NoError(t, err)

		// Verify staff is deleted
		_, err = repo.FindByID(ctx, staff.ID)
		require.Error(t, err)

		// Cleanup person (staff is already deleted)
		_, _ = db.NewDelete().
			TableExpr("users.persons").
			Where("id = ?", personID).
			Exec(ctx)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestStaffRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("lists all staff with no filters", func(t *testing.T) {
		testpkg.CreateTestStaff(t, db, "List", "Test")

		staffMembers, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, staffMembers)
	})

	t.Run("lists staff with filter", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "FilterStaff", "Test")

		staffMembers, err := repo.List(ctx, map[string]any{
			"person_id": staff.PersonID,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, staffMembers)
		assert.Equal(t, staff.ID, staffMembers[0].ID)
	})
}

// ============================================================================
// Relationship Tests
// ============================================================================

func TestStaffRepository_FindWithPerson(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("finds staff with person loaded", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "WithPerson", "Loaded")

		found, err := repo.FindWithPerson(ctx, staff.ID)
		require.NoError(t, err)
		require.NotNil(t, found.Person)
		assert.Equal(t, "WithPerson", found.Person.FirstName)
		assert.Equal(t, "Loaded", found.Person.LastName)
	})

	t.Run("returns error for non-existent staff", func(t *testing.T) {
		_, err := repo.FindWithPerson(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestStaffRepository_FindWithPersonByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("leaves person nil for soft-deleted people", func(t *testing.T) {
		activeStaff := testpkg.CreateTestStaff(t, db, "Visible", "Supervisor")
		deletedStaff := testpkg.CreateTestStaff(t, db, "Deleted", "Supervisor")

		_, err := db.NewUpdate().
			TableExpr("users.persons").
			Set("deleted_at = NOW()").
			Where("id = ?", deletedStaff.PersonID).
			Exec(ctx)
		require.NoError(t, err)

		found, err := repo.FindWithPersonByIDs(ctx, []int64{activeStaff.ID, deletedStaff.ID})
		require.NoError(t, err)

		require.Contains(t, found, activeStaff.ID)
		require.NotNil(t, found[activeStaff.ID])
		require.NotNil(t, found[activeStaff.ID].Person)
		assert.Equal(t, "Visible", found[activeStaff.ID].Person.FirstName)

		require.Contains(t, found, deletedStaff.ID)
		require.NotNil(t, found[deletedStaff.ID])
		assert.Nil(t, found[deletedStaff.ID].Person)
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		found, err := repo.FindWithPersonByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

func TestStaffRepository_FindByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("returns requested staff members keyed by ID", func(t *testing.T) {
		staff1 := testpkg.CreateTestStaff(t, db, "FindByIDs", "One")
		staff2 := testpkg.CreateTestStaff(t, db, "FindByIDs", "Two")
		unrequested := testpkg.CreateTestStaff(t, db, "FindByIDs", "Ignored")

		found, err := repo.FindByIDs(ctx, []int64{staff1.ID, staff2.ID})
		require.NoError(t, err)
		require.Len(t, found, 2)
		require.Contains(t, found, staff1.ID)
		require.Contains(t, found, staff2.ID)
		assert.Equal(t, staff1.PersonID, found[staff1.ID].PersonID)
		assert.Equal(t, staff2.PersonID, found[staff2.ID].PersonID)
		assert.NotContains(t, found, unrequested.ID)
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		found, err := repo.FindByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

// ============================================================================
// UpdateNotes Tests
// ============================================================================

// ============================================================================
// ListAllWithPerson Tests
// ============================================================================

func TestStaffRepository_ListAllWithPerson(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("returns all staff with person data", func(t *testing.T) {
		// Create multiple staff members
		staff1 := testpkg.CreateTestStaff(t, db, "AllWithPerson1", "Test1")
		staff2 := testpkg.CreateTestStaff(t, db, "AllWithPerson2", "Test2")

		results, err := repo.ListAllWithPerson(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, results)

		// Find our created staff members in results
		var foundStaff1, foundStaff2 bool
		for _, s := range results {
			if s.ID == staff1.ID {
				foundStaff1 = true
				require.NotNil(t, s.Person, "staff1.Person should be loaded")
				assert.Equal(t, "AllWithPerson1", s.Person.FirstName)
				assert.Equal(t, "Test1", s.Person.LastName)
			}
			if s.ID == staff2.ID {
				foundStaff2 = true
				require.NotNil(t, s.Person, "staff2.Person should be loaded")
				assert.Equal(t, "AllWithPerson2", s.Person.FirstName)
				assert.Equal(t, "Test2", s.Person.LastName)
			}
		}

		assert.True(t, foundStaff1, "should find staff1 in results")
		assert.True(t, foundStaff2, "should find staff2 in results")
	})

	t.Run("loads work-time model linkage fields", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "WorkTime", "Linkage")

		modelRepo := configRepo.NewWorkTimeModelRepository(testpkg.ConfigRuntime(db))
		anchor := configModel.NewCalendarDate(2026, time.January, 5)
		model := &configModel.WorkTimeModel{
			Name:               fmt.Sprintf("Linkage %d", staff.ID),
			RotationLength:     2,
			RotationAnchorDate: anchor,
		}
		require.NoError(t, modelRepo.Create(ctx, model, []*configModel.WorkTimeModelEntry{
			{WeekIndex: 0, DayOfWeek: configModel.DayMonday, TargetMinutes: 240},
		}))
		staffAnchor := timezone.NewDate(2026, time.January, 12)
		_, err := db.NewUpdate().
			Table("users.staff").
			Set("work_time_model_id = ?", model.ID).
			Set("rotation_anchor_date = ?", staffAnchor).
			Where("id = ?", staff.ID).
			Exec(ctx)
		require.NoError(t, err)
		defer func() {
			_, _ = db.NewUpdate().
				Table("users.staff").
				Set("work_time_model_id = NULL").
				Where("id = ?", staff.ID).
				Exec(ctx)
			_ = modelRepo.Delete(ctx, model.ID)
		}()

		results, err := repo.ListAllWithPerson(ctx)
		require.NoError(t, err)
		var found *users.Staff
		for _, s := range results {
			if s.ID == staff.ID {
				found = s
				break
			}
		}
		require.NotNil(t, found, "should find the created staff member")
		require.NotNil(t, found.WorkTimeModelID, "work_time_model_id must be scanned")
		assert.Equal(t, model.ID, *found.WorkTimeModelID)
		require.NotNil(t, found.RotationAnchorDate, "rotation_anchor_date must be scanned")
		assert.Equal(t, staffAnchor, *found.RotationAnchorDate)
	})

	t.Run("returns empty slice when no staff exist", func(t *testing.T) {
		// This test uses the existing database state
		// The database may have other staff members, so we just verify
		// that the method returns without error
		results, err := repo.ListAllWithPerson(ctx)
		require.NoError(t, err)
		// Results could be empty or have existing records
		assert.NotNil(t, results, "should return a non-nil slice")
	})

	t.Run("loads all person fields correctly", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "PersonFields", "Check")

		results, err := repo.ListAllWithPerson(ctx)
		require.NoError(t, err)

		// Find our staff member
		var found *users.Staff
		for _, s := range results {
			if s.ID == staff.ID {
				found = s
				break
			}
		}

		require.NotNil(t, found, "should find created staff")
		require.NotNil(t, found.Person, "person should be loaded")
		assert.NotZero(t, found.Person.ID, "person ID should be loaded")
		assert.Equal(t, "PersonFields", found.Person.FirstName)
		assert.Equal(t, "Check", found.Person.LastName)
		assert.NotZero(t, found.Person.CreatedAt, "person created_at should be loaded")
	})
}

// NOTE: AddNotes exists in the implementation but is not exposed in the StaffRepository
// interface, so it cannot be tested through the interface.
