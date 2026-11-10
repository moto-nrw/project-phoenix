package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestOffboardingTraegerCapabilityRequiresLocalActiveMapping(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	local := testpkg.CreateTestAccount(t, db, "offboarding-local")
	foreign := testpkg.CreateTestAccount(t, db, "offboarding-foreign")
	testpkg.UnclaimTestAccount(t, db, foreign.ID)
	testpkg.MapAccountToTenant(t, db, foreign.ID, testpkg.NewTenantScope(t, db).TenantID)
	var organizationID int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, testpkg.Tenant(t)).Scan(ctx, &organizationID))
	testpkg.CreateTestTraegerMember(t, db, organizationID, local.ID)
	testpkg.CreateTestTraegerMember(t, db, organizationID, foreign.ID)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
		var retained bool
		require.NoError(t, tx.NewRaw(`SELECT platform.account_has_retained_traeger_membership(?)`, local.ID).Scan(ctx, &retained))
		require.True(t, retained)
		require.NoError(t, tx.NewRaw(`SELECT platform.account_has_retained_traeger_membership(?)`, foreign.ID).Scan(ctx, &retained))
		require.False(t, retained, "another school's account must reveal no membership fact")
		_, err := tx.ExecContext(ctx, `UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`, local.ID, testpkg.Tenant(t))
		require.NoError(t, err)
		require.NoError(t, tx.NewRaw(`SELECT platform.account_has_retained_traeger_membership(?)`, local.ID).Scan(ctx, &retained))
		require.False(t, retained, "the capability must use the caller's transaction snapshot")
		_, err = tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', '', true)`)
		require.NoError(t, err)
		require.NoError(t, tx.NewRaw(`SELECT platform.account_has_retained_traeger_membership(?)`, foreign.ID).Scan(ctx, &retained))
		require.False(t, retained, "no school scope authorizes no account")
		return nil
	}))
}

func TestOffboardingTraegerCapabilityPrivilegesAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	for range 2 {
		require.NoError(t, offboardingTraegerMembershipDown(ctx, db))
	}
	require.NoError(t, offboardingTraegerMembershipUp(ctx, db))
	var secured bool
	require.NoError(t, db.NewRaw(`SELECT prosecdef AND proconfig @> ARRAY['search_path=pg_catalog, platform, auth']
		AND pg_get_function_result(oid) = 'boolean'
		FROM pg_proc WHERE oid = 'platform.account_has_retained_traeger_membership(bigint)'::regprocedure`).Scan(ctx, &secured))
	require.True(t, secured, "the definer capability returns only a Boolean under a fixed search path")
	for _, role := range []string{"phoenix_auth", "phoenix_admin"} {
		var allowed bool
		require.NoError(t, db.NewRaw(`SELECT has_function_privilege(?, 'platform.account_has_retained_traeger_membership(bigint)', 'EXECUTE')`, role).Scan(ctx, &allowed))
		require.False(t, allowed, "only phoenix_tenant gets an execution grant; PUBLIC gets none")
	}
	var tableAccess bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'platform.organization_accounts', 'SELECT')`).Scan(ctx, &tableAccess))
	require.False(t, tableAccess, "the school must not gain direct access to Träger memberships")
}
