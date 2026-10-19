package contracttest_test

import (
	"testing"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

func TestGuestRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Guest
	ctx := testpkg.Ctx(t)

	t.Run("creates guest with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Guest", "Create")

		guest := &peopleValues.Guest{
			StaffID:           staff.ID,
			ActivityExpertise: "Music",
			Organization:      "Test Org",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, guest)
		require.NoError(t, err)
		assert.NotZero(t, guest.ID)

	})

	t.Run("creates guest with contact info", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Guest", "Contact")

		guest := &peopleValues.Guest{
			StaffID:           staff.ID,
			ActivityExpertise: "Art",
			ContactEmail:      "guest@test.local",
			ContactPhone:      "+49123456789",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, guest)
		require.NoError(t, err)
		assert.NotZero(t, guest.ID)

	})

	t.Run("creates guest with date range", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Guest", "Dates")

		startDate := calendar.TodayDate()
		endDate := startDate.AddDays(90)

		guest := &peopleValues.Guest{
			StaffID:           staff.ID,
			ActivityExpertise: "Dance",
			StartDate:         &startDate,
			EndDate:           &endDate,
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, guest)
		require.NoError(t, err)
		assert.NotZero(t, guest.ID)
		assert.NotNil(t, guest.StartDate)
		assert.NotNil(t, guest.EndDate)

	})

	t.Run("fails with nil guest", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails without staff ID", func(t *testing.T) {
		guest := &peopleValues.Guest{
			ActivityExpertise: "Music",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, guest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "staff ID")
	})

	t.Run("fails without activity expertise", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Guest", "NoExp")

		guest := &peopleValues.Guest{
			StaffID: staff.ID,
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, guest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "activity expertise")
	})

	t.Run("fails with invalid email", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Guest", "BadEmail")

		guest := &peopleValues.Guest{
			StaffID:           staff.ID,
			ActivityExpertise: "Music",
			ContactEmail:      "not-an-email",
		}

		err := testpkg.TenantCommand1(t, ctx, db, repo.Create, guest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "email")
	})
}

func TestGuestRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Guest
	ctx := testpkg.Ctx(t)

	t.Run("finds existing guest", func(t *testing.T) {
		guest := testpkg.CreateTestGuest(t, db, "FindByID")

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(guest.ID))
		require.NoError(t, err)
		assert.Equal(t, guest.ID, found.ID)
		assert.Equal(t, guest.ActivityExpertise, found.ActivityExpertise)
	})

	t.Run("returns error for non-existent guest", func(t *testing.T) {
		_, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(int64(999999)))
		require.Error(t, err)
	})
}

func TestGuestRepository_FindByStaffID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Guest
	ctx := testpkg.Ctx(t)

	t.Run("finds guest by staff ID", func(t *testing.T) {
		guest := testpkg.CreateTestGuest(t, db, "FindByStaff")

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByStaffID, guest.StaffID)
		require.NoError(t, err)
		assert.Equal(t, guest.ID, found.ID)
		assert.Equal(t, guest.StaffID, found.StaffID)
	})

	t.Run("returns error for non-existent staff ID", func(t *testing.T) {
		_, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByStaffID, int64(999999))
		require.Error(t, err)
	})
}

func TestGuestRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Guest
	ctx := testpkg.Ctx(t)

	t.Run("updates guest", func(t *testing.T) {
		guestFixture := testpkg.CreateTestGuest(t, db, "Update")
		guest, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(guestFixture.ID))
		require.NoError(t, err)

		guest.ActivityExpertise = "Updated Expertise"
		guest.Organization = "Updated Org"

		err = testpkg.TenantCommand1(t, ctx, db, repo.Update, guest)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(guest.ID))
		require.NoError(t, err)
		assert.Equal(t, "Updated Expertise", found.ActivityExpertise)
		assert.Equal(t, "Updated Org", found.Organization)
	})

	t.Run("fails with nil guest", func(t *testing.T) {
		err := testpkg.TenantCommand1(t, ctx, db, repo.Update, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGuestRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Guest
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing guest", func(t *testing.T) {
		guest := testpkg.CreateTestGuest(t, db, "Delete")

		err := testpkg.TenantCommand1(t, ctx, db, repo.Delete, any(guest.ID))
		require.NoError(t, err)

		_, err = testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(guest.ID))
		require.Error(t, err)
	})

	// NOTE: Base repository Delete returns nil for non-existent records
}

// ============================================================================
// Query Tests
// ============================================================================

// ============================================================================
// List and Filter Tests
// ============================================================================

func TestGuestRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Guest
	ctx := testpkg.Ctx(t)

	t.Run("lists guests with organization filter", func(t *testing.T) {
		guestFixture := testpkg.CreateTestGuest(t, db, "ListOrg")
		guest, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(guestFixture.ID))
		require.NoError(t, err)

		guest.Organization = "ListTestOrg"
		err = testpkg.TenantCommand1(t, ctx, db, repo.Update, guest)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.List, map[string]interface{}{
			"organization_like": "ListTest",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, found)
	})

	t.Run("lists guests with expertise filter", func(t *testing.T) {
		guestFixture := testpkg.CreateTestGuest(t, db, "ListExp")
		guest, err := testpkg.TenantQuery1(t, ctx, db, repo.FindByID, any(guestFixture.ID))
		require.NoError(t, err)

		guest.ActivityExpertise = "ListTestExpertise"
		err = testpkg.TenantCommand1(t, ctx, db, repo.Update, guest)
		require.NoError(t, err)

		found, err := testpkg.TenantQuery1(t, ctx, db, repo.List, map[string]interface{}{
			"expertise_like": "ListTest",
		})
		require.NoError(t, err)
		assert.NotEmpty(t, found)
	})
}
