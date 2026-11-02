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
	ID            int64  `bun:"id"`
	SchoolClass   string `bun:"school_class"`
	MembershipID  int64  `bun:"membership_id"`
	HealthData    []byte `bun:"health_data"`
}

func TestDirectoryProjectionReturnsJoinedValuesOnlyForTheNamedTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := testpkg.CreateTestStudent(t, db, "Own", "Directory", "1A")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Foreign", "Directory", "2B")
	reader := compose.NewReader(testpkg.TestHealthDataCipher)
	var membershipID int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_school_memberships WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL`,
		testpkg.Tenant(t), own.ID).Scan(testpkg.Ctx(t), &membershipID))
	allergy := "Allergie"
	sealed, err := reader.SealStudentHealth(testpkg.Tenant(t), membershipID, &allergy)
	require.NoError(t, err)
	_, err = db.ExecContext(testpkg.Ctx(t), `UPDATE users.student_care_profiles SET health_data = ? WHERE tenant_id = ? AND membership_id = ?`, sealed, testpkg.Tenant(t), membershipID)
	require.NoError(t, err)
	for _, school := range []struct {
		tenantID, studentID int64
		class               string
	}{{testpkg.Tenant(t), own.ID, "1A"}, {otherTenant, foreign.ID, "2B"}} {
		check := func(ctx context.Context, db bun.IDB) {
			t.Helper()
			var rows []studentRow
			require.NoError(t, reader.StudentRows(db, school.tenantID).Column("id", "school_class", "membership_id", "health_data").Where("student.id IN (?, ?)", own.ID, foreign.ID).Scan(ctx, &rows))
			require.Len(t, rows, 1)
			require.Equal(t, school.studentID, rows[0].ID)
			require.Equal(t, school.class, rows[0].SchoolClass)
			if school.studentID == own.ID {
				require.NotContains(t, string(rows[0].HealthData), "Allergie", "the projection carries the health field sealed")
				opened, err := reader.OpenStudentHealth(school.tenantID, rows[0].MembershipID, rows[0].HealthData)
				require.NoError(t, err)
				require.NotNil(t, opened)
				require.Equal(t, "Allergie", *opened)
			}
			var modeled []studentRow
			require.NoError(t, reader.StudentModelRows(db, school.tenantID, &modeled).Where("student.id IN (?, ?)", own.ID, foreign.ID).Scan(ctx))
			require.Equal(t, rows, modeled)
		}
		check(testpkg.Ctx(t), db) // explicit predicate, even with elevated access
		require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, school.tenantID, func(ctx context.Context, tx bun.Tx) error { check(ctx, tx); return nil }))
	}
}
