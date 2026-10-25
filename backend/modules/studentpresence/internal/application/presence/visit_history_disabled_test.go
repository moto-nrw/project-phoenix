package presence_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisabledVisitHistoryDeletesOnlyCompletedPreviousDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	day := calendar.NewDate(2026, 10, 25)
	cleanup := setupCleanupService(t, db, func() time.Time { return day.BerlinMidnight().Add(time.Hour) })
	_, err := db.NewRaw(`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = 'gdpr.attendance_log_enabled'`, testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, db, "Synthetisch", "Raumhistorie", "3a")
	insertConsent(t, db, student.ID, true, 31)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	before := day.BerlinMidnight().Add(-time.Microsecond)
	at := day.BerlinMidnight()
	old := testpkg.CreateTestVisit(t, db, student.ID, group.ID, before.Add(-time.Hour), &before)
	today := testpkg.CreateTestVisit(t, db, student.ID, group.ID, before.Add(-time.Hour), &at)
	open := testpkg.CreateTestVisit(t, db, student.ID, group.ID, before.Add(-time.Hour), nil)

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	otherStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Synthetisch", "AndereSchule", "3a")
	otherGroup := testpkg.CreateTestActiveGroupForTenant(t, db, otherTenant)
	otherVisit := testpkg.CreateTestVisitForTenant(t, db, otherTenant, otherStudent.ID, otherGroup.ID, before.Add(-time.Hour), &before)

	rows, err := testSchoolPresence(t, db).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{ClosedSince: &at})
	require.NoError(t, err)
	require.Len(t, rows, 2, "today's completed and old open visits remain readable")
	preview, err := testpkg.TenantQuery0(t, testpkg.Ctx(t), db, cleanup.PreviewCleanup)
	require.NoError(t, err)
	assert.EqualValues(t, 1, preview.TotalVisits)
	result, err := cleanup.CleanupExpiredVisits(testpkg.Ctx(t))
	require.NoError(t, err)
	require.True(t, result.Success)
	assert.EqualValues(t, 1, result.RecordsDeleted)
	assert.False(t, visitExists(t, db, old.ID))
	assert.True(t, visitExists(t, db, today.ID))
	assert.True(t, visitExists(t, db, open.ID))
	require.Len(t, visitRetentionAudits(t, db, student.ID), 1)
	assert.True(t, visitExists(t, db, otherVisit.ID), "another school's completed visits must remain")
	assert.Empty(t, visitRetentionAudits(t, db, otherStudent.ID))
}

func TestDisabledVisitHistorySettingRollsBackOnAuditFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	day := calendar.NewDate(2026, 3, 29)
	command, err := api.NewCleanupAuditCommand(slog.Default())
	require.NoError(t, err)
	injected := errors.New("synthetic audit failure")
	fault := &retentionAuditFault{command: command, afterAppend: injected}
	repos := api.NewRetentionCleanupRepositories(testpkg.NewDatabase(t, db), fault)
	cleanup := activeService.NewCleanupService(testSchoolPresence(t, db), nil, owners.NewDeletionAudit(repos.Deletion), nil, func() calendar.Date { return day })
	student := testpkg.CreateTestStudent(t, db, "Synthetisch", "Rollback", "3a")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	exit := day.BerlinMidnight().Add(-time.Microsecond)
	visit := testpkg.CreateTestVisit(t, db, student.ID, group.ID, exit.Add(-time.Hour), &exit)
	require.NoError(t, cleanup.ApplyAttendanceLogSetting(testpkg.Ctx(t), true))
	assert.True(t, visitExists(t, db, visit.ID))
	require.ErrorIs(t, cleanup.ApplyAttendanceLogSetting(testpkg.Ctx(t), false), tenant.ErrTransactionRequired)
	require.ErrorIs(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, cleanup.ApplyAttendanceLogSetting, false), injected)
	assert.True(t, visitExists(t, db, visit.ID))
	assert.Empty(t, visitRetentionAudits(t, db, student.ID))
	fault.afterAppend = nil
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, cleanup.ApplyAttendanceLogSetting, false))
	assert.False(t, visitExists(t, db, visit.ID))
	require.Len(t, visitRetentionAudits(t, db, student.ID), 1)
}

type unavailableVisitHistorySettings struct{ err error }

func (s unavailableVisitHistorySettings) VisitHistoryRetention(context.Context) (bool, int, error) {
	return true, 30, s.err
}

func TestVisitHistoryCleanupRefusesUnreadableSetting(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	injected := errors.New("synthetic setting failure")
	cleanup := activeService.NewCleanupService(testSchoolPresence(t, db), nil, nil, unavailableVisitHistorySettings{injected})
	result, err := cleanup.CleanupExpiredVisits(testpkg.Ctx(t))
	require.ErrorIs(t, err, injected)
	assert.False(t, result.Success)
	assert.Zero(t, result.RecordsDeleted)
	_, err = cleanup.PreviewCleanup(testpkg.Ctx(t))
	require.ErrorIs(t, err, injected)
	_, err = cleanup.GetRetentionStatistics(testpkg.Ctx(t))
	require.ErrorIs(t, err, injected)
}
