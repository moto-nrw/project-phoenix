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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("maps staff with an account and omits staff without one", func(t *testing.T) {
		withAccount, account := testpkg.CreateTestStaffWithAccount(t, db, "Mapped", "Staff")
		testpkg.MapAccountToTenant(t, db, account.ID, testpkg.Tenant(t))
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
		testpkg.MapAccountToTenant(t, db, account.ID, testpkg.Tenant(t))

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

	t.Run("excludes inactive accounts and tenant mappings", func(t *testing.T) {
		inactiveAccountStaff, inactiveAccount := testpkg.CreateTestStaffWithAccount(t, db, "Inactive", "Account")
		testpkg.MapAccountToTenant(t, db, inactiveAccount.ID, testpkg.Tenant(t))
		inactiveMappingStaff, inactiveMappingAccount := testpkg.CreateTestStaffWithAccount(t, db, "Inactive", "Mapping")
		testpkg.MapAccountToTenant(t, db, inactiveMappingAccount.ID, testpkg.Tenant(t))

		defer testpkg.CleanupStaffFixtures(t, db, inactiveAccountStaff.ID, inactiveMappingStaff.ID)
		defer testpkg.CleanupAuthFixtures(t, db, inactiveAccount.ID, inactiveMappingAccount.ID)

		_, err := db.NewUpdate().
			TableExpr("auth.accounts").
			Set("active = FALSE").
			Where("id = ?", inactiveAccount.ID).
			Exec(ctx)
		require.NoError(t, err)
		_, err = db.NewUpdate().
			TableExpr("auth.account_tenants").
			Set("status = 'inactive'").
			Where("account_id = ?", inactiveMappingAccount.ID).
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Exec(ctx)
		require.NoError(t, err)

		result, err := repo.ListAccountIDsByStaffIDs(ctx, []int64{inactiveAccountStaff.ID, inactiveMappingStaff.ID})
		require.NoError(t, err)
		assert.Empty(t, result)
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

// TestStaffRepository_ListAllStaffAccountIDs pins the candidate set a
// background job addresses when it means "the team of this school".
//
// The eligibility rules match the push-subscription finder deliberately: both
// answer "may this person be addressed in this school right now", and two
// answers to that question would drift.
func TestStaffRepository_ListAllStaffAccountIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).Staff
	ctx := testpkg.Ctx(t)

	t.Run("lists mapped staff and skips the rest", func(t *testing.T) {
		mapped, mappedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Mapped", "Colleague")
		testpkg.MapAccountToTenant(t, db, mappedAccount.ID, testpkg.Tenant(t))

		unmapped, unmappedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Unmapped", "Colleague")
		// "Unmapped" is the point of this fixture: drop the mapping
		// CreateTestAccount adds for the test's own tenant (#2419).
		testpkg.UnclaimTestAccount(t, db, unmappedAccount.ID)

		deactivated, deactivatedAccount := testpkg.CreateTestStaffWithAccount(t, db, "Deactivated", "Colleague")
		testpkg.MapAccountToTenant(t, db, deactivatedAccount.ID, testpkg.Tenant(t))

		accountless := testpkg.CreateTestStaff(t, db, "Accountless", "Colleague")

		defer testpkg.CleanupStaffFixtures(t, db, mapped.ID, unmapped.ID, deactivated.ID, accountless.ID)
		defer testpkg.CleanupAuthFixtures(t, db, mappedAccount.ID, unmappedAccount.ID, deactivatedAccount.ID)

		_, err := db.NewUpdate().
			TableExpr("auth.accounts").
			Set("active = FALSE").
			Where("id = ?", deactivatedAccount.ID).
			Exec(ctx)
		require.NoError(t, err)

		result, err := repo.ListAllStaffAccountIDs(ctx)
		require.NoError(t, err)

		assert.Equal(t, mappedAccount.ID, result[mapped.ID])

		_, hasUnmapped := result[unmapped.ID]
		assert.False(t, hasUnmapped, "an account without an active mapping does not belong to this school")

		_, hasDeactivated := result[deactivated.ID]
		assert.False(t, hasDeactivated, "a deactivated account is not addressable")

		_, hasAccountless := result[accountless.ID]
		assert.False(t, hasAccountless, "staff without a login cannot receive anything")
	})

	t.Run("excludes a soft-deleted staff member", func(t *testing.T) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Offboarded", "Colleague")
		testpkg.MapAccountToTenant(t, db, account.ID, testpkg.Tenant(t))

		defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
		defer testpkg.CleanupAuthFixtures(t, db, account.ID)

		_, err := db.NewUpdate().
			TableExpr("users.staff").
			Set("deleted_at = NOW()").
			Where("id = ?", staff.ID).
			Exec(ctx)
		require.NoError(t, err)

		result, err := repo.ListAllStaffAccountIDs(ctx)
		require.NoError(t, err)

		_, present := result[staff.ID]
		assert.False(t, present, "an offboarded staff member is not addressable")
	})

	t.Run("does not list another school's staff", func(t *testing.T) {
		const otherTenant int64 = 99045
		testpkg.EnsureTestTenant(t, db, otherTenant)

		foreign := testpkg.CreateTestStaffForTenant(t, db, otherTenant, "Foreign", "Colleague")
		defer testpkg.CleanupStaffFixtures(t, db, foreign.ID)

		result, err := repo.ListAllStaffAccountIDs(ctx)
		require.NoError(t, err)

		_, present := result[foreign.ID]
		assert.False(t, present, "tenant 1 must not list another school's staff")
	})
}
