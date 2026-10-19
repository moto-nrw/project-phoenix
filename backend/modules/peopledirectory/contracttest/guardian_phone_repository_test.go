package contracttest_test

import (
	"fmt"
	"testing"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api"

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

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("creates phone number with valid data", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "create-phone")

		phone := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 123456",
			PhoneType:         peopleValues.PhoneTypeHome,
			IsPrimary:         true,
			Priority:          1,
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone)
		require.NoError(t, err)
		assert.NotZero(t, phone.ID)

		// Cleanup phone first, then guardian
	})

	t.Run("creates phone number with label", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "phone-with-label")

		label := "Büro"
		phone := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 987654",
			PhoneType:         peopleValues.PhoneTypeWork,
			Label:             &label,
			IsPrimary:         false,
			Priority:          2,
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone)
		require.NoError(t, err)
		assert.NotZero(t, phone.ID)
		assert.Equal(t, &label, phone.Label)

	})

	t.Run("fails with missing guardian profile ID", func(t *testing.T) {
		phone := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: 0, // Invalid
			PhoneNumber:       "+49 30 123456",
			PhoneType:         peopleValues.PhoneTypeMobile,
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})

	t.Run("fails with empty phone number", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "empty-phone")

		phone := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "",
			PhoneType:         peopleValues.PhoneTypeMobile,
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone)
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

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("finds existing phone number", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "find-phone")

		phone := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 111111",
			PhoneType:         peopleValues.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, phone.ID)
		require.NoError(t, err)
		assert.Equal(t, phone.ID, found.ID)
		assert.Equal(t, phone.PhoneNumber, found.PhoneNumber)
		assert.Equal(t, phone.PhoneType, found.PhoneType)
	})

	t.Run("returns error for non-existent phone", func(t *testing.T) {
		_, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, int64(999999))
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

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("finds all phone numbers for guardian", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "find-all-phones")

		// Create multiple phones
		phone1 := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 111111",
			PhoneType:         peopleValues.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone1)
		require.NoError(t, err)

		phone2 := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 222222",
			PhoneType:         peopleValues.PhoneTypeHome,
			IsPrimary:         false,
			Priority:          2,
		}
		err = testpkg.TenantCommand1(t, ctx, db, repo.Create, phone2)
		require.NoError(t, err)

		phones, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByGuardianID, guardian.ID)
		require.NoError(t, err)
		assert.Len(t, phones, 2)

		// Should be ordered by is_primary DESC, then priority ASC
		assert.True(t, phones[0].IsPrimary)
		assert.Equal(t, 1, phones[0].Priority)
	})

	t.Run("returns empty slice for guardian with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "no-phones")

		phones, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByGuardianID, guardian.ID)
		require.NoError(t, err)
		assert.Empty(t, phones)
	})

	t.Run("returns empty slice for non-existent guardian", func(t *testing.T) {
		phones, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByGuardianID, int64(999999))
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

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("updates phone number fields", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "update-phone")

		phone := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 777888",
			PhoneType:         peopleValues.PhoneTypeMobile,
			IsPrimary:         false,
			Priority:          1,
		}
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone)
		require.NoError(t, err)

		// Update fields
		phone.PhoneNumber = "+49 30 999000"
		phone.PhoneType = peopleValues.PhoneTypeWork
		label := "Updated Label"
		phone.Label = &label
		phone.IsPrimary = true

		err = testpkg.TenantCommand1(t, ctx, db, repo.Update, phone)
		require.NoError(t, err)

		// Verify update
		updated, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, phone.ID)
		require.NoError(t, err)
		assert.Equal(t, "+49 30 999000", updated.PhoneNumber)
		assert.Equal(t, peopleValues.PhoneTypeWork, updated.PhoneType)
		assert.Equal(t, &label, updated.Label)
		assert.True(t, updated.IsPrimary)
	})

	t.Run("returns error for non-existent phone", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "update-nonexistent")

		phone := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 123456",
			PhoneType:         peopleValues.PhoneTypeMobile,
		}
		phone.ID = 999999 // Non-existent ID

		err := testpkg.TenantCommand1(t, ctx, db, repo.Update, phone)
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

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing phone number", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "delete-phone")

		phone := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 111000",
			PhoneType:         peopleValues.PhoneTypeMobile,
			Priority:          1,
		}
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone)
		require.NoError(t, err)

		err = testpkg.TenantCommand1(t, ctx, db, repo.Delete, phone.ID)
		require.NoError(t, err)

		// Verify deletion
		_, err = testpkg.TenantQuery1(t, ctx, db, repo.FindByID, phone.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error for non-existent phone", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.Delete, int64(999999))
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

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("sets phone as primary and unsets others", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary")

		// Create first phone as primary
		phone1 := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 222111",
			PhoneType:         peopleValues.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone1)
		require.NoError(t, err)

		// Create second phone as non-primary
		phone2 := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 333111",
			PhoneType:         peopleValues.PhoneTypeHome,
			IsPrimary:         false,
			Priority:          2,
		}
		err = testpkg.TenantCommand1(t, ctx, db, repo.Create, phone2)
		require.NoError(t, err)

		// Set phone2 as primary
		err = testpkg.TenantCommand2(t, ctx, db, repo.SetPrimary, phone2.ID, guardian.ID)
		require.NoError(t, err)

		// Verify phone2 is now primary
		updated2, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, phone2.ID)
		require.NoError(t, err)
		assert.True(t, updated2.IsPrimary)

		// Verify phone1 is no longer primary
		updated1, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, phone1.ID)
		require.NoError(t, err)
		assert.False(t, updated1.IsPrimary)
	})

	t.Run("returns error for non-existent phone", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "set-primary-error")

		err := testpkg.TenantCommand2(t, ctx, db, repo.SetPrimary, int64(999999), guardian.ID)
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

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("unsets all primary flags", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "unset-primary")

		phone := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 444111",
			PhoneType:         peopleValues.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          1,
		}
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone)
		require.NoError(t, err)

		err = testpkg.TenantCommand1(t, ctx, db, repo.UnsetAllPrimary, guardian.ID)
		require.NoError(t, err)

		// Verify no phone is primary anymore
		phones, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByGuardianID, guardian.ID)
		require.NoError(t, err)
		for _, p := range phones {
			assert.False(t, p.IsPrimary, "phone %d should no longer be primary", p.ID)
		}
	})

	t.Run("succeeds even with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "unset-empty")

		err := testpkg.TenantCommand1(t, ctx, db, repo.UnsetAllPrimary, guardian.ID)
		require.NoError(t, err)
	})
}

// ============================================================================
// CountByGuardianID Tests
// ============================================================================

func TestGuardianPhoneNumberRepository_CountByGuardianID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("counts phone numbers correctly", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "count-phones")

		// Create multiple phones
		for i := range 3 {
			phone := &peopleValues.GuardianProfilePhoneSnapshot{
				GuardianProfileID: guardian.ID,
				PhoneNumber:       fmt.Sprintf("+49 30 %d00%d00", i+1, i+1),
				PhoneType:         peopleValues.PhoneTypeMobile,
				IsPrimary:         i == 0,
				Priority:          i + 1,
			}
			err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone)
			require.NoError(t, err)
		}

		count, err := testpkg.TenantQuery1(t, ctx, db, repo.CountByGuardianID, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("returns zero for guardian with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "count-zero")

		count, err := testpkg.TenantQuery1(t, ctx, db, repo.CountByGuardianID, guardian.ID)
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

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).GuardianPhoneNumber
	ctx := testpkg.Ctx(t)

	t.Run("returns 1 for guardian with no phones", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "next-priority-empty")

		priority, err := testpkg.TenantQuery1(t, ctx, db, repo.GetNextPriority, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, priority)
	})

	t.Run("returns next priority value", func(t *testing.T) {
		guardian := testpkg.CreateTestGuardianProfile(t, db, "next-priority")

		// Create phone with priority 3
		phone := &peopleValues.GuardianProfilePhoneSnapshot{
			GuardianProfileID: guardian.ID,
			PhoneNumber:       "+49 30 555111",
			PhoneType:         peopleValues.PhoneTypeMobile,
			IsPrimary:         true,
			Priority:          3,
		}
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, phone)
		require.NoError(t, err)

		priority, err := testpkg.TenantQuery1(t, ctx, db, repo.GetNextPriority, guardian.ID)
		require.NoError(t, err)
		assert.Equal(t, 4, priority) // Should be max(3) + 1 = 4
	})
}
