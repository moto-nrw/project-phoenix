package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A failed savepoint control of the permission copy is fatal (E19, #6316,
// PR #6380 table F): the copy neither repeats itself in the untrusted
// transaction nor offers the runtime a SQLSTATE to replay, and nothing of it
// commits. With an intact rollback the copy still repeats after a deleted
// permission (#4574, unchanged).
func TestAssignRolePermissionsTreatsAFailedSavepointAsFatal(t *testing.T) {
	t.Parallel()
	probe := testpkg.NewFaultProbe(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, probe.Super, tenantID)
	store := lockingStore(probe.Pool, tenantID)
	deletedPermission := pgdriver.NewError(map[byte]string{'S': "ERROR", 'V': "ERROR", 'C': "23503", 'n': "fk_role_permissions_permission", 'M': "injected deleted permission"})

	cases := []struct {
		name      string
		arm       func()
		fatal     bool
		committed int
	}{
		{name: "F-a_rollback_fails_40P01", fatal: true, arm: func() {
			probe.Faults.Arm(testpkg.FaultNone, nil)
			probe.Faults.AlsoFailStatement("INSERT INTO auth.role_permissions", deletedPermission)
			probe.Faults.AlsoFail(testpkg.FaultSavepointRestore, testpkg.InjectedFailure("40P01"))
		}},
		{name: "F-a_rollback_fails_bad_conn", fatal: true, arm: func() {
			probe.Faults.Arm(testpkg.FaultNone, nil)
			probe.Faults.AlsoFailStatement("INSERT INTO auth.role_permissions", deletedPermission)
			probe.Faults.AlsoFail(testpkg.FaultSavepointRestore, testpkg.InjectedFailure("bad_conn"))
		}},
		{name: "F-b_release_fails_40001", fatal: true, arm: func() {
			probe.Faults.Arm(testpkg.FaultSavepointRelease, testpkg.InjectedFailure("40001"))
		}},
		{name: "F-c_deleted_permission_rollback_ok", committed: 1, arm: func() {
			probe.Faults.Arm(testpkg.FaultNone, nil)
			probe.Faults.AlsoFailStatement("INSERT INTO auth.role_permissions", deletedPermission)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			unique := time.Now().UnixNano()
			role := testpkg.CreateTestRoleForTenant(t, probe.Super, fmt.Sprintf("sp-%d", unique), tenantID)
			permission := testpkg.CreateTestPermission(t, probe.Super, fmt.Sprintf("sp-%d", unique), fmt.Sprintf("sp_%d", unique), "read")
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			c.arm()
			err := probe.Pool.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
				return store.AssignRolePermissions(context.WithValue(ctx, heldTxKey{}, tx), role.ID, []int64{permission.ID})
			})
			require.True(t, probe.Faults.Fired(), "every armed fault fired")
			var committed int
			require.NoError(t, probe.Super.NewRaw(`SELECT count(*) FROM auth.role_permissions WHERE role_id = ?`, role.ID).Scan(ctx, &committed))
			t.Logf("CELL %s err=%v committed=%d", c.name, err, committed)
			require.Equal(t, c.committed, committed)
			if !c.fatal {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tenant.ErrSavepointControl, "the runtime does not replay a failed savepoint control")
		})
	}
}
