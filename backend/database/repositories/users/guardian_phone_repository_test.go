package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Create Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("creates phone number with valid data", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "create-phone")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 123456",
			PhoneType:         users.PhoneTypeHome,
			IsPrimary:         true,
			Priority:          1,
		}

		err := repo.Create(ctx, phone)
		require.NoError(t, err)
		assert.NotZero(t, phone.ID)

		// Cleanup phone first, then guardian
		testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone.ID)
	})

	t.Run("creates phone number with label", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "phone-with-label")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		label := "Büro"
		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 987654",
			PhoneType:         users.PhoneTypeWork,
			Label:             &label,
			IsPrimary:         false,
			Priority:          2,
		}

		err := repo.Create(ctx, phone)
		require.NoError(t, err)
		assert.NotZero(t, phone.ID)
		assert.Equal(t, &label, phone.Label)

		testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone.ID)
	})

	t.Run("fails with missing guardian profile ID", func(t *testing.T) {
		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: 0, // Invalid
			PhoneNumber:       "+49 30 123456",
			PhoneType:         users.PhoneTypeMobile,
		}

		err := repo.Create(ctx, phone)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("fails with empty phone number", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "empty-phone")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "",
			PhoneType:         users.PhoneTypeMobile,
		}

		err := repo.Create(ctx, phone)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

// ============================================================================
// FindByID Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("finds existing phone number", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "find-phone")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 111111",
			PhoneType:         users.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := repo.Create(ctx, phone)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone.ID)

		found, err := repo.FindByID(ctx, phone.ID)
		require.NoError(t, err)
		assert.Equal(t, phone.ID, found.ID)
		assert.Equal(t, phone.PhoneNumber, found.PhoneNumber)
		assert.Equal(t, phone.PhoneType, found.PhoneType)
	})

	t.Run("returns error for non-existent phone", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ============================================================================
// FindByGuardianID Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_FindByGuardianID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("finds all phone numbers for guardian", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "find-all-phones")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		// Create multiple phones
		phone1 := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 111111",
			PhoneType:         users.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := repo.Create(ctx, phone1)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone1.ID)

		phone2 := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 222222",
			PhoneType:         users.PhoneTypeHome,
			IsPrimary:         false,
			Priority:          2,
		}
		err = repo.Create(ctx, phone2)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone2.ID)

		phones, err := repo.FindByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Len(t, phones, 2)

		// Should be ordered by is_primary DESC, then priority ASC
		assert.True(t, phones[0].IsPrimary)
		assert.Equal(t, 1, phones[0].Priority)
	})

	t.Run("returns empty slice for guardian with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "no-phones")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		phones, err := repo.FindByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Empty(t, phones)
	})

	t.Run("returns empty slice for non-existent guardian", func(t *testing.T) {
		phones, err := repo.FindByGuardianID(ctx, int64(999999))
		require.NoError(t, err)
		assert.Empty(t, phones)
	})
}

// ============================================================================
// GetPrimary Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_GetPrimary(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("gets primary phone number", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "get-primary")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		// Create primary phone
		primaryPhone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 PRIMARY",
			PhoneType:         users.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := repo.Create(ctx, primaryPhone)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", primaryPhone.ID)

		// Create non-primary phone
		secondaryPhone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 SECONDARY",
			PhoneType:         users.PhoneTypeHome,
			IsPrimary:         false,
			Priority:          2,
		}
		err = repo.Create(ctx, secondaryPhone)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", secondaryPhone.ID)

		primary, err := repo.GetPrimary(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, primaryPhone.ID, primary.ID)
		assert.True(t, primary.IsPrimary)
	})

	t.Run("returns error when no primary exists", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "no-primary")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		// Create non-primary phone only
		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 NOT-PRIMARY",
			PhoneType:         users.PhoneTypeMobile,
			IsPrimary:         false,
			Priority:          1,
		}
		err := repo.Create(ctx, phone)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone.ID)

		_, err = repo.GetPrimary(ctx, guardian.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ============================================================================
// Update Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("updates phone number fields", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "update-phone")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 ORIGINAL",
			PhoneType:         users.PhoneTypeMobile,
			IsPrimary:         false,
			Priority:          1,
		}
		err := repo.Create(ctx, phone)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone.ID)

		// Update fields
		phone.PhoneNumber = "+49 30 UPDATED"
		phone.PhoneType = users.PhoneTypeWork
		label := "Updated Label"
		phone.Label = &label
		phone.IsPrimary = true

		err = repo.Update(ctx, phone)
		require.NoError(t, err)

		// Verify update
		updated, err := repo.FindByID(ctx, phone.ID)
		require.NoError(t, err)
		assert.Equal(t, "+49 30 UPDATED", updated.PhoneNumber)
		assert.Equal(t, users.PhoneTypeWork, updated.PhoneType)
		assert.Equal(t, &label, updated.Label)
		assert.True(t, updated.IsPrimary)
	})

	t.Run("returns error for non-existent phone", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "update-nonexistent")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 123456",
			PhoneType:         users.PhoneTypeMobile,
		}
		phone.ID = 999999 // Non-existent ID

		err := repo.Update(ctx, phone)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ============================================================================
// Delete Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("deletes existing phone number", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-phone")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 DELETE-ME",
			PhoneType:         users.PhoneTypeMobile,
			Priority:          1,
		}
		err := repo.Create(ctx, phone)
		require.NoError(t, err)

		err = repo.Delete(ctx, phone.ID)
		require.NoError(t, err)

		// Verify deletion
		_, err = repo.FindByID(ctx, phone.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error for non-existent phone", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ============================================================================
// SetPrimary Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_SetPrimary(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("sets phone as primary and unsets others", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		// Create first phone as primary
		phone1 := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 FIRST",
			PhoneType:         users.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := repo.Create(ctx, phone1)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone1.ID)

		// Create second phone as non-primary
		phone2 := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 SECOND",
			PhoneType:         users.PhoneTypeHome,
			IsPrimary:         false,
			Priority:          2,
		}
		err = repo.Create(ctx, phone2)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone2.ID)

		// Set phone2 as primary
		err = repo.SetPrimary(ctx, phone2.ID, guardian.ID)
		require.NoError(t, err)

		// Verify phone2 is now primary
		updated2, err := repo.FindByID(ctx, phone2.ID)
		require.NoError(t, err)
		assert.True(t, updated2.IsPrimary)

		// Verify phone1 is no longer primary
		updated1, err := repo.FindByID(ctx, phone1.ID)
		require.NoError(t, err)
		assert.False(t, updated1.IsPrimary)
	})

	t.Run("returns error for non-existent phone", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary-error")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		err := repo.SetPrimary(ctx, int64(999999), guardian.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ============================================================================
// UnsetAllPrimary Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_UnsetAllPrimary(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("unsets all primary flags", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "unset-primary")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		// Create primary phone
		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 PRIMARY",
			PhoneType:         users.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := repo.Create(ctx, phone)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone.ID)

		err = repo.UnsetAllPrimary(ctx, guardian.ID)
		require.NoError(t, err)

		// Verify no primary exists
		_, err = repo.GetPrimary(ctx, guardian.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("succeeds even with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "unset-empty")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		err := repo.UnsetAllPrimary(ctx, guardian.ID)
		require.NoError(t, err)
	})
}

// ============================================================================
// CountByGuardianID Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_CountByGuardianID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("counts phone numbers correctly", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "count-phones")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		// Create multiple phones
		for i := range 3 {
			phone := &users.GuardianPhoneNumber{
				GuardianProfileID: guardian.ID,
				PhoneNumber:       "+49 30 " + string(rune('0'+i)),
				PhoneType:         users.PhoneTypeMobile,
				IsPrimary:         i == 0,
				Priority:          i + 1,
			}
			err := repo.Create(ctx, phone)
			require.NoError(t, err)
			defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone.ID)
		}

		count, err := repo.CountByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("returns zero for guardian with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "count-zero")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		count, err := repo.CountByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

// ============================================================================
// DeleteByGuardianID Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_DeleteByGuardianID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("deletes all phone numbers for guardian", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-all")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		// Create multiple phones
		for i := range 2 {
			phone := &users.GuardianPhoneNumber{
				GuardianProfileID: guardian.ID,
				PhoneNumber:       "+49 30 DELETE-" + string(rune('0'+i)),
				PhoneType:         users.PhoneTypeMobile,
				IsPrimary:         i == 0,
				Priority:          i + 1,
			}
			err := repo.Create(ctx, phone)
			require.NoError(t, err)
			// No defer needed - we'll delete all
		}

		err := repo.DeleteByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)

		// Verify all deleted
		phones, err := repo.FindByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Empty(t, phones)
	})

	t.Run("succeeds even with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-empty")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		err := repo.DeleteByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)
	})
}

// ============================================================================
// GetNextPriority Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_GetNextPriority(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).GuardianPhoneNumber
	ctx := context.Background()

	t.Run("returns 1 for guardian with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "next-priority-empty")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		priority, err := repo.GetNextPriority(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, priority)
	})

	t.Run("returns next priority value", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "next-priority")
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_profiles", guardian.ID)

		// Create phone with priority 3
		phone := &users.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 PRIORITY",
			PhoneType:         users.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          3,
		}
		err := repo.Create(ctx, phone)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "users.guardian_phone_numbers", phone.ID)

		priority, err := repo.GetNextPriority(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 4, priority) // Should be max(3) + 1 = 4
	})
}
