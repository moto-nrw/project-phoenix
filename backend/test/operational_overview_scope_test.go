// School-wide operational overview scope (#2380).
//
// The setting decides who may see and operate EVERY running module of a
// school. The acceptance criteria demand proof that the freigabe never opens
// another school, so these tests exercise the real settings service and real
// phoenix_tenant transactions rather than a mock: a unit test with a stubbed
// resolver would pass even if the setting itself leaked across tenants.
package test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"

	// Registers the real setting definitions — the gate resolves the key
	// against the production registry, not a test stand-in.
	_ "github.com/moto-nrw/project-phoenix/services/config/defaults"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// overviewStaffContext stands in for the caller's staff record. The staff
// lookup itself is covered by the authorize unit tests; what these tests pin
// is the tenancy of the SETTING and of the group list.
type overviewStaffContext struct{ staff *usersModel.Staff }

func (c overviewStaffContext) GetCurrentStaff(context.Context) (*usersModel.Staff, error) {
	return c.staff, nil
}

func (c overviewStaffContext) HasCurrentStaff(context.Context) (bool, error) {
	return c.staff != nil, nil
}

// setOverviewScope stores the tenant override the way the settings API does.
func setOverviewScope(tb testing.TB, db *bun.DB, tenantID int64, scope string) {
	tb.Helper()

	repository := configRepo.NewSettingValueRepository(ConfigRuntime(db))
	value := &configModel.SettingValue{
		SettingKey: configModel.KeyOperationalOverviewScope,
		Value:      json.RawMessage(`"` + scope + `"`),
	}
	value.SetTenantID(tenantID)
	require.NoError(tb, repository.Upsert(tenant.WithTenantID(context.Background(), tenantID), value))
}

func staffClaimsCtx(ctx context.Context, _ int64) context.Context {
	return ctx
}

// TestOperationalOverviewScopeIsTenantScoped is the core cross-tenant claim:
// school A opening its running modules to all staff leaves school B exactly
// where it was.
func TestOperationalOverviewScopeIsTenantScoped(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)

	tenantA := UniqueTestTenantID(t)
	tenantB := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, tenantA)
	EnsureTestTenant(t, db, tenantB)

	setOverviewScope(t, db, tenantA, configModel.OverviewScopeAllStaff)

	settings := configService.NewSettingsService(
		configRepo.NewSettingValueRepository(ConfigRuntime(db)), nil, nil, SettingsRuntime(t, db), slog.Default(),
	)
	caller := overviewStaffContext{staff: &usersModel.Staff{}}

	assertScope := func(tb testing.TB, tenantID int64, wantScope string, wantOverview bool) {
		tb.Helper()
		err := WithTenantTx(t, context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			ctx := staffClaimsCtx(txCtx, tenantID)

			scope, err := authorize.OperationalOverviewScope(ctx, settings, false)
			require.NoError(tb, err)
			assert.Equal(tb, wantScope, scope)

			allowed, err := authorize.HasOperationalOverview(ctx, settings, caller, false, false)
			require.NoError(tb, err)
			assert.Equal(tb, wantOverview, allowed)
			return nil
		})
		require.NoError(tb, err)
	}

	assertScope(t, tenantA, configModel.OverviewScopeAllStaff, true)
	assertScope(t, tenantB, configModel.OverviewScopeOwn, false)

	// Deactivation: back on the restrictive scope, school A closes again.
	setOverviewScope(t, db, tenantA, configModel.OverviewScopeOwn)
	assertScope(t, tenantA, configModel.OverviewScopeOwn, false)
}

// TestOperationalOverviewNeverCrossesTenants pins what the broad list actually
// returns: even with the widest scope, the enumeration a covered caller gets
// stops at their own school's active groups. RLS is what guarantees that, so
// the query runs through a real phoenix_tenant transaction.
func TestOperationalOverviewNeverCrossesTenants(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)

	tenantA := UniqueTestTenantID(t)
	tenantB := UniqueTestTenantID(t)
	EnsureTestTenant(t, db, tenantA)
	EnsureTestTenant(t, db, tenantB)

	// Both schools open their running modules to all staff — the worst case
	// for a leak, since neither side's scope narrows the query.
	setOverviewScope(t, db, tenantA, configModel.OverviewScopeAllStaff)
	setOverviewScope(t, db, tenantB, configModel.OverviewScopeAllStaff)

	groupA := CreateTestActiveGroupForTenant(t, db, tenantA)
	groupB := CreateTestActiveGroupForTenant(t, db, tenantB)

	repository := activeRepo.NewGroupRepository(db)

	assertSeesOnlyOwn := func(tb testing.TB, ownTenant, ownGroup, foreignGroup int64) {
		tb.Helper()
		err := WithTenantTx(t, context.Background(), db, ownTenant, func(txCtx context.Context, _ bun.Tx) error {
			groups, err := repository.List(txCtx, nil)
			require.NoError(tb, err)

			ids := make(map[int64]bool, len(groups))
			for _, group := range groups {
				ids[group.ID] = true
			}
			assert.True(tb, ids[ownGroup], "a school must see its own running module")
			assert.False(tb, ids[foreignGroup], "the school-wide overview leaked another school's running module")
			return nil
		})
		require.NoError(tb, err)
	}

	assertSeesOnlyOwn(t, tenantA, groupA.ID, groupB.ID)
	assertSeesOnlyOwn(t, tenantB, groupB.ID, groupA.ID)
}
