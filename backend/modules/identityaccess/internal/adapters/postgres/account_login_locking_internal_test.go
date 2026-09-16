package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type heldTxKey struct{}

// lockingStore runs every statement on the transaction the context carries,
// the way the composition's database runtime does.
func lockingStore(db *bun.DB, tenantID int64) *Store {
	return New(func(ctx context.Context) (bun.IDB, error) {
		if tx, ok := ctx.Value(heldTxKey{}).(bun.Tx); ok {
			return tx, nil
		}
		return db, nil
	}, func(context.Context) TenantScope { return TenantScope{TenantID: tenantID} })
}

// TestLockAccountPermissionSources pins the locking contract the school-portal
// mint guard and the staff preview depend on (#2207, #2893). They read an
// account's effective permissions and write them into a JWT that stays valid
// for a full AUTH_JWT_EXPIRY. Locking the account row is not enough: a bare
// permission revocation touches neither the account nor its role rows and
// would commit straight through the mint's window. These cases fail if the
// FOR SHARE clauses are ever dropped from either statement. Moved from the
// retired PermissionRepository.LockAccountPermissionSourcesForTenant (#3225).
func TestLockAccountPermissionSources(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	ctx := testpkg.Ctx(t)
	store := lockingStore(db, tenantID)

	unique := time.Now().UnixNano()
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("perm-lock-%d", unique))
	role := testpkg.CreateTestRoleForTenant(t, db, fmt.Sprintf("perm-lock-%d", unique), tenantID)
	directPermission := testpkg.CreateTestPermission(t, db, fmt.Sprintf("perm-lock-direct-%d", unique), "perm_lock_direct", "read")
	rolePermission := testpkg.CreateTestPermission(t, db, fmt.Sprintf("perm-lock-role-%d", unique), "perm_lock_role", "read")
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)

	_, err := db.ExecContext(ctx,
		`INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())`,
		account.ID, role.ID, tenantID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO auth.role_permissions (role_id, permission_id, created_at, updated_at) VALUES (?, ?, NOW(), NOW())`,
		role.ID, rolePermission.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO auth.account_permissions (account_id, permission_id, tenant_id, granted, created_at, updated_at)
		 VALUES (?, ?, ?, true, NOW(), NOW())`,
		account.ID, directPermission.ID, tenantID)
	require.NoError(t, err)

	hold := func(t *testing.T) bun.Tx {
		t.Helper()
		holder, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = holder.Rollback() })
		_, err = store.LockAccountPermissionSources(context.WithValue(ctx, heldTxKey{}, holder), account.ID, tenantID)
		require.NoError(t, err)
		return holder
	}

	t.Run("a direct grant cannot be revoked while the lock is held", func(t *testing.T) {
		holder := hold(t)
		revoke := func(tx bun.Tx) error {
			_, execErr := tx.ExecContext(ctx,
				`DELETE FROM auth.account_permissions WHERE account_id = ? AND permission_id = ? AND tenant_id = ?`,
				account.ID, directPermission.ID, tenantID)
			return execErr
		}

		err := runWithLockTimeout(ctx, db, "200ms", revoke)
		require.Error(t, err, "revoking a direct grant must wait for the mint transaction")
		assert.True(t, isLockTimeoutError(err), "expected lock_timeout error, got: %v", err)

		require.NoError(t, holder.Rollback())
		require.NoError(t, runWithLockTimeout(ctx, db, "200ms", revoke), "the revocation goes through once the mint released the lock")
	})

	t.Run("a role permission cannot be revoked while the lock is held", func(t *testing.T) {
		hold(t)
		err := runWithLockTimeout(ctx, db, "200ms", func(tx bun.Tx) error {
			_, execErr := tx.ExecContext(ctx,
				`DELETE FROM auth.role_permissions WHERE role_id = ? AND permission_id = ?`,
				role.ID, rolePermission.ID)
			return execErr
		})
		require.Error(t, err, "revoking a permission from a held role must wait for the mint transaction")
		assert.True(t, isLockTimeoutError(err), "expected lock_timeout error, got: %v", err)
	})

	t.Run("two concurrent mints do not block each other", func(t *testing.T) {
		// FOR SHARE, not FOR UPDATE: two logins of the same Lehrkraft (phone
		// and laptop) must not serialize on the permission rows.
		hold(t)
		err := runWithLockTimeout(ctx, db, "500ms", func(tx bun.Tx) error {
			_, lockErr := store.LockAccountPermissionSources(context.WithValue(ctx, heldTxKey{}, tx), account.ID, tenantID)
			return lockErr
		})
		assert.NoError(t, err, "concurrent FOR SHARE acquisitions must not block each other")
	})
}

// runWithLockTimeout opens a transaction with a short lock_timeout, runs fn in
// it and rolls back, so a blocked lock request surfaces as a deterministic
// error instead of a goroutine hang.
func runWithLockTimeout(ctx context.Context, db *bun.DB, timeout string, fn func(bun.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, "SET LOCAL lock_timeout = ?", timeout); err != nil {
		return err
	}
	return fn(tx)
}

// isLockTimeoutError reports whether PostgreSQL refused the lock request due
// to lock_timeout (SQLSTATE 55P03).
func isLockTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "lock timeout") ||
		strings.Contains(msg, "lock_not_available") ||
		strings.Contains(msg, "55P03")
}
