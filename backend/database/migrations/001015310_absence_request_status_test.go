package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestAbsenceRequestStatusMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	require.NoError(t, absenceRequestStatusDown(ctx, db))
	t.Cleanup(func() { require.NoError(t, absenceRequestStatusUp(ctx, db)) })

	var legacyRequestID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO active.excused_absence_requests
			(tenant_id, student_id, submitted_by, dates, note)
		VALUES (?, ?, ?, '["2026-08-21"]'::jsonb, 'Altbestand')
		RETURNING id
	`, chain.TenantID, chain.StudentID, chain.AccountID).Scan(ctx, &legacyRequestID))

	require.NoError(t, absenceRequestStatusUp(ctx, db))

	var status string
	require.NoError(t, db.NewRaw(`
		SELECT absence_status
		FROM active.excused_absence_requests
		WHERE id = ?
	`, legacyRequestID).Scan(ctx, &status))
	assert.Equal(t, "excused", status, "legacy requests must keep their original meaning")

	_, err := db.ExecContext(ctx, `
		INSERT INTO active.excused_absence_requests
			(tenant_id, student_id, submitted_by, dates, note, absence_status)
		VALUES ($1, $2, $3, '["2026-08-22"]'::jsonb, 'Ausflug', 'class_trip')
	`, chain.TenantID, chain.StudentID, chain.AccountID)
	assert.Error(t, err, "the database must reject staff-only absence statuses")

	var sickRequestID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO active.excused_absence_requests
			(tenant_id, student_id, submitted_by, dates, note, absence_status)
		VALUES (?, ?, ?, '["2026-08-23"]'::jsonb, 'Fieber', 'sick')
		RETURNING id
	`, chain.TenantID, chain.StudentID, chain.AccountID).Scan(ctx, &sickRequestID))
	assert.Error(t, absenceRequestStatusDown(ctx, db), "downgrade must not erase sickness semantics")

	_, err = db.NewDelete().
		TableExpr("active.excused_absence_requests").
		Where("id = ?", sickRequestID).
		Exec(ctx)
	require.NoError(t, err)
}
