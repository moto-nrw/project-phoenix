package api

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestParentMessageLeaders_RoleGrantAndAdminWildcard pins who reads the
// leadership conversations (#5200): a custom role holding
// parent_messages:leadership and an account with admin:* are leadership; a
// staff member also needs users:read to open the staff messaging routes.
func TestParentMessageLeaders_RoleGrantAndAdminWildcard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	identity, err := NewTestIdentityAccess(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)
	leaders := owners.ParentMessageLeadership{Staff: NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Staff, Admins: identity}
	ctx := testpkg.Ctx(t)

	_, roleAccount := testpkg.CreateTestCalendarStaff(t, db, "Swantje", "Meier")
	_, adminAccount := testpkg.CreateTestCalendarStaff(t, db, "Anton", "Admin")
	_, bystanderAccount := testpkg.CreateTestCalendarStaff(t, db, "Bodo", "Betreuer")
	// No base user role: this custom-role staff member has leadership access
	// without the users:read permission required by the messaging router.
	_, leadershipOnlyAccount := testpkg.CreateTestStaffWithAccount(t, db, "Lena", "Leitung")

	none, err := testpkg.TenantQuery0(t, ctx, db, leaders.ParentMessageLeaders)
	require.NoError(t, err)
	assert.Empty(t, none, "nobody is leadership before the permission is granted")

	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), roleAccount.ID, "parent_messages:leadership")
	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), roleAccount.ID, "users:read")
	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), leadershipOnlyAccount.ID, "parent_messages:leadership")
	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), bystanderAccount.ID, "users:read")
	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), adminAccount.ID, "admin:*")
	// The school's administration account reads every child through admin:*
	// without a staff record, so it reads the leadership conversations too
	// and must be among the readers parents see.
	officeAccount := testpkg.CreateTestAccount(t, db, "verwaltung")
	testpkg.EnsureAccountTenant(t, db, officeAccount.ID, testpkg.Tenant(t))
	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), officeAccount.ID, "admin:*")
	// A role holding the permission without a staff record reads no child,
	// so it is no reader.
	strayAccount := testpkg.CreateTestAccount(t, db, "ohne-personal")
	testpkg.EnsureAccountTenant(t, db, strayAccount.ID, testpkg.Tenant(t))
	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), strayAccount.ID, "parent_messages:leadership")

	listed, err := testpkg.TenantQuery0(t, ctx, db, leaders.ParentMessageLeaders)
	require.NoError(t, err)
	accounts := make([]int64, 0, len(listed))
	for _, leader := range listed {
		accounts = append(accounts, leader.AccountID)
	}
	assert.ElementsMatch(t, []int64{roleAccount.ID, adminAccount.ID, officeAccount.ID}, accounts)
	assert.NotContains(t, accounts, bystanderAccount.ID)
	assert.NotContains(t, accounts, strayAccount.ID)
	assert.NotContains(t, accounts, leadershipOnlyAccount.ID, "leadership permission alone cannot open messages")
	assert.Equal(t, []string{"Anton A.", "Swantje M.", communication.ParentMessageUnnamedLeaderLabel}, communication.ParentMessageLeaderShortNames(listed),
		"named persons by name, the administration account under a neutral label")

	ids, err := testpkg.TenantQuery0(t, ctx, db, leaders.LeadershipAccountIDs)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{roleAccount.ID, adminAccount.ID, officeAccount.ID}, ids, "the leadership push reaches the same accounts")
}
