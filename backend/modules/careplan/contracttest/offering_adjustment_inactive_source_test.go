package contracttest_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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

func runInactiveSourceCorrection(t *testing.T, email string, periods divergingPeriodCase) {
	t.Helper()
	f := setupInactiveSourceCorrection(t, email, periods)
	f.correctFrom(t, nil,
		enrollmentTest.OfferingAdjustmentSelection{OfferingID: f.first.ID, SelectedDays: []string{"mon", "wed"}},
		monday(f.second))
	running := f.singleRowOf(t, "undated setup", f.studentID)
	require.Equal(t, []int{1, 3}, running.SelectedWeekdays, "sanity: the first source plans Monday and Wednesday")
	require.True(t, running.ValidFrom < string(inactiveSourceToday), "sanity: the row started before today")
	neighbour := f.singleRowOf(t, "undated setup", f.neighbourID)

	windowMonday := f.materializeOn(t, f.windowMonday)
	plannedWindowMonday := f.participants(t, windowMonday)
	childMonday, ok := plannedWindowMonday[f.studentID]
	require.True(t, ok, "sanity: the child is planned on the Monday before the switch")
	neighbourMonday, ok := plannedWindowMonday[f.neighbourID]
	require.True(t, ok, "sanity: the other child is planned on that Monday")
	windowWednesday := f.materializeOn(t, f.windowMonday.AddDays(2))
	require.Contains(t, f.participants(t, windowWednesday), f.studentID)
	decidedWednesday := f.materializeOn(t, f.windowMonday.AddDays(9))
	decided := f.markAttendance(t, decidedWednesday, map[string]any{
		"status": studentpresence.SessionAttendanceAbsent, "manual_status_at": time.Now(),
	})
	switchMonday := f.materializeOn(t, f.switchDate)
	childSwitchMonday, ok := f.participants(t, switchMonday)[f.studentID]
	require.True(t, ok, "sanity: the child is planned on the switch Monday")
	recordedWednesday := f.materializeOn(t, f.switchDate.AddDays(2))
	recorded := f.markAttendance(t, recordedWednesday, map[string]any{
		"status": studentpresence.SessionAttendancePresent, "checked_in_at": time.Now(),
	})

	f.deactivate(t, f.first.ID)
	f.correctAtSwitch(t, monday(f.second))

	rows := f.rowsOf(t, f.studentID)
	require.Len(t, rows, 2, "the running row is split at today")
	assert.Equal(t, running.ID, rows[0].ID, "the running row keeps its id")
	assert.Equal(t, running.ValidFrom, rows[0].ValidFrom, "history stays")
	assert.Equal(t, running.CalendarPeriodID, rows[0].CalendarPeriodID)
	assert.Equal(t, []int{1, 3}, rows[0].SelectedWeekdays, "history keeps both weekdays")
	if assert.NotNil(t, rows[0].ValidUntil, "the running row must not stay open") {
		assert.Equal(t, string(inactiveSourceToday), *rows[0].ValidUntil, "the inactive source leaves from today, not from the switch")
	}
	successor := rows[1]
	assert.Equal(t, string(inactiveSourceToday), successor.ValidFrom, "the valid source plans on from today")
	assert.Equal(t, []int{1}, successor.SelectedWeekdays, "the valid source plans Monday only")
	f.requirePeriod(t, "successor of the valid source", successor)

	assert.Equal(t, childMonday, f.participants(t, windowMonday)[f.studentID],
		"a Monday before the switch keeps the child's participant row untouched")
	assert.Equal(t, childSwitchMonday, f.participants(t, switchMonday)[f.studentID],
		"the switch Monday keeps the child's participant row untouched")
	assert.NotContains(t, f.participants(t, windowWednesday), f.studentID,
		"the inactive source's Wednesday before the switch leaves at once")
	kept := f.requireParticipant(t, "hand decision before the switch", decidedWednesday, f.studentID, decided)
	assert.Equal(t, studentpresence.SessionAttendanceAbsent, kept.Status, "the hand decision keeps its status")
	assert.NotNil(t, kept.ManualStatusAt, "the hand decision keeps its marker")
	keptRecord := f.requireParticipant(t, "recorded attendance after the switch", recordedWednesday, f.studentID, recorded)
	assert.Equal(t, studentpresence.SessionAttendancePresent, keptRecord.Status, "recorded attendance keeps its status")
	assert.NotNil(t, keptRecord.CheckedInAt, "recorded attendance keeps its check-in")
	requireUnchanged(t, "inactive source (other child)", neighbour, f.singleRowOf(t, "inactive source", f.neighbourID))
	assert.Equal(t, neighbourMonday, f.participants(t, windowMonday)[f.neighbourID],
		"the other child's participant row stays untouched")
	later := f.participants(t, f.materializeOn(t, f.switchDate.AddDays(14)))
	assert.Contains(t, later, f.studentID, "a Monday materialized after the correction plans the child")
	assert.Contains(t, later, f.neighbourID)
	assert.NotContains(t, f.participants(t, f.materializeOn(t, f.switchDate.AddDays(16))), f.studentID,
		"a Wednesday materialized after the correction does not plan the child")

	snapshot := map[int64][]timetable.StudentEnrollment{f.studentID: f.rowsOf(t, f.studentID), f.neighbourID: f.rowsOf(t, f.neighbourID)}
	planned := map[int64]map[int64]testpkg.InstanceStudent{}
	for _, instanceID := range []int64{windowMonday, windowWednesday, decidedWednesday, switchMonday, recordedWednesday} {
		planned[instanceID] = f.participants(t, instanceID)
	}
	require.NoError(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), f.env.db, f.env.bookings.ResyncStudentSourcedRosters,
		[]int64{f.studentID, f.neighbourID}, inactiveSourceToday))
	f.requireUnchangedSince(t, "student resync after the correction", snapshot, planned)
	f.resyncTenant(t)
	f.requireUnchangedSince(t, "tenant-wide resync after the correction", snapshot, planned)
	f.correctAtSwitch(t, monday(f.second))
	f.requireUnchangedSince(t, "repeated correction with an inactive source", snapshot, planned)
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
