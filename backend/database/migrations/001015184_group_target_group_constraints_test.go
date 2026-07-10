package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// This test exercises the schema installed by the normal migration runner. It
// deliberately does not invoke the migration's Up function, so an already
// recorded 1.15.182 cannot hide a missing upgrade migration.
func TestGroupTargetConstraintsRejectInvalidDirectWrites(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		testpkg.CleanupTenantTestData(t, db, tenantID)
		_, err := db.NewDelete().TableExpr("platform.schools").Where("id = ?", tenantID).Exec(cleanupCtx)
		require.NoError(t, err)
		_, err = db.NewDelete().TableExpr("platform.organizations").Where("id = ?", tenantID).Exec(cleanupCtx)
		require.NoError(t, err)
	})
	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "Target-Class-Whitespace")

	require.Error(t, setTargetSchoolClassForConstraintTest(db, tenantID, group.ID, " \t\n\r "))
	require.Error(t, setTargetSchoolClassForConstraintTest(db, tenantID, group.ID, "\u00a0"))
	require.NoError(t, setTargetSchoolClassForConstraintTest(db, tenantID, group.ID, " Klasse 3a "))
	require.Error(t, setIncompleteGradeTargetForConstraintTest(db, tenantID, group.ID))
}

func setIncompleteGradeTargetForConstraintTest(db *bun.DB, tenantID, groupID int64) error {
	_, err := db.NewRaw(`
		UPDATE activities.groups
		SET target_group_type = 'jahrgang',
			target_grade_level = NULL,
			target_school_class = NULL
		WHERE tenant_id = ? AND id = ?
	`, tenantID, groupID).Exec(context.Background())
	return err
}

func setTargetSchoolClassForConstraintTest(db *bun.DB, tenantID, groupID int64, schoolClass string) error {
	_, err := db.NewRaw(`
		UPDATE activities.groups
		SET target_group_type = 'klasse', target_grade_level = NULL, target_school_class = ?
		WHERE tenant_id = ? AND id = ?
	`, schoolClass, tenantID, groupID).Exec(context.Background())
	return err
}
