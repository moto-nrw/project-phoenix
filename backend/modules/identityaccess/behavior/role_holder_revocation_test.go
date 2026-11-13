package behavior_test

import (
	"context"
	"errors"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestRoleChangesRevokeEveryHolderSession(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"delete", "replace", "remove"} {
		t.Run(change, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			rbac := roleAdministrationOf(t, setupAuthService(t, db))
			ctx := testpkg.Ctx(t)
			role := testpkg.CreateTestRole(t, db, "revoked-role")
			permission := testpkg.CreateTestPermission(t, db, "revoked-permission", "children", "read")
			require.NoError(t, rbac.AssignPermissionToRole(ctx, role.ID, permission.ID))
			other := testpkg.CreateTestAccount(t, db, "unaffected-holder")
			testpkg.CreateTestToken(t, db, other.ID, "access")
			holders := []int64{}
			for _, name := range []string{"first-holder", "second-holder"} {
				account := testpkg.CreateTestAccount(t, db, name)
				holders = append(holders, account.ID)
				_, err := db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`, account.ID, role.ID, testpkg.Tenant(t)).Exec(ctx)
				require.NoError(t, err)
				testpkg.CreateTestToken(t, db, account.ID, "access")
				testpkg.CreateTestToken(t, db, account.ID, "refresh")
			}
			mutate := func(ctx context.Context) error {
				switch change {
				case "delete":
					return rbac.DeleteRole(ctx, role.ID)
				case "replace":
					return rbac.ReplaceRolePermissions(ctx, role.ID, nil)
				default:
					return rbac.RemovePermissionFromRole(ctx, role.ID, permission.ID)
				}
			}
			rollback := errors.New("rollback role change")
			err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
				require.NoError(t, mutate(txCtx))
				return rollback
			})
			require.ErrorIs(t, err, rollback)
			for _, id := range holders {
				require.Equal(t, 2, liveSessionsAtSchool(t, db, id), "rollback keeps sessions")
			}
			require.NoError(t, mutate(ctx))
			for _, id := range holders {
				require.Zero(t, liveSessionsAtSchool(t, db, id), "no holder session outlives the role change")
			}
			require.Equal(t, 1, liveSessionsAtSchool(t, db, other.ID), "unrelated sessions stay alive")
		})
	}
}

func TestRoleDeletionRevokesASessionMintedBeforeItsHolderLock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := setupAuthService(t, db)
	rbac := roleAdministrationOf(t, service)
	holder := newFenceAccount(t, db, service, "role-definition-fence")
	ctx := context.Background()
	mint, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = mint.Rollback() }()
	_, err = mint.ExecContext(ctx, `SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, holder.accountID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- rbac.DeleteRole(testpkg.Ctx(t), holder.target) }()
	waitForLockWaiters(t, db, 1)
	_, err = mint.ExecContext(ctx, `INSERT INTO auth.tokens (account_id, tenant_id, token, expiry, family_id, generation, portal_scope)
 VALUES (?, ?, ?, NOW() + INTERVAL '1 hour', ?, 0, 'tenant')`, holder.accountID, testpkg.Tenant(t), sha256Hex("role-definition-fence"), "role-definition-fence")
	require.NoError(t, err)
	require.NoError(t, mint.Commit())
	require.NoError(t, awaitResult(t, done))
	require.Zero(t, liveSessionsAtSchool(t, db, holder.accountID), "a session minted before deletion's holder lock is revoked")
}

func TestRemovingAnUnheldRolePermissionKeepsSessions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rbac := roleAdministrationOf(t, setupAuthService(t, db))
	role := testpkg.CreateTestRole(t, db, "permission-no-op")
	permission := testpkg.CreateTestPermission(t, db, "permission-not-held", "children", "read")
	account := testpkg.CreateTestAccount(t, db, "no-op-role-holder")
	_, err := db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`, account.ID, role.ID, testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	testpkg.CreateTestToken(t, db, account.ID, "access")
	require.NoError(t, rbac.RemovePermissionFromRole(testpkg.Ctx(t), role.ID, permission.ID))
	require.Equal(t, 1, liveSessionsAtSchool(t, db, account.ID), "a no-op must not sign holders out")
}
