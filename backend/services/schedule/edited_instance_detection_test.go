package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// diffOccurrenceWithExpectedStudents is pure (no DB), so its truth table is exhaustively
// unit-testable. `expected` models what the template would materialize on the
// occurrence's date (computed date-/exception-aware by expectedSlotsOn). IDs are
// multi-digit throwaway values (never DB rows) to stay clear of the hermetic-ID
// gate.

const (
	edPeriodID = int64(700)
	edRoomA    = int64(810)
	edRoomB    = int64(820)
	edStaffP   = int64(910) // primary supervisor
	edStaffS   = int64(920) // secondary supervisor
	edStaffX   = int64(930) // not on the template
	edStudA    = int64(1010)
	edStudB    = int64(1020)
	edStudC    = int64(1030)
	edTitle    = "Fußball AG"
)

// edWednesday is a Wednesday (ISO weekday 3) inside the period.
var edWednesday = timezone.NewDate(2026, time.June, 10)

func edClock(h, m int) time.Time {
	return time.Date(2000, time.January, 1, h, m, 0, 0, time.UTC)
}

// edExpected is the template projection for the occurrence's date: one slot,
// 15:00–16:00 in room A.
func edExpected() []materialParams {
	return []materialParams{
		{StartTime: edClock(15, 0), EndTime: edClock(16, 0), RoomID: edRoomA},
	}
}

func edSupervisors() []*activities.SupervisorPlanned {
	pid := edPeriodID
	return []*activities.SupervisorPlanned{
		{StaffID: edStaffP, IsPrimary: true, CalendarPeriodID: &pid},
		{StaffID: edStaffS, IsPrimary: false, CalendarPeriodID: &pid},
	}
}

func edEnrollments() []*activities.StudentEnrollment {
	pid := edPeriodID
	return []*activities.StudentEnrollment{
		{StudentID: edStudA, CalendarPeriodID: &pid},
		{StudentID: edStudB, CalendarPeriodID: &pid},
	}
}

// edPristineInstance is a materialized occurrence that matches the template
// exactly — diffOccurrence must report NO changes for it.
func edPristineInstance() *scheduleModel.ActivityInstance {
	pid := edPeriodID
	return &scheduleModel.ActivityInstance{
		Title:            edTitle,
		Notes:            nil,
		Date:             edWednesday,
		StartTime:        edClock(15, 0),
		EndTime:          edClock(16, 0),
		RoomID:           edRoomA,
		CalendarPeriodID: &pid,
		Status:           scheduleModel.InstanceStatusPlanned,
	}
}

// edPristineStaff mirrors the template supervisors as fresh (non-substitute) rows.
func edPristineStaff() []*scheduleModel.InstanceStaff {
	return []*scheduleModel.InstanceStaff{
		{StaffID: edStaffP, IsPrimary: true},
		{StaffID: edStaffS, IsPrimary: false},
	}
}

func edPristineStudents() []*scheduleModel.InstanceStudent {
	return []*scheduleModel.InstanceStudent{
		{StudentID: edStudA, Status: scheduleModel.AttendanceStatusExpected},
		{StudentID: edStudB, Status: scheduleModel.AttendanceStatusExpected},
	}
}

// edDiff runs diffOccurrence with the standard template roster inputs and the
// given occurrence state + date projection.
func edDiff(
	inst *scheduleModel.ActivityInstance,
	staff []*scheduleModel.InstanceStaff,
	students []*scheduleModel.InstanceStudent,
	expected []materialParams,
) []string {
	expectedStudentIDs := expectedStudentIDsOn(
		edEnrollments(), nil, nil, inst.Date, calendarPeriodID(inst),
	)
	return diffOccurrenceWithExpectedStudents(
		inst,
		edTitle,
		expectedStudentIDs,
		edSupervisors(),
		staff,
		students,
		expected,
	)
}

func TestDiffOccurrence_Pristine_NoChanges(t *testing.T) {
	t.Parallel()

	changes := edDiff(edPristineInstance(), edPristineStaff(), edPristineStudents(), edExpected())
	assert.Empty(t, changes, "an occurrence identical to its template must report no edits")
}

func TestDiffOccurrence_Title(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	inst.Title = "Fußball AG (Ausnahme)"
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Equal(t, []string{EditedChangeTitle}, changes)
}

func TestDiffOccurrence_Notes(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	note := "Heute im Turnraum treffen"
	inst.Notes = &note
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Equal(t, []string{EditedChangeNotes}, changes)
}

func TestDiffOccurrence_BlankNotesIgnored(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	blank := "   "
	inst.Notes = &blank
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Empty(t, changes, "whitespace-only notes are not a real edit")
}

func TestDiffOccurrence_Description(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	desc := "Bitte Sportkleidung mitbringen"
	inst.Description = &desc
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Equal(t, []string{EditedChangeDescription}, changes)
}

func TestDiffOccurrence_BlankDescriptionIgnored(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	blank := "  "
	inst.Description = &blank
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Empty(t, changes, "whitespace-only description is not a real edit")
}

func TestDiffOccurrence_Room(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	inst.RoomID = edRoomB
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Equal(t, []string{EditedChangeRoom}, changes)
}

func TestDiffOccurrence_EndTimeChange(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	inst.EndTime = edClock(17, 0) // template ends 16:00
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Equal(t, []string{EditedChangeTime}, changes)
}

func TestDiffOccurrence_StartTimeNoLongerMatches(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	inst.StartTime = edClock(14, 0) // no expected slot at 14:00 → moved/time-edited
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Equal(t, []string{EditedChangeTime}, changes)
}

func TestDiffOccurrence_NoExpectedSlotOnDate(t *testing.T) {
	t.Parallel()

	// The template materializes nothing on this occurrence's date (off-cycle /
	// out-of-range / cancelled) — expectedSlotsOn returns empty, so the row is a
	// lost `time` edit even though its start matches a template weekday elsewhere.
	inst := edPristineInstance()
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), nil)
	assert.Equal(t, []string{EditedChangeTime}, changes)
}

func TestExpectedSlotsOn_LegacyWeekendScheduleIsNotMaterializable(t *testing.T) {
	t.Parallel()

	// materializeTemplate skips legacy Saturday/Sunday schedules. The edited
	// instance projection must do the same so it warns before a re-plan removes
	// an existing weekend row without replacing it.
	service := &materializationService{}
	saturday := timezone.NewDate(2026, time.June, 13)

	slots := service.expectedSlotsOn(
		&activities.Group{},
		[]*activities.Schedule{{Weekday: 6}},
		nil,
		nil,
		nil,
		saturday,
	)

	assert.Empty(t, slots)
}

func TestDiffOccurrence_ExceptionShiftedStartIsMatched(t *testing.T) {
	t.Parallel()

	// A modified exception shifts the template start to 16:00 for this date, so
	// materialization creates the occurrence at 16:00–17:00. The (already
	// exception-aware) expected projection reflects that; a pristine occurrence
	// at 16:00 must NOT be flagged as a lost time edit (regression: #1907 review).
	inst := edPristineInstance()
	inst.StartTime = edClock(16, 0)
	inst.EndTime = edClock(17, 0)
	expected := []materialParams{
		{StartTime: edClock(16, 0), EndTime: edClock(17, 0), RoomID: edRoomA},
	}
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), expected)
	assert.Empty(t, changes, "an occurrence at the exception-shifted start is not a lost edit")
}

func TestDiffOccurrence_RoomMatchesExceptionOverride(t *testing.T) {
	t.Parallel()

	// Exception overrides the room to B for this date; expectedSlotsOn folds that
	// in, so an occurrence in room B matches and is not flagged.
	inst := edPristineInstance()
	inst.RoomID = edRoomB
	expected := []materialParams{
		{StartTime: edClock(15, 0), EndTime: edClock(16, 0), RoomID: edRoomB},
	}
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), expected)
	assert.Empty(t, changes)
}

func TestDiffOccurrence_StaffAdded(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	staff := append(edPristineStaff(), &scheduleModel.InstanceStaff{StaffID: edStaffX})
	changes := edDiff(inst, staff, edPristineStudents(), edExpected())
	assert.Equal(t, []string{EditedChangeStaff}, changes)
}

func TestDiffOccurrence_StaffRemoved(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	staff := []*scheduleModel.InstanceStaff{{StaffID: edStaffP, IsPrimary: true}}
	changes := edDiff(inst, staff, edPristineStudents(), edExpected())
	assert.Equal(t, []string{EditedChangeStaff}, changes)
}

func TestDiffOccurrence_PrimaryFlagChange(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	staff := []*scheduleModel.InstanceStaff{
		{StaffID: edStaffP, IsPrimary: false}, // was primary on the template
		{StaffID: edStaffS, IsPrimary: true},
	}
	changes := edDiff(inst, staff, edPristineStudents(), edExpected())
	assert.Equal(t, []string{EditedChangeStaff}, changes)
}

func TestDiffOccurrence_StudentsChanged(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	students := []*scheduleModel.InstanceStudent{
		{StudentID: edStudA, Status: scheduleModel.AttendanceStatusExpected},
		{StudentID: edStudC, Status: scheduleModel.AttendanceStatusExpected},
	}
	changes := edDiff(inst, edPristineStaff(), students, edExpected())
	assert.Equal(t, []string{EditedChangeStudents}, changes)
}

func TestDiffOccurrence_StatusDayAbsenceIsNotAttendanceEdit(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	statusDayID := int64(4401)
	students := []*scheduleModel.InstanceStudent{
		{StudentID: edStudA, Status: scheduleModel.AttendanceStatusAbsent, StudentStatusDayID: &statusDayID},
		{StudentID: edStudB, Status: scheduleModel.AttendanceStatusExpected},
	}
	changes := edDiff(inst, edPristineStaff(), students, edExpected())
	assert.Empty(t, changes, "a status-day absence is rematerialized, not a lost edit")
}

func TestDiffOccurrence_PickupExceptionAbsenceIsNotAttendanceEdit(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	pickupID := int64(4402)
	students := []*scheduleModel.InstanceStudent{
		{StudentID: edStudA, Status: scheduleModel.AttendanceStatusAbsent, PickupExceptionID: &pickupID},
		{StudentID: edStudB, Status: scheduleModel.AttendanceStatusExpected},
	}
	changes := edDiff(inst, edPristineStaff(), students, edExpected())
	assert.Empty(t, changes, "a pickup-exception absence is rematerialized, not a lost edit")
}

func TestDiffOccurrence_ManualPresentIsAttendanceEdit(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	now := time.Now()
	students := []*scheduleModel.InstanceStudent{
		{StudentID: edStudA, Status: scheduleModel.AttendanceStatusPresent, CheckedInAt: &now},
		{StudentID: edStudB, Status: scheduleModel.AttendanceStatusExpected},
	}
	changes := edDiff(inst, edPristineStaff(), students, edExpected())
	assert.Equal(t, []string{EditedChangeAttendance}, changes)
}

func TestDiffOccurrence_ManualAbsentIsAttendanceEdit(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	now := time.Now()
	students := []*scheduleModel.InstanceStudent{
		{StudentID: edStudA, Status: scheduleModel.AttendanceStatusAbsent, ManualStatusAt: &now},
		{StudentID: edStudB, Status: scheduleModel.AttendanceStatusExpected},
	}
	changes := edDiff(inst, edPristineStaff(), students, edExpected())
	assert.Equal(t, []string{EditedChangeAttendance}, changes)
}

func TestDiffOccurrence_NotScheduledIsAttendanceEdit(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	students := []*scheduleModel.InstanceStudent{
		{StudentID: edStudA, Status: scheduleModel.AttendanceStatusExpected, NotScheduled: true},
		{StudentID: edStudB, Status: scheduleModel.AttendanceStatusExpected},
	}
	changes := edDiff(inst, edPristineStaff(), students, edExpected())
	assert.Equal(t, []string{EditedChangeAttendance}, changes)
}

// --- Vertretungsplan deviations must NOT count as edits (ReplanWeek preserves them) ---

func TestDiffOccurrence_SubstituteNotAnEdit(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	// A planned staff marked absent (row kept) + a substitute row added — the
	// classic /deviations shape. The planned set is unchanged; the substitute
	// is excluded. No edit.
	staff := []*scheduleModel.InstanceStaff{
		{StaffID: edStaffP, IsPrimary: true, IsAbsent: true},
		{StaffID: edStaffS, IsPrimary: false},
		{StaffID: edStaffX, IsSubstitute: true},
	}
	changes := edDiff(inst, staff, edPristineStudents(), edExpected())
	assert.Empty(t, changes, "absence + substitute is a preserved deviation, not a lost edit")
}

func TestDiffOccurrence_UnderstaffedAckNotAnEdit(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	inst.UnderstaffedAck = true
	note := "bewusst unbesetzt"
	inst.UnderstaffedNote = &note
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Empty(t, changes, "understaffed-ack is a preserved deviation")
}

func TestDiffOccurrence_RequiredStaffPinNotAnEdit(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	pin := 3
	inst.RequiredStaff = &pin
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Empty(t, changes, "the required_staff pin is preserved by ReplanWeek")
}

func TestDiffOccurrence_MultipleChangesSorted(t *testing.T) {
	t.Parallel()

	inst := edPristineInstance()
	inst.Title = "Anders"
	inst.RoomID = edRoomB
	note := "Notiz"
	inst.Notes = &note
	changes := edDiff(inst, edPristineStaff(), edPristineStudents(), edExpected())
	assert.Equal(t, []string{EditedChangeNotes, EditedChangeRoom, EditedChangeTitle}, changes,
		"changes are returned sorted")
}
