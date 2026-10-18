// School-wide operational overview scope (#2380).
//
// The setting decides who may see EVERY running module of a school. The
// acceptance criteria demand proof that the freigabe never opens another
// school, so this test exercises the real settings service and real
// phoenix_tenant transactions rather than a mock: a unit test with a stubbed
// resolver would pass even if the setting itself leaked across tenants.
package config

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	configRepository "github.com/moto-nrw/project-phoenix/modules/settings/internal/adapters/configpostgres"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	// Registers the real setting definitions — the decision resolves the key
	// against the production registry, not a test stand-in.
	_ "github.com/moto-nrw/project-phoenix/modules/settings/internal/defaults"
)

// overviewStaff stands in for the caller's staff identity. The staff lookup
// itself is covered by the authorize unit tests; what this test pins is the
// tenancy of the SETTING.
type overviewStaff struct{}

func (overviewStaff) HasCurrentStaff(context.Context) (bool, error) { return true, nil }

// setOverviewScope stores the tenant override the way the settings API does.
func setOverviewScope(tb testing.TB, db *bun.DB, tenantID int64, scope string) {
	tb.Helper()

	repository := configRepository.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	value := &configModel.SettingValue{
		SettingKey: configModel.KeyOperationalOverviewScope,
		Value:      json.RawMessage(`"` + scope + `"`),
	}
	value.SetTenantID(tenantID)
	require.NoError(tb, repository.Upsert(testpkg.ContextForTenant(context.Background(), tenantID), value))
}

// TestOperationalOverviewScopeIsTenantScoped is the core cross-tenant claim:
// school A opening its running modules to all staff leaves school B exactly
// where it was.
func TestOperationalOverviewScopeIsTenantScoped(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	setOverviewScope(t, db, tenantA, configModel.OverviewScopeOwn)

	settings := NewSettingsService(
		configRepository.NewSettingValueRepository(testpkg.ConfigRuntime(db)), nil, nil, testpkg.SettingsRuntime(t, db), slog.Default(),
	)

	assertScope := func(tb testing.TB, tenantID int64, wantScope string, wantOverview bool) {
		tb.Helper()
		err := testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
			scope, err := settings.ResolveString(ctx, configModel.KeyOperationalOverviewScope)
			require.NoError(tb, err)
			assert.Equal(tb, wantScope, scope)

			allowed, err := securityruntime.CanViewOperationalOverview(ctx, settings, overviewStaff{}, false, false)
			require.NoError(tb, err)
			assert.Equal(tb, wantOverview, allowed)
			return nil
		})
		require.NoError(tb, err)
	}

	assertScope(t, tenantA, configModel.OverviewScopeOwn, false)
	assertScope(t, tenantB, configModel.OverviewScopeAllStaff, true)

	// Changing school A back to the whole-team scope leaves school B untouched.
	setOverviewScope(t, db, tenantA, configModel.OverviewScopeAllStaff)
	assertScope(t, tenantA, configModel.OverviewScopeAllStaff, true)
	assertScope(t, tenantB, configModel.OverviewScopeAllStaff, true)
}
