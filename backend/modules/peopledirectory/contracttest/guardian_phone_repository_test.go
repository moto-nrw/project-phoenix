package contracttest_test

import (
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Create Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("creates phone number with valid data", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "create-phone")

		phone := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 123456",
			PhoneType:         userModels.PhoneTypeHome,
			IsPrimary:         true,
			Priority:          1,
		}

		err := repo.Create(ctx, phone)
		require.NoError(t, err)
		assert.NotZero(t, phone.ID)

		// Cleanup phone first, then guardian
	})

	t.Run("creates phone number with label", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "phone-with-label")

		label := "Büro"
		phone := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 987654",
			PhoneType:         userModels.PhoneTypeWork,
			Label:             &label,
			IsPrimary:         false,
			Priority:          2,
		}

		err := repo.Create(ctx, phone)
		require.NoError(t, err)
		assert.NotZero(t, phone.ID)
		assert.Equal(t, &label, phone.Label)

	})

	t.Run("fails with missing guardian profile ID", func(t *testing.T) {
		phone := &userModels.GuardianPhoneNumber{
			GuardianProfileID: 0, // Invalid
			PhoneNumber:       "+49 30 123456",
			PhoneType:         userModels.PhoneTypeMobile,
		}

		err := repo.Create(ctx, phone)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("fails with empty phone number", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "empty-phone")

		phone := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "",
			PhoneType:         userModels.PhoneTypeMobile,
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("finds existing phone number", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "find-phone")

		phone := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 111111",
			PhoneType:         userModels.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := repo.Create(ctx, phone)
		require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("finds all phone numbers for guardian", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "find-all-phones")

		// Create multiple phones
		phone1 := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 111111",
			PhoneType:         userModels.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := repo.Create(ctx, phone1)
		require.NoError(t, err)

		phone2 := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 222222",
			PhoneType:         userModels.PhoneTypeHome,
			IsPrimary:         false,
			Priority:          2,
		}
		err = repo.Create(ctx, phone2)
		require.NoError(t, err)

		phones, err := repo.FindByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Len(t, phones, 2)

		// Should be ordered by is_primary DESC, then priority ASC
		assert.True(t, phones[0].IsPrimary)
		assert.Equal(t, 1, phones[0].Priority)
	})

	t.Run("returns empty slice for guardian with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "no-phones")

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

// ============================================================================
// Update Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("updates phone number fields", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "update-phone")

		phone := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 777888",
			PhoneType:         userModels.PhoneTypeMobile,
			IsPrimary:         false,
			Priority:          1,
		}
		err := repo.Create(ctx, phone)
		require.NoError(t, err)

		// Update fields
		phone.PhoneNumber = "+49 30 999000"
		phone.PhoneType = userModels.PhoneTypeWork
		label := "Updated Label"
		phone.Label = &label
		phone.IsPrimary = true

		err = repo.Update(ctx, phone)
		require.NoError(t, err)

		// Verify update
		updated, err := repo.FindByID(ctx, phone.ID)
		require.NoError(t, err)
		assert.Equal(t, "+49 30 999000", updated.PhoneNumber)
		assert.Equal(t, userModels.PhoneTypeWork, updated.PhoneType)
		assert.Equal(t, &label, updated.Label)
		assert.True(t, updated.IsPrimary)
	})

	t.Run("returns error for non-existent phone", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "update-nonexistent")

		phone := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 123456",
			PhoneType:         userModels.PhoneTypeMobile,
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing phone number", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-phone")

		phone := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 111000",
			PhoneType:         userModels.PhoneTypeMobile,
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("sets phone as primary and unsets others", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary")

		// Create first phone as primary
		phone1 := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 222111",
			PhoneType:         userModels.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := repo.Create(ctx, phone1)
		require.NoError(t, err)

		// Create second phone as non-primary
		phone2 := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 333111",
			PhoneType:         userModels.PhoneTypeHome,
			IsPrimary:         false,
			Priority:          2,
		}
		err = repo.Create(ctx, phone2)
		require.NoError(t, err)

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

		err := repo.SetPrimary(ctx, int64(999999), guardian.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ============================================================================
// UnsetAllPrimary Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_UnsetAllPrimary(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("unsets all primary flags", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "unset-primary")

		phone := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 444111",
			PhoneType:         userModels.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := repo.Create(ctx, phone)
		require.NoError(t, err)

		err = repo.UnsetAllPrimary(ctx, guardian.ID)
		require.NoError(t, err)

		// Verify no phone is primary anymore
		phones, err := repo.FindByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)
		for _, p := range phones {
			assert.False(t, p.IsPrimary, "phone %d should no longer be primary", p.ID)
		}
	})

	t.Run("succeeds even with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "unset-empty")

		err := repo.UnsetAllPrimary(ctx, guardian.ID)
		require.NoError(t, err)
	})
}

// ============================================================================
// CountByGuardianID Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_CountByGuardianID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("counts phone numbers correctly", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "count-phones")

		// Create multiple phones
		for i := range 3 {
			phone := &userModels.GuardianPhoneNumber{
				GuardianProfileID: guardian.ID,
				PhoneNumber:       fmt.Sprintf("+49 30 %d00%d00", i+1, i+1),
				PhoneType:         userModels.PhoneTypeMobile,
				IsPrimary:         i == 0,
				Priority:          i + 1,
			}
			err := repo.Create(ctx, phone)
			require.NoError(t, err)
		}

		count, err := repo.CountByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("returns zero for guardian with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "count-zero")

		count, err := repo.CountByGuardianID(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

// ============================================================================
// GetNextPriority Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_GetNextPriority(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := testutil.NewPeopleRepositorySuiteFactory(db).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("returns 1 for guardian with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "next-priority-empty")

		priority, err := repo.GetNextPriority(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, priority)
	})

	t.Run("returns next priority value", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "next-priority")

		// Create phone with priority 3
		phone := &userModels.GuardianPhoneNumber{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 555111",
			PhoneType:         userModels.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          3,
		}
		err := repo.Create(ctx, phone)
		require.NoError(t, err)

		priority, err := repo.GetNextPriority(ctx, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 4, priority) // Should be max(3) + 1 = 4
	})
}
