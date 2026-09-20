package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentCareMigrationPreservesFlagsWithoutCurrentStatusDays(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeCutover(t)
	ctx := t.Context()
	tenantID := testpkg.Tenant(t)
	ids := studentOwnerFixture(t, db, tenantID, 4)
	for i, id := range ids {
		_, err := db.NewRaw(`UPDATE users.students SET sick = ?, excused = ?,
			sick_since = timestamp '2000-01-07 08:15:00',
			excused_since = timestamp '2000-01-07 09:30:00'
			WHERE tenant_id = ? AND id = ?`, i == 0 || i == 2, i == 1 || i == 2, tenantID, id).Exec(ctx)
		require.NoError(t, err)
	}
	_, err := db.NewRaw(`INSERT INTO active.student_status_days
		(tenant_id, student_id, date, status, reported_at, source)
		VALUES (?, ?, date '2000-01-07', 'sick', timestamp '2000-01-07 08:15:00', 'manual')`, tenantID, ids[0]).Exec(ctx)
	require.NoError(t, err)
	before := studentRowsJSON(t, db, tenantID)
	var statusDaysBefore string
	require.NoError(t, db.NewRaw(`SELECT jsonb_agg(to_jsonb(d) ORDER BY id)::text
		FROM active.student_status_days d WHERE tenant_id = ?`, tenantID).Scan(ctx, &statusDaysBefore))

	require.NoError(t, studentOwnerCutoverPrecondition(ctx, db), "preflight must not require fabricated status days")
	require.NoError(t, studentOwnerBackfillUp(ctx, db))
	require.NoError(t, studentOwnerCutoverPrecondition(ctx, db), "completed backfill must also pass")
	require.NoError(t, studentOwnerCutoverUp(ctx, db))
	require.JSONEq(t, before, studentRowsJSON(t, db, tenantID), "the archive-backed rollback view preserves every source field")
	require.NoError(t, studentCareAbsenceCompatibilityUp(ctx, db))
	require.JSONEq(t, before, studentRowsJSON(t, db, tenantID), "the care-backed rollback view preserves flags, timestamps and all other fields")

	var mismatches int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.students_legacy a
		LEFT JOIN users.student_school_memberships m ON m.tenant_id = a.tenant_id AND m.student_profile_id = a.id
		LEFT JOIN users.student_care_profiles c ON c.tenant_id = m.tenant_id AND c.membership_id = m.id
		WHERE a.tenant_id = ? AND (c.membership_id IS NULL OR
		(c.sick, c.sick_since, c.excused, c.excused_since)
		IS DISTINCT FROM (a.sick, a.sick_since, a.excused, a.excused_since))`, tenantID).Scan(ctx, &mismatches))
	require.Zero(t, mismatches, "every source absence value must reach the care owner")
	var statusDaysAfter string
	require.NoError(t, db.NewRaw(`SELECT jsonb_agg(to_jsonb(d) ORDER BY id)::text
		FROM active.student_status_days d WHERE tenant_id = ?`, tenantID).Scan(ctx, &statusDaysAfter))
	require.JSONEq(t, statusDaysBefore, statusDaysAfter, "migration must neither invent nor modify status days")
}

func TestStudentCareAbsenceOwnerAndRollbackImageShareState(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Care", "Compatibility", "2a")
	_, err := db.NewRaw(`UPDATE users.student_care_profiles AS c SET sick = true
		FROM users.student_school_memberships AS m
		WHERE m.tenant_id = ? AND m.student_profile_id = ?
		AND c.tenant_id = m.tenant_id AND c.membership_id = m.id`, testpkg.Tenant(t), student.ID).Exec(ctx)
	require.NoError(t, err)
	var sick bool
	require.NoError(t, db.NewRaw(`SELECT sick FROM users.students WHERE tenant_id = ? AND id = ?`, testpkg.Tenant(t), student.ID).Scan(ctx, &sick))
	require.True(t, sick, "the previous image must see the owner command")
	_, err = db.NewRaw(`UPDATE users.students SET sick = false, excused = true WHERE tenant_id = ? AND id = ?`, testpkg.Tenant(t), student.ID).Exec(ctx)
	require.NoError(t, err)
	var excused bool
	require.NoError(t, db.NewRaw(`SELECT c.sick, c.excused FROM users.student_care_profiles AS c
		JOIN users.student_school_memberships AS m ON m.tenant_id = c.tenant_id AND m.id = c.membership_id
		WHERE m.tenant_id = ? AND m.student_profile_id = ?`, testpkg.Tenant(t), student.ID).Scan(ctx, &sick, &excused))
	require.False(t, sick)
	require.True(t, excused, "rollback writes must reach the same care owner")
}
