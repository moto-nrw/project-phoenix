package compose

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The retention runs against the real Timetable owner inside each test's own
// tenant. The Settings Platform, the Audit Platform and the
// Änderungsprotokoll are consumer ports the composition root binds; here
// they are recorders, so every run is checked for the audit records it
// hands over. A fixed clock keeps the retention window off the wall clock.

var cleanupToday = timezone.NewDate(2026, time.June, 15)

func cleanupClock() time.Time { return cleanupToday.BerlinMidnight().Add(12 * time.Hour) }

// recordingDeletionAudit keeps the audit records a run appends; write, when
// set, persists them too (the rollback test writes them in the transaction).
type recordingDeletionAudit struct {
	records []StudentDeletionRecord
	err     error
	write   func(context.Context, StudentDeletionRecord) error
}

func (a *recordingDeletionAudit) RecordTimetableRetention(ctx context.Context, record StudentDeletionRecord) error {
	if a.err != nil {
		return a.err
	}
	a.records = append(a.records, record)
	if a.write != nil {
		return a.write(ctx, record)
	}
	return nil
}

type fixedRetentionSettings struct {
	days int
	err  error
}

func (s fixedRetentionSettings) TimetableRetentionDays(context.Context) (int, error) {
	return s.days, s.err
}

type recordingDeviationRetention struct{ cutoffs []timezone.Date }

func (r *recordingDeviationRetention) DeleteDeviationEventsBefore(_ context.Context, cutoff timezone.Date) (int64, error) {
	r.cutoffs = append(r.cutoffs, cutoff)
	return 0, nil
}

// cleanupFixture holds the database, the owner and the room of one test.
type cleanupFixture struct {
	db     *bun.DB
	ctx    context.Context
	owner  *timetable.Module
	roomID int64
}

func newCleanupFixture(t *testing.T) cleanupFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Cleanup-Room-%d", time.Now().UnixNano()))
	return cleanupFixture{db: db, ctx: testpkg.Ctx(t), owner: buildModule(t, db), roomID: room.ID}
}

func (f cleanupFixture) cleanup(t *testing.T, deps TimetableCleanupDependencies) timetable.TimetableCleanup {
	t.Helper()
	deps.Timetable = f.owner
	if deps.Clock == nil {
		deps.Clock = cleanupClock
	}
	if deps.Audit == nil {
		deps.Audit = &recordingDeletionAudit{}
	}
	cleanup, err := NewTimetableCleanup(deps)
	require.NoError(t, err)
	return cleanup
}

func (f cleanupFixture) instance(t *testing.T, daysAgo int, status string, templateID *int64) int64 {
	t.Helper()
	return testpkg.CreateTestActivityInstance(t, f.db, cleanupToday.AddDays(-daysAgo), f.roomID,
		testpkg.ActivityInstanceOpts{Status: status, ActivityGroupID: templateID}).ID
}

func (f cleanupFixture) template(t *testing.T) int64 {
	t.Helper()
	return testpkg.CreateTestActivityGroup(t, f.db, fmt.Sprintf("Cleanup-Template-%d", time.Now().UnixNano())).ID
}

func (f cleanupFixture) exception(t *testing.T, templateID int64, daysAgo int) int64 {
	t.Helper()
	exception, err := f.owner.CreateActivityException(f.ctx, timetable.ActivityExceptionInput{
		ActivityGroupID: templateID,
		ExceptionDate:   cleanupToday.AddDays(-daysAgo).String(),
		ExceptionType:   scheduleModels.ActivityExceptionCancelled,
	})
	require.NoError(t, err)
	return exception.ID
}

func (f cleanupFixture) attachStudent(t *testing.T, instanceID, studentID int64, note *string) int64 {
	t.Helper()
	row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: studentID, Note: note}
	row.SetTenantID(testpkg.Tenant(t))
	_, err := f.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_students`).Exec(f.ctx)
	require.NoError(t, err, "insert instance_students")
	return row.ID
}

func (f cleanupFixture) attachStaff(t *testing.T, instanceID, staffID int64) int64 {
	t.Helper()
	row := &scheduleModels.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
	row.SetTenantID(testpkg.Tenant(t))
	_, err := f.db.NewInsert().Model(row).ModelTableExpr(`schedule.instance_staff`).Exec(f.ctx)
	require.NoError(t, err, "insert instance_staff")
	return row.ID
}

func (f cleanupFixture) student(t *testing.T, first string) int64 {
	t.Helper()
	return testpkg.CreateTestStudent(t, f.db, first, fmt.Sprintf("Cleanup-%d", time.Now().UnixNano()), "3a").ID
}

func (f cleanupFixture) assertRow(t *testing.T, table string, id int64, wantExists bool, msgAndArgs ...any) {
	t.Helper()
	got, err := f.db.NewSelect().Table(table).Where("id = ?", id).Exists(f.ctx)
	require.NoError(t, err)
	if wantExists {
		assert.True(t, got, append([]any{fmt.Sprintf("%s row %d should exist", table, id)}, msgAndArgs...)...)
	} else {
		assert.False(t, got, append([]any{fmt.Sprintf("%s row %d should be deleted", table, id)}, msgAndArgs...)...)
	}
}

func TestTimetableCleanupDeletesOldRowsKeepsFreshRows(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	deviations := &recordingDeviationRetention{}
	cleanup := f.cleanup(t, TimetableCleanupDependencies{DeviationEvents: deviations})

	oldID1 := f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)
	oldID2 := f.instance(t, 400, scheduleModels.InstanceStatusCancelled, nil)
	recentID := f.instance(t, 30, scheduleModels.InstanceStatusPlanned, nil)
	template := f.template(t)
	oldExceptionID := f.exception(t, template, 400)
	recentExceptionID := f.exception(t, template, 30)

	result, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 2, result.InstancesDeleted, "2 old instances deleted")
	assert.Equal(t, 1, result.ExceptionsDeleted, "1 old exception deleted")
	assert.Equal(t, 365, result.RetentionDays)
	assert.Equal(t, cleanupToday.AddDays(-365), result.CutoffDate)
	assert.Equal(t, []timezone.Date{cleanupToday.AddDays(-365)}, deviations.cutoffs,
		"the Änderungsprotokoll is cut at the same date as the instances")

	f.assertRow(t, "schedule.activity_instances", oldID1, false)
	f.assertRow(t, "schedule.activity_instances", oldID2, false)
	f.assertRow(t, "schedule.activity_instances", recentID, true)
	f.assertRow(t, "schedule.activity_exceptions", oldExceptionID, false)
	f.assertRow(t, "schedule.activity_exceptions", recentExceptionID, true)
}

func TestTimetableCleanupEmptyTenantRecordsNoAudit(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	audit := &recordingDeletionAudit{}
	cleanup := f.cleanup(t, TimetableCleanupDependencies{Audit: audit})

	for range 2 {
		result, err := cleanup.CleanupExpiredTimetableData(f.ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, result.InstancesDeleted)
		assert.Equal(t, 0, result.ExceptionsDeleted)
		assert.Equal(t, 0, result.StudentsAffected)
	}
	assert.Empty(t, audit.records, "nothing about a child was deleted, so nothing is audited")
}

func TestTimetableCleanupSecondRunDeletesNothing(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	cleanup := f.cleanup(t, TimetableCleanupDependencies{})
	f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)

	first, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, first.InstancesDeleted)
	second, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, second.InstancesDeleted, "second run is a no-op")
	assert.True(t, second.Success)
}

func TestTimetableCleanupCascadesToStaffAndParticipants(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	cleanup := f.cleanup(t, TimetableCleanupDependencies{})
	instanceID := f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)
	staff := testpkg.CreateTestStaff(t, f.db, "Cascade", fmt.Sprintf("Staff-%d", time.Now().UnixNano()))
	staffRowID := f.attachStaff(t, instanceID, staff.ID)
	sensitiveNote := "contains GDPR-sensitive free text about this student's behavior"
	studentRowID := f.attachStudent(t, instanceID, f.student(t, "Cascade"), &sensitiveNote)

	_, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)

	f.assertRow(t, "schedule.instance_staff", staffRowID, false)
	f.assertRow(t, "schedule.instance_students", studentRowID, false, "the participant row and its note go with the instance")
}

func TestTimetableCleanupDeletesEveryStatusPastRetention(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	cleanup := f.cleanup(t, TimetableCleanupDependencies{})
	for _, status := range []string{
		scheduleModels.InstanceStatusPlanned,
		scheduleModels.InstanceStatusActive,
		scheduleModels.InstanceStatusCompleted,
		scheduleModels.InstanceStatusCancelled,
	} {
		f.instance(t, 400, status, nil)
	}

	result, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, result.InstancesDeleted, "all four statuses deleted past retention")
}

func TestTimetableCleanupLeavesTemplatesUntouched(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	cleanup := f.cleanup(t, TimetableCleanupDependencies{})
	template := f.template(t)
	f.instance(t, 400, scheduleModels.InstanceStatusCompleted, &template)
	f.exception(t, template, 400)

	result, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.InstancesDeleted)
	assert.Equal(t, 1, result.ExceptionsDeleted)
	f.assertRow(t, "activities.groups", template, true,
		"only instances and exceptions are retention-scoped")
}

func TestTimetableCleanupAuditsOncePerAffectedStudent(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	audit := &recordingDeletionAudit{}
	cleanup := f.cleanup(t, TimetableCleanupDependencies{Audit: audit})
	first := f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)
	second := f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)
	fresh := f.instance(t, 30, scheduleModels.InstanceStatusPlanned, nil)
	both := f.student(t, "Both")
	one := f.student(t, "One")
	f.attachStudent(t, first, both, nil)
	f.attachStudent(t, second, both, nil)
	f.attachStudent(t, first, one, nil)
	f.attachStudent(t, fresh, one, nil)

	result, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, result.StudentsAffected)

	byStudent := map[int64]StudentDeletionRecord{}
	for _, record := range audit.records {
		byStudent[record.StudentID] = record
	}
	require.Len(t, byStudent, 2, "one audit record per affected student")
	assert.Equal(t, 2, byStudent[both].RecordsDeleted, "both expired slots of the child")
	assert.ElementsMatch(t, []int64{first, second}, byStudent[both].InstanceIDsSample)
	assert.Equal(t, 1, byStudent[one].RecordsDeleted, "the fresh slot survives and is not counted")
	for _, record := range audit.records {
		assert.Equal(t, testpkg.Tenant(t), record.TenantID)
		assert.Equal(t, 365, record.RetentionDays)
		assert.Equal(t, cleanupToday.AddDays(-365), record.CutoffDate)
	}
}

func TestTimetableCleanupBoundsTheAuditSample(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	audit := &recordingDeletionAudit{}
	cleanup := f.cleanup(t, TimetableCleanupDependencies{Audit: audit})
	student := f.student(t, "Sample")
	for range cleanupAuditSampleCap + 2 {
		f.attachStudent(t, f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil), student, nil)
	}

	_, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	require.Len(t, audit.records, 1)
	assert.Equal(t, cleanupAuditSampleCap+2, audit.records[0].RecordsDeleted)
	assert.Len(t, audit.records[0].InstanceIDsSample, cleanupAuditSampleCap)
}

func TestTimetableCleanupTenantIsolation(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	cleanup := f.cleanup(t, TimetableCleanupDependencies{})
	f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, f.db, otherTenantID)
	otherRoom := testpkg.CreateTestRoomForTenant(t, f.db, otherTenantID, fmt.Sprintf("Other-Room-%d", time.Now().UnixNano()))
	other := testpkg.CreateTestActivityInstanceForTenant(t, f.db, otherTenantID, cleanupToday.AddDays(-400), otherRoom.ID,
		testpkg.ActivityInstanceOpts{Status: scheduleModels.InstanceStatusCompleted})

	result, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.InstancesDeleted)
	f.assertRow(t, "schedule.activity_instances", other.ID, true, "another tenant's row survives")
}

func TestTimetableCleanupRetentionWindow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		settings RetentionSettings
		want     int
	}{
		{name: "tenant override narrows the window", settings: fixedRetentionSettings{days: 30}, want: 30},
		{name: "registry default", settings: fixedRetentionSettings{days: 180}, want: 180},
		{name: "non-positive value falls back", settings: fixedRetentionSettings{days: 0}, want: 365},
		{name: "resolution failure falls back", settings: fixedRetentionSettings{days: 42, err: errors.New("settings down")}, want: 365},
		{name: "no settings wired", settings: nil, want: 365},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newCleanupFixture(t)
			cleanup := f.cleanup(t, TimetableCleanupDependencies{Settings: tc.settings})
			preview, err := cleanup.PreviewExpiredTimetableData(f.ctx)
			require.NoError(t, err)
			assert.Equal(t, tc.want, preview.RetentionDays)
			assert.Equal(t, cleanupToday.AddDays(-tc.want), preview.CutoffDate)
		})
	}
}

func TestTimetableCleanupRetentionOverrideDeletesInsideTheNarrowWindow(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	cleanup := f.cleanup(t, TimetableCleanupDependencies{Settings: fixedRetentionSettings{days: 30}})
	oldID := f.instance(t, 60, scheduleModels.InstanceStatusCompleted, nil)
	freshID := f.instance(t, 10, scheduleModels.InstanceStatusCompleted, nil)

	result, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 30, result.RetentionDays, "tenant override must win over registry default")
	assert.Equal(t, 1, result.InstancesDeleted, "60-day-old row past 30-day window")
	f.assertRow(t, "schedule.activity_instances", oldID, false)
	f.assertRow(t, "schedule.activity_instances", freshID, true)
}

func TestTimetableCleanupRequiresTenant(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	cleanup := f.cleanup(t, TimetableCleanupDependencies{})

	_, err := cleanup.CleanupExpiredTimetableData(context.Background())
	require.ErrorContains(t, err, "no tenant in context")
	_, err = cleanup.PreviewExpiredTimetableData(context.Background())
	require.ErrorContains(t, err, "no tenant in context")
	_, err = cleanup.GetStats(context.Background())
	require.ErrorContains(t, err, "no tenant in context")
}

func TestTimetableCleanupPreviewCountsWithoutDeleting(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	cleanup := f.cleanup(t, TimetableCleanupDependencies{})
	oldID := f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)
	recentID := f.instance(t, 30, scheduleModels.InstanceStatusPlanned, nil)
	template := f.template(t)
	f.exception(t, template, 500)
	student := f.student(t, "Preview")
	f.attachStudent(t, oldID, student, nil)

	preview, err := cleanup.PreviewExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, preview.InstancesToDelete, "1 old instance counted")
	assert.Equal(t, 1, preview.ExceptionsToDelete)
	assert.Equal(t, 1, preview.StudentsAffected)
	assert.Equal(t, 365, preview.RetentionDays)
	require.NotNil(t, preview.OldestInstance)
	assert.Equal(t, cleanupToday.AddDays(-400), *preview.OldestInstance)
	require.NotNil(t, preview.OldestException)
	assert.Equal(t, cleanupToday.AddDays(-500), *preview.OldestException)

	f.assertRow(t, "schedule.activity_instances", oldID, true, "preview must not delete")
	f.assertRow(t, "schedule.activity_instances", recentID, true)
}

func TestTimetableCleanupPreviewOfEmptyTenant(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	cleanup := f.cleanup(t, TimetableCleanupDependencies{})
	f.instance(t, 30, scheduleModels.InstanceStatusPlanned, nil)

	preview, err := cleanup.PreviewExpiredTimetableData(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, preview.InstancesToDelete)
	assert.Equal(t, 0, preview.ExceptionsToDelete)
	assert.Nil(t, preview.OldestInstance, "no row before the cutoff → nil oldest")
	assert.Nil(t, preview.OldestException)
}

func TestTimetableCleanupStatsReportTotals(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	cleanup := f.cleanup(t, TimetableCleanupDependencies{})
	f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)
	f.instance(t, 30, scheduleModels.InstanceStatusPlanned, nil)

	stats, err := cleanup.GetStats(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.TotalInstances, "stats count every instance regardless of age")
	assert.Equal(t, 0, stats.TotalExceptions)
	assert.Equal(t, 365, stats.RetentionDays)
	assert.Equal(t, cleanupToday.AddDays(-365), stats.CutoffDate)
	require.NotNil(t, stats.OldestInstance)
	assert.Equal(t, cleanupToday.AddDays(-400), *stats.OldestInstance)
	assert.Nil(t, stats.OldestException, "no exceptions → nil oldest exception")
}

func TestTimetableCleanupWithoutAuditFailsBeforeDeleting(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	instanceID := f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)
	f.attachStudent(t, instanceID, f.student(t, "NoAudit"), nil)
	cleanup, err := NewTimetableCleanup(TimetableCleanupDependencies{Timetable: f.owner, Clock: cleanupClock})
	require.NoError(t, err)

	_, err = cleanup.CleanupExpiredTimetableData(f.ctx)
	require.ErrorContains(t, err, "audit repo not configured")
	f.assertRow(t, "schedule.activity_instances", instanceID, true, "the deletes never ran")
}

func TestTimetableCleanupAuditFailureBubblesBeforeDeleting(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	instanceID := f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)
	f.attachStudent(t, instanceID, f.student(t, "AuditFail"), nil)
	wantErr := errors.New("simulated audit failure")
	cleanup := f.cleanup(t, TimetableCleanupDependencies{Audit: &recordingDeletionAudit{err: wantErr}})

	_, err := cleanup.CleanupExpiredTimetableData(f.ctx)
	require.ErrorIs(t, err, wantErr, "audit failure must bubble up")
	f.assertRow(t, "schedule.activity_instances", instanceID, true,
		"audit-before-delete ordering means the DELETE did not fire")
}

// TestTimetableCleanupRollbackUndoesEverything proves the atomicity the
// scheduler and the CLI rely on: when the caller's tenant transaction rolls
// back after a successful run, the audit rows and the deletes are both
// undone.
func TestTimetableCleanupRollbackUndoesEverything(t *testing.T) {
	t.Parallel()
	f := newCleanupFixture(t)
	first := f.instance(t, 400, scheduleModels.InstanceStatusCompleted, nil)
	second := f.instance(t, 400, scheduleModels.InstanceStatusCancelled, nil)
	student := f.student(t, "Rollback")
	f.attachStudent(t, first, student, nil)
	f.attachStudent(t, second, student, nil)

	rollbackErr := errors.New("simulated caller-side failure after cleanup")
	err := testpkg.WithTenantTx(t, f.ctx, f.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		audit := &recordingDeletionAudit{write: func(ctx context.Context, record StudentDeletionRecord) error {
			_, err := tx.NewRaw(`INSERT INTO audit.data_deletions
				(tenant_id, student_id, deletion_type, records_deleted, deleted_by)
				VALUES (?, ?, 'timetable_retention', ?, 'system')`,
				record.TenantID, record.StudentID, record.RecordsDeleted).Exec(ctx)
			return err
		}}
		result, cleanupErr := f.cleanup(t, TimetableCleanupDependencies{Audit: audit}).CleanupExpiredTimetableData(txCtx)
		require.NoError(t, cleanupErr, "cleanup itself must succeed")
		require.Equal(t, 2, result.InstancesDeleted)
		require.Equal(t, 1, result.StudentsAffected)
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)

	f.assertRow(t, "schedule.activity_instances", first, true, "rollback must restore the first instance")
	f.assertRow(t, "schedule.activity_instances", second, true, "rollback must restore the second instance")
	audits, err := f.db.NewSelect().Table("audit.data_deletions").
		Where("tenant_id = ? AND student_id = ? AND deletion_type = 'timetable_retention'", testpkg.Tenant(t), student).
		Count(f.ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, audits, "rollback must undo audit rows — otherwise the compliance log lies about deleted data")
}

func TestNewTimetableCleanupRequiresTheOwner(t *testing.T) {
	t.Parallel()
	_, err := NewTimetableCleanup(TimetableCleanupDependencies{})
	require.EqualError(t, err, "timetable cleanup: the timetable owner is required")

	cleanup, err := NewTimetableCleanup(TimetableCleanupDependencies{Timetable: buildModule(t, testpkg.SetupTestDB(t)), Audit: &recordingDeletionAudit{}})
	require.NoError(t, err, "a nil logger falls back to the default logger")
	_, err = cleanup.CleanupExpiredTimetableData(testpkg.Ctx(t))
	require.NoError(t, err)
}
