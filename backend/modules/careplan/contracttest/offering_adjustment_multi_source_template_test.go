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
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A dated correction on a Regeltermin sourced from several offerings (#5234).
// The per-link drafts never cover such a template; the union resync decides
// about the child's row there. Dropping one source while another still plans
// the same weekdays changes nothing: the row and the planned participants of
// the materialized occurrences keep their ids. Only a changed union is split
// at the switch date, and a template the child loses every source of is
// capped as before. Other children of the template are never touched.

// multiSourceCorrection is one approved child holding both sources of a
// Monday/Wednesday template, next to a second child holding them too.
type multiSourceCorrection struct {
	divergingPeriodFixture
	first, second *offeringContract.CareOfferingRecord
	neighbourID   int64
	switchDate    calendar.Date
}

func setupMultiSourceCorrection(t *testing.T, email string, periods divergingPeriodCase) multiSourceCorrection {
	t.Helper()
	periods.multiSource = true
	f := setupDivergingPeriodFixture(t, email, periods)
	require.Len(t, f.template.SourceCareOfferingIDs, 2)
	second := &offeringContract.CareOfferingRecord{}
	second.ID = f.template.SourceCareOfferingIDs[1]
	_, _, neighbourID := submitApprovedAdjustmentChild(t, f.env, "nachbar-"+email, "Nachbar",
		[]*offeringContract.CareOfferingRecord{f.source, second})
	switchDate := calendar.Date(f.env.sourcePhase.ServiceStartDate).AddDays(60)
	return multiSourceCorrection{divergingPeriodFixture: f, first: f.source, second: second, neighbourID: neighbourID, switchDate: switchDate}
}

// correctFrom runs the correction capability with the selections, dated at
// the switch date or undated.
func (f multiSourceCorrection) correctFrom(t *testing.T, effectiveFrom *calendar.Date, selections ...enrollmentTest.OfferingAdjustmentSelection) {
	t.Helper()
	_, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), f.env.db, f.env.decision.UpdateChildOfferings, enrollmentTest.UpdateChildOfferingsInput{
		RequestID:      f.requestID,
		ChildID:        f.childID,
		ActorAccountID: f.env.creatorID,
		ActorRole:      "admin",
		Reason:         "Quelle abgewählt",
		EffectiveFrom:  effectiveFrom,
		Offerings:      selections,
	})
	require.NoError(t, err)
}

func (f multiSourceCorrection) correctAtSwitch(t *testing.T, selections ...enrollmentTest.OfferingAdjustmentSelection) {
	t.Helper()
	switchDate := f.switchDate
	f.correctFrom(t, &switchDate, selections...)
}

// rowsOf returns the template's rows of one student.
func (f multiSourceCorrection) rowsOf(t *testing.T, studentID int64) []timetable.StudentEnrollment {
	t.Helper()
	var rows []timetable.StudentEnrollment
	for _, row := range loadTemplateEnrollments(t, f.env, f.template.ID) {
		if row.StudentID == studentID {
			rows = append(rows, row)
		}
	}
	return rows
}

func (f multiSourceCorrection) singleRowOf(t *testing.T, step string, studentID int64) timetable.StudentEnrollment {
	t.Helper()
	rows := f.rowsOf(t, studentID)
	require.Len(t, rows, 1, "%s: student %d keeps exactly one row on the template", step, studentID)
	if f.wantPeriodID == nil {
		assert.Nil(t, rows[0].CalendarPeriodID, "%s: schedules without one shared period plan a row without one", step)
	} else if assert.NotNil(t, rows[0].CalendarPeriodID, "%s", step) {
		assert.Equal(t, *f.wantPeriodID, *rows[0].CalendarPeriodID, "%s: the row carries the schedules' shared period", step)
	}
	return rows[0]
}

// childRow is the corrected child's only row, planning Monday.
func (f multiSourceCorrection) childRow(t *testing.T, step string) timetable.StudentEnrollment {
	t.Helper()
	row := f.singleRowOf(t, step, f.studentID)
	assert.Equal(t, []int{1}, row.SelectedWeekdays, "%s", step)
	return row
}

// materialize runs the Timetable owner's materializer for the day the given
// number of days after the first Monday on or after the switch that has not
// passed yet (the occurrence reconciler acts from the real today on), and
// returns the template's instance there.
func (f multiSourceCorrection) materialize(t *testing.T, daysAfterMonday int) int64 {
	t.Helper()
	date := firstFutureMondayOnOrAfter(f.switchDate).AddDays(daysAfterMonday)
	_, err := f.timetable.Materialization.MaterializeForTenant(testpkg.Ctx(t), date, date, timetable.MaterializationSourceManual)
	require.NoError(t, err)
	var ids []int64
	require.NoError(t, testpkg.SetupTestDB(t).NewSelect().Column("id").TableExpr("schedule.activity_instances").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("activity_group_id = ?", f.template.ID).
		Where("date = ?", date).
		Scan(testpkg.Ctx(t), &ids))
	require.Len(t, ids, 1, "the template has one instance on %s", date)
	return ids[0]
}

// participants maps the instance's students to their participant rows.
func (f multiSourceCorrection) participants(t *testing.T, instanceID int64) map[int64]testpkg.InstanceStudent {
	t.Helper()
	byStudent := make(map[int64]testpkg.InstanceStudent)
	for _, row := range loadSourcedInstanceStudents(t, f.env, instanceID) {
		byStudent[row.StudentID] = row
	}
	return byStudent
}

// requireSameParticipants asserts that a step neither removed nor re-created
// a participant row of the instance.
func (f multiSourceCorrection) requireSameParticipants(t *testing.T, step string, instanceID int64, before map[int64]testpkg.InstanceStudent) {
	t.Helper()
	after := f.participants(t, instanceID)
	require.Len(t, after, len(before), "%s: the instance keeps its participants", step)
	for studentID, row := range before {
		if assert.Contains(t, after, studentID, "%s: student %d stays planned", step, studentID) {
			assert.Equal(t, row.ID, after[studentID].ID, "%s: student %d keeps the participant row", step, studentID)
			assert.Equal(t, row.Status, after[studentID].Status, "%s", step)
		}
	}
}

// requireStable runs the student-scoped and the tenant-wide resync and
// asserts that neither touches the rows or the participants.
func (f multiSourceCorrection) requireStable(t *testing.T, rows map[int64]timetable.StudentEnrollment, instanceID int64, participants map[int64]testpkg.InstanceStudent) {
	t.Helper()
	require.NoError(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), f.env.db, f.env.bookings.ResyncStudentSourcedRosters,
		[]int64{f.studentID, f.neighbourID}, decisionTestToday))
	f.resyncTenant(t)
	for studentID, row := range rows {
		requireUnchanged(t, "resync after correction", row, f.singleRowOf(t, "resync after correction", studentID))
	}
	f.requireSameParticipants(t, "resync after correction", instanceID, participants)
}

func monday(selection *offeringContract.CareOfferingRecord) enrollmentTest.OfferingAdjustmentSelection {
	return enrollmentTest.OfferingAdjustmentSelection{OfferingID: selection.ID, SelectedDays: []string{"mon"}}
}

// runUnchangedUnionCorrection drops one of the two sources at a dated switch
// while the other keeps the child's Monday.
func runUnchangedUnionCorrection(t *testing.T, email string, periods divergingPeriodCase, dropFirst bool) {
	t.Helper()
	f := setupMultiSourceCorrection(t, email, periods)
	child := f.childRow(t, "approval")
	neighbour := f.singleRowOf(t, "approval", f.neighbourID)
	occurrence := f.materialize(t, 0)
	planned := f.participants(t, occurrence)
	require.Contains(t, planned, f.studentID)
	require.Contains(t, planned, f.neighbourID)

	kept := f.second
	if !dropFirst {
		kept = f.first
	}
	f.correctAtSwitch(t, monday(kept))
	requireUnchanged(t, "dated correction", child, f.childRow(t, "dated correction"))
	requireUnchanged(t, "dated correction (other child)", neighbour, f.singleRowOf(t, "dated correction", f.neighbourID))
	f.requireSameParticipants(t, "dated correction", occurrence, planned)

	f.correctAtSwitch(t, monday(kept))
	requireUnchanged(t, "repeated dated correction", child, f.childRow(t, "repeated dated correction"))
	f.requireSameParticipants(t, "repeated dated correction", occurrence, planned)

	rows := map[int64]timetable.StudentEnrollment{f.studentID: child, f.neighbourID: neighbour}
	f.requireStable(t, rows, occurrence, planned)
	later := f.participants(t, f.materialize(t, 7))
	assert.Contains(t, later, f.studentID, "the child stays planned on later occurrences")
	assert.Contains(t, later, f.neighbourID)
}

func TestDatedCorrection_MultiSourceDropSecondSourceWithoutPeriodKeepsRow(t *testing.T) {
	t.Parallel()
	runUnchangedUnionCorrection(t, "mehrquellen-zweite-ohne@example.com", divergingPeriodCase{}, false)
}

func TestDatedCorrection_MultiSourceDropFirstSourceWithoutPeriodKeepsRow(t *testing.T) {
	t.Parallel()
	runUnchangedUnionCorrection(t, "mehrquellen-erste-ohne@example.com", divergingPeriodCase{}, true)
}

func TestDatedCorrection_MultiSourceDropSourceWithSharedPeriodKeepsRow(t *testing.T) {
	t.Parallel()
	runUnchangedUnionCorrection(t, "mehrquellen-zeitraum@example.com",
		divergingPeriodCase{template: "P", monday: "Q", wednesday: "Q", want: "Q"}, true)
}

// A union that really shrinks is split at the switch: the running row keeps
// its id and history up to the switch, the successor plans what remains, and
// only purely planned participants of the lost weekday leave. A hand
// decision on such an occurrence stays.
func TestDatedCorrection_MultiSourceShrinkingUnionSplitsAtSwitch(t *testing.T) {
	t.Parallel()
	f := setupMultiSourceCorrection(t, "mehrquellen-kleiner@example.com", divergingPeriodCase{})
	f.correctFrom(t, nil,
		enrollmentTest.OfferingAdjustmentSelection{OfferingID: f.first.ID, SelectedDays: []string{"mon", "wed"}},
		monday(f.second))
	running := f.singleRowOf(t, "undated setup", f.studentID)
	require.Equal(t, []int{1, 3}, running.SelectedWeekdays, "sanity: the first source plans Monday and Wednesday")
	neighbour := f.singleRowOf(t, "undated setup", f.neighbourID)

	mondayOccurrence := f.materialize(t, 0)
	plannedWednesday := f.materialize(t, 2)
	decidedWednesday := f.materialize(t, 9)
	require.Contains(t, f.participants(t, plannedWednesday), f.studentID)
	decided := f.participants(t, decidedWednesday)[f.studentID]
	require.NotZero(t, decided.ID)
	testpkg.UpdateSessionAttendance(t, testpkg.Ctx(t), f.env.db, decided.ID, map[string]any{
		"status": studentpresence.SessionAttendanceAbsent, "manual_status_at": time.Now(),
	})

	f.correctAtSwitch(t, monday(f.second))

	rows := f.rowsOf(t, f.studentID)
	require.Len(t, rows, 2, "the running row is split at the switch")
	assert.Equal(t, running.ID, rows[0].ID, "the running row keeps its id")
	assert.Equal(t, running.ValidFrom, rows[0].ValidFrom)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, string(f.switchDate), *rows[0].ValidUntil, "the running row ends at the switch")
	assert.Equal(t, []int{1, 3}, rows[0].SelectedWeekdays)
	assert.Equal(t, string(f.switchDate), rows[1].ValidFrom, "the successor starts at the switch")
	assert.Equal(t, []int{1}, rows[1].SelectedWeekdays, "the successor plans the remaining Monday")
	requireUnchanged(t, "shrinking correction (other child)", neighbour, f.singleRowOf(t, "shrinking correction", f.neighbourID))

	assert.Contains(t, f.participants(t, mondayOccurrence), f.studentID, "the remaining Monday stays planned")
	assert.NotContains(t, f.participants(t, plannedWednesday), f.studentID, "the lost Wednesday leaves the planned occurrence")
	kept := f.participants(t, decidedWednesday)
	if assert.Contains(t, kept, f.studentID, "a hand decision survives the shrink") {
		assert.Equal(t, decided.ID, kept[f.studentID].ID)
		assert.Equal(t, studentpresence.SessionAttendanceAbsent, kept[f.studentID].Status)
	}

	successor := rows[1]
	f.correctAtSwitch(t, monday(f.second))
	f.resyncTenant(t)
	require.NoError(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), f.env.db, f.env.bookings.ResyncStudentSourcedRosters, []int64{f.studentID}, decisionTestToday))
	stable := f.rowsOf(t, f.studentID)
	require.Len(t, stable, 2, "a repeated correction and resyncs keep the split")
	requireUnchanged(t, "repeated shrinking correction", rows[0], stable[0])
	requireUnchanged(t, "repeated shrinking correction", successor, stable[1])
}

// Losing every source of the template caps the running row at the switch and
// removes the child from the planned occurrence after it; the other child
// stays.
func TestDatedCorrection_MultiSourceAllSourcesDroppedCapsRow(t *testing.T) {
	t.Parallel()
	f := setupMultiSourceCorrection(t, "mehrquellen-alle@example.com", divergingPeriodCase{})
	running := f.childRow(t, "approval")
	neighbour := f.singleRowOf(t, "approval", f.neighbourID)
	occurrence := f.materialize(t, 0)
	require.Contains(t, f.participants(t, occurrence), f.studentID)
	neighbourParticipant := f.participants(t, occurrence)[f.neighbourID]

	other := createAdjustmentCareOfferingWith(t, f.env, "Ferienbetreuung", nil)
	f.correctAtSwitch(t, monday(other))

	capped := f.singleRowOf(t, "dropping every source", f.studentID)
	assert.Equal(t, running.ID, capped.ID)
	require.NotNil(t, capped.ValidUntil)
	assert.Equal(t, string(f.switchDate), *capped.ValidUntil, "the running row ends at the switch")
	requireUnchanged(t, "dropping every source (other child)", neighbour, f.singleRowOf(t, "dropping every source", f.neighbourID))
	after := f.participants(t, occurrence)
	assert.NotContains(t, after, f.studentID, "the child leaves the occurrence after the switch")
	if assert.Contains(t, after, f.neighbourID) {
		assert.Equal(t, neighbourParticipant.ID, after[f.neighbourID].ID, "the other child keeps its participant row")
	}
}
