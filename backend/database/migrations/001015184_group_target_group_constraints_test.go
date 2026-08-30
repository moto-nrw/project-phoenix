package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// This test exercises the schema installed by the normal migration runner. It
// deliberately does not invoke the migration's Up function, so an already
// recorded 1.15.182 cannot hide a missing upgrade migration.
func TestGroupTargetConstraintsRejectInvalidDirectWrites(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.CleanupTenantTestData(t, db, tenantID)
	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "Target-Class-Whitespace")

	require.Error(t, setTargetSchoolClassForConstraintTest(db, tenantID, group.ID, " \t\n\r "))
	require.Error(t, setTargetSchoolClassForConstraintTest(db, tenantID, group.ID, "\u00a0"))
	require.NoError(t, setTargetSchoolClassForConstraintTest(db, tenantID, group.ID, " Klasse 3a "))
	require.Error(t, setIncompleteGradeTargetForConstraintTest(db, tenantID, group.ID))
}

func setIncompleteGradeTargetForConstraintTest(db *testpkg.DB, tenantID, groupID int64) error {
	_, err := db.NewRaw(`
		UPDATE activities.groups
		SET target_group_type = 'jahrgang',
			target_grade_level = NULL,
			target_school_class = NULL
		WHERE tenant_id = ? AND id = ?
	`, tenantID, groupID).Exec(context.Background())
	return err
}

func setTargetSchoolClassForConstraintTest(db *testpkg.DB, tenantID, groupID int64, schoolClass string) error {
	_, err := db.NewRaw(`
		UPDATE activities.groups
		SET target_group_type = 'klasse', target_grade_level = NULL, target_school_class = ?
		WHERE tenant_id = ? AND id = ?
	`, schoolClass, tenantID, groupID).Exec(context.Background())
	return err
}
