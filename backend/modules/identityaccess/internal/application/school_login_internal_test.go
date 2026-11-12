package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ported from services/auth (#3251): the school mint guard re-verifies the
// facts a school session rests on inside the mint transaction, in the lock
// order every revocation path walks, and the school-portal tenant scan does
// not mask lookup failures as "no portal role".

func newSchoolMintFixture(t *testing.T) (*authFixture, domain.LoginAccount) {
	t.Helper()
	f := newAuthFixture(t)
	f.store.addAccount(1, "teacher@example.com", "hash:secret", true)
	f.store.addMapping(1, 10, lehrkraftRole())
	f.store.permissions[mappingKey{1, 10}] = []string{"school:read"}
	f.schools.add(10, 100, "school-10", true, false)
	return f, f.store.accounts[1]
}

// mintSchool runs the school mint pipeline for school 10.
func mintSchool(f *authFixture, account domain.LoginAccount, recheckMFAGate bool) (mintedSession, error) {
	return f.auth.mintAccountSession(context.Background(), mintRequest{
		portal: domain.PortalSchool, account: account, tenantID: 10,
		recheckMFAGate: recheckMFAGate,
	})
}

// requireOrdered asserts that want appears in calls in this order.
func requireOrdered(t *testing.T, calls, want []string, message string) {
	t.Helper()
	next := 0
	for _, call := range calls {
		if next < len(want) && call == want[next] {
			next++
		}
	}
	assert.Equal(t, len(want), next, "%s: %v in %v", message, want, calls)
}

func TestSchoolMint_LiveSessionPasses(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)

	minted, err := mintSchool(f, account, false)
	require.NoError(t, err)
	claims := minted.claims
	require.NotNil(t, claims)
	assert.Equal(t, domain.PortalSchool.Claim, claims.Scope)
	assert.Equal(t, int64(10), claims.TenantID)
	assert.Equal(t, []string{"lehrkraft"}, claims.RoleNames)
	assert.Equal(t, []string{"school:read"}, claims.Permissions)
	assert.Len(t, f.store.sessionsOf(account.ID), 1)
	requireOrdered(t, f.store.calls, []string{
		"FindLoginAccount(forUpdate)", "LockActiveTenantMappingShared",
		"ListAccountRolesAtTenant(forShare)", "LockAccountPermissionSources",
		"ListAccountRolesAtTenant", "ListAccountPermissionsAtTenant", "RecordAccountLogin", "InsertAccountSession",
	}, "the account row is locked first, then the mapping and role rows, then the permission sources")
}

func TestSchoolMint_Refusals(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		arrange func(f *authFixture)
		want    error
	}{
		"revoked membership": {
			arrange: func(f *authFixture) { f.store.inactive[mappingKey{1, 10}] = true },
			want:    identityaccess.ErrTenantAccessDenied,
		},
		"revoked portal role": {
			arrange: func(f *authFixture) { f.store.roles[mappingKey{1, 10}] = nil },
			want:    identityaccess.ErrAccountNoSchoolPortalRole,
		},
		"deactivated school": {
			arrange: func(f *authFixture) { f.schools.add(10, 100, "school-10", false, false) },
			want:    identityaccess.ErrTenantNotFound,
		},
		"soft-deleted school": {
			arrange: func(f *authFixture) { f.schools.add(10, 100, "school-10", true, true) },
			want:    identityaccess.ErrTenantNotFound,
		},
		"deactivated account": {
			arrange: func(f *authFixture) { f.store.addAccount(1, "teacher@example.com", "hash:secret", false) },
			want:    identityaccess.ErrAccountInactive,
		},
		"vanished account": {
			arrange: func(f *authFixture) { delete(f.store.accounts, 1) },
			want:    domain.ErrAccountNotFound,
		},
		"account lookup failure is not a credential failure": {
			arrange: func(f *authFixture) { f.store.findAccountErr = errDB },
			want:    errDB,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f, account := newSchoolMintFixture(t)
			tc.arrange(f)
			minted, err := mintSchool(f, account, false)
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, minted.claims, "an aborted mint must not publish claims")
			assert.Empty(t, f.store.sessionsOf(account.ID), "a refused mint persists nothing")
			assert.NotContains(t, f.store.calls, "RecordAccountLogin", "a refused mint records no login")
		})
	}
}

func TestSchoolMint_ClaimsDropPermissionRevokedMidFlight(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)
	// The permissions are read under the lock, not from the pre-transaction
	// snapshot: a revocation that committed first is reflected in the claims.
	f.store.permissions[mappingKey{1, 10}] = nil

	minted, err := mintSchool(f, account, false)
	require.NoError(t, err)
	require.NotNil(t, minted.claims)
	assert.Empty(t, minted.claims.Permissions)
}

func newSchoolMintFixtureWithMFA(t *testing.T, requiredInTx bool) (*authFixture, domain.LoginAccount) {
	t.Helper()
	f, account := newSchoolMintFixture(t)
	f.mfa.configured = true
	f.mfa.policyInTx = fakePolicy{func([]string) bool { return requiredInTx }}
	return f, account
}

func TestSchoolMint_MFARequirementAppearingMidLoginAbortsMint(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixtureWithMFA(t, true)

	minted, err := mintSchool(f, account, true)
	require.ErrorIs(t, err, errSchoolMFARequiredAtMint)
	assert.Nil(t, minted.claims)
	assert.Empty(t, f.store.sessionsOf(account.ID))
}

func TestSchoolMint_MFARecheckPassesWhenNotRequired(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixtureWithMFA(t, false)

	minted, err := mintSchool(f, account, true)
	require.NoError(t, err)
	require.NotNil(t, minted.claims)
}

func TestSchoolMint_MFAPolicyIsReadInsideTheMintTransaction(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixtureWithMFA(t, false)

	_, err := mintSchool(f, account, true)
	require.NoError(t, err)
	require.NotEmpty(t, f.mfa.policyInTxInAdminTx)
	assert.True(t, f.mfa.policyInTxInAdminTx[0], "the policy is re-read past the request cache, on the mint transaction")
}

func TestSchoolMint_UnreadableMFAPolicyFailsClosed(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixtureWithMFA(t, false)
	f.mfa.policyInTxErr = errDB

	minted, err := mintSchool(f, account, true)
	require.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable)
	assert.Nil(t, minted.claims)
}

func TestSchoolMint_TakesMFAPolicyLockOnceBeforeTheAccountRow(t *testing.T) {
	t.Parallel()
	// Writing the school's mfa_mode takes a foreign-key lock on the writing
	// admin's own account row, so the mint pins the policy before it locks
	// auth.accounts; the reverse order deadlocks an admin who flips mfa_mode
	// while their own school login is in flight.
	f, account := newSchoolMintFixtureWithMFA(t, false)

	_, err := mintSchool(f, account, true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(f.store.calls), 2)
	assert.Equal(t, []string{"LockMFAPolicy", "FindLoginAccount(forUpdate)"}, f.store.calls[:2],
		"the policy lock must be the first lock of the mint transaction")
	assert.Equal(t, []string{"LockMFAPolicy"}, f.mfaLock.calls, "one policy lock per mint")
}

func TestSchoolMint_UnavailableMFAPolicyLockFailsClosed(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixtureWithMFA(t, false)
	f.mfaLock.err = errors.New("lock unavailable")

	minted, err := mintSchool(f, account, true)
	require.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable)
	assert.Nil(t, minted.claims)
	assert.Equal(t, []string{"LockMFAPolicy"}, f.store.calls, "a failed policy lock must abort before the mint touches any row")
}

func TestSchoolRefreshGuard_ReportsRevokedPortalRoleAsAccessDenied(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)
	f.store.roles[mappingKey{1, 10}] = nil

	var claims *domain.AccountClaimsPayload
	err := f.runtime.WithAdminTx(context.Background(), func(txCtx context.Context) error {
		var err error
		claims, err = sessionPolicies[domain.SessionKind(domain.PortalSchool.Name)].guard(f.auth, sessionBinding{op: "refresh token", renewal: refreshSession, tenantID: 10})(txCtx, account)
		return err
	})
	require.ErrorIs(t, err, identityaccess.ErrTenantAccessDenied, "the refresh route maps the access-denied sentinel")
	assert.Nil(t, claims)
}

func TestFindSchoolPortalTenant(t *testing.T) {
	t.Parallel()
	t.Run("mapping lookup error propagates", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		f.store.listTenantsErr = errDB
		_, _, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1, false)
		require.ErrorIs(t, err, errDB)
	})
	t.Run("school lookup error propagates when no school qualifies", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		f.schools.findErr = errDB
		_, _, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1, false)
		require.ErrorIs(t, err, errDB, "a full sweep of lookup failures is not masked as no portal role")
	})
	t.Run("dead school is skipped", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		f.schools.add(10, 100, "school-10", true, true)
		f.store.addMapping(1, 11, lehrkraftRole())
		f.schools.add(11, 110, "school-11", true, false)
		found, tenantID, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1, false)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, int64(11), tenantID)
	})
	t.Run("role lookup error propagates", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		f.store.listRolesErr = errDB
		_, _, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1, false)
		require.ErrorIs(t, err, errDB)
	})
	t.Run("no portal role anywhere", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		f.store.roles[mappingKey{1, 10}] = []domain.RoleAssignment{{RoleID: 1, Name: "user"}}
		found, tenantID, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1, false)
		require.NoError(t, err)
		assert.False(t, found)
		assert.Zero(t, tenantID)
	})
	t.Run("first live school with the role wins", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		found, tenantID, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1, false)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, int64(10), tenantID)
	})
}

func TestSchoolLoginTenantSelectsABookedSchoolAfterAnUnbookedOne(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)
	f.store.addMapping(1, 11, lehrkraftRole())
	f.schools.add(11, 110, "school-11", true, false)
	f.portals.withoutSchoolPortal = map[int64]bool{10: true}

	tenantID, err := f.auth.schoolLoginTenant(context.Background(), account, "", "", "")

	require.NoError(t, err)
	assert.Equal(t, int64(11), tenantID, "an implicit login selects a later school that booked the portal")
}

func TestSchoolLoginTenantRefusesAnExplicitUnbookedSchool(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)
	f.store.addMapping(1, 11, lehrkraftRole())
	f.schools.add(11, 110, "school-11", true, false)
	f.portals.withoutSchoolPortal = map[int64]bool{10: true}

	_, err := f.auth.schoolLoginTenant(context.Background(), account, "school-10", "", "")

	require.Error(t, err)
	_, notBooked := identityaccess.PortalNotBooked(err)
	assert.True(t, notBooked, "an explicit selection must not silently switch to another school")
}

func TestHasSchoolPortalRoleAtTenant(t *testing.T) {
	t.Parallel()
	f, _ := newSchoolMintFixture(t)

	has, err := f.auth.hasSchoolPortalRoleAtTenant(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.True(t, has)

	f.store.roles[mappingKey{1, 10}] = nil
	has, err = f.auth.hasSchoolPortalRoleAtTenant(context.Background(), 1, 10)
	require.NoError(t, err)
	assert.False(t, has, "no role rows report no role, not an error")

	f.store.listRolesErr = errDB
	_, err = f.auth.hasSchoolPortalRoleAtTenant(context.Background(), 1, 10)
	require.ErrorIs(t, err, errDB)
}

func TestLoadSchoolMetadataForTenant_LookupFailuresPropagate(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)

	f.store.listRolesErr = errDB
	_, err := f.auth.loadSchoolMetadataForTenant(context.Background(), account, 10)
	require.ErrorIs(t, err, errDB)
	f.store.listRolesErr = nil

	f.store.hasMappingErr = errDB
	_, err = f.auth.loadSchoolMetadataForTenant(context.Background(), account, 10)
	require.ErrorIs(t, err, errDB)
	f.store.hasMappingErr = nil

	payload, err := f.auth.loadSchoolMetadataForTenant(context.Background(), account, 10)
	require.NoError(t, err)
	assert.Equal(t, domain.PortalSchool.Claim, payload.Scope)
}
