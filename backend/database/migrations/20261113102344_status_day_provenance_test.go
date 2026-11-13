package migrations

import (
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStatusDayProvenanceRollbackAllowsLegacyReReport(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Rollback", "Status", "1a")
	staff := testpkg.CreateTestAccount(t, db, "status-rollback-staff")

	_, err := db.ExecContext(ctx, `
		INSERT INTO active.student_status_days
			(tenant_id, student_id, date, status, reported_at, source,
			 cleared_at, cleared_source, cleared_by_account_id)
		VALUES
			(?, ?, '2031-03-04', 'sick', NOW() - INTERVAL '2 hours', 'parent', NOW(), 'manual', ?),
			(?, ?, '2031-03-05', 'excused', NOW() - INTERVAL '2 hours', 'end_of_day', NOW(), 'manual', ?)
	`, testpkg.Tenant(t), student.ID, staff.ID, testpkg.Tenant(t), student.ID, staff.ID)
	require.NoError(t, err)
	var historyEndID int64
	require.NoError(t, db.NewRaw(`
		SELECT MAX(id) FROM schedule.meal_sickness_status_history WHERE student_id = ?
	`, student.ID).Scan(ctx, &historyEndID))
	require.NoError(t, statusDayProvenanceDown(ctx, db))

	// Match the previous writer's conflict update: it does not know the
	// retained provenance columns and only clears cleared_at.
	_, err = db.ExecContext(ctx, `
		INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
		SELECT tenant_id, student_id, date, status, NOW(), 'parent'
		FROM active.student_status_days WHERE student_id = ?
		ON CONFLICT (tenant_id, student_id, date, status) DO UPDATE
		SET reported_at = EXCLUDED.reported_at, cleared_at = NULL, source = EXCLUDED.source
	`, student.ID)
	require.NoError(t, err, "the previous application must be able to re-report cleared days after rollback")

	var standing int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM active.student_status_days
		WHERE student_id = ? AND cleared_at IS NULL
			AND cleared_source IS NULL AND cleared_by_account_id IS NULL
	`, student.ID).Scan(ctx, &standing))
	require.Equal(t, 2, standing)

	require.NoError(t, statusDayProvenanceUp(ctx, db), "reapplying the migration must accept legacy writes after rollback")
	var clearEvents int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM schedule.meal_sickness_status_history
		WHERE student_id = ? AND id <= ? AND source = 'manual' AND account_id = ? AND cleared_at IS NOT NULL
	`, student.ID, historyEndID, staff.ID).Scan(ctx, &clearEvents))
	require.Equal(t, 2, clearEvents, "rollback keeps the recorded clearing source and account in the history")
}
