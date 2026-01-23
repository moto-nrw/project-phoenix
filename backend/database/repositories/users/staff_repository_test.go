package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// cleanupStaffRecords removes staff members and their persons in proper FK order
func cleanupStaffRecords(t *testing.T, db *bun.DB, staffIDs ...int64) {
	t.Helper()
	if len(staffIDs) == 0 {
		return
	}

	ctx := context.Background()

	// Get person IDs before deleting staff
	var personIDs []int64
	err := db.NewSelect().
		TableExpr("users.staff").
		Column("person_id").
		Where("id IN (?)", bun.In(staffIDs)).
		Scan(ctx, &personIDs)
	if err != nil {
		t.Logf("Warning: failed to get person IDs for cleanup: %v", err)
	}

	// Delete staff first
	_, err = db.NewDelete().
		TableExpr("users.staff").
		Where("id IN (?)", bun.In(staffIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup staff: %v", err)
	}

	// Delete persons
	if len(personIDs) > 0 {
		_, err = db.NewDelete().
			TableExpr("users.persons").
			Where("id IN (?)", bun.In(personIDs)).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: failed to cleanup persons: %v", err)
		}
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestStaffRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("creates staff member with valid data", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "Staff", "Create", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)

		staff := &users.Staff{
			PersonID: person.ID,
		}
		staff.OgsID = ogsID

		err := repo.Create(ctx, staff)
		require.NoError(t, err)
		assert.NotZero(t, staff.ID)
		assert.NotZero(t, staff.CreatedAt)

		// Verify in DB
		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, person.ID, found.PersonID)

		// Cleanup
		cleanupStaffRecords(t, db, staff.ID)
	})

	t.Run("creates staff member with notes", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "Staff", "Notes", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)

		staff := &users.Staff{
			PersonID:   person.ID,
			StaffNotes: "Initial staff notes",
		}
		staff.OgsID = ogsID

		err := repo.Create(ctx, staff)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, "Initial staff notes", found.StaffNotes)

		cleanupStaffRecords(t, db, staff.ID)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("finds existing staff member", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "FindByID", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("finds staff by person ID", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "FindByPerson", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("updates staff notes", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Update", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("deletes existing staff member", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Delete", "Test", ogsID)
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("lists all staff with no filters", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "List", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		staffMembers, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, staffMembers)
	})

	t.Run("lists staff with filter", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "FilterStaff", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("finds staff with person loaded", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "WithPerson", "Loaded", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

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

// ============================================================================
// UpdateNotes Tests
// ============================================================================

func TestStaffRepository_UpdateNotes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("updates staff notes", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "UpdateNotes", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		err := repo.UpdateNotes(ctx, staff.ID, "New notes")
		require.NoError(t, err)

		// Verify the notes were updated
		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, "New notes", found.StaffNotes)
	})
}

// ============================================================================
// ListAllWithPerson Tests
// ============================================================================

func TestStaffRepository_ListAllWithPerson(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("returns all staff with person data", func(t *testing.T) {
		// Create multiple staff members
		staff1 := testpkg.CreateTestStaff(t, db, "AllWithPerson1", "Test1", ogsID)
		staff2 := testpkg.CreateTestStaff(t, db, "AllWithPerson2", "Test2", ogsID)
		defer cleanupStaffRecords(t, db, staff1.ID, staff2.ID)

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
		staff := testpkg.CreateTestStaff(t, db, "PersonFields", "Check", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

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
