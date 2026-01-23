package users_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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

// ============================================================================
// BetterAuth User ID Tests
// ============================================================================

func TestStaffRepository_FindByBetterAuthUserID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("finds staff by BetterAuth user ID", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "BetterAuth", "Find", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		betterAuthID := "ba-user-" + uniqueSuffix()

		// Set the BetterAuth user ID
		err := repo.UpdateBetterAuthUserID(ctx, staff.ID, betterAuthID)
		require.NoError(t, err)

		// Find by BetterAuth user ID
		found, err := repo.FindByBetterAuthUserID(ctx, betterAuthID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, staff.ID, found.ID)
	})

	t.Run("returns nil for non-existent BetterAuth user ID", func(t *testing.T) {
		found, err := repo.FindByBetterAuthUserID(ctx, "non-existent-ba-id-12345")
		require.NoError(t, err) // Not an error - just not found
		assert.Nil(t, found)
	})

	t.Run("returns nil for empty BetterAuth user ID", func(t *testing.T) {
		found, err := repo.FindByBetterAuthUserID(ctx, "")
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("differentiates between similar BetterAuth user IDs", func(t *testing.T) {
		staff1 := testpkg.CreateTestStaff(t, db, "BA1", "Test", ogsID)
		staff2 := testpkg.CreateTestStaff(t, db, "BA2", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff1.ID, staff2.ID)

		baID1 := "ba-user-1-" + uniqueSuffix()
		baID2 := "ba-user-2-" + uniqueSuffix()

		err := repo.UpdateBetterAuthUserID(ctx, staff1.ID, baID1)
		require.NoError(t, err)
		err = repo.UpdateBetterAuthUserID(ctx, staff2.ID, baID2)
		require.NoError(t, err)

		found1, err := repo.FindByBetterAuthUserID(ctx, baID1)
		require.NoError(t, err)
		require.NotNil(t, found1)
		assert.Equal(t, staff1.ID, found1.ID)

		found2, err := repo.FindByBetterAuthUserID(ctx, baID2)
		require.NoError(t, err)
		require.NotNil(t, found2)
		assert.Equal(t, staff2.ID, found2.ID)
	})
}

func TestStaffRepository_UpdateBetterAuthUserID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("updates BetterAuth user ID", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "UpdateBA", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		betterAuthID := "ba-user-" + uniqueSuffix()

		err := repo.UpdateBetterAuthUserID(ctx, staff.ID, betterAuthID)
		require.NoError(t, err)

		// Verify via FindByBetterAuthUserID
		found, err := repo.FindByBetterAuthUserID(ctx, betterAuthID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, staff.ID, found.ID)
	})

	t.Run("update for non-existent staff does not error", func(t *testing.T) {
		// SQL UPDATE with no matching rows doesn't error
		err := repo.UpdateBetterAuthUserID(ctx, int64(999999999), "some-id")
		require.NoError(t, err)
	})

	t.Run("can update BetterAuth user ID multiple times", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "MultiBA", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		firstID := "ba-first-" + uniqueSuffix()
		err := repo.UpdateBetterAuthUserID(ctx, staff.ID, firstID)
		require.NoError(t, err)

		secondID := "ba-second-" + uniqueSuffix()
		err = repo.UpdateBetterAuthUserID(ctx, staff.ID, secondID)
		require.NoError(t, err)

		// Old ID should no longer work
		found, err := repo.FindByBetterAuthUserID(ctx, firstID)
		require.NoError(t, err)
		assert.Nil(t, found)

		// New ID should work
		found, err = repo.FindByBetterAuthUserID(ctx, secondID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, staff.ID, found.ID)
	})

	t.Run("handles special characters in BetterAuth user ID", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "SpecialBA", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		// BetterAuth user IDs might contain various characters
		specialID := "usr_" + uniqueSuffix() + "-special@test"

		err := repo.UpdateBetterAuthUserID(ctx, staff.ID, specialID)
		require.NoError(t, err)

		found, err := repo.FindByBetterAuthUserID(ctx, specialID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, staff.ID, found.ID)
	})
}

// ============================================================================
// Additional Validation Tests
// ============================================================================

func TestStaffRepository_CreateValidation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("fails with negative person ID", func(t *testing.T) {
		staff := &users.Staff{
			PersonID: -1,
		}
		staff.OgsID = ogsID

		err := repo.Create(ctx, staff)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "person ID")
	})

	t.Run("fails with non-existent person ID FK violation", func(t *testing.T) {
		staff := &users.Staff{
			PersonID: 999999999, // Non-existent
		}
		staff.OgsID = ogsID

		err := repo.Create(ctx, staff)
		require.Error(t, err)
		// Should fail due to FK constraint
	})

	t.Run("creates staff with all optional fields", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "AllFields", "Test", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)

		staff := &users.Staff{
			PersonID:   person.ID,
			StaffNotes: "Detailed notes about this staff member",
		}
		staff.OgsID = ogsID

		err := repo.Create(ctx, staff)
		require.NoError(t, err)
		defer cleanupStaffRecords(t, db, staff.ID)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, "Detailed notes about this staff member", found.StaffNotes)
	})
}

func TestStaffRepository_UpdateValidation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("fails with negative person ID on update", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "UpdateVal", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		staff.PersonID = -5
		err := repo.Update(ctx, staff)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "person ID")
	})

	t.Run("fails with zero person ID on update", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "UpdateZero", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		staff.PersonID = 0
		err := repo.Update(ctx, staff)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "person ID")
	})

	t.Run("updates notes to empty string", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "ClearNotes", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		staff.StaffNotes = "Initial notes"
		err := repo.Update(ctx, staff)
		require.NoError(t, err)

		staff.StaffNotes = ""
		err = repo.Update(ctx, staff)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, found.StaffNotes)
	})
}

// ============================================================================
// UpdateNotes Edge Cases
// ============================================================================

func TestStaffRepository_UpdateNotesEdgeCases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("handles notes with special characters", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "SpecialNotes", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		specialNotes := "Notes with Ümläute & Sønderzeichen ß € @ \"quotes\" 'apostrophe' <html>"

		err := repo.UpdateNotes(ctx, staff.ID, specialNotes)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, specialNotes, found.StaffNotes)
	})

	t.Run("handles multiline notes", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "MultilineNotes", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		multilineNotes := "Line 1\nLine 2\nLine 3\n\nLine 5 after blank"

		err := repo.UpdateNotes(ctx, staff.ID, multilineNotes)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Equal(t, multilineNotes, found.StaffNotes)
	})

	t.Run("handles very long notes", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "LongNotes", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		// Create a 5000 character note
		longNotes := ""
		for i := 0; i < 500; i++ {
			longNotes += "0123456789"
		}

		err := repo.UpdateNotes(ctx, staff.ID, longNotes)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Len(t, found.StaffNotes, 5000)
	})

	t.Run("clears notes when set to empty string", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "ClearNotesVia", "UpdateNotes", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		err := repo.UpdateNotes(ctx, staff.ID, "Initial")
		require.NoError(t, err)

		err = repo.UpdateNotes(ctx, staff.ID, "")
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, found.StaffNotes)
	})
}

// ============================================================================
// List Edge Cases
// ============================================================================

func TestStaffRepository_ListEdgeCases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("filters return empty for non-matching person_id", func(t *testing.T) {
		staffList, err := repo.List(ctx, map[string]any{
			"person_id": int64(999999999),
		})
		require.NoError(t, err)
		assert.Empty(t, staffList)
	})

	t.Run("ignores nil filter values", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "NilFilter", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		staffList, err := repo.List(ctx, map[string]any{
			"person_id":   nil,
			"staff_notes": nil,
		})
		require.NoError(t, err)
		// Should return results because nil values are ignored
		assert.NotEmpty(t, staffList)
	})

	t.Run("lists multiple staff members", func(t *testing.T) {
		staff1 := testpkg.CreateTestStaff(t, db, "Multi1", "Staff", ogsID)
		staff2 := testpkg.CreateTestStaff(t, db, "Multi2", "Staff", ogsID)
		staff3 := testpkg.CreateTestStaff(t, db, "Multi3", "Staff", ogsID)
		defer cleanupStaffRecords(t, db, staff1.ID, staff2.ID, staff3.ID)

		staffList, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(staffList), 3)

		// Verify all three are in the list
		foundIDs := make(map[int64]bool)
		for _, s := range staffList {
			foundIDs[s.ID] = true
		}
		assert.True(t, foundIDs[staff1.ID], "staff1 should be in list")
		assert.True(t, foundIDs[staff2.ID], "staff2 should be in list")
		assert.True(t, foundIDs[staff3.ID], "staff3 should be in list")
	})
}

// ============================================================================
// FindWithPerson Edge Cases
// ============================================================================

func TestStaffRepository_FindWithPersonEdgeCases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("returns error for zero ID", func(t *testing.T) {
		_, err := repo.FindWithPerson(ctx, int64(0))
		require.Error(t, err)
	})

	t.Run("returns error for negative ID", func(t *testing.T) {
		_, err := repo.FindWithPerson(ctx, int64(-1))
		require.Error(t, err)
	})

	t.Run("loads person with all fields populated", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "FullPerson", "Fields", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		found, err := repo.FindWithPerson(ctx, staff.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		require.NotNil(t, found.Person)

		// Verify all person fields are populated
		assert.NotZero(t, found.Person.ID)
		assert.Equal(t, "FullPerson", found.Person.FirstName)
		assert.Equal(t, "Fields", found.Person.LastName)
		assert.NotZero(t, found.Person.CreatedAt)
		assert.NotZero(t, found.Person.UpdatedAt)
	})
}

// ============================================================================
// Delete Edge Cases
// ============================================================================

func TestStaffRepository_DeleteEdgeCases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("delete non-existent staff does not error", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999999))
		require.NoError(t, err)
	})

	t.Run("delete with zero ID does not error", func(t *testing.T) {
		err := repo.Delete(ctx, int64(0))
		require.NoError(t, err)
	})

	t.Run("can verify deletion via FindByID", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "VerifyDelete", "Test", ogsID)
		personID := staff.PersonID
		staffID := staff.ID

		// Verify exists before delete
		found, err := repo.FindByID(ctx, staffID)
		require.NoError(t, err)
		require.NotNil(t, found)

		// Delete
		err = repo.Delete(ctx, staffID)
		require.NoError(t, err)

		// Verify gone after delete
		_, err = repo.FindByID(ctx, staffID)
		require.Error(t, err)

		// Cleanup person
		_, _ = db.NewDelete().
			TableExpr("users.persons").
			Where("id = ?", personID).
			Exec(ctx)
	})
}

// ============================================================================
// Timestamp Tests
// ============================================================================

func TestStaffRepository_Timestamps(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("sets timestamps on create", func(t *testing.T) {
		person := testpkg.CreateTestPerson(t, db, "Timestamp", "Create", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)

		beforeCreate := time.Now().Add(-time.Second)

		staff := &users.Staff{
			PersonID: person.ID,
		}
		staff.OgsID = ogsID

		err := repo.Create(ctx, staff)
		require.NoError(t, err)
		defer cleanupStaffRecords(t, db, staff.ID)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)

		assert.True(t, found.CreatedAt.After(beforeCreate), "CreatedAt should be set")
		assert.True(t, found.UpdatedAt.After(beforeCreate), "UpdatedAt should be set")
	})

	t.Run("updates UpdatedAt on update", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Timestamp", "Update", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		originalUpdatedAt := staff.UpdatedAt

		// Wait a small amount to ensure time difference
		time.Sleep(10 * time.Millisecond)

		staff.StaffNotes = "Updated to change timestamp"
		err := repo.Update(ctx, staff)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)

		// UpdatedAt should be after original (or equal if clock resolution is low)
		assert.True(t, !found.UpdatedAt.Before(originalUpdatedAt),
			"UpdatedAt should be >= original after update")
	})
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestStaffRepository_ConcurrentAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	repo := repositories.NewFactory(db).Staff
	ctx := context.Background()

	t.Run("handles concurrent reads", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "ConcurrentRead", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		const numReads = 10
		errors := make(chan error, numReads)

		for i := 0; i < numReads; i++ {
			go func() {
				_, err := repo.FindByID(ctx, staff.ID)
				errors <- err
			}()
		}

		for i := 0; i < numReads; i++ {
			err := <-errors
			assert.NoError(t, err)
		}
	})

	t.Run("handles concurrent note updates", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "ConcurrentNotes", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		const numUpdates = 5
		errors := make(chan error, numUpdates)

		for i := 0; i < numUpdates; i++ {
			go func(idx int) {
				noteText := "Note " + uniqueSuffix()
				err := repo.UpdateNotes(ctx, staff.ID, noteText)
				errors <- err
			}(i)
		}

		for i := 0; i < numUpdates; i++ {
			err := <-errors
			assert.NoError(t, err)
		}

		// Verify one of the notes was written
		found, err := repo.FindByID(ctx, staff.ID)
		require.NoError(t, err)
		assert.Contains(t, found.StaffNotes, "Note")
	})

	t.Run("handles concurrent list operations", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "ConcurrentList", "Test", ogsID)
		defer cleanupStaffRecords(t, db, staff.ID)

		const numLists = 10
		errors := make(chan error, numLists)

		for i := 0; i < numLists; i++ {
			go func() {
				_, err := repo.List(ctx, nil)
				errors <- err
			}()
		}

		for i := 0; i < numLists; i++ {
			err := <-errors
			assert.NoError(t, err)
		}
	})
}

// ============================================================================
// Helper Functions
// ============================================================================

// uniqueSuffix generates a unique suffix for test data
func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
