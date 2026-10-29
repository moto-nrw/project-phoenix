package contracttest_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A Regeltermin sourced from A (Monday and Wednesday) and B (Monday) whose
// source A turned inactive behind the catalog's guard (#5305). A contributes
// nothing from today on; B keeps planning the child's booked Monday. A dated
// correction onto B therefore reconciles the template for the child from
// today, not only from its switch: A's Wednesdays leave at once, the row
// is split at today, and the student and tenant-wide resyncs that apply the
// same rule afterwards change nothing, ids included. Recorded and hand-decided
// attendance, the other child and the period formula of #5176 stay.

// inactiveSourceToday is the booking today of the decision env; the phase
// starts before it so the row carries history the correction must keep.
const inactiveSourceToday = decisionTestToday

// inactiveSourceCorrection is the fixture with a switch two weeks after the
// first Monday that has not passed yet, so occurrences between today and
// the switch can be materialized.
type inactiveSourceCorrection struct {
	multiSourceCorrection
	// windowMonday is the Monday two weeks before the switch.
	windowMonday calendar.Date
}

func setupInactiveSourceCorrection(t *testing.T, email string, periods divergingPeriodCase) inactiveSourceCorrection {
	t.Helper()
	periods.phaseStart = inactiveSourceToday.AddDays(-10)
	f := setupMultiSourceCorrection(t, email, periods)
	windowMonday := firstFutureMondayOnOrAfter(f.switchDate)
	f.switchDate = windowMonday.AddDays(14)
	return inactiveSourceCorrection{multiSourceCorrection: f, windowMonday: windowMonday}
}

// materializeOn materializes the template on one date and returns its
// instance there.
func (f inactiveSourceCorrection) materializeOn(t *testing.T, date calendar.Date) int64 {
	t.Helper()
	return f.materialize(t, int(f.switchDate.DaysUntil(date)))
}

// inactiveSourcePlan is the child's planning before source A turns
// inactive: one row Monday+Wednesday that started before today, and
// occurrences before and after the switch, one with a hand decision and one
// with recorded attendance.
type inactiveSourcePlan struct {
	inactiveSourceCorrection
	running, neighbour timetable.StudentEnrollment
	// Instances of the template.
	windowMonday, windowWednesday, decidedWednesday, switchMonday, recordedWednesday int64
	// Participant rows on them.
	childMonday, childSwitchMonday, neighbourMonday, neighbourSwitchMonday, decided, recorded testpkg.InstanceStudent
}

func planInactiveSource(t *testing.T, f inactiveSourceCorrection) inactiveSourcePlan {
	t.Helper()
	f.correctFrom(t, nil,
		enrollmentTest.OfferingAdjustmentSelection{OfferingID: f.first.ID, SelectedDays: []string{"mon", "wed"}},
		monday(f.second))
	p := inactiveSourcePlan{inactiveSourceCorrection: f}
	p.running = f.singleRowOf(t, "undated setup", f.studentID)
	require.Equal(t, []int{1, 3}, p.running.SelectedWeekdays, "sanity: the first source plans Monday and Wednesday")
	require.True(t, p.running.ValidFrom < string(inactiveSourceToday), "sanity: the row started before today")
	p.neighbour = f.singleRowOf(t, "undated setup", f.neighbourID)

	p.windowMonday = f.materializeOn(t, f.windowMonday)
	p.childMonday = p.requirePlanned(t, p.windowMonday, f.studentID)
	p.neighbourMonday = p.requirePlanned(t, p.windowMonday, f.neighbourID)
	p.windowWednesday = f.materializeOn(t, f.windowMonday.AddDays(2))
	p.requirePlanned(t, p.windowWednesday, f.studentID)
	p.decidedWednesday = f.materializeOn(t, f.windowMonday.AddDays(9))
	p.decided = f.markAttendance(t, p.decidedWednesday, map[string]any{
		"status": studentpresence.SessionAttendanceAbsent, "manual_status_at": time.Now(),
	})
	p.switchMonday = f.materializeOn(t, f.switchDate)
	p.childSwitchMonday = p.requirePlanned(t, p.switchMonday, f.studentID)
	p.neighbourSwitchMonday = p.requirePlanned(t, p.switchMonday, f.neighbourID)
	p.recordedWednesday = f.materializeOn(t, f.switchDate.AddDays(2))
	p.recorded = f.markAttendance(t, p.recordedWednesday, map[string]any{
		"status": studentpresence.SessionAttendancePresent, "checked_in_at": time.Now(),
	})
	return p
}

func (p inactiveSourcePlan) requirePlanned(t *testing.T, instanceID, studentID int64) testpkg.InstanceStudent {
	t.Helper()
	participant, ok := p.participants(t, instanceID)[studentID]
	require.True(t, ok, "sanity: student %d is planned on instance %d", studentID, instanceID)
	return participant
}

// requireSplitAtToday checks the child's rows after the dated correction:
// history up to today keeps the row and both weekdays, and from today on
// only the valid source's Monday is planned, until successorUntil. It
// returns the successor.
func (p inactiveSourcePlan) requireSplitAtToday(t *testing.T, successorUntil calendar.Date) timetable.StudentEnrollment {
	t.Helper()
	rows := p.rowsOf(t, p.studentID)
	require.Len(t, rows, 2, "the running row is split at today")
	assert.Equal(t, p.running.ID, rows[0].ID, "the running row keeps its id")
	assert.Equal(t, p.running.ValidFrom, rows[0].ValidFrom, "history stays")
	assert.Equal(t, p.running.CalendarPeriodID, rows[0].CalendarPeriodID)
	assert.Equal(t, []int{1, 3}, rows[0].SelectedWeekdays, "history keeps both weekdays")
	if assert.NotNil(t, rows[0].ValidUntil, "the running row must not stay open") {
		assert.Equal(t, string(inactiveSourceToday), *rows[0].ValidUntil, "the inactive source leaves from today, not from the switch")
	}
	successor := rows[1]
	assert.Equal(t, string(inactiveSourceToday), successor.ValidFrom, "the valid source plans on from today")
	assert.Equal(t, []int{1}, successor.SelectedWeekdays, "the valid source plans Monday only")
	if assert.NotNil(t, successor.ValidUntil) {
		assert.Equal(t, string(successorUntil), *successor.ValidUntil)
	}
	p.requirePeriod(t, "successor of the valid source", successor)
	return successor
}

// requireProtected checks what no correction may touch or keep: the
// inactive source's Wednesday before the switch leaves at once, the Monday
// before the switch keeps its participant row, hand-decided and recorded
// attendance stay, and the other child is not touched at all.
func (p inactiveSourcePlan) requireProtected(t *testing.T) {
	t.Helper()
	assert.Equal(t, p.childMonday, p.participants(t, p.windowMonday)[p.studentID],
		"a Monday before the switch keeps the child's participant row untouched")
	assert.NotContains(t, p.participants(t, p.windowWednesday), p.studentID,
		"the inactive source's Wednesday before the switch leaves at once")
	kept := p.requireParticipant(t, "hand decision before the switch", p.decidedWednesday, p.studentID, p.decided)
	assert.Equal(t, studentpresence.SessionAttendanceAbsent, kept.Status, "the hand decision keeps its status")
	assert.NotNil(t, kept.ManualStatusAt, "the hand decision keeps its marker")
	keptRecord := p.requireParticipant(t, "recorded attendance after the switch", p.recordedWednesday, p.studentID, p.recorded)
	assert.Equal(t, studentpresence.SessionAttendancePresent, keptRecord.Status, "recorded attendance keeps its status")
	assert.NotNil(t, keptRecord.CheckedInAt, "recorded attendance keeps its check-in")
	requireUnchanged(t, "inactive source (other child)", p.neighbour, p.singleRowOf(t, "inactive source", p.neighbourID))
	assert.Equal(t, p.neighbourMonday, p.participants(t, p.windowMonday)[p.neighbourID],
		"the other child's participant row stays untouched")
	assert.Equal(t, p.neighbourSwitchMonday, p.participants(t, p.switchMonday)[p.neighbourID],
		"the other child's participant row stays untouched after the switch")
}

// requireStable runs the student and tenant-wide resyncs and the same
// correction again; none of them may change a row or a participant row,
// ids included. otherTemplateRows reloads rows of further templates the
// correction planned.
func (p inactiveSourcePlan) requireStable(t *testing.T, selection enrollmentTest.OfferingAdjustmentSelection, otherTemplateRows func() []timetable.StudentEnrollment) {
	t.Helper()
	snapshot := map[int64][]timetable.StudentEnrollment{p.studentID: p.rowsOf(t, p.studentID), p.neighbourID: p.rowsOf(t, p.neighbourID)}
	planned := map[int64]map[int64]testpkg.InstanceStudent{}
	for _, instanceID := range []int64{p.windowMonday, p.windowWednesday, p.decidedWednesday, p.switchMonday, p.recordedWednesday} {
		planned[instanceID] = p.participants(t, instanceID)
	}
	other := otherTemplateRows()
	for _, step := range []struct {
		name string
		run  func()
	}{
		{"student resync after the correction", func() {
			require.NoError(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), p.env.db, p.env.bookings.ResyncStudentSourcedRosters,
				[]int64{p.studentID, p.neighbourID}, inactiveSourceToday))
		}},
		{"tenant-wide resync after the correction", func() { p.resyncTenant(t) }},
		{"repeated correction with an inactive source", func() { p.correctAtSwitch(t, selection) }},
	} {
		step.run()
		p.requireUnchangedSince(t, step.name, snapshot, planned)
		after := otherTemplateRows()
		require.Len(t, after, len(other), "%s: other templates keep their rows", step.name)
		for i := range other {
			requireUnchanged(t, step.name, other[i], after[i])
		}
	}
}

func noOtherTemplateRows() []timetable.StudentEnrollment { return nil }

// The correction keeps B on the template: the successor plans B's Monday
// from today to the phase end.
func runInactiveSourceCorrection(t *testing.T, email string, periods divergingPeriodCase) {
	t.Helper()
	p := planInactiveSource(t, setupInactiveSourceCorrection(t, email, periods))
	p.deactivate(t, p.first.ID)
	p.correctAtSwitch(t, monday(p.second))

	p.requireSplitAtToday(t, longPhaseEnd.AddDays(1))
	p.requireProtected(t)
	assert.Equal(t, p.childSwitchMonday, p.participants(t, p.switchMonday)[p.studentID],
		"the switch Monday keeps the child's participant row untouched")
	later := p.participants(t, p.materializeOn(t, p.switchDate.AddDays(14)))
	assert.Contains(t, later, p.studentID, "a Monday materialized after the correction plans the child")
	assert.Contains(t, later, p.neighbourID)
	assert.NotContains(t, p.participants(t, p.materializeOn(t, p.switchDate.AddDays(16))), p.studentID,
		"a Wednesday materialized after the correction does not plan the child")
	p.requireStable(t, monday(p.second), noOtherTemplateRows)
}

// The correction switches from A/B to C, so the A/B template only the old
// selection feeds is reconciled from today as well (#5305 review): A's
// Wednesday leaves at once, B's Monday stays planned until the switch, and
// after it the child is planned through C only. With cFeedsTemplate C
// sources a Regeltermin of its own on Monday; without, C feeds no
// Regeltermin and the child leaves every sourced template at the switch.
func runInactiveSourceSwitchAway(t *testing.T, email string, periods divergingPeriodCase, cFeedsTemplate bool) {
	t.Helper()
	f := setupInactiveSourceCorrection(t, email, periods)
	c := createAdjustmentCareOfferingWith(t, f.env, "Ferienbetreuung", nil)
	otherRows := noOtherTemplateRows
	var cTemplate *timetable.Group
	if cFeedsTemplate {
		cTemplate = f.singleSourceTemplate(t, c)
		otherRows = func() []timetable.StudentEnrollment { return loadTemplateEnrollments(t, f.env, cTemplate.ID) }
	}
	p := planInactiveSource(t, f)
	p.deactivate(t, p.first.ID)
	p.correctAtSwitch(t, monday(c))

	p.requireSplitAtToday(t, p.switchDate)
	p.requireProtected(t)
	assert.NotContains(t, p.participants(t, p.switchMonday), p.studentID,
		"after the switch the old template no longer plans the child")
	later := p.participants(t, p.materializeOn(t, p.switchDate.AddDays(14)))
	assert.NotContains(t, later, p.studentID, "a Monday materialized after the correction does not plan the child")
	assert.Contains(t, later, p.neighbourID)
	if cFeedsTemplate {
		rows := otherRows()
		require.Len(t, rows, 1, "C plans the child on its own Regeltermin")
		assert.Equal(t, p.studentID, rows[0].StudentID)
		assert.Equal(t, string(p.switchDate), rows[0].ValidFrom, "C plans from the switch")
		assert.Equal(t, []int{1}, rows[0].SelectedWeekdays)
		assert.Contains(t, p.participants(t, templateInstanceOn(t, cTemplate.ID, p.switchDate)), p.studentID,
			"the switch Monday of C's Regeltermin plans the child")
	}
	p.requireStable(t, monday(c), otherRows)
}

// singleSourceTemplate creates a Monday Regeltermin sourced from one
// offering, without a period.
func (f inactiveSourceCorrection) singleSourceTemplate(t *testing.T, offering *offeringContract.CareOfferingRecord) *timetable.Group {
	t.Helper()
	group := createCareOfferingTemplateGroup(t, f.env.db, "EinzelQuelle")
	group.TargetGroupType = timetable.TargetGroupTypeOffering
	group.SourceCareOfferingIDs = []int64{offering.ID}
	require.NoError(t, timetableCompose.NativeGroupRecords(testTimetableRepositories(f.env.db).Timetable).Update(testpkg.Ctx(t), group))
	createCareOfferingTemplateSchedule(t, f.env.db, group.ID, timetable.WeekdayMonday, nil)
	return group
}

// templateInstanceOn returns the template's materialized instance on date.
func templateInstanceOn(t *testing.T, templateID int64, date calendar.Date) int64 {
	t.Helper()
	var ids []int64
	require.NoError(t, testpkg.SetupTestDB(t).NewSelect().Column("id").TableExpr("schedule.activity_instances").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", templateID).
		Where("date = ?", date).
		Scan(testpkg.Ctx(t), &ids))
	require.Len(t, ids, 1, "template %d has one instance on %s", templateID, date)
	return ids[0]
}

// markAttendance writes the child's attendance on the instance and returns
// the participant row.
func (f inactiveSourceCorrection) markAttendance(t *testing.T, instanceID int64, values map[string]any) testpkg.InstanceStudent {
	t.Helper()
	participant, ok := f.participants(t, instanceID)[f.studentID]
	require.True(t, ok, "sanity: the child is planned on instance %d", instanceID)
	testpkg.UpdateSessionAttendance(t, testpkg.Ctx(t), f.env.db, participant.ID, values)
	return f.participants(t, instanceID)[f.studentID]
}

func TestDatedCorrection_InactiveSourceWithoutPeriodKeepsValidSource(t *testing.T) {
	t.Parallel()
	runInactiveSourceCorrection(t, "mehrquellen-inaktiv-ohne@example.com", divergingPeriodCase{})
}

func TestDatedCorrection_InactiveSourceWithSharedPeriodKeepsValidSource(t *testing.T) {
	t.Parallel()
	runInactiveSourceCorrection(t, "mehrquellen-inaktiv-zeitraum@example.com",
		divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q", want: "Q"})
}

func TestDatedCorrection_InactiveSourceWithMixedPeriodsKeepsValidSource(t *testing.T) {
	t.Parallel()
	runInactiveSourceCorrection(t, "mehrquellen-inaktiv-gemischt@example.com",
		divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q2", want: ""})
}

func TestDatedCorrection_InactiveSourceSwitchAwayWithoutPeriod(t *testing.T) {
	t.Parallel()
	runInactiveSourceSwitchAway(t, "mehrquellen-inaktiv-wechsel-ohne@example.com", divergingPeriodCase{}, true)
}

func TestDatedCorrection_InactiveSourceSwitchAwayWithSharedPeriod(t *testing.T) {
	t.Parallel()
	runInactiveSourceSwitchAway(t, "mehrquellen-inaktiv-wechsel-zeitraum@example.com",
		divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q", want: "Q"}, true)
}

func TestDatedCorrection_InactiveSourceSwitchAwayWithMixedPeriods(t *testing.T) {
	t.Parallel()
	runInactiveSourceSwitchAway(t, "mehrquellen-inaktiv-wechsel-gemischt@example.com",
		divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q2", want: ""}, true)
}

// Dropping every source of the template for an offering that feeds no
// Regeltermin at all.
func TestDatedCorrection_InactiveSourceDropAllSources(t *testing.T) {
	t.Parallel()
	runInactiveSourceSwitchAway(t, "mehrquellen-inaktiv-alle-ab@example.com", divergingPeriodCase{}, false)
}

// The exception is for the resyncs of a template's stored sources. The
// resync behind a catalog edit (the catalog's guard) and the template save
// keep refusing a template with an inactive source and write nothing.
func TestInactiveSource_CatalogEditAndTemplateSaveStillRefuse(t *testing.T) {
	t.Parallel()
	f := setupMultiSourceCorrection(t, "mehrquellen-inaktiv-streng@example.com", divergingPeriodCase{phaseStart: inactiveSourceToday.AddDays(-10)})
	snapshot := map[int64][]timetable.StudentEnrollment{f.studentID: f.rowsOf(t, f.studentID), f.neighbourID: f.rowsOf(t, f.neighbourID)}
	f.deactivate(t, f.first.ID)

	scoped, ok := f.env.bookings.(enrollmentTest.SourcedTemplateResyncer)
	require.True(t, ok, "decision service must implement the offering-scoped resync")
	for _, offeringID := range []int64{f.first.ID, f.second.ID} {
		err := scoped.ResyncTemplatesSourcedFromOffering(testpkg.Ctx(t), offeringID, inactiveSourceToday)
		require.Error(t, err, "the catalog edit of offering %d is refused", offeringID)
		assert.Contains(t, err.Error(), "is inactive")
	}
	err := offeringResyncer(t, f.env).ResyncTemplateOfferingRoster(testpkg.Ctx(t), timetable.OfferingRosterResyncInput{
		TemplateID:       f.template.ID,
		OfferingIDs:      f.template.SourceCareOfferingIDs,
		CalendarPeriodID: f.template.CalendarPeriodID,
		EffectiveFrom:    inactiveSourceToday,
	})
	require.Error(t, err, "the template save is refused")
	assert.Contains(t, err.Error(), "is inactive")
	f.requireUnchangedSince(t, "refused resyncs", snapshot, nil)
}
