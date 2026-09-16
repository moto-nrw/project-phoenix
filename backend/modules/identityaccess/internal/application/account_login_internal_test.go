package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ported from services/auth (#3251): tenant resolution, refresh tenant
// validation, the singleflight key and the claims material loader run over
// the module's ports instead of the legacy repository layer.

var errDB = errors.New("database is down")

func TestResolveAccountTenantBySlug_UnknownSlugIsTenantNotFound(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "user")

	_, _, err := f.auth.resolveAccountTenantBySlug(context.Background(), 1, "nowhere")
	require.ErrorIs(t, err, domain.ErrTenantNotFound)
}

func TestResolveAccountTenantBySlug_PropagatesLookupErrors(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "user")
	f.schools.subdomainErr = errDB

	_, _, err := f.auth.resolveAccountTenantBySlug(context.Background(), 1, "school-10")
	require.ErrorIs(t, err, errDB)
	require.NotErrorIs(t, err, domain.ErrTenantNotFound)
}

func TestResolveAccountTenantBySlug_DeletedOrInactiveSchoolIsTenantNotFound(t *testing.T) {
	t.Parallel()
	for name, live := range map[string][2]bool{"deleted": {true, true}, "inactive": {false, false}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newAuthFixture(t)
			f.store.addAccount(1, "user1@example.com", "hash:secret", true)
			f.store.addMapping(1, 10)
			f.schools.add(10, 100, "school-10", live[0], live[1])

			_, _, err := f.auth.resolveAccountTenantBySlug(context.Background(), 1, "school-10")
			require.ErrorIs(t, err, domain.ErrTenantNotFound)
		})
	}
}

func TestResolveAccountTenantBySlug_RequiresActiveMappingForTenantScope(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "user")
	f.schools.add(11, 110, "school-11", true, false)

	_, _, err := f.auth.resolveAccountTenantBySlug(context.Background(), 1, "school-11")
	require.ErrorIs(t, err, domain.ErrTenantAccessDenied)

	f.store.hasMappingErr = errDB
	_, _, err = f.auth.resolveAccountTenantBySlug(context.Background(), 1, "school-10")
	require.ErrorIs(t, err, domain.ErrTenantAccessDenied, "a mapping lookup failure denies rather than grants")
}

func TestResolveAccountTenantBySlug_OrgScopeReachesOwnOrganizationOnly(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.store.addAccount(1, "user1@example.com", "hash:secret", true)
	f.schools.add(10, 100, "school-10", true, false)
	f.schools.add(11, 200, "school-11", true, false)
	ctx := withScope(context.Background(), domain.ScopeOrg, 100)

	tenantID, orgID, err := f.auth.resolveAccountTenantBySlug(ctx, 1, "school-10")
	require.NoError(t, err)
	assert.Equal(t, int64(10), tenantID)
	assert.Equal(t, int64(100), orgID)

	_, _, err = f.auth.resolveAccountTenantBySlug(ctx, 1, "school-11")
	require.ErrorIs(t, err, domain.ErrTenantAccessDenied)
}

func TestResolveAccountTenantDefault_SkipsDeadSchoolsInMappingOrder(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.store.addAccount(1, "user1@example.com", "hash:secret", true)
	f.store.addMapping(1, 10)
	f.store.addMapping(1, 11)
	f.store.addMapping(1, 12)
	f.schools.add(10, 100, "school-10", true, true)
	f.schools.add(11, 110, "school-11", false, false)
	f.schools.add(12, 120, "school-12", true, false)

	tenantID, orgID, err := f.auth.resolveAccountTenantDefault(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, int64(12), tenantID)
	assert.Equal(t, int64(120), orgID)
}

func TestResolveAccountTenantDefault_NoLiveSchoolIsTenantNotFound(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.store.addAccount(1, "user1@example.com", "hash:secret", true)
	f.store.addMapping(1, 10)
	f.schools.add(10, 100, "school-10", true, true)

	_, _, err := f.auth.resolveAccountTenantDefault(context.Background(), 1)
	require.ErrorIs(t, err, domain.ErrTenantNotFound)

	_, _, err = f.auth.resolveAccountTenantDefault(context.Background(), 2)
	require.ErrorIs(t, err, domain.ErrTenantNotFound, "an account without mappings has no default tenant")
}

func TestResolveAccountTenantDefault_PropagatesLookupErrors(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "user")

	f.store.listTenantsErr = errDB
	_, _, err := f.auth.resolveAccountTenantDefault(context.Background(), 1)
	require.ErrorIs(t, err, errDB)
	f.store.listTenantsErr = nil

	f.schools.listActiveErr = errDB
	_, _, err = f.auth.resolveAccountTenantDefault(context.Background(), 1)
	require.ErrorIs(t, err, errDB)
}

func TestLoadAccountMetadataForTenant(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "admin")
	f.store.permissions[mappingKey{1, 10}] = []string{"students:read"}
	f.persons.names[personKey{1, 10}] = [2]string{"Ada", "Lovelace"}
	account := f.store.accounts[1]

	payload, err := f.auth.loadAccountMetadataForTenant(context.Background(), account, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"admin"}, payload.RoleNames)
	assert.Equal(t, []string{"students:read"}, payload.Permissions)
	assert.Equal(t, "Ada", payload.FirstName)
	assert.Equal(t, "Lovelace", payload.LastName)
	assert.True(t, payload.IsAdmin)
	assert.Equal(t, int64(10), payload.TenantID)
	assert.Equal(t, int64(100), payload.OrgID)
	assert.Equal(t, 1, f.runtime.adminTxCount, "claims material loads in one administrative transaction")

	f.schools.add(11, 110, "school-11", true, true)
	_, err = f.auth.loadAccountMetadataForTenant(context.Background(), account, 11)
	require.ErrorIs(t, err, domain.ErrTenantNotFound, "a deleted school answers with re-login")

	_, err = f.auth.loadAccountMetadataForTenant(context.Background(), account, 99)
	require.ErrorIs(t, err, domain.ErrTenantNotFound, "an unknown school answers with re-login")

	payload, err = f.auth.loadAccountMetadataForTenant(context.Background(), account, 0)
	require.NoError(t, err, "tenantless claims skip the school lookup")
	assert.Zero(t, payload.TenantID)
	assert.Zero(t, payload.OrgID)
}

func TestLoadAccountMetadataForTenant_QueryFailuresRefuseTheMint(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(1, 10, "user")
	account := f.store.accounts[1]

	f.store.listRolesErr = errDB
	_, err := f.auth.loadAccountMetadataForTenant(context.Background(), account, 10)
	require.ErrorIs(t, err, errDB, "an empty role set is not a harmless degradation")
	f.store.listRolesErr = nil

	f.store.listPermissionsErr = errDB
	_, err = f.auth.loadAccountMetadataForTenant(context.Background(), account, 10)
	require.ErrorIs(t, err, errDB)
	f.store.listPermissionsErr = nil

	f.persons.findErr = errDB
	_, err = f.auth.loadAccountMetadataForTenant(context.Background(), account, 10)
	require.ErrorIs(t, err, errDB)
}

// The person name comes from the target school; without a person row there
// the other mapped schools are consulted and used only when they agree.
func TestLoadPersonNamesForTenant_FallbackAcrossMappedSchools(t *testing.T) {
	t.Parallel()
	f := newAuthFixture(t)
	f.seedStaff(7, 10, "user")
	f.seedStaff(7, 11, "user")
	f.seedStaff(7, 12, "user")
	f.schools.add(13, 130, "school-13", true, false)
	ctx := context.Background()

	f.persons.names[personKey{7, 10}] = [2]string{"Target", "School"}
	f.persons.names[personKey{7, 11}] = [2]string{"Other", "School"}
	first, last, err := f.auth.loadPersonNamesForTenant(ctx, 7, 10)
	require.NoError(t, err)
	assert.Equal(t, [2]string{"Target", "School"}, [2]string{first, last}, "the target school wins")

	delete(f.persons.names, personKey{7, 10})
	first, last, err = f.auth.loadPersonNamesForTenant(ctx, 7, 10)
	require.NoError(t, err)
	assert.Equal(t, [2]string{"Other", "School"}, [2]string{first, last}, "without a row at the target the mapped school's name is used")

	f.persons.names[personKey{7, 12}] = [2]string{"Different", "Name"}
	first, last, err = f.auth.loadPersonNamesForTenant(ctx, 7, 10)
	require.NoError(t, err)
	assert.Empty(t, first+last, "schools that disagree yield no name")

	delete(f.persons.names, personKey{7, 12})
	f.persons.names[personKey{7, 13}] = [2]string{"Unmapped", "School"}
	first, last, err = f.auth.loadPersonNamesForTenant(ctx, 7, 10)
	require.NoError(t, err)
	assert.Equal(t, [2]string{"Other", "School"}, [2]string{first, last}, "schools the account is not mapped to are ignored")

	f.store.listTenantsErr = errDB
	_, _, err = f.auth.loadPersonNamesForTenant(ctx, 7, 10)
	require.ErrorIs(t, err, errDB)
}

func TestValidateTenantAccess(t *testing.T) {
	t.Parallel()
	claims := domain.RefreshClaims{AccountID: 1, TenantID: 10}
	cases := map[string]struct {
		arrange func(f *authFixture)
		want    error
	}{
		"mapping lookup failure is retryable": {
			arrange: func(f *authFixture) { f.store.hasMappingErr = errDB },
			want:    errDB,
		},
		"missing mapping is terminal": {
			arrange: func(f *authFixture) { f.store.inactive[mappingKey{1, 10}] = true },
			want:    domain.ErrTenantAccessDenied,
		},
		"school lookup failure propagates": {
			arrange: func(f *authFixture) { f.schools.findErr = errDB },
			want:    errDB,
		},
		"unknown school is tenant not found": {
			arrange: func(f *authFixture) { delete(f.schools.schools, 10) },
			want:    domain.ErrTenantNotFound,
		},
		"deleted school is tenant not found": {
			arrange: func(f *authFixture) { f.schools.add(10, 100, "school-10", true, true) },
			want:    domain.ErrTenantNotFound,
		},
		"inactive school is tenant not found": {
			arrange: func(f *authFixture) { f.schools.add(10, 100, "school-10", false, false) },
			want:    domain.ErrTenantNotFound,
		},
		"live mapping and school pass": {arrange: func(*authFixture) {}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newAuthFixture(t)
			f.seedStaff(1, 10, "user")
			tc.arrange(f)
			err := f.auth.validateTenantAccess(context.Background(), claims)
			if tc.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.want)
			if errors.Is(tc.want, errDB) {
				require.NotErrorIs(t, err, domain.ErrTenantAccessDenied)
				require.NotErrorIs(t, err, domain.ErrTenantNotFound)
			}
		})
	}
}

func TestRefreshSingleflightKeyBindsIndependentProof(t *testing.T) {
	t.Parallel()
	plain := refreshSingleflightKey("token", nil)
	withProof := refreshSingleflightKey("token", []byte{1, 2, 3})
	assert.NotEqual(t, plain, withProof, "a refresh token alone must not join another caller's recovery")
	assert.Equal(t, withProof, refreshSingleflightKey("token", []byte{1, 2, 3}))
	assert.NotEqual(t, refreshSingleflightKey("other", nil), plain)
}

func TestIsTokenFamilyConflict(t *testing.T) {
	t.Parallel()
	assert.False(t, isTokenFamilyConflict(nil))
	assert.False(t, isTokenFamilyConflict(errDB))
	assert.True(t, isTokenFamilyConflict(errors.New(`duplicate key value violates unique constraint "uk_tokens_family_generation"`)))
}

func TestPersistedPortalScope(t *testing.T) {
	t.Parallel()
	assert.Equal(t, domain.PortalScopeTenant, domain.PersistedPortalScope(domain.ScopeTenant))
	assert.Equal(t, domain.PortalScopeTenant, domain.PersistedPortalScope(domain.PortalScopeTenant))
	assert.Equal(t, domain.PortalScopeOrg, domain.PersistedPortalScope(domain.PortalScopeOrg))
	assert.Equal(t, domain.PortalScopeParent, domain.PersistedPortalScope(domain.PortalScopeParent))
	assert.Equal(t, domain.PortalScopeSchool, domain.PersistedPortalScope(domain.PortalScopeSchool))
	assert.Equal(t, domain.PortalScopeTenant, domain.PersistedPortalScope(""), "the empty JWT scope is the tenant portal")
	assert.Equal(t, domain.PortalScopeUnknown, domain.PersistedPortalScope("unexpected-new-scope"))
}

func TestPushPortalsForScope(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{pushPortalParent}, pushPortalsForScope(domain.PortalScopeParent))
	assert.Equal(t, []string{pushPortalSchool}, pushPortalsForScope(domain.PortalScopeSchool))
	assert.Equal(t, []string{pushPortalStaff}, pushPortalsForScope(domain.PortalScopeTenant))
	assert.Equal(t, []string{pushPortalStaff}, pushPortalsForScope(domain.PortalScopeOrg))
	assert.ElementsMatch(t, []string{pushPortalStaff, pushPortalParent, pushPortalSchool}, pushPortalsForScope(domain.PortalScopeUnknown))
	assert.ElementsMatch(t, []string{pushPortalStaff, pushPortalParent, pushPortalSchool}, pushPortalsForScope(""))
}
