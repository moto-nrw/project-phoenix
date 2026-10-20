package compose

import (
	"context"
	"testing"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentLocksRequireCallerTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	module, err := New(Dependencies{DB: db, StudentDirectory: testpkg.StudentDirectory{}, StudentOwners: lockTestOwners{}, Observe: func(Observation) {}}, nil)
	require.NoError(t, err)
	own := testpkg.CreateTestStudent(t, db, "Lock", "Own", "1a")
	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherSchool, "Lock", "Foreign", "1a")
	locks := map[string]func(context.Context) error{
		"student":    func(ctx context.Context) error { return module.LockStudent(ctx, own.ID) },
		"class gate": module.LockEnrollmentClassWrites,
		"photo gate": module.LockStudentPhotoFeature,
		"records": func(ctx context.Context) error {
			rows, err := module.LockStudentRecordsByID(ctx, []int64{own.ID, foreign.ID})
			if err == nil {
				require.Len(t, rows, 1)
				require.Equal(t, own.ID, rows[0].ID)
			}
			return err
		},
		"enrollment": func(ctx context.Context) error {
			_, err := module.ReadEnrollmentStudent(ctx, own.ID, "update")
			return err
		},
		"mutation": func(ctx context.Context) error {
			_, err := module.FindStudentRecordForMutation(ctx, own.ID)
			return err
		},
	}
	for name, lock := range locks {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, lock(ctx), tenant.ErrTransactionRequired)
			require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error { return lock(txCtx) }))
		})
	}
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		require.Error(t, module.LockStudent(txCtx, foreign.ID))
		_, err := module.ReadEnrollmentStudent(txCtx, foreign.ID, "update")
		require.Error(t, err)
		_, err = module.FindStudentRecordForMutation(txCtx, foreign.ID)
		require.Error(t, err)
		return nil
	}))
}

// Foreign owner commands are deliberately unbound. The gate port verifies
// ambient scope here; School Membership tests its PostgreSQL gate itself.
type lockTestOwners struct {
	peoplePorts.StudentOwners
}

func (o lockTestOwners) LockClassWrites(ctx context.Context, _ bool) error {
	return tenant.RequireTransaction(ctx)
}
