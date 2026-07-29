package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaffRepository_ListAccountIDsByStaffIDs pins the staff -> login account
// mapping used to address people by account instead of by staff row.
func TestStaffRepository_ListAccountIDsByStaffIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.TenantContext(1)

	t.Run("maps staff with an account and omits staff without one", func(t *testing.T) {
		withAccount, account := testpkg.CreateTestStaffWithAccount(t, db, "Mapped", "Staff")
		withoutAccount := testpkg.CreateTestStaff(t, db, "Accountless", "Staff")

		defer testpkg.CleanupStaffFixtures(t, db, withAccount.ID, withoutAccount.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		result, err := repo.ListAccountIDsByStaffIDs(ctx, []int64{withAccount.ID, withoutAccount.ID})
		require.NoError(t, err)

		assert.Equal(t, account.ID, result[withAccount.ID])

		// Absent rather than mapped to zero: a caller ranging over the map must
		// never end up addressing account 0.
		_, present := result[withoutAccount.ID]
		assert.False(t, present, "staff without a login account must not appear")
	})

	t.Run("excludes a soft-deleted staff member", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Offboarded", "Staff")

		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		_, err := db.NewUpdate().
			TableExpr("users.staff").
			Set("deleted_at = NOW()").
			Where("id = ?", staff.ID).
			Exec(ctx)
		require.NoError(t, err)

		result, err := repo.ListAccountIDsByStaffIDs(ctx, []int64{staff.ID})
		require.NoError(t, err)

		_, present := result[staff.ID]
		assert.False(t, present, "an offboarded staff member is not addressable")
	})

	t.Run("empty input short-circuits", func(t *testing.T) {
		result, err := repo.ListAccountIDsByStaffIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("does not resolve another tenant's staff", func(t *testing.T) {
		const otherTenant int64 = 99044
		testpkg.EnsureTestTenant(t, db, otherTenant)

		foreignStaff := testpkg.CreateTestStaffForTenant(t, db, otherTenant, "Foreign", "Staff")
		defer testpkg.CleanupStaffFixtures(t, db, foreignStaff.ID)

		result, err := repo.ListAccountIDsByStaffIDs(ctx, []int64{foreignStaff.ID})
		require.NoError(t, err)

		_, present := result[foreignStaff.ID]
		assert.False(t, present, "tenant 1 must not resolve another school's staff")
	})
}
