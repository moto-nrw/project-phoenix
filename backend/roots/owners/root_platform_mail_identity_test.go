package owners

import (
	"context"
	"log/slog"
	"testing"

	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The reply-to setting must be read for the target school even when the mail
// is built inside a transaction that cannot serve that school: the operator
// invites a school admin inside an administrative transaction, and a tenant
// transaction belongs to exactly one school (#4535).
func TestTenantMailIdentityReadsReplyToOutsideForeignTransaction(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	settings, err := NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	organizations, err := NewOrganizationTenancy(database)
	require.NoError(t, err)
	identity := TenantMailIdentity(organizations, settings.Settings, RootEnvironment{Logger: slog.Default()})

	target := testpkg.Tenant(t)
	targetCtx := testpkg.Ctx(t)
	setReplyToAddress(t, db, target, "antworten@ziel-schule.example")

	resolve := func(t *testing.T, ctx context.Context) string {
		t.Helper()
		got, err := identity.ResolveReplyTo(ctx, target)
		require.NoError(t, err)
		return got.Address
	}

	t.Run("operator administrative transaction", func(t *testing.T) {
		var got string
		require.NoError(t, tenant.WithinAdmin(targetCtx, func(adminCtx context.Context) error {
			got = resolve(t, tenant.WithTenantID(adminCtx, target))
			return nil
		}))
		require.Equal(t, "antworten@ziel-schule.example", got)
	})

	t.Run("transaction of another school", func(t *testing.T) {
		other := testpkg.OwnTenant(t)
		setReplyToAddress(t, db, other, "antworten@andere-schule.example")
		var got string
		require.NoError(t, tenant.WithinTenant(testpkg.Ctx(t), tenantID(t, other), func(otherCtx context.Context) error {
			got = resolve(t, otherCtx)
			return nil
		}))
		require.Equal(t, "antworten@ziel-schule.example", got, "only the target school's setting may be read")
	})

	t.Run("transaction of the same school", func(t *testing.T) {
		var got string
		require.NoError(t, tenant.WithinTenant(targetCtx, tenantID(t, target), func(txCtx context.Context) error {
			got = resolve(t, txCtx)
			return nil
		}))
		require.Equal(t, "antworten@ziel-schule.example", got)
	})
}

func tenantID(t *testing.T, id int64) tenant.TenantID {
	t.Helper()
	value, err := tenant.NewTenantID(id)
	require.NoError(t, err)
	return value
}

func setReplyToAddress(t *testing.T, db *bun.DB, tenantID int64, address string) {
	t.Helper()
	_, err := db.ExecContext(t.Context(), `
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, ?, to_jsonb(?::text))
		ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
		tenantID, configModels.KeyEmailReplyToAddress, address)
	require.NoError(t, err)
}
