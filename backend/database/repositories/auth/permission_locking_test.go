package auth_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestPermissionRepository_LockAccountPermissionSourcesForTenant pins the
// locking contract the school-portal mint guard depends on (#2207).
//
// The guard reads an account's effective permissions and writes them into a
// JWT that stays valid for a full AUTH_JWT_EXPIRY. Locking the account row is
// not enough: a bare permission revocation (RemovePermissionFromAccount,
// RemovePermissionFromRole) touches neither the account nor its role rows and
// would commit straight through the guard's window. These tests fail if the
// FOR SHARE clauses are ever dropped from either statement.
func TestPermissionRepository_LockAccountPermissionSourcesForTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	// Registered as a cleanup, not deferred: defers run BEFORE t.Cleanup, so a
	// deferred close would shut the pool down under every fixture teardown
	// below and leave their rows in the shared test database.

	repo := authRepo.NewPermissionRepository(db)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	unique := time.Now().UnixNano()
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("perm-lock-%d", unique))
	role := testpkg.CreateTestRoleForTenant(t, db, fmt.Sprintf("perm-lock-%d", unique), tenantID)
	directPermission := testpkg.CreateTestPermission(t, db, fmt.Sprintf("perm-lock-direct-%d", unique), "perm_lock_direct", "read")
	rolePermission := testpkg.CreateTestPermission(t, db, fmt.Sprintf("perm-lock-role-%d", unique), "perm_lock_role", "read")

	// Cleanup is LIFO: the association rows go first, the tenant last.
	t.Cleanup(func() { testpkg.CleanupTestTenant(t, db, tenantID) })
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, account.ID) })
	t.Cleanup(func() { testpkg.CleanupPermissionRecords(t, db, directPermission.ID, rolePermission.ID) })
	t.Cleanup(func() { testpkg.CleanupRoleRecords(t, db, role.ID) })

	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	accountRole := &authModels.AccountRole{AccountID: account.ID, RoleID: role.ID}
	accountRole.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(accountRole).ModelTableExpr(`auth.account_roles`).Exec(ctx)
	require.NoError(t, err)

	_, err = db.NewInsert().
		Model(&authModels.RolePermission{RoleID: role.ID, PermissionID: rolePermission.ID}).
		ModelTableExpr(`auth.role_permissions`).
		Exec(ctx)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO auth.account_permissions (account_id, permission_id, tenant_id, granted, created_at, updated_at)
		 VALUES (?, ?, ?, true, NOW(), NOW())`,
		account.ID, directPermission.ID, tenantID)
	require.NoError(t, err)

	t.Run("a direct grant cannot be revoked while the lock is held", func(t *testing.T) {
		holder, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = holder.Rollback() }()

		require.NoError(t, repo.LockAccountPermissionSourcesForTenant(
			modelBase.ContextWithTx(ctx, &holder), account.ID, tenantID))

		err = runWithLockTimeout(ctx, db, "200ms", func(txCtx context.Context) error {
			_, execErr := repoBase.GetDB(txCtx, db).ExecContext(txCtx,
				`DELETE FROM auth.account_permissions WHERE account_id = ? AND permission_id = ? AND tenant_id = ?`,
				account.ID, directPermission.ID, tenantID)
			return execErr
		})
		require.Error(t, err, "revoking a direct grant must wait for the mint transaction")
		assert.True(t, isLockTimeoutError(err), "expected lock_timeout error, got: %v", err)

		require.NoError(t, holder.Rollback())
		err = runWithLockTimeout(ctx, db, "200ms", func(txCtx context.Context) error {
			_, execErr := repoBase.GetDB(txCtx, db).ExecContext(txCtx,
				`DELETE FROM auth.account_permissions WHERE account_id = ? AND permission_id = ? AND tenant_id = ?`,
				account.ID, directPermission.ID, tenantID)
			return execErr
		})
		require.NoError(t, err, "the revocation must go through once the mint released the lock")
	})

	t.Run("a role permission cannot be revoked while the lock is held", func(t *testing.T) {
		holder, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = holder.Rollback() }()

		require.NoError(t, repo.LockAccountPermissionSourcesForTenant(
			modelBase.ContextWithTx(ctx, &holder), account.ID, tenantID))

		err = runWithLockTimeout(ctx, db, "200ms", func(txCtx context.Context) error {
			_, execErr := repoBase.GetDB(txCtx, db).ExecContext(txCtx,
				`DELETE FROM auth.role_permissions WHERE role_id = ? AND permission_id = ?`,
				role.ID, rolePermission.ID)
			return execErr
		})
		require.Error(t, err, "revoking a permission from a held role must wait for the mint transaction")
		assert.True(t, isLockTimeoutError(err), "expected lock_timeout error, got: %v", err)
	})

	t.Run("two concurrent mints do not block each other", func(t *testing.T) {
		// FOR SHARE, not FOR UPDATE: two logins of the same Lehrkraft (phone and
		// laptop) must not serialize on the permission rows.
		holder, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = holder.Rollback() }()

		require.NoError(t, repo.LockAccountPermissionSourcesForTenant(
			modelBase.ContextWithTx(ctx, &holder), account.ID, tenantID))

		err = runWithLockTimeout(ctx, db, "500ms", func(txCtx context.Context) error {
			return repo.LockAccountPermissionSourcesForTenant(txCtx, account.ID, tenantID)
		})
		assert.NoError(t, err, "concurrent FOR SHARE acquisitions must not block each other")
	})
}

// runWithLockTimeout opens a transaction with a short lock_timeout, invokes fn
// with a tx-scoped context and rolls back, so a blocked lock request surfaces
// as a deterministic error instead of a goroutine hang.
func runWithLockTimeout(ctx context.Context, db *bun.DB, timeout string, fn func(context.Context) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = ?", timeout); err != nil {
		return err
	}

	return fn(modelBase.ContextWithTx(ctx, &tx))
}

// isLockTimeoutError reports whether PostgreSQL refused the lock request due to
// lock_timeout (SQLSTATE 55P03).
func isLockTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "lock timeout") ||
		strings.Contains(msg, "lock_not_available") ||
		strings.Contains(msg, "55P03")
}
