package parentportal_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestParentAbsenceRealCommitFailureRollsBackOwnersAndPublishesNothing(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	cfg, bc, db := writeServiceConfig(t, true, true)
	day := calendar.NewDate(2026, 8, 24)
	cfg.Now = func() time.Time { return day.BerlinMidnight().Add(12 * time.Hour) }
	svc := parentportalcompose.New(cfg)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.Ctx(t)
	reachedCommit := false
	rejected := tenant.WithCommitGuard(ctx, func(txCtx context.Context) error {
		transaction, ok := tenant.TransactionFromContext(txCtx)
		require.True(t, ok)
		tx := transaction.(bun.Tx)
		count, err := tx.NewSelect().Table("active.student_status_days").Where("tenant_id = ? AND student_id = ?", chain.TenantID, chain.StudentID).Count(txCtx)
		require.NoError(t, err)
		if count == 0 {
			return nil
		} // authorization reads may commit before the write UoW
		require.EqualValues(t, 1, count, "the owner write succeeded before commit")
		var sick bool
		require.NoError(t, tx.NewRaw(`SELECT c.sick FROM users.student_care_profiles c JOIN users.student_school_memberships m ON m.tenant_id=c.tenant_id AND m.id=c.membership_id WHERE m.tenant_id = ? AND m.student_profile_id = ? AND m.deleted_at IS NULL`, chain.TenantID, chain.StudentID).Scan(txCtx, &sick))
		require.True(t, sick, "both owners wrote inside the outer unit of work")
		require.Empty(t, bc.Calls(), "nothing publishes before the database commit")
		// Both statements succeed. Only COMMIT checks this deferred uniqueness
		// constraint, so the runtime must handle a real PostgreSQL commit error.
		_, err = tx.ExecContext(txCtx, `CREATE TEMP TABLE parent_commit_probe (id integer, CONSTRAINT parent_commit_probe_key UNIQUE (id) DEFERRABLE INITIALLY DEFERRED) ON COMMIT DROP`)
		require.NoError(t, err)
		_, err = tx.ExecContext(txCtx, `INSERT INTO parent_commit_probe VALUES (1), (1)`)
		require.NoError(t, err)
		reachedCommit = true
		return nil
	})
	result, err := svc.SubmitSickNote(rejected, chain.AccountID, chain.StudentID, []calendar.Date{day}, "Arzttermin", absencerecords.StudentStatusDaySick, nil)
	require.True(t, reachedCommit)
	require.ErrorContains(t, err, "parent_commit_probe_key")
	require.Nil(t, result)
	count, readErr := db.NewSelect().Table("active.student_status_days").Where("tenant_id = ? AND student_id = ?", chain.TenantID, chain.StudentID).Count(ctx)
	require.NoError(t, readErr)
	require.Zero(t, count)
	var sick bool
	require.NoError(t, db.NewRaw(`SELECT c.sick FROM users.student_care_profiles c JOIN users.student_school_memberships m ON m.tenant_id=c.tenant_id AND m.id=c.membership_id WHERE m.tenant_id = ? AND m.student_profile_id = ? AND m.deleted_at IS NULL`, chain.TenantID, chain.StudentID).Scan(ctx, &sick))
	require.False(t, sick, "the second owner's live flag rolled back too")
	require.Empty(t, bc.Calls(), "a commit failure discards queued broadcasts")
	result, err = svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID, []calendar.Date{day}, "Arzttermin", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)
	require.Len(t, result.StatusDays, 1)
	require.Equal(t, day, result.StatusDays[0].Date)
	require.Contains(t, tenantBroadcastIDs(bc), chain.TenantID, "retry publishes only after a successful real commit")
	count, readErr = db.NewSelect().Table("active.student_status_days").Where("tenant_id = ? AND student_id = ?", chain.TenantID, chain.StudentID).Count(ctx)
	require.NoError(t, readErr)
	require.EqualValues(t, 1, count, "the retry commits exactly one owner row")
}
