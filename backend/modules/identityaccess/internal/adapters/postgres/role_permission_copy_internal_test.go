package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestAssignRolePermissionsSkipsDeletedPermissions pins the template copy
// against a permission the moto team deletes while a school creates a role
// from „Wie Verwaltung“ (#4574). Before, the copy assigned the permissions it
// had read one by one and answered a deleted one with a foreign key 500.
func TestAssignRolePermissionsSkipsDeletedPermissions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	ctx := testpkg.Ctx(t)
	store := lockingStore(db, tenantID)

	fixture := func(t *testing.T) (roleID, kept, deleted int64) {
		t.Helper()
		unique := time.Now().UnixNano()
		role := testpkg.CreateTestRoleForTenant(t, db, fmt.Sprintf("copy-%d", unique), tenantID)
		keptPermission := testpkg.CreateTestPermission(t, db, fmt.Sprintf("copy-kept-%d", unique), fmt.Sprintf("copy_kept_%d", unique), "read")
		deletedPermission := testpkg.CreateTestPermission(t, db, fmt.Sprintf("copy-deleted-%d", unique), fmt.Sprintf("copy_deleted_%d", unique), "read")
		return role.ID, keptPermission.ID, deletedPermission.ID
	}
	copied := func(t *testing.T, roleID int64) []int64 {
		t.Helper()
		var ids []int64
		require.NoError(t, db.NewSelect().Table("auth.role_permissions").Column("permission_id").
			Where("role_id = ?", roleID).Scan(ctx, &ids))
		return ids
	}

	t.Run("a permission deleted before the copy is skipped", func(t *testing.T) {
		t.Parallel()
		roleID, kept, deleted := fixture(t)
		_, err := db.ExecContext(ctx, `DELETE FROM auth.permissions WHERE id = ?`, deleted)
		require.NoError(t, err)

		err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			return store.AssignRolePermissions(context.WithValue(ctx, heldTxKey{}, tx), roleID, []int64{kept, deleted})
		})
		require.NoError(t, err)
		assert.Equal(t, []int64{kept}, copied(t, roleID))
	})

	t.Run("a deletion that commits during the copy is skipped", func(t *testing.T) {
		t.Parallel()
		roleID, kept, deleted := fixture(t)

		// The deletion holds the permission row until it commits, so the
		// copy's foreign key check waits on it and then finds it gone.
		deletion, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = deletion.Rollback() })
		_, err = deletion.ExecContext(ctx, `DELETE FROM auth.permissions WHERE id = ?`, deleted)
		require.NoError(t, err)

		copyTx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = copyTx.Rollback() })
		var copyPID int
		require.NoError(t, copyTx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &copyPID))

		done := make(chan error, 1)
		go func() {
			done <- store.AssignRolePermissions(context.WithValue(ctx, heldTxKey{}, copyTx), roleID, []int64{kept, deleted})
		}()
		require.Eventually(t, func() bool {
			var waiting bool
			err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid = ? AND wait_event_type = 'Lock')`, copyPID).Scan(ctx, &waiting)
			return err == nil && waiting
		}, 10*time.Second, 20*time.Millisecond, "the copy must wait on the deleted permission row")
		require.NoError(t, deletion.Commit())

		require.NoError(t, <-done)
		require.NoError(t, copyTx.Commit())
		assert.Equal(t, []int64{kept}, copied(t, roleID))
	})
}
