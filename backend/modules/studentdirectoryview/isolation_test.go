package studentdirectoryview_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type studentRow struct {
	bun.BaseModel `bun:"alias:student"`
	ID            int64   `bun:"id"`
	SchoolClass   string  `bun:"school_class"`
	HealthInfo    *string `bun:"health_info"`
}

func TestDirectoryProjectionReturnsJoinedValuesOnlyForTheNamedTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := testpkg.CreateTestStudent(t, db, "Own", "Directory", "1A")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Foreign", "Directory", "2B")
	_, err := db.ExecContext(testpkg.Ctx(t), `UPDATE users.student_care_profiles c SET health_info = 'Allergie' FROM users.student_school_memberships m WHERE c.tenant_id=m.tenant_id AND c.membership_id=m.id AND m.tenant_id=? AND m.student_profile_id=?`, testpkg.Tenant(t), own.ID)
	require.NoError(t, err)
	for _, school := range []struct {
		tenantID, studentID int64
		class               string
	}{{testpkg.Tenant(t), own.ID, "1A"}, {otherTenant, foreign.ID, "2B"}} {
		check := func(ctx context.Context, db bun.IDB) {
			t.Helper()
			var rows []studentRow
			require.NoError(t, compose.NewReader().StudentRows(db, school.tenantID).Column("id", "school_class", "health_info").Where("student.id IN (?, ?)", own.ID, foreign.ID).Scan(ctx, &rows))
			require.Len(t, rows, 1)
			require.Equal(t, school.studentID, rows[0].ID)
			require.Equal(t, school.class, rows[0].SchoolClass)
			if school.studentID == own.ID {
				require.NotNil(t, rows[0].HealthInfo)
				require.Equal(t, "Allergie", *rows[0].HealthInfo)
			}
			var modeled []studentRow
			require.NoError(t, compose.NewReader().StudentModelRows(db, school.tenantID, &modeled).Where("student.id IN (?, ?)", own.ID, foreign.ID).Scan(ctx))
			require.Equal(t, rows, modeled)
		}
		check(testpkg.Ctx(t), db) // explicit predicate, even with elevated access
		require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, school.tenantID, func(ctx context.Context, tx bun.Tx) error { check(ctx, tx); return nil }))
	}
}
