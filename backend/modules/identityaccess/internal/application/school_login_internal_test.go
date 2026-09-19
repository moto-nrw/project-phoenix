package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
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

func runSchoolMintGuard(t *testing.T, f *authFixture, account domain.LoginAccount, opts ...schoolMintOption) (*domain.AccountClaimsPayload, error) {
	t.Helper()
	var claims *domain.AccountClaimsPayload
	guard := f.auth.schoolMintGuard(account.ID, 10, &claims, opts...)
	err := f.runtime.WithAdminTx(context.Background(), func(txCtx context.Context) error {
		return guard(txCtx, account)
	})
	return claims, err
}

func staticPolicy(required bool, err error) mfaPolicyResolver {
	return func(context.Context) (ports.MFAPolicy, error) {
		if err != nil {
			return nil, err
		}
		return fakePolicy{func([]string) bool { return required }}, nil
	}
}

func TestSchoolMintGuard_LiveSessionPasses(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)

	claims, err := runSchoolMintGuard(t, f, account)
	require.NoError(t, err)
	// The guard is also where the JWT payload comes from: a nil payload on a
	// passing guard would mint a token with no roles and no permissions.
	require.NotNil(t, claims)
	assert.Equal(t, domain.ScopeSchool, claims.Scope)
	assert.Equal(t, int64(10), claims.TenantID)
	assert.Equal(t, []string{"lehrkraft"}, claims.RoleNames)
	assert.Equal(t, []string{"school:read"}, claims.Permissions)
	assert.Equal(t, []string{
		"FindLoginAccount(forUpdate)", "LockActiveTenantMappingShared", "HasActiveAccountTenant",
		"ListAccountRolesAtTenant(forShare)", "LockAccountPermissionSources",
		"ListAccountRolesAtTenant", "ListAccountPermissionsAtTenant",
	}, f.store.calls, "the account row is locked first, then the mapping and role rows, then the permission sources")
}

func TestSchoolMintGuard_Refusals(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		arrange func(f *authFixture)
		want    error
	}{
		"revoked membership": {
			arrange: func(f *authFixture) { f.store.inactive[mappingKey{1, 10}] = true },
			want:    domain.ErrTenantAccessDenied,
		},
		"revoked portal role": {
			arrange: func(f *authFixture) { f.store.roles[mappingKey{1, 10}] = nil },
			want:    domain.ErrAccountNoSchoolPortalRole,
		},
		"deactivated school": {
			arrange: func(f *authFixture) { f.schools.add(10, 100, "school-10", false, false) },
			want:    domain.ErrTenantNotFound,
		},
		"soft-deleted school": {
			arrange: func(f *authFixture) { f.schools.add(10, 100, "school-10", true, true) },
			want:    domain.ErrTenantNotFound,
		},
		"deactivated account": {
			arrange: func(f *authFixture) { f.store.addAccount(1, "teacher@example.com", "hash:secret", false) },
			want:    domain.ErrAccountInactive,
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
			claims, err := runSchoolMintGuard(t, f, account)
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, claims, "an aborted mint must not publish claims")
		})
	}
}

func TestSchoolMintGuard_ClaimsDropPermissionRevokedMidFlight(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)
	// The permissions are read under the lock, not from the pre-transaction
	// snapshot: a revocation that committed first is reflected in the claims.
	f.store.permissions[mappingKey{1, 10}] = nil

	claims, err := runSchoolMintGuard(t, f, account)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Empty(t, claims.Permissions)
}

func TestCreateRefreshSessionGuarded_GuardRefusalWritesNoToken(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)
	f.store.inactive[mappingKey{1, 10}] = true
	var claims *domain.AccountClaimsPayload

	_, err := f.auth.createRefreshSessionGuarded(context.Background(), account, 10, domain.ScopeSchool, f.auth.schoolMintGuard(account.ID, 10, &claims), "")
	require.ErrorIs(t, err, domain.ErrTenantAccessDenied, "the guard's sentinel surfaces verbatim")
	assert.Empty(t, f.store.sessionsOf(account.ID), "a refused mint persists nothing")
	assert.NotContains(t, f.store.calls, "RecordAccountLogin", "a refused mint records no login")
	assert.NotContains(t, f.store.calls, "InsertAccountSession")
}

func TestSchoolMintGuard_MFARequirementAppearingMidLoginAbortsMint(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)

	claims, err := runSchoolMintGuard(t, f, account, withMFAGateRecheck(staticPolicy(true, nil)))
	require.ErrorIs(t, err, errSchoolMFARequiredAtMint)
	assert.Nil(t, claims)
	assert.Empty(t, f.store.sessionsOf(account.ID))
}

func TestSchoolMintGuard_MFARecheckPassesWhenNotRequired(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)

	claims, err := runSchoolMintGuard(t, f, account, withMFAGateRecheck(staticPolicy(false, nil)))
	require.NoError(t, err)
	require.NotNil(t, claims)
}

func TestSchoolMintGuard_MFAPolicyIsReadInsideTheMintTransaction(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)

	_, err := runSchoolMintGuard(t, f, account, withMFAGateRecheck(f.auth.freshSchoolMFAPolicy(account.ID, 10)))
	require.NoError(t, err)
	require.Len(t, f.mfa.policyInTxInAdminTx, 1)
	assert.True(t, f.mfa.policyInTxInAdminTx[0], "the policy is re-read past the request cache, on the mint transaction")
}

func TestSchoolMintGuard_UnreadableMFAPolicyFailsClosed(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)

	claims, err := runSchoolMintGuard(t, f, account, withMFAGateRecheck(staticPolicy(false, errDB)))
	require.ErrorIs(t, err, domain.ErrMFAStatusUnavailable)
	assert.Nil(t, claims)
}

func TestSchoolMintGuard_TakesMFAPolicyLockBeforeTheAccountRow(t *testing.T) {
	t.Parallel()
	// Writing the school's mfa_mode takes a foreign-key lock on the writing
	// admin's own account row, so the mint pins the policy before it locks
	// auth.accounts; the reverse order deadlocks an admin who flips mfa_mode
	// while their own school login is in flight.
	f, account := newSchoolMintFixture(t)

	claims, err := runSchoolMintGuard(t, f, account, withMFAGateRecheck(staticPolicy(false, nil)))
	require.NoError(t, err)
	require.NotNil(t, claims)
	require.GreaterOrEqual(t, len(f.store.calls), 2)
	assert.Equal(t, []string{"LockMFAPolicy", "FindLoginAccount(forUpdate)"}, f.store.calls[:2],
		"the policy lock must be the first lock of the mint transaction")
}

func TestSchoolMintGuard_UnavailableMFAPolicyLockFailsClosed(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)
	f.mfaLock.err = errors.New("lock unavailable")

	claims, err := runSchoolMintGuard(t, f, account, withMFAGateRecheck(staticPolicy(false, nil)))
	require.ErrorIs(t, err, domain.ErrMFAStatusUnavailable)
	assert.Nil(t, claims)
	assert.Equal(t, []string{"LockMFAPolicy"}, f.store.calls, "a failed policy lock must abort before the mint touches any row")
}

func TestSchoolMintGuard_WithoutRecheckTakesNoPolicyLock(t *testing.T) {
	t.Parallel()
	// The MFA exchange and the school switch have their second factor
	// settled; taking the policy lock there would serialize those mints
	// against every mfa_mode write for nothing.
	f, account := newSchoolMintFixture(t)

	claims, err := runSchoolMintGuard(t, f, account)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Empty(t, f.mfaLock.calls)
}

func TestSchoolRefreshMintGuard_ReportsRevokedPortalRoleAsAccessDenied(t *testing.T) {
	t.Parallel()
	f, account := newSchoolMintFixture(t)
	f.store.roles[mappingKey{1, 10}] = nil
	var claims *domain.AccountClaimsPayload

	err := f.runtime.WithAdminTx(context.Background(), func(txCtx context.Context) error {
		return f.auth.schoolRefreshMintGuard(account.ID, 10, &claims)(txCtx, account)
	})
	require.ErrorIs(t, err, domain.ErrTenantAccessDenied, "the refresh route maps the access-denied sentinel")
	assert.Nil(t, claims)
}

func TestFindSchoolPortalTenant(t *testing.T) {
	t.Parallel()
	t.Run("mapping lookup error propagates", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		f.store.listTenantsErr = errDB
		_, _, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1)
		require.ErrorIs(t, err, errDB)
	})
	t.Run("school lookup error propagates when no school qualifies", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		f.schools.findErr = errDB
		_, _, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1)
		require.ErrorIs(t, err, errDB, "a full sweep of lookup failures is not masked as no portal role")
	})
	t.Run("dead school is skipped", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		f.schools.add(10, 100, "school-10", true, true)
		f.store.addMapping(1, 11, lehrkraftRole())
		f.schools.add(11, 110, "school-11", true, false)
		found, tenantID, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, int64(11), tenantID)
	})
	t.Run("role lookup error propagates", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		f.store.listRolesErr = errDB
		_, _, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1)
		require.ErrorIs(t, err, errDB)
	})
	t.Run("no portal role anywhere", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		f.store.roles[mappingKey{1, 10}] = []domain.RoleAssignment{{RoleID: 1, Name: "user"}}
		found, tenantID, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1)
		require.NoError(t, err)
		assert.False(t, found)
		assert.Zero(t, tenantID)
	})
	t.Run("first live school with the role wins", func(t *testing.T) {
		t.Parallel()
		f, _ := newSchoolMintFixture(t)
		found, tenantID, err := f.auth.findSchoolPortalTenantForAccount(context.Background(), 1)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, int64(10), tenantID)
	})
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
	assert.Equal(t, domain.ScopeSchool, payload.Scope)
}
