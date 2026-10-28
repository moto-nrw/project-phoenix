package api

import (
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
// staff member without either is not.
func TestParentMessageLeaders_RoleGrantAndAdminWildcard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	leaders := owners.ParentMessageLeadership{Staff: NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Staff}
	ctx := testpkg.Ctx(t)

	_, roleAccount := testpkg.CreateTestCalendarStaff(t, db, "Swantje", "Meier")
	_, adminAccount := testpkg.CreateTestCalendarStaff(t, db, "Anton", "Admin")
	_, bystanderAccount := testpkg.CreateTestCalendarStaff(t, db, "Bodo", "Betreuer")

	none, err := testpkg.TenantQuery0(t, ctx, db, leaders.ParentMessageLeaders)
	require.NoError(t, err)
	assert.Empty(t, none, "nobody is leadership before the permission is granted")

	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), roleAccount.ID, "parent_messages:leadership")
	testpkg.GrantTestPermission(t, db, testpkg.Tenant(t), adminAccount.ID, "admin:*")

	listed, err := testpkg.TenantQuery0(t, ctx, db, leaders.ParentMessageLeaders)
	require.NoError(t, err)
	accounts := make([]int64, 0, len(listed))
	for _, leader := range listed {
		accounts = append(accounts, leader.AccountID)
	}
	assert.ElementsMatch(t, []int64{roleAccount.ID, adminAccount.ID}, accounts)
	assert.NotContains(t, accounts, bystanderAccount.ID)
	assert.Equal(t, []string{"Anton A.", "Swantje M."}, communication.ParentMessageLeaderShortNames(listed), "ordered by name, short form")

	ids, err := testpkg.TenantQuery0(t, ctx, db, leaders.LeadershipAccountIDs)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{roleAccount.ID, adminAccount.ID}, ids, "the leadership push reaches the same accounts")
}
