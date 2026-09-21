package behavior_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
)

// overviewScopeSettings resolves the one school-wide access setting (#2380)
// and errors on any other key, so a live-update path reaching for a second
// access rule fails the test loudly.
func overviewScopeSettings(scope string) *configtest.Mock {
	return &configtest.Mock{
		ResolveStringFn: func(_ context.Context, key string) (string, error) {
			if key != configModels.KeyOperationalOverviewScope {
				return "", fmt.Errorf("unexpected settings key: %s", key)
			}
			return scope, nil
		},
	}
}

// failingOverviewSettings stands in for a settings fault: the caller context
// falls back to the caller's own supervisions rather than opening the school.
func failingOverviewSettings() *configtest.Mock {
	return &configtest.Mock{
		ResolveStringFn: func(context.Context, string) (string, error) {
			return "", errors.New("settings unavailable")
		},
	}
}

type overviewStaffFake struct {
	staff bool
	calls int
}

func (s *overviewStaffFake) HasCurrentStaff(context.Context) (bool, error) {
	s.calls++
	if !s.staff {
		return false, errors.New("user not linked to a staff member")
	}
	return true, nil
}

type overviewCase struct {
	name            string
	settings        *configtest.Mock
	admin           bool
	staff           bool
	assignmentBound bool
	want            bool
	wantErr         bool
}

// The school-wide overview rule the caller context's subscription asks
// through the overview port; each case names the old SSE subscription test
// it carries.
var overviewCases = []overviewCase{
	{name: "admin with admins scope (AdminWithSettingEnabled, WildcardAdminWithoutStaff)", settings: overviewScopeSettings(configModels.OverviewScopeAdmins), admin: true, want: true},
	{name: "admin with own scope (AdminWithOwnScope)", settings: overviewScopeSettings(configModels.OverviewScopeOwn), admin: true, want: true},
	{name: "non-admin with admins scope (NonAdmin)", settings: overviewScopeSettings(configModels.OverviewScopeAdmins), staff: true},
	{name: "caregiver with own scope (OwnScopeKeepsCaregiverNarrow)", settings: overviewScopeSettings(configModels.OverviewScopeOwn), staff: true},
	{name: "staff with all_staff scope (AllStaffScopeSubscribesNonAdmin)", settings: overviewScopeSettings(configModels.OverviewScopeAllStaff), staff: true, want: true},
	{name: "non-staff with all_staff scope (AllStaffScopeDeniesNonStaff)", settings: overviewScopeSettings(configModels.OverviewScopeAllStaff)},
	{name: "admin with setting error (SettingErrorFallsBack)", settings: failingOverviewSettings(), admin: true, wantErr: true},
	{name: "non-admin with setting error (NonAdminStaffError)", settings: failingOverviewSettings(), staff: true, wantErr: true},
	{name: "school portal with all_staff scope", settings: overviewScopeSettings(configModels.OverviewScopeAllStaff), admin: true, staff: true, assignmentBound: true},
}

func TestCallerOverviewAppliesTheOverviewScope(t *testing.T) {
	t.Parallel()

	for _, tc := range overviewCases {
		t.Run(tc.name, func(t *testing.T) {
			staff := &overviewStaffFake{staff: tc.staff}

			got, err := services.CallerOverview{Settings: tc.settings}.HasOperationalOverview(context.Background(), staff, tc.assignmentBound, tc.admin)

			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// Without settings (the old nil settings service) the overview never grants
// the school-wide scope, not even to an admin; the root then binds no
// overview port at all.
func TestCallerOverviewWithoutSettingsNeverGrants(t *testing.T) {
	t.Parallel()

	got, err := services.CallerOverview{}.HasOperationalOverview(context.Background(), &overviewStaffFake{staff: true}, false, true)

	require.NoError(t, err)
	assert.False(t, got)
}

// The staff lookup runs only for ordinary callers in all_staff mode.
func TestCallerOverviewAsksForStaffOnlyUnderAllStaff(t *testing.T) {
	t.Parallel()

	admin := &overviewStaffFake{staff: true}
	_, err := services.CallerOverview{Settings: overviewScopeSettings(configModels.OverviewScopeAllStaff)}.
		HasOperationalOverview(context.Background(), admin, false, true)
	require.NoError(t, err)
	assert.Zero(t, admin.calls)

	own := &overviewStaffFake{staff: true}
	_, err = services.CallerOverview{Settings: overviewScopeSettings(configModels.OverviewScopeOwn)}.
		HasOperationalOverview(context.Background(), own, false, false)
	require.NoError(t, err)
	assert.Zero(t, own.calls)
}

func claimsContext(claims authjwt.AppClaims, permissions ...string) context.Context {
	ctx := context.WithValue(context.Background(), authjwt.CtxClaims, claims)
	return context.WithValue(ctx, authjwt.CtxPermissions, permissions)
}

func TestCallerFromClaimsWithoutClaimsIsUnauthenticated(t *testing.T) {
	t.Parallel()

	assert.Zero(t, services.CallerFromClaims(context.Background()))
}

func TestCallerFromClaimsMapsTheVerifiedPrincipal(t *testing.T) {
	t.Parallel()

	caller := services.CallerFromClaims(claimsContext(authjwt.AppClaims{ID: 42, TenantID: 10, Scope: "tenant", IsAdmin: true}, "users:update"))

	assert.True(t, caller.Authenticated)
	assert.Equal(t, int64(42), caller.AccountID)
	assert.Equal(t, int64(10), caller.ClaimsTenantID)
	assert.Zero(t, caller.TenantID, "the request tenant is added by the compose layer")
	assert.Equal(t, "tenant", caller.Scope)
	assert.False(t, caller.SchoolScope)
	assert.True(t, caller.AdminRole)
	assert.False(t, caller.AdminWildcard)
}

// The school portal (#2527) is assignment-bound.
func TestCallerFromClaimsMarksTheSchoolPortal(t *testing.T) {
	t.Parallel()

	assert.True(t, services.CallerFromClaims(claimsContext(authjwt.AppClaims{ID: 42, TenantID: 10, Scope: "school"})).SchoolScope)
	assert.False(t, services.CallerFromClaims(claimsContext(authjwt.AppClaims{ID: 42, TenantID: 10, Scope: "tenant"})).SchoolScope)
}

// A system-wide admin permission counts as effective admin without the admin
// role claim (successor of TestResolveSSESubscription_WildcardAdminWithoutStaff).
func TestCallerFromClaimsDetectsTheAdminWildcard(t *testing.T) {
	t.Parallel()

	for _, permission := range []string{"admin:*", "*:*"} {
		t.Run(permission, func(t *testing.T) {
			caller := services.CallerFromClaims(claimsContext(authjwt.AppClaims{ID: 42, TenantID: 10}, permission))
			assert.True(t, caller.AdminWildcard)
			assert.False(t, caller.AdminRole)
		})
	}
	assert.False(t, services.CallerFromClaims(claimsContext(authjwt.AppClaims{ID: 42, TenantID: 10}, "users:update")).AdminWildcard)
}
