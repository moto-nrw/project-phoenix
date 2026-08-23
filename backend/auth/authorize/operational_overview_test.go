package authorize

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type overviewSettingsStub struct {
	scope string
	err   error
	// seen records every key asked for, so the test can prove the gate reads
	// exactly one setting and never falls back to the group mode.
	seen []string
}

func (s *overviewSettingsStub) ResolveString(_ context.Context, key string) (string, error) {
	s.seen = append(s.seen, key)
	if s.err != nil {
		return "", s.err
	}
	return s.scope, nil
}

type overviewUserContextStub struct {
	staff *users.Staff
	err   error
}

func (s *overviewUserContextStub) GetCurrentStaff(context.Context) (*users.Staff, error) {
	return s.staff, s.err
}

func overviewStaffCtx() context.Context {
	return context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{ID: 7, TenantID: 3})
}

func overviewAdminCtx() context.Context {
	return context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{ID: 8, TenantID: 3, IsAdmin: true})
}

func overviewWildcardCtx() context.Context {
	return context.WithValue(overviewStaffCtx(), jwt.CtxPermissions, []string{"admin:*"})
}

func overviewVerifiedStaff() *overviewUserContextStub {
	return &overviewUserContextStub{staff: &users.Staff{Model: base.Model{ID: 12}}}
}

func TestHasOperationalOverview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		scope   string
		ctx     context.Context
		userCtx StudentAccessUserContext
		want    bool
	}{
		{name: "own scope denies staff", scope: configModel.OverviewScopeOwn, ctx: overviewStaffCtx(), userCtx: overviewVerifiedStaff()},
		{name: "own scope denies admins", scope: configModel.OverviewScopeOwn, ctx: overviewAdminCtx(), userCtx: overviewVerifiedStaff()},
		{name: "admins scope allows admins", scope: configModel.OverviewScopeAdmins, ctx: overviewAdminCtx(), userCtx: overviewVerifiedStaff(), want: true},
		{name: "admins scope allows admin wildcard", scope: configModel.OverviewScopeAdmins, ctx: overviewWildcardCtx(), userCtx: overviewVerifiedStaff(), want: true},
		{name: "admins scope denies staff", scope: configModel.OverviewScopeAdmins, ctx: overviewStaffCtx(), userCtx: overviewVerifiedStaff()},
		{name: "all staff scope allows staff", scope: configModel.OverviewScopeAllStaff, ctx: overviewStaffCtx(), userCtx: overviewVerifiedStaff(), want: true},
		{name: "all staff scope allows admins", scope: configModel.OverviewScopeAllStaff, ctx: overviewAdminCtx(), userCtx: overviewVerifiedStaff(), want: true},
		{name: "all staff scope denies accounts without a staff record", scope: configModel.OverviewScopeAllStaff, ctx: overviewStaffCtx(), userCtx: &overviewUserContextStub{}},
		{name: "all staff scope denies when the staff lookup fails", scope: configModel.OverviewScopeAllStaff, ctx: overviewStaffCtx(), userCtx: &overviewUserContextStub{err: errors.New("boom")}},
		{name: "unknown scope collapses to own", scope: "everyone", ctx: overviewAdminCtx(), userCtx: overviewVerifiedStaff()},
		{name: "empty scope collapses to own", scope: "", ctx: overviewAdminCtx(), userCtx: overviewVerifiedStaff()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			settings := &overviewSettingsStub{scope: tt.scope}
			got, err := HasOperationalOverview(tt.ctx, settings, tt.userCtx)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, []string{configModel.KeyOperationalOverviewScope}, settings.seen,
				"the gate must read exactly one setting — the group mode is not an access rule")
		})
	}
}

func TestHasOperationalOverviewSettingsFault(t *testing.T) {
	t.Parallel()

	settings := &overviewSettingsStub{err: errors.New("database unavailable")}

	got, err := HasOperationalOverview(overviewAdminCtx(), settings, overviewVerifiedStaff())

	require.Error(t, err, "a settings fault must surface, never be read as a tenant choice")
	assert.False(t, got)
}

func TestHasOperationalOverviewWithoutSettings(t *testing.T) {
	t.Parallel()

	got, err := HasOperationalOverview(overviewAdminCtx(), nil, overviewVerifiedStaff())

	require.NoError(t, err)
	assert.False(t, got, "no settings service means the restrictive default")
}

// The all_staff scope must not resolve the staff record for callers already
// covered as admins — that lookup is a query per request.
func TestHasOperationalOverviewSkipsStaffLookupForAdmins(t *testing.T) {
	t.Parallel()

	userCtx := &overviewUserContextStub{err: errors.New("must not be called")}

	got, err := HasOperationalOverview(overviewAdminCtx(), &overviewSettingsStub{scope: configModel.OverviewScopeAllStaff}, userCtx)

	require.NoError(t, err)
	assert.True(t, got)
}
