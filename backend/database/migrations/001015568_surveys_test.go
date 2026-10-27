package migrations

import (
	"encoding/json"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSurveysKeepEarlierDeletionKinds pins the subject check of 1.15.568
// (#4781): it adds both survey kinds and keeps the homework kind of 1.15.510;
// the rollback restores the check of 1.15.510.
func TestSurveysKeepEarlierDeletionKinds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, retireDirectPermissionsDown(t.Context(), db))
	ctx := t.Context()
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	logRow := func(kind string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO audit.data_deletions (tenant_id, deletion_type, records_deleted, deleted_by)
			VALUES (?, ?, 1, 'system')`, school, kind)
		return err
	}

	for _, kind := range []string{homeworkRetentionKind, parentSurveyRetentionKind, staffSurveyRetentionKind} {
		require.NoError(t, logRow(kind), "the subject check accepts %s", kind)
	}
	_, err := db.ExecContext(ctx, `DELETE FROM audit.data_deletions WHERE tenant_id = ? AND deletion_type IN (?, ?)`,
		school, parentSurveyRetentionKind, staffSurveyRetentionKind)
	require.NoError(t, err)

	require.NoError(t, surveysDown(ctx, db))
	require.NoError(t, logRow(homeworkRetentionKind), "the restored check keeps the homework kind")
	assert.ErrorContains(t, logRow(parentSurveyRetentionKind), "chk_data_deletions_subject", "before 1.15.568 the survey kind needs a subject")
}

// TestSurveysRollbackPreservesSchoolErasure proves that the rollback of
// 1.15.568 reinstalls the erasure functions without the survey tables: the
// plan, the preview and a whole erasure still work after the tables are gone.
func TestSurveysRollbackPreservesSchoolErasure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	require.NoError(t, retireDirectPermissionsDown(t.Context(), db))
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	student := testpkg.CreateTestStudentForTenant(t, db, school, "Um", "Frage", "3a")
	operator := testpkg.CreateTestOperator(t, db)

	// Roll back the later meal migrations first: their foreign keys extend the
	// erasure fingerprint restored by the survey rollback.
	require.NoError(t, mealOrderErasureDown(t.Context(), db))
	require.NoError(t, mealCaterersDown(t.Context(), db))
	require.NoError(t, mealRegistrationLogDown(t.Context(), db))
	require.NoError(t, mealOrdersDown(t.Context(), db))
	require.NoError(t, mealPermissionsDown(t.Context(), db))
	require.NoError(t, mealAbsenceHistoryDown(t.Context(), db))
	require.NoError(t, surveysDown(t.Context(), db))
	var data string
	require.NoError(t, db.NewRaw(`SELECT platform.school_erasure_preview(?, ?)::text`, school, operator.ID).Scan(t.Context(), &data))
	var counts map[string]int64
	require.NoError(t, json.Unmarshal([]byte(data), &counts))
	for _, table := range surveyTables {
		assert.NotContains(t, counts, table)
	}
	assert.Contains(t, counts, "education.class_homework", "the homework tables of 1.15.510 stay in the plan")
	assert.EqualValues(t, 1, counts["users.student_profiles"])
	for _, entry := range readErasurePlan(t, db) {
		assert.NotContains(t, surveyTables, entry.Name)
	}
	result := eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school))
	require.Equal(t, "db_done", result.Status, "preflight, deletion and residual checks must work after rollback: %v", result.ErrorCode)
	var left int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_profiles WHERE id = ?`, student.ID).Scan(t.Context(), &left))
	assert.Zero(t, left)
}
