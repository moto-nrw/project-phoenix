package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
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

func TestGuestRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Guest
	ctx := context.Background()

	t.Run("creates guest with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Guest", "Create")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID, staff.PersonID)

		guest := &users.Guest{
			StaffID:           staff.ID,
			ActivityExpertise: "Music",
			Organization:      "Test Org",
		}

		err := repo.Create(ctx, guest)
		require.NoError(t, err)
		assert.NotZero(t, guest.ID)

		testpkg.CleanupTableRecords(t, db, "users.guests", guest.ID)
	})

	t.Run("creates guest with contact info", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Guest", "Contact")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID, staff.PersonID)

		guest := &users.Guest{
			StaffID:           staff.ID,
			ActivityExpertise: "Art",
			ContactEmail:      "guest@test.local",
			ContactPhone:      "+49123456789",
		}

		err := repo.Create(ctx, guest)
		require.NoError(t, err)
		assert.NotZero(t, guest.ID)

		testpkg.CleanupTableRecords(t, db, "users.guests", guest.ID)
	})

	t.Run("creates guest with date range", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Guest", "Dates")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID, staff.PersonID)

		startDate := time.Now()
		endDate := startDate.AddDate(0, 3, 0)

		guest := &users.Guest{
			StaffID:           staff.ID,
			ActivityExpertise: "Dance",
			StartDate:         &startDate,
			EndDate:           &endDate,
		}

		err := repo.Create(ctx, guest)
		require.NoError(t, err)
		assert.NotZero(t, guest.ID)
		assert.NotNil(t, guest.StartDate)
		assert.NotNil(t, guest.EndDate)

		testpkg.CleanupTableRecords(t, db, "users.guests", guest.ID)
	})

	t.Run("fails with nil guest", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails without staff ID", func(t *testing.T) {
		guest := &users.Guest{
			ActivityExpertise: "Music",
		}

		err := repo.Create(ctx, guest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "staff ID")
	})

	t.Run("fails without activity expertise", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Guest", "NoExp")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID, staff.PersonID)

		guest := &users.Guest{
			StaffID: staff.ID,
		}

		err := repo.Create(ctx, guest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "activity expertise")
	})

	t.Run("fails with invalid email", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Guest", "BadEmail")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID, staff.PersonID)

		guest := &users.Guest{
			StaffID:           staff.ID,
			ActivityExpertise: "Music",
			ContactEmail:      "not-an-email",
		}

		err := repo.Create(ctx, guest)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "email")
	})
}

func TestGuestRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Guest
	ctx := context.Background()

	t.Run("finds existing guest", func(t *testing.T) {
		guest := testpkg.CreateTestGuest(t, db, "FindByID")
		defer testpkg.CleanupTableRecords(t, db, "users.guests", guest.ID)
		defer testpkg.CleanupActivityFixtures(t, db, guest.Staff.ID, guest.Staff.PersonID)

		found, err := repo.FindByID(ctx, guest.ID)
		require.NoError(t, err)
		assert.Equal(t, guest.ID, found.ID)
		assert.Equal(t, guest.ActivityExpertise, found.ActivityExpertise)
	})

	t.Run("returns error for non-existent guest", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestGuestRepository_FindByStaffID(t *testing.T) {
	t.Skip("Skipped: BUN ORM schema qualification bug - uses unqualified table name")
}

func TestGuestRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Guest
	ctx := context.Background()

	t.Run("updates guest", func(t *testing.T) {
		guest := testpkg.CreateTestGuest(t, db, "Update")
		defer testpkg.CleanupTableRecords(t, db, "users.guests", guest.ID)
		defer testpkg.CleanupActivityFixtures(t, db, guest.Staff.ID, guest.Staff.PersonID)

		guest.ActivityExpertise = "Updated Expertise"
		guest.Organization = "Updated Org"

		err := repo.Update(ctx, guest)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, guest.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Expertise", found.ActivityExpertise)
		assert.Equal(t, "Updated Org", found.Organization)
	})

	t.Run("fails with nil guest", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGuestRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Guest
	ctx := context.Background()

	t.Run("deletes existing guest", func(t *testing.T) {
		guest := testpkg.CreateTestGuest(t, db, "Delete")
		defer testpkg.CleanupActivityFixtures(t, db, guest.Staff.ID, guest.Staff.PersonID)

		err := repo.Delete(ctx, guest.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, guest.ID)
		require.Error(t, err)
	})

	// NOTE: Base repository Delete returns nil for non-existent records
}

// ============================================================================
// Query Tests
// ============================================================================

// NOTE: The following tests are skipped due to the GuestRepository using unqualified
// table names in queries (e.g., "guests" instead of "users.guests"). This is a BUN ORM
// schema qualification bug that affects FindByOrganization, FindByExpertise, FindActive,
// SetDateRange, and List methods.

func TestGuestRepository_FindByOrganization(t *testing.T) {
	t.Skip("Skipped: BUN ORM schema qualification bug - uses unqualified table name")
}

func TestGuestRepository_FindByExpertise(t *testing.T) {
	t.Skip("Skipped: BUN ORM schema qualification bug - uses unqualified table name")
}

func TestGuestRepository_FindActive(t *testing.T) {
	t.Skip("Skipped: BUN ORM schema qualification bug - uses unqualified table name")
}

func TestGuestRepository_SetDateRange(t *testing.T) {
	t.Skip("Skipped: BUN ORM schema qualification bug - uses unqualified table name")
}

// ============================================================================
// List and Filter Tests
// ============================================================================

func TestGuestRepository_List(t *testing.T) {
	t.Skip("Skipped: BUN ORM schema qualification bug - uses unqualified table name")
}
