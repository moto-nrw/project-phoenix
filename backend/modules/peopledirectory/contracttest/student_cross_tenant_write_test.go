package contracttest_test

import (
	"context"
	"testing"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestCrossTenantStudentWrite_RowsAffectedGuard pins the child's writes of the
// cross-tenant write guard (test.TestCrossTenantWrite_RowsAffectedGuard): a
// tenant's context that targets another school's child finds no row. The
// child's write path is People Directory's (#3349), so this has to be the
// composed repository the composition root hands out.
func TestCrossTenantStudentWrite_RowsAffectedGuard(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	studentA := testpkg.CreateTestStudentForTenant(t, db, tenantA, "WriteA", "Student", "1a")
	ctxForTenant := func(tenantID int64) context.Context {
		return tenant.WithTenantID(testpkg.WithPackageTenantRuntime(context.Background()), tenantID)
	}
	ctxA := ctxForTenant(tenantA)
	ctxB := ctxForTenant(tenantB)

	t.Run("student update blocked", func(t *testing.T) {
		repo := api.NewUnboundStudentRepository(testpkg.NewDatabase(t, db))
		stored, err := repo.FindByID(ctxA, studentA.ID)
		require.NoError(t, err)
		err = repo.Update(ctxB, stored)
		require.Error(t, err, "cross-tenant student update must fail")
		require.ErrorIs(t, err, peopleValues.ErrStudentRowMissing,
			"another tenant's child must not exist for this writer")
	})

	t.Run("student UpdateStatus blocked", func(t *testing.T) {
		// The lifecycle status is the owner's write (#3349).
		repo := api.NewUnboundStudentRepository(testpkg.NewDatabase(t, db))
		err := repo.UpdateStatus(ctxB, studentA.ID, peopleValues.StudentStatusInactive)
		require.Error(t, err, "cross-tenant UpdateStatus must fail")
		require.ErrorIs(t, err, peopleValues.ErrStudentRowMissing,
			"another tenant's child must not exist for this writer")
	})
}
