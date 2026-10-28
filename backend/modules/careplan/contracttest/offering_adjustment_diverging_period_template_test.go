package contracttest_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Sourced Regeltermine whose rows must carry the period their instances
// materialize under (#5176). Timetable gives each instance its schedule's
// period, else the template's, and counts a row only when the row's period
// is NULL or that period. So a sourced row carries the one period all the
// template's schedules resolve to, NULL when they resolve to different
// periods or none — in approval, both corrections and every resync alike.
// Otherwise the child drops off the instances, and every switch between a
// correction and a resync replaces an unchanged row.

// periodKey names one of the fixture's planning periods: "" is none, "P"
// the template's, "Q" and "Q2" two schedule periods. All contain the phase.
type periodKey string

type divergingPeriodCase struct {
	template  periodKey
	monday    periodKey
	wednesday periodKey
	want      periodKey
	// multiSource sources the template from both of the child's offerings,
	// so approval and corrections plan it through the resync fan-out.
	multiSource bool
}

// The materializer and the instance reconciler only reach occurrences from
// the real today on, so these tests stretch the phase from 2026-09-01 to
// 2099-07-31: every Monday they materialize lies inside the phase and its
// periods until 2099, while the fixed past dates stay inside it too.
var longPhaseEnd = calendar.NewDate(2099, 7, 31)

// setupLongPhaseDecisionTest is setupDecisionTest with the phase stretched
// to longPhaseEnd.
func setupLongPhaseDecisionTest(t *testing.T) *decisionTestEnv {
	t.Helper()
	env := setupDecisionTest(t)
	env.sourcePhase.ServiceEndDate = enrollmentTest.Date(longPhaseEnd)
	updateSourcePhase(t, env)
	return env
}

// createLongPhasePeriod creates a period that contains the stretched phase.
func createLongPhasePeriod(t *testing.T, env *decisionTestEnv, name string) int64 {
	t.Helper()
	return createDivergingCalendarPeriod(t, env, name, calendar.NewDate(2026, 8, 1), longPhaseEnd.AddDays(31))
}

func createDivergingCalendarPeriod(t *testing.T, env *decisionTestEnv, name string, start, end calendar.Date) int64 {
	t.Helper()
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) },
		testpkg.NewDatabase(t, env.db), testpkg.Ctx(t), testpkg.Tenant(t),
		uniqueSchemaName(name+"-"+t.Name()), start, end)
	return period.ID
}

// divergingPeriodFixture is one approved child holding the template's source
// offering (Monday) plus another offering.
type divergingPeriodFixture struct {
	env          *decisionTestEnv
	template     *timetable.Group
	source       *offeringContract.CareOfferingRecord
	requestID    int64
	childID      int64
	studentID    int64
	wantPeriodID *int64
	timetable    api.TimetableTestModule
}

func setupDivergingPeriodFixture(t *testing.T, email string, c divergingPeriodCase) divergingPeriodFixture {
	t.Helper()
	env := setupLongPhaseDecisionTest(t)
	source := createAdjustmentCareOfferingWith(t, env, "Mittagessen", nil)
	other := createAdjustmentCareOfferingWith(t, env, "Spaetbetreuung", nil)
	periods := map[periodKey]*int64{"": nil}
	for _, key := range []periodKey{"P", "Q", "Q2"} {
		id := createLongPhasePeriod(t, env, "zeitraum-"+string(key))
		periods[key] = &id
	}
	group := createCareOfferingTemplateGroup(t, env.db, "MittagessenZeitraum")
	group.TargetGroupType = timetable.TargetGroupTypeOffering
	group.SourceCareOfferingIDs = []int64{source.ID}
	if c.multiSource {
		group.SourceCareOfferingIDs = append(group.SourceCareOfferingIDs, other.ID)
	}
	group.CalendarPeriodID = periods[c.template]
	require.NoError(t, timetableCompose.NativeGroupRecords(testTimetableRepositories(env.db).Timetable).Update(testpkg.Ctx(t), group))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayMonday, periods[c.monday])
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayWednesday, periods[c.wednesday])

	module, _, _, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, env.db), testpkg.TenantRuntime(t, env.db))
	require.NoError(t, err)

	requestID, childID, studentID := submitApprovedAdjustmentChild(t, env, email, "Zeitraum",
		[]*offeringContract.CareOfferingRecord{source, other})
	return divergingPeriodFixture{
		env: env, template: group, source: source,
		requestID: requestID, childID: childID, studentID: studentID,
		wantPeriodID: periods[c.want], timetable: module,
	}
}

// requireSingleRow loads the template's only row and checks it carries the
// period the template's instances materialize under.
func (f divergingPeriodFixture) requireSingleRow(t *testing.T, step string) timetable.StudentEnrollment {
	t.Helper()
	rows := loadTemplateEnrollments(t, f.env, f.template.ID)
	require.Len(t, rows, 1, "%s: the child keeps exactly one row on the template", step)
	assert.Equal(t, f.studentID, rows[0].StudentID, "%s", step)
	if f.wantPeriodID == nil {
		assert.Nil(t, rows[0].CalendarPeriodID, "%s: schedules without one shared period plan a row without one", step)
	} else if assert.NotNil(t, rows[0].CalendarPeriodID, "%s: the row carries the schedules' shared period", step) {
		assert.Equal(t, *f.wantPeriodID, *rows[0].CalendarPeriodID, "%s: the row carries the schedules' shared period", step)
	}
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays, "%s", step)
	return rows[0]
}

// requireUnchanged asserts that a step neither replaced, re-dated nor
// re-planned the row.
func requireUnchanged(t *testing.T, step string, before, after timetable.StudentEnrollment) {
	t.Helper()
	assert.Equal(t, before.ID, after.ID, "%s: the unchanged row must not be replaced", step)
	assert.Equal(t, before.CalendarPeriodID, after.CalendarPeriodID, "%s", step)
	assert.Equal(t, before.SelectedWeekdays, after.SelectedWeekdays, "%s", step)
	assert.Equal(t, before.ValidFrom, after.ValidFrom, "%s", step)
	assert.Equal(t, before.ValidUntil, after.ValidUntil, "%s: the row must not be capped", step)
}

// requirePlannedOn materializes date with the Timetable owner's materializer
// and asserts the child is planned on the template's instance there.
func (f divergingPeriodFixture) requirePlannedOn(t *testing.T, step string, date calendar.Date) {
	t.Helper()
	_, err := f.timetable.Materialization.MaterializeForTenant(testpkg.Ctx(t), date, date, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	var instanceIDs []int64
	require.NoError(t, f.env.db.NewSelect().
		Column("id").
		TableExpr("schedule.activity_instances").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", f.template.ID).
		Where("date = ?", date).
		Scan(testpkg.Ctx(t), &instanceIDs))
	require.Len(t, instanceIDs, 1, "%s: the template has one instance on %s", step, date)
	var students []int64
	for _, row := range loadSourcedInstanceStudents(t, f.env, instanceIDs[0]) {
		students = append(students, row.StudentID)
	}
	assert.Equal(t, []int64{f.studentID}, students, "%s: the child is planned on the instance of %s", step, date)
}

func (f divergingPeriodFixture) resyncTenant(t *testing.T) {
	t.Helper()
	resyncAll, ok := f.env.bookings.(interface {
		ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom calendar.Date) error
	})
	require.True(t, ok, "decision service must implement the tenant-wide resync")
	require.NoError(t, resyncAll.ResyncOfferingSourcedTemplates(testpkg.Ctx(t), offeringResyncToday))
}

// resyncOffering runs the offering-scoped resync behind phase and offering
// edits.
func (f divergingPeriodFixture) resyncOffering(t *testing.T) {
	t.Helper()
	scoped, ok := f.env.bookings.(enrollmentTest.SourcedTemplateResyncer)
	require.True(t, ok, "decision service must implement the offering-scoped resync")
	require.NoError(t, scoped.ResyncTemplatesSourcedFromOffering(testpkg.Ctx(t), f.source.ID, offeringResyncToday))
}

// resyncTemplateSave mirrors the template save's resync, which hands over
// the template's stored period.
func (f divergingPeriodFixture) resyncTemplateSave(t *testing.T, effectiveFrom calendar.Date) {
	t.Helper()
	require.NoError(t, offeringResyncer(t, f.env).ResyncTemplateOfferingRoster(testpkg.Ctx(t), timetable.OfferingRosterResyncInput{
		TemplateID:       f.template.ID,
		OfferingIDs:      []int64{f.source.ID},
		CalendarPeriodID: f.template.CalendarPeriodID,
		EffectiveFrom:    effectiveFrom,
	}))
}

// correct replaces the child's second offering; the template's source stays
// booked on Monday. A nil effectiveFrom is the undated correction.
func (f divergingPeriodFixture) correct(t *testing.T, name string, effectiveFrom *calendar.Date) {
	t.Helper()
	replacement := createAdjustmentCareOfferingWith(t, f.env, name, nil)
	_, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), f.env.db, f.env.decision.UpdateChildOfferings, enrollmentTest.UpdateChildOfferingsInput{
		RequestID:      f.requestID,
		ChildID:        f.childID,
		ActorAccountID: f.env.creatorID,
		ActorRole:      "admin",
		Reason:         "Anderes Angebot",
		EffectiveFrom:  effectiveFrom,
		Offerings: []enrollmentTest.OfferingAdjustmentSelection{
			{OfferingID: f.source.ID, SelectedDays: []string{"mon"}},
			{OfferingID: replacement.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)
}

// runDivergingPeriodCycle alternates approval, both corrections and every
// resync entry. The row keeps the materialization period throughout,
// resyncs leave it untouched, and the child stays planned on the instances.
func runDivergingPeriodCycle(t *testing.T, email string, c divergingPeriodCase) {
	t.Helper()
	f := setupDivergingPeriodFixture(t, email, c)
	phaseStart := calendar.Date(f.env.sourcePhase.ServiceStartDate)
	monday := firstFutureMondayOnOrAfter(phaseStart)

	approved := f.requireSingleRow(t, "approval")
	f.requirePlannedOn(t, "approval", monday)

	f.resyncTenant(t)
	requireUnchanged(t, "tenant-wide resync after approval", approved, f.requireSingleRow(t, "tenant-wide resync after approval"))
	f.resyncTenant(t)
	requireUnchanged(t, "repeated tenant-wide resync", approved, f.requireSingleRow(t, "repeated tenant-wide resync"))
	f.requirePlannedOn(t, "tenant-wide resync", monday)
	f.requirePlannedOn(t, "tenant-wide resync", monday.AddDays(7))

	// The undated correction rebuilds the child's rows; it must rebuild them
	// on the period the resync uses.
	f.correct(t, "Fruehbetreuung", nil)
	corrected := f.requireSingleRow(t, "undated correction")
	f.requirePlannedOn(t, "undated correction", monday.AddDays(7))
	f.requirePlannedOn(t, "undated correction", monday.AddDays(14))

	f.resyncTemplateSave(t, phaseStart)
	requireUnchanged(t, "template-save resync after undated correction", corrected, f.requireSingleRow(t, "template-save resync after undated correction"))
	// A resync taking effect mid-phase must not cap the running row and
	// re-seed it on another period.
	f.resyncTemplateSave(t, phaseStart.AddDays(30))
	requireUnchanged(t, "mid-phase template-save resync", corrected, f.requireSingleRow(t, "mid-phase template-save resync"))
	f.requirePlannedOn(t, "template-save resync", monday.AddDays(14))

	switchDate := phaseStart.AddDays(60)
	f.correct(t, "Ferienbetreuung", &switchDate)
	if c.multiSource {
		// A dated correction dropping one source of a multi-source template
		// caps the row at the switch date and re-seeds it, with or without
		// any period (also on development). Here only the period matters:
		// every segment carries the materialization period.
		f.requireRowsCarryPeriod(t, "dated correction")
		f.resyncTenant(t)
		f.requireRowsCarryPeriod(t, "tenant-wide resync after dated correction")
		f.requirePlannedOn(t, "tenant-wide resync after dated correction", monday.AddDays(21))
		return
	}
	requireUnchanged(t, "dated correction", corrected, f.requireSingleRow(t, "dated correction"))

	f.resyncTenant(t)
	requireUnchanged(t, "tenant-wide resync after dated correction", corrected, f.requireSingleRow(t, "tenant-wide resync after dated correction"))
	f.resyncOffering(t)
	requireUnchanged(t, "offering-scoped resync", corrected, f.requireSingleRow(t, "offering-scoped resync"))
	f.requirePlannedOn(t, "tenant-wide resync after dated correction", monday.AddDays(21))
}

// requireRowsCarryPeriod checks that every row of the template carries the
// period its instances materialize under.
func (f divergingPeriodFixture) requireRowsCarryPeriod(t *testing.T, step string) {
	t.Helper()
	rows := loadTemplateEnrollments(t, f.env, f.template.ID)
	require.NotEmpty(t, rows, "%s", step)
	for _, row := range rows {
		assert.Equal(t, f.studentID, row.StudentID, "%s", step)
		if f.wantPeriodID == nil {
			assert.Nil(t, row.CalendarPeriodID, "%s", step)
		} else if assert.NotNil(t, row.CalendarPeriodID, "%s", step) {
			assert.Equal(t, *f.wantPeriodID, *row.CalendarPeriodID, "%s", step)
		}
	}
}

func TestSourcedRowPeriod_SchedulePeriodWinsOverDivergingTemplatePeriod(t *testing.T) {
	t.Parallel()
	runDivergingPeriodCycle(t, "zeitraum-abweichend@example.com", divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q", want: "Q"})
}

func TestSourcedRowPeriod_MultiSourceTemplateUsesSchedulePeriod(t *testing.T) {
	t.Parallel()
	runDivergingPeriodCycle(t, "zeitraum-mehrquellen@example.com", divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q", want: "Q", multiSource: true})
}

func TestSourcedRowPeriod_SchedulePeriodAppliesToTemplateWithoutPeriod(t *testing.T) {
	t.Parallel()
	runDivergingPeriodCycle(t, "zeitraum-ohne-termin@example.com", divergingPeriodCase{template: "", monday: "Q", wednesday: "Q", want: "Q"})
}

func TestSourcedRowPeriod_MixedSchedulePeriodsPlanWithoutPeriod(t *testing.T) {
	t.Parallel()
	runDivergingPeriodCycle(t, "zeitraum-gemischt@example.com", divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q2", want: ""})
}

func TestSourcedRowPeriod_ScheduleAndTemplateFallbackDisagreePlanWithoutPeriod(t *testing.T) {
	t.Parallel()
	runDivergingPeriodCycle(t, "zeitraum-rueckfall@example.com", divergingPeriodCase{template: "P", monday: "Q", wednesday: "", want: ""})
}

func TestSourcedRowPeriod_MatchingTemplateAndSchedulePeriodStays(t *testing.T) {
	t.Parallel()
	runDivergingPeriodCycle(t, "zeitraum-gleich@example.com", divergingPeriodCase{template: "P", monday: "P", wednesday: "", want: "P"})
}

// Connecting a source to a template whose schedule period covers only part
// of the enrollment phase must not fail on the phase check (I4): the check
// runs against the template's own period, as on development, while the row
// carries the schedule period its instances materialize under.
func TestSourcedRowPeriod_ConnectingSourceWithNarrowerSchedulePeriodSucceeds(t *testing.T) {
	t.Parallel()
	env := setupLongPhaseDecisionTest(t)
	ctx := testpkg.Ctx(t)
	source := createSourceOffering(t, env, "MittagessenTeil", nil)
	studentID, childID := submitAndApproveOfferingChild(t, env, source.ID, "zeitraum-teil@example.com", "Teil", 2)
	templatePeriodID := createLongPhasePeriod(t, env, "termin")
	// The schedule period leaves out the phase's first month.
	narrowID := createDivergingCalendarPeriod(t, env, "zeitplan-teil", calendar.NewDate(2026, 10, 1), longPhaseEnd.AddDays(-30))
	group := createCareOfferingTemplateGroup(t, env.db, "MittagessenTeil")
	group.CalendarPeriodID = &templatePeriodID
	require.NoError(t, timetableCompose.NativeGroupRecords(testTimetableRepositories(env.db).Timetable).Update(ctx, group))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayMonday, &narrowID)

	resyncer := offeringResyncer(t, env)
	templates, err := api.NewTimetableHTTP(testpkg.NewDatabase(t, env.db),
		api.WithOfferingRosterResync(resyncer.ResyncTemplateOfferingRoster),
		api.WithOfferingSourceValidator(env.offeringCatalog.ValidateTemplateOfferingSource))
	require.NoError(t, err)
	module, _, _, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, env.db), testpkg.TenantRuntime(t, env.db))
	require.NoError(t, err)
	monday := firstFutureMondayOnOrAfter(calendar.Date(env.sourcePhase.ServiceStartDate))

	err = testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, connectErr := templates.RosterReview.ConnectOfferingSource(txCtx, timetable.ConnectOfferingSourceCommand{
			TemplateID: group.ID, OfferingIDs: []int64{source.ID}, EffectiveDate: monday,
		})
		return connectErr
	})
	require.NoError(t, err, "connecting the source must not fail on the phase check, as on development")

	f := divergingPeriodFixture{
		env: env, template: group, source: source, childID: childID, studentID: studentID,
		wantPeriodID: &narrowID, timetable: module,
	}
	f.requireSingleRow(t, "connect source")
	f.requirePlannedOn(t, "connect source", monday)
	f.resyncTenant(t)
	settled := f.requireSingleRow(t, "tenant-wide resync after connect")
	f.resyncTenant(t)
	requireUnchanged(t, "repeated tenant-wide resync", settled, f.requireSingleRow(t, "repeated tenant-wide resync"))
	f.requirePlannedOn(t, "tenant-wide resync", monday.AddDays(7))
}

// The one-time rewrite of a row a former resync left on the template period
// (#5176 deploy) reaches only future instances: past instances, their
// recorded attendance and their missing children stay as they are.
func TestSourcedRowPeriod_RewriteLeavesPastInstancesAndAttendance(t *testing.T) {
	t.Parallel()
	f := setupDivergingPeriodFixture(t, "zeitraum-vergangen@example.com", divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q", want: "Q"})
	ctx := testpkg.Ctx(t)
	// Reproduce the state development's resync wrote: the row on P.
	_, err := f.env.db.NewUpdate().
		TableExpr("activities.student_enrollments").
		Set("calendar_period_id = ?", *f.template.CalendarPeriodID).
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", f.template.ID).
		Exec(ctx)
	require.NoError(t, err)
	stale := loadTemplateEnrollments(t, f.env, f.template.ID)
	require.Len(t, stale, 1)

	monday := firstFutureMondayOnOrAfter(calendar.Date(f.env.sourcePhase.ServiceStartDate))
	// Two Mondays of the phase that have already passed.
	attended, missed := calendar.NewDate(2026, 9, 14), calendar.NewDate(2026, 9, 7)
	require.NotNil(t, f.template.PlannedRoomID)
	schedulePeriodID := f.wantPeriodID
	pastAttended := testpkg.CreateTestActivityInstance(t, f.env.db, attended, *f.template.PlannedRoomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &f.template.ID, CalendarPeriodID: schedulePeriodID,
	})
	pastMissed := testpkg.CreateTestActivityInstance(t, f.env.db, missed, *f.template.PlannedRoomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &f.template.ID, CalendarPeriodID: schedulePeriodID,
	})
	checkedIn := time.Date(2026, 9, 14, 12, 5, 0, 0, time.UTC)
	recorded := testpkg.CreateTestInstanceStudent(t, f.env.db, pastAttended.ID, f.studentID, testpkg.ScheduleAttendanceStatusPresent, testpkg.InstanceStudentOpts{
		CheckedInAt: &checkedIn,
	})
	// The stale P row leaves the child off the future instance.
	_, err = f.timetable.Materialization.MaterializeForTenant(ctx, monday, monday, timetable.MaterializationSourceManual)
	require.NoError(t, err)

	f.resyncTenant(t)

	rewritten := f.requireSingleRow(t, "one-time rewrite")
	assert.NotEqual(t, stale[0].ID, rewritten.ID, "sanity: the rewrite replaced the stale row")
	f.requirePlannedOn(t, "one-time rewrite", monday)
	rows := loadSourcedInstanceStudents(t, f.env, pastAttended.ID)
	require.Len(t, rows, 1, "the past instance keeps exactly its recorded attendance")
	assert.Equal(t, recorded.ID, rows[0].ID)
	assert.Equal(t, testpkg.ScheduleAttendanceStatusPresent, rows[0].Status)
	require.NotNil(t, rows[0].CheckedInAt)
	assert.True(t, checkedIn.Equal(*rows[0].CheckedInAt), "the recorded check-in stays untouched")
	assert.Empty(t, loadSourcedInstanceStudents(t, f.env, pastMissed.ID), "a past instance the child was missing from is not backfilled")
}

// A split ("Dieser und alle folgenden") that drops the period carries the
// child's sourced row onto a successor whose schedules pin none. The
// materializer then picks the period by date, so the successor's row must
// carry none either — not the period the predecessor's schedules shared — and
// the next resync must leave it alone.
func TestSourcedRowPeriod_SplitWithoutPeriodPlansSuccessorWithoutPeriod(t *testing.T) {
	t.Parallel()
	f := setupDivergingPeriodFixture(t, "zeitraum-teilung@example.com", divergingPeriodCase{template: "", monday: "Q", wednesday: "Q", want: "Q"})
	ctx := testpkg.Ctx(t)
	f.requireSingleRow(t, "approval")
	resyncer := offeringResyncer(t, f.env)
	templates, err := api.NewTimetableHTTP(testpkg.NewDatabase(t, f.env.db),
		api.WithOfferingRosterResync(resyncer.ResyncTemplateOfferingRoster),
		api.WithOfferingSourceValidator(f.env.offeringCatalog.ValidateTemplateOfferingSource))
	require.NoError(t, err)
	require.NotNil(t, f.template.PlannedRoomID)
	monday := firstFutureMondayOnOrAfter(calendar.Date(f.env.sourcePhase.ServiceStartDate))

	var split *timetable.SplitTemplateResult
	err = testpkg.WithTenantTx(t, ctx, f.env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var splitErr error
		split, splitErr = templates.Templates.SplitTemplate(txCtx, timetable.SplitTemplateCommand{
			TemplateID:      f.template.ID,
			EffectiveDate:   monday,
			Name:            f.template.Name,
			Type:            timetable.GroupTypeCare,
			Weekdays:        []int{timetable.WeekdayMonday, timetable.WeekdayWednesday},
			StartTime:       time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
			EndTime:         time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
			RoomID:          *f.template.PlannedRoomID,
			CategoryID:      f.template.CategoryID,
			TargetGroupType: timetable.TargetGroupTypeOffering,
			GradeLevelMax:   testpkg.MaxSchoolGradeLevel,
		})
		return splitErr
	})
	require.NoError(t, err)
	successor, err := timetableCompose.NativeGroupRecords(testTimetableRepositories(f.env.db).Timetable).FindByID(ctx, split.NewTemplateID)
	require.NoError(t, err)
	require.Nil(t, successor.CalendarPeriodID, "sanity: the successor pins no period")
	require.Equal(t, []int64{f.source.ID}, successor.SourceCareOfferingIDs, "sanity: the successor keeps the source")

	s := f
	s.template, s.wantPeriodID = successor, nil
	settled := s.requireSingleRow(t, "split")
	s.requirePlannedOn(t, "split", monday)
	s.requirePlannedOn(t, "split", monday.AddDays(7))
	s.resyncTenant(t)
	requireUnchanged(t, "tenant-wide resync after split", settled, s.requireSingleRow(t, "tenant-wide resync after split"))
	s.resyncTenant(t)
	requireUnchanged(t, "repeated tenant-wide resync after split", settled, s.requireSingleRow(t, "repeated tenant-wide resync after split"))
	s.requirePlannedOn(t, "tenant-wide resync after split", monday.AddDays(14))
}
