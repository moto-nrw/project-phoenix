package contracttest_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Restoring a planned care end puts the removed sourced rows back verbatim,
// with the planning period recorded when they were removed (#5235). When the
// template's schedules moved to another period meanwhile, the child must be
// planned on the template's new occurrences right after the restore: the
// care-exit workflow re-plans the restored children against the current
// sources in the same unit of work. Unchanged rows keep their id.

// The exit is planned on the decision suites' fixed day, before the phase
// starts on 2026-09-01. The materializer and the occurrence reconciler only
// act from the real today on, so occurrences are materialized on future
// Mondays, numbered in weeks after the first one (plannedMonday); the
// stretched phase and periods contain all of them until 2099.
const restoreLifecycleToday = decisionTestToday

// careExitRestoration is the care-exit workflow's entry the routes bind for
// confirming and cancelling a care end.
type careExitRestoration interface {
	Confirm(ctx context.Context, token string, input careplan.CareExitInput, actorAccountID int64) (*careplan.CareExitResult, error)
	Cancel(ctx context.Context, studentIDs []int64, actorAccountID int64) (int, error)
}

// restoreFixture is one approved child on a sourced Monday template whose
// schedules all start in period Q.
type restoreFixture struct {
	divergingPeriodFixture
	lifecycle careplan.CareLifecycle
	workflow  careExitRestoration
	actorID   int64
}

// lateSourceFrom is when a late-booked child holds the template's source
// offering: after the exits these tests plan, so the exit deletes the row.
var lateSourceFrom = calendar.NewDate(2026, 10, 1)

// setupRestoreFixture approves the child with the template's source offering
// from the phase start, or with lateSource only from lateSourceFrom on.
func setupRestoreFixture(t *testing.T, email string, lateSource bool) restoreFixture {
	t.Helper()
	env := setupLongPhaseDecisionTest(t)
	source := createAdjustmentCareOfferingWith(t, env, "Mittagessen", nil)
	other := createAdjustmentCareOfferingWith(t, env, "Spaetbetreuung", nil)
	q := createLongPhasePeriod(t, env, "zeitraum-Q")
	group := createCareOfferingTemplateGroup(t, env.db, "MittagessenStorno")
	group.TargetGroupType = timetable.TargetGroupTypeOffering
	group.SourceCareOfferingIDs = []int64{source.ID}
	require.NoError(t, timetableCompose.NativeGroupRecords(testTimetableRepositories(env.db).Timetable).Update(testpkg.Ctx(t), group))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayMonday, &q)
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayWednesday, &q)
	module, _, _, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, env.db), testpkg.TenantRuntime(t, env.db))
	require.NoError(t, err)

	booked := []*offeringContract.CareOfferingRecord{source, other}
	if lateSource {
		booked = []*offeringContract.CareOfferingRecord{other}
	}
	requestID, childID, studentID := submitApprovedAdjustmentChild(t, env, email, "Storno", booked)
	if lateSource {
		from := lateSourceFrom
		_, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db, env.decision.UpdateChildOfferings, enrollmentTest.UpdateChildOfferingsInput{
			RequestID: requestID, ChildID: childID, ActorAccountID: env.creatorID, ActorRole: "admin",
			Reason: "Mittagessen ab Oktober", EffectiveFrom: &from,
			Offerings: []enrollmentTest.OfferingAdjustmentSelection{
				{OfferingID: source.ID, SelectedDays: []string{"mon"}},
				{OfferingID: other.ID, SelectedDays: []string{"mon"}},
			},
		})
		require.NoError(t, err)
	}
	f := divergingPeriodFixture{
		env: env, template: group, source: source,
		requestID: requestID, childID: childID, studentID: studentID,
		wantPeriodID: &q, timetable: module,
	}
	lifecycle := newCareLifecycleServiceAt(t, env.db, restoreLifecycleToday)
	workflow := api.NewCarePlanRestoration(lifecycle, env.bookings, func() calendar.Date { return restoreLifecycleToday })
	return restoreFixture{divergingPeriodFixture: f, lifecycle: lifecycle, workflow: workflow, actorID: careActor(t, env.db)}
}

// plannedMonday is the future Monday weeks after the first one on or after
// lateSourceFrom.
func plannedMonday(weeks int) calendar.Date {
	return firstFutureMondayOnOrAfter(lateSourceFrom).AddDays(7 * weeks)
}

// requirePlannedInWeek materializes the Monday of the week and asserts the
// child is planned on the template's instance there.
func (f restoreFixture) requirePlannedInWeek(t *testing.T, step string, weeks int) {
	t.Helper()
	f.requirePlannedOn(t, step, plannedMonday(weeks))
}

// moveSchedule re-pins the template's schedule on the weekday to a period.
func (f restoreFixture) moveSchedule(t *testing.T, weekday int, periodID *int64) {
	t.Helper()
	res, err := testpkg.SetupTestDB(t).NewUpdate().TableExpr("activities.schedules").
		Set("calendar_period_id = ?", periodID).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", f.template.ID).
		Where("weekday = ?", weekday).
		Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
}

// endCare plans the child's care end through the lifecycle's preview and
// confirmation.
func (f restoreFixture) endCare(t *testing.T, lastCareDay calendar.Date) *careplan.CareExitResult {
	t.Helper()
	return endCare(t, testpkg.Ctx(t), f.lifecycle, f.actorID, f.exitInput(lastCareDay))
}

// changeEnd previews on the lifecycle and confirms through the workflow, as
// the routes do.
func (f restoreFixture) changeEnd(t *testing.T, lastCareDay calendar.Date) *careplan.CareExitResult {
	t.Helper()
	ctx := testpkg.Ctx(t)
	preview, err := testpkg.TenantQuery1(t, ctx, f.env.db, f.lifecycle.Preview, f.exitInput(lastCareDay))
	require.NoError(t, err)
	require.False(t, preview.Blocked, "preview must not be blocked: %+v", preview.Students)
	result, err := testpkg.TenantQuery3(t, ctx, f.env.db, f.workflow.Confirm, preview.Token, f.exitInput(lastCareDay), f.actorID)
	require.NoError(t, err)
	return result
}

func (f restoreFixture) exitInput(lastCareDay calendar.Date) careplan.CareExitInput {
	return careplan.CareExitInput{StudentIDs: []int64{f.studentID}, LastCareDay: lastCareDay, Reason: careplan.CareExitReasonMovedAway}
}

func (f restoreFixture) cancel(t *testing.T) {
	t.Helper()
	cancelled, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), f.env.db, f.workflow.Cancel, []int64{f.studentID}, f.actorID)
	require.NoError(t, err)
	require.Equal(t, 1, cancelled)
}

// requireStable runs the student-scoped and the tenant-wide resync once more
// and asserts that neither touches the re-planned row.
func (f restoreFixture) requireStable(t *testing.T, before timetable.StudentEnrollment) {
	t.Helper()
	require.NoError(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), f.env.db, f.env.bookings.ResyncStudentSourcedRosters, []int64{f.studentID}, restoreLifecycleToday))
	requireUnchanged(t, "repeated student resync", before, f.requireSingleRow(t, "repeated student resync"))
	f.resyncTenant(t)
	requireUnchanged(t, "tenant-wide resync after restore", before, f.requireSingleRow(t, "tenant-wide resync after restore"))
}

// materializedInstance materializes the Monday of the week and returns the
// template's instance there.
func (f restoreFixture) materializedInstance(t *testing.T, weeks int) int64 {
	t.Helper()
	date := plannedMonday(weeks)
	_, err := f.timetable.Materialization.MaterializeForTenant(testpkg.Ctx(t), date, date, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	var ids []int64
	require.NoError(t, f.env.db.NewSelect().Column("id").TableExpr("schedule.activity_instances").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", f.template.ID).
		Where("date = ?", date).
		Scan(testpkg.Ctx(t), &ids))
	require.Len(t, ids, 1)
	return ids[0]
}

// requireLedgeredPeriod asserts the exit recorded the child's sourced row
// with its original id and period.
func (f restoreFixture) requireLedgeredPeriod(t *testing.T, row timetable.StudentEnrollment) {
	t.Helper()
	removals, err := carePlanRecords(f.env.db).ListCareExitRemovals(testpkg.Ctx(t), []int64{f.studentID})
	require.NoError(t, err)
	for _, removal := range removals {
		if removal.EnrollmentID != nil && *removal.EnrollmentID == row.ID {
			assert.Equal(t, row.CalendarPeriodID, removal.CalendarPeriodID, "the ledger keeps the removed row's period")
			return
		}
	}
	t.Fatalf("the ledger holds no entry for enrollment %d: %+v", row.ID, removals)
}

// TestCareExitCancel_ReplansDeletedRowOnMovedSchedulePeriod is the issue's
// reproduction: the exit deletes the row (it starts after the last care
// day), the schedules move from Q to R, and the cancellation restores the
// row with Q. Without the re-plan the child is missing on every new R
// occurrence. A hand-set status and a manual booking come back unchanged.
func TestCareExitCancel_ReplansDeletedRowOnMovedSchedulePeriod(t *testing.T) {
	t.Parallel()
	f := setupRestoreFixture(t, "storno-zeitraum-geloescht@example.com", true)
	ctx := testpkg.Ctx(t)
	approved := f.requireSingleRow(t, "approval")

	// A planned occurrence the office marked by hand before the exit.
	markedInstance := f.materializedInstance(t, 1)
	participants := loadSourcedInstanceStudents(t, f.env, markedInstance)
	require.Len(t, participants, 1)
	manualAt := time.Now()
	testpkg.UpdateSessionAttendance(t, ctx, f.env.db, participants[0].ID, map[string]any{
		"status": studentpresence.SessionAttendanceAbsent, "manual_status_at": manualAt,
	})
	// A booking by hand on another block, starting after the exit.
	plain := testpkg.CreateTestActivityGroup(t, f.env.db, "Theater")
	manual := &timetable.StudentEnrollment{StudentID: f.studentID, ActivityGroupID: plain.ID, ValidFrom: "2026-10-05"}
	manual.TenantID = testpkg.Tenant(t)
	require.NoError(t, timetableCompose.NativeEnrollmentRecords(testTimetableRepositories(f.env.db).StudentEnrollment).Create(ctx, manual))

	f.endCare(t, calendar.NewDate(2026, 9, 14))
	require.Empty(t, loadTemplateEnrollments(t, f.env, f.template.ID), "the exit deleted the sourced row")
	f.requireLedgeredPeriod(t, approved)
	require.Empty(t, loadSourcedInstanceStudents(t, f.env, markedInstance), "the exit removed the planned participant")

	r := createLongPhasePeriod(t, f.env, "zeitraum-R")
	f.moveSchedule(t, timetable.WeekdayMonday, &r)
	f.moveSchedule(t, timetable.WeekdayWednesday, &r)
	f.resyncTemplateSave(t, restoreLifecycleToday)
	require.Empty(t, loadTemplateEnrollments(t, f.env, f.template.ID), "the template-save resync does not plan the leaving child")

	f.cancel(t)
	f.wantPeriodID = &r
	replanned := f.requireSingleRow(t, "cancellation")
	f.requirePlannedInWeek(t, "cancellation", 4)
	f.requireStable(t, replanned)
	f.requirePlannedInWeek(t, "repeated resync", 5)

	restored := loadSourcedInstanceStudents(t, f.env, markedInstance)
	require.Len(t, restored, 1, "the hand-marked occurrence comes back")
	assert.Equal(t, studentpresence.SessionAttendanceAbsent, restored[0].Status, "the hand-set status survives the re-plan")
	assert.NotNil(t, restored[0].ManualStatusAt, "the manual marker survives the re-plan")

	manualRows := loadTemplateEnrollments(t, f.env, plain.ID)
	require.Len(t, manualRows, 1, "the manual booking is back")
	assert.Equal(t, manual.ID, manualRows[0].ID, "the manual booking keeps its id")
	assert.Nil(t, manualRows[0].EnrollmentRequestChildID)
}

// TestCareExitCancel_ReplansCappedRowOnMovedSchedulePeriod covers the row the
// exit only capped: the care ends inside the row's validity.
func TestCareExitCancel_ReplansCappedRowOnMovedSchedulePeriod(t *testing.T) {
	t.Parallel()
	f := setupRestoreFixture(t, "storno-zeitraum-gekappt@example.com", false)
	approved := f.requireSingleRow(t, "approval")

	f.endCare(t, calendar.NewDate(2026, 9, 14))
	capped := f.requireSingleRow(t, "exit")
	require.Equal(t, approved.ID, capped.ID)
	require.NotNil(t, capped.ValidUntil)
	assert.Equal(t, "2026-09-15", validDay(capped.ValidUntil), "the exit caps the row after the last care day")

	r := createLongPhasePeriod(t, f.env, "zeitraum-R")
	f.moveSchedule(t, timetable.WeekdayMonday, &r)
	f.moveSchedule(t, timetable.WeekdayWednesday, &r)

	f.cancel(t)
	f.wantPeriodID = &r
	replanned := f.requireSingleRow(t, "cancellation")
	assert.Equal(t, validDay(approved.ValidUntil), validDay(replanned.ValidUntil), "the cancelled exit no longer ends the row")
	f.requirePlannedInWeek(t, "cancellation", 4)
	f.requireStable(t, replanned)
}

// TestCareExitCancel_ReplansRestoredRowWithoutPeriodOnMixedSchedules: only
// Monday moves to R, Wednesday stays on Q. The re-planned row carries no
// period (#5176), so it counts for both schedules' occurrences.
func TestCareExitCancel_ReplansRestoredRowWithoutPeriodOnMixedSchedules(t *testing.T) {
	t.Parallel()
	f := setupRestoreFixture(t, "storno-zeitraum-gemischt@example.com", true)
	f.requireSingleRow(t, "approval")

	f.endCare(t, calendar.NewDate(2026, 9, 14))
	r := createLongPhasePeriod(t, f.env, "zeitraum-R")
	f.moveSchedule(t, timetable.WeekdayMonday, &r)

	f.cancel(t)
	f.wantPeriodID = nil
	replanned := f.requireSingleRow(t, "cancellation")
	f.requirePlannedInWeek(t, "cancellation", 4)
	f.requireStable(t, replanned)
}

// TestCareExitCancel_KeepsUnchangedRestoredRow: without a schedule change the
// cancellation restores the original row and the re-plan leaves it as is.
func TestCareExitCancel_KeepsUnchangedRestoredRow(t *testing.T) {
	t.Parallel()
	f := setupRestoreFixture(t, "storno-zeitraum-unveraendert@example.com", true)
	approved := f.requireSingleRow(t, "approval")

	f.endCare(t, calendar.NewDate(2026, 9, 14))
	f.cancel(t)
	requireUnchanged(t, "cancellation", approved, f.requireSingleRow(t, "cancellation"))
	f.requirePlannedInWeek(t, "cancellation", 4)
	f.requireStable(t, approved)
}

// TestCareExitReconfirm_ReplansRestoredRowBeforeNewEnd changes a planned end
// after the schedules moved: the confirmation first restores the earlier
// exit's rows (care_lifecycle.go applyCareExitConfirmation), then applies
// the later end. The child is planned on R until the new last care day.
func TestCareExitReconfirm_ReplansRestoredRowBeforeNewEnd(t *testing.T) {
	t.Parallel()
	f := setupRestoreFixture(t, "neues-ende-zeitraum@example.com", true)
	approved := f.requireSingleRow(t, "approval")

	first := f.changeEnd(t, calendar.NewDate(2026, 9, 14))
	assert.Empty(t, first.ReappliedStudentIDs, "a first exit restores nothing")
	r := createLongPhasePeriod(t, f.env, "zeitraum-R")
	f.moveSchedule(t, timetable.WeekdayMonday, &r)
	f.moveSchedule(t, timetable.WeekdayWednesday, &r)

	lastCareDay := calendar.NewDate(2098, 7, 31)
	second := f.changeEnd(t, lastCareDay)
	assert.Equal(t, []int64{f.studentID}, second.ReappliedStudentIDs)
	f.wantPeriodID = &r
	replanned := f.requireSingleRow(t, "changed end")
	require.NotNil(t, replanned.ValidUntil)
	assert.Equal(t, lastCareDay.AddDays(1).String(), validDay(replanned.ValidUntil), "the row ends after the new last care day")
	f.requirePlannedInWeek(t, "changed end", 4)

	// The changed end stays cancellable and is re-planned again.
	f.cancel(t)
	cancelled := f.requireSingleRow(t, "cancellation after changed end")
	assert.Equal(t, validDay(approved.ValidUntil), validDay(cancelled.ValidUntil), "the cancelled exit no longer ends the row")
	f.requireStable(t, cancelled)
	f.requirePlannedInWeek(t, "cancellation after changed end", 7)
}

// TestCareExitCancel_ReplansOnlyTheCancelledChild: the re-plan is scoped to
// the restored child. Another child's stale row on the same template stays
// as it is until the template's own resync; no tenant-wide resync runs.
func TestCareExitCancel_ReplansOnlyTheCancelledChild(t *testing.T) {
	t.Parallel()
	f := setupRestoreFixture(t, "storno-zeitraum-scope@example.com", true)
	_, _, siblingID := submitApprovedAdjustmentChild(t, f.env, "storno-zeitraum-geschwister@example.com", "Geschwister",
		[]*offeringContract.CareOfferingRecord{f.source})
	before := templateRowsByStudent(t, f)
	require.Contains(t, before, siblingID)

	f.endCare(t, calendar.NewDate(2026, 9, 14))
	r := createLongPhasePeriod(t, f.env, "zeitraum-R")
	f.moveSchedule(t, timetable.WeekdayMonday, &r)
	f.moveSchedule(t, timetable.WeekdayWednesday, &r)
	f.cancel(t)

	after := templateRowsByStudent(t, f)
	require.NotNil(t, after[f.studentID].CalendarPeriodID)
	assert.Equal(t, r, *after[f.studentID].CalendarPeriodID, "the cancelled child is re-planned")
	requireUnchanged(t, "sibling outside the scope", before[siblingID], after[siblingID])
}

// TestCareExitCancel_ToleratesDriftedSource: a source that turned inactive
// while the exit was planned rejects the resync. The cancellation stays as
// forgiving as before: it succeeds and keeps the restored row.
func TestCareExitCancel_ToleratesDriftedSource(t *testing.T) {
	t.Parallel()
	f := setupRestoreFixture(t, "storno-zeitraum-drift@example.com", true)
	approved := f.requireSingleRow(t, "approval")

	f.endCare(t, calendar.NewDate(2026, 9, 14))
	_, err := f.env.db.NewRaw(`UPDATE enrollment.care_offerings SET is_active = false WHERE id = ?`, f.source.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	f.cancel(t)
	requireUnchanged(t, "cancellation with drifted source", approved, f.requireSingleRow(t, "cancellation with drifted source"))
}

// templateRowsByStudent indexes the template's rows by student; every
// student holds one row.
func templateRowsByStudent(t *testing.T, f restoreFixture) map[int64]timetable.StudentEnrollment {
	t.Helper()
	rows := make(map[int64]timetable.StudentEnrollment)
	for _, row := range loadTemplateEnrollments(t, f.env, f.template.ID) {
		require.NotContains(t, rows, row.StudentID, "one row per student")
		rows[row.StudentID] = row
	}
	return rows
}

// validDay is the calendar day of a scanned DATE column.
func validDay(value *string) string {
	if value == nil || len(*value) < 10 {
		return ""
	}
	return (*value)[:10]
}
