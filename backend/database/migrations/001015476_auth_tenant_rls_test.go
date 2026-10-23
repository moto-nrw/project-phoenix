package migrations

import (
	"context"
	"errors"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var repairedTenantRLSTables = []string{
	"auth.account_tenants",
	"auth.mfa_email_challenges",
	"auth.mfa_trusted_devices",
	"auth.passkey_sessions",
	"auth.parent_declaration_permission_grants",
}

func seedAuthTenantRLS(t *testing.T, db *testpkg.DB, tenantID int64) {
	t.Helper()
	ctx := t.Context()
	account := testpkg.CreateTestAccount(t, db, "rls-coverage@example.invalid")
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	_, err := db.ExecContext(ctx, `INSERT INTO auth.mfa_email_challenges
		(account_id, tenant_id, scope, code_hash, expires_at) VALUES (?, ?, '', 'test-placeholder', NOW() + INTERVAL '1 hour')`, account.ID, tenantID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO auth.mfa_trusted_devices
		(account_id, tenant_id, token_hash, expires_at) VALUES (?, ?, 'test-placeholder', NOW() + INTERVAL '1 hour')`, account.ID, tenantID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO auth.passkey_sessions
		(id, account_id, tenant_id, purpose, rp_id, expected_origin, session_json, expires_at)
		VALUES (?, ?, ?, 'login', 'localhost', 'http://localhost', '{}'::jsonb, NOW() + INTERVAL '1 hour')`, fmt.Sprintf("rls-%d", tenantID), account.ID, tenantID)
	require.NoError(t, err)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Test", "RLS", "RLS")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, tenantID, "Test", "RLS", "rls@example.invalid")
	link := testpkg.CreateTestStudentGuardianLinkForTenant(t, db, tenantID, student.ID, guardian.ID, "primary_guardian")
	_, err = db.ExecContext(ctx, `INSERT INTO auth.parent_declaration_permission_grants (tenant_id, relationship_id) VALUES (?, ?)`, tenantID, link.ID)
	require.NoError(t, err)
}

func TestAuthTenantRLSRejectsCrossSchoolAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	ours := testpkg.Tenant(t)
	theirs := testpkg.NewTenantScope(t, db).TenantID
	seedAuthTenantRLS(t, db, ours)
	seedAuthTenantRLS(t, db, theirs)
	for _, table := range repairedTenantRLSTables {
		t.Run(table, func(t *testing.T) {
			require.NoError(t, testpkg.WithTenantTx(t, ctx, db, ours, func(ctx context.Context, tx bun.Tx) error {
				var own, foreign int
				require.NoError(t, tx.NewRaw(`SELECT count(*) FROM ? WHERE tenant_id = ?`, bun.Ident(table), ours).Scan(ctx, &own))
				require.Positive(t, own, "own school rows stay accessible")
				require.NoError(t, tx.NewRaw(`SELECT count(*) FROM ? WHERE tenant_id = ?`, bun.Ident(table), theirs).Scan(ctx, &foreign))
				require.Zero(t, foreign, "another school's rows must not be visible")
				for _, statement := range []string{"UPDATE ? SET tenant_id = tenant_id WHERE tenant_id = ?", "DELETE FROM ? WHERE tenant_id = ?"} {
					result, err := tx.ExecContext(ctx, statement, bun.Ident(table), theirs)
					require.NoError(t, err)
					count, err := result.RowsAffected()
					require.NoError(t, err)
					require.Zero(t, count, "another school's rows must not be changed")
				}
				return nil
			}))
			err := testpkg.WithTenantTx(t, ctx, db, ours, func(ctx context.Context, tx bun.Tx) error {
				_, err := tx.ExecContext(ctx, `UPDATE ? SET tenant_id = ? WHERE tenant_id = ?`, bun.Ident(table), theirs, ours)
				return err
			})
			require.ErrorContains(t, err, "row-level security", "WITH CHECK must reject reassignment to another school")
		})
	}
}

func TestAuthTenantRLSKeepsPreAuthenticationAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ours := testpkg.Tenant(t)
	theirs := testpkg.NewTenantScope(t, db).TenantID
	seedAuthTenantRLS(t, db, ours)
	seedAuthTenantRLS(t, db, theirs)
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	ctx := t.Context()
	_, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_auth; SET LOCAL app.current_tenant_id = ''`)
	require.NoError(t, err)
	for _, table := range []string{"auth.account_tenants", "auth.mfa_email_challenges", "auth.mfa_trusted_devices", "auth.passkey_sessions"} {
		for _, tenantID := range []int64{ours, theirs} {
			var count int
			require.NoError(t, tx.NewRaw(`SELECT count(*) FROM ? WHERE tenant_id = ?`, bun.Ident(table), tenantID).Scan(ctx, &count))
			require.Positive(t, count, "pre-auth flows need their existing cross-school reads")
		}
	}
	_, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`)
	require.NoError(t, err)
	for _, table := range repairedTenantRLSTables {
		var count int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM ?`, bun.Ident(table)).Scan(ctx, &count))
		require.Zero(t, count, "without school scope the request role must see no rows, including through the pre-auth policy")
	}
}

func TestActiveSchoolCapabilityRequiresLocalAccountAndKeepsSnapshot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	ours := testpkg.Tenant(t)
	theirs := testpkg.NewTenantScope(t, db).TenantID
	shared := testpkg.CreateTestAccount(t, db, "shared-rls@example.invalid")
	testpkg.MapAccountToTenant(t, db, shared.ID, theirs)
	foreign := testpkg.CreateTestAccount(t, db, "foreign-rls@example.invalid")
	testpkg.UnclaimTestAccount(t, db, foreign.ID)
	testpkg.MapAccountToTenant(t, db, foreign.ID, theirs)
	rollback := errors.New("roll back local membership change")
	err := testpkg.WithTenantTx(t, ctx, db, ours, func(ctx context.Context, tx bun.Tx) error {
		var ids []int64
		require.NoError(t, tx.NewRaw(`SELECT tenant_id FROM auth.active_school_ids_for_account(?)`, shared.ID).Scan(ctx, &ids))
		require.ElementsMatch(t, []int64{ours, theirs}, ids, "offboarding must preserve this locally shared account at the other school")
		ids = nil
		require.NoError(t, tx.NewRaw(`SELECT tenant_id FROM auth.active_school_ids_for_account(?)`, foreign.ID).Scan(ctx, &ids))
		require.Empty(t, ids, "the capability must not reveal schools of a foreign-only account")
		_, err := tx.ExecContext(ctx, `UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`, shared.ID, ours)
		require.NoError(t, err)
		require.NoError(t, tx.NewRaw(`SELECT tenant_id FROM auth.active_school_ids_for_account(?)`, shared.ID).Scan(ctx, &ids))
		require.Empty(t, ids, "the same transaction sees its uncommitted removal of local access")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	tx, err := db.Begin()
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback()) }()
	_, err = tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant; SET LOCAL app.current_tenant_id = ''`)
	require.NoError(t, err)
	var ids []int64
	require.NoError(t, tx.NewRaw(`SELECT tenant_id FROM auth.active_school_ids_for_account(?)`, shared.ID).Scan(ctx, &ids))
	require.Empty(t, ids, "a request role without school context cannot use the capability")
}
