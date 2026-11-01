package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGroupWeekProtocolsMigrationRoundTrip pins that 1.15.719 runs twice,
// rolls back to the 1.15.718 payroll erasure without its tables, and applies again;
// the school erasure then removes a school's protocols with their days.
func TestGroupWeekProtocolsMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	exists := func() bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass('schedule.group_week_protocols') IS NOT NULL`).Scan(ctx, &found))
		return found
	}
	// The wage types (1.15.724, #4807), the school vacations (1.15.721,
	// #5345) and the Elternsprechtag (1.15.720, #4801) extend this erasure;
	// they go first.
	require.NoError(t, payrollWageTypesDown(ctx, db))
	require.NoError(t, schoolVacationsDown(ctx, db))
	require.NoError(t, parentConferencesDown(ctx, db))
	require.NoError(t, groupWeekProtocolsUp(ctx, db), "the migration is idempotent")
	require.NoError(t, groupWeekProtocolsDown(ctx, db))
	assert.False(t, exists())
	var payrollInPlan bool
	for _, entry := range readErasurePlan(t, db) {
		assert.NotContains(t, []string{"schedule.group_week_protocols", "schedule.group_week_protocol_days"}, entry.Name)
		payrollInPlan = payrollInPlan || entry.Name == "active.payroll_dispatches"
	}
	assert.True(t, payrollInPlan, "rolling back protocols preserves payroll erasure")
	require.NoError(t, groupWeekProtocolsUp(ctx, db))
	assert.True(t, exists())
	require.NoError(t, parentConferencesUp(ctx, db))
	require.NoError(t, schoolVacationsUp(ctx, db))
	require.NoError(t, payrollWageTypesUp(ctx, db))

	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	insertPayrollDispatch(t, db, school)
	var protocolID int64
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.group_week_protocols (tenant_id, scope, group_id, group_name, week_start)
		VALUES (?, 'group', 1, 'Bären', '2027-09-06') RETURNING id`, school).Scan(ctx, &protocolID))
	erasureExec(t, db, `INSERT INTO schedule.group_week_protocol_days (tenant_id, protocol_id, day, note) VALUES (?, ?, '2027-09-06', 'Notiz')`, school, protocolID)
	erasureExec(t, db, `INSERT INTO schedule.group_week_protocol_pdfs (protocol_id, tenant_id, filename, pdf) VALUES (?, ?, 'wochenprotokoll.pdf', '%PDF')`, protocolID, school)
	_, err := db.ExecContext(ctx, `INSERT INTO schedule.group_week_protocols (tenant_id, scope, group_id, group_name, week_start)
		VALUES (?, 'group', 1, 'Bären', '2027-09-07')`, school)
	require.Error(t, err, "a week starts on a Monday")

	result := eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school))
	require.Equal(t, "db_done", result.Status, "the erasure covers the weekly protocols")
	assert.EqualValues(t, 1, result.RowCounts["active.payroll_dispatches"], "the combined erasure retains payroll coverage")
	assert.Zero(t, countPayrollDispatches(t, db, school))
	assert.EqualValues(t, 1, result.RowCounts["schedule.group_week_protocols"])
	assert.EqualValues(t, 1, result.RowCounts["schedule.group_week_protocol_days"])
	assert.EqualValues(t, 1, result.RowCounts["schedule.group_week_protocol_pdfs"], "the stored PDF goes with the school")
}
