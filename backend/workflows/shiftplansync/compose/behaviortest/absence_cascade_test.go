// Hermetic integration tests for #6847: every absence in force cascades into
// the plans, not only a sick report.
//
// Invariants:
//   - A1: a vacation request leaves the plans alone; its approval cancels the
//     person's shifts and marks their blocks absent under the neutral label
//     "Abwesenheit", and the history logs an ordinary absence, never a
//     Krankmeldung. Withdrawing the approved request gives every day back.
//   - A2: a shift planned on an approved vacation day takes the same state.
//   - A3: a reported training cascades at once; a single half day does not.
package compose_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// vacationDay lies after both clocks the vacation flow reads: the suite's
// fixed cascade clock and the live clock of the withdrawal's past-day guard.
var vacationDay = calendar.NewDate(2099, time.January, 6)

// vacationRequester is a staff member with an account, so the request's
// audit and withdrawal have an actor.
type vacationRequester struct {
	staffID   int64
	accountID int64
}

func (e *sickCascadeEnv) newVacationRequester(t *testing.T) vacationRequester {
	t.Helper()
	staff, account := testpkg.CreateTestStaffWithAccount(t, e.db, "Urlaub", fmt.Sprintf("Antrag-%d", time.Now().UnixNano()))
	e.inTx(t, func(ctx context.Context) error {
		return e.absences.TimeTracking.StaffAbsences.UpsertVacationQuota(ctx, staff.ID, vacationDay.Year(), 30, 0)
	})
	return vacationRequester{staffID: staff.ID, accountID: account.ID}
}

// requestVacation stores an open vacation request for one day.
func (e *sickCascadeEnv) requestVacation(t *testing.T, staffID int64, day calendar.Date) int64 {
	t.Helper()
	var absenceID int64
	require.NoError(t, e.db.NewRaw(
		`INSERT INTO active.staff_absences (tenant_id, staff_id, absence_type, date_start, date_end, status, created_by, working_days, requested_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1, NOW()) RETURNING id`,
		e.tenantID, staffID, workforce.AbsenceTypeVacation, day, day, workforce.AbsenceStatusRequested, staffID,
	).Scan(e.ctx, &absenceID))
	return absenceID
}

func assertAbsentStamp(t *testing.T, row *timetable.InstanceStaff, absenceID int64, msg string) {
	t.Helper()
	assert.True(t, row.IsAbsent, msg+": absent")
	if assert.NotNil(t, row.SickAbsenceID, msg+": stamp") {
		assert.Equal(t, absenceID, *row.SickAbsenceID, msg+": stamp names the absence")
	}
	if assert.NotNil(t, row.AbsenceReason, msg+": reason") {
		assert.Equal(t, timetable.AbsentBlockReason, *row.AbsenceReason, msg+": neutral reason, no art")
	}
}

func assertAbsenceCancelled(t *testing.T, shift workforce.StaffShift, absenceID int64, msg string) {
	t.Helper()
	assert.True(t, shift.Cancelled, msg+": cancelled")
	if assert.NotNil(t, shift.SickAbsenceID, msg+": stamp") {
		assert.Equal(t, absenceID, *shift.SickAbsenceID, msg+": stamp names the absence")
	}
	if assert.NotNil(t, shift.ChangeReason, msg+": reason") {
		assert.Equal(t, workforce.AbsenceShiftChangeReason, *shift.ChangeReason, msg+": neutral reason, no art")
	}
}

// eventsFor lists the Änderungsprotokoll entries of one staff member and type.
func (e *sickCascadeEnv) eventsFor(t *testing.T, staffID int64, day calendar.Date, eventType string) []deviationEvent {
	t.Helper()
	var filtered []deviationEvent
	for _, ev := range e.deviationEvents(t, day, day) {
		if ev.EventType == eventType && ev.SubjectStaffID != nil && *ev.SubjectStaffID == staffID {
			filtered = append(filtered, ev)
		}
	}
	return filtered
}

// A1: request → nothing; approval → shift and block leave the plan;
// withdrawal → both come back.
func TestAbsenceCascade_ApprovedVacationLeavesThePlansUntilWithdrawn(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	requester := e.newVacationRequester(t)
	shift := e.createShift(t, requester.staffID, vacationDay, "08:00", "12:00", nil)
	_, row := e.createBlock(t, vacationDay, timetable.InstanceStatusPlanned, "09:00", requester.staffID, testpkg.InstanceStaffOpts{})
	absenceID := e.requestVacation(t, requester.staffID, vacationDay)

	assert.False(t, e.reloadShift(t, shift.ID).Cancelled, "an open request leaves the shift planned")
	assert.False(t, e.reloadRow(t, row.ID).IsAbsent, "an open request leaves the block staffed")

	e.inTx(t, func(ctx context.Context) error {
		_, err := e.absences.SickReports.ApproveAbsence(ctx, absenceID, requester.accountID, e.admin.ID, "")
		return err
	})

	assertAbsenceCancelled(t, e.reloadShift(t, shift.ID), absenceID, "approved vacation day")
	assertAbsentStamp(t, e.reloadRow(t, row.ID), absenceID, "approved vacation day")
	assert.Empty(t, e.eventsFor(t, requester.staffID, vacationDay, eventSickReported), "a vacation is no Krankmeldung")
	marked := e.eventsFor(t, requester.staffID, vacationDay, "absence")
	require.Len(t, marked, 1, "the history logs an ordinary absence")
	var cause struct {
		Cause     string `json:"cause"`
		AbsenceID int64  `json:"absence_id"`
	}
	require.NoError(t, json.Unmarshal(marked[0].NewValue, &cause))
	assert.Equal(t, "absence", cause.Cause)
	assert.Equal(t, absenceID, cause.AbsenceID)

	e.inTx(t, func(ctx context.Context) error {
		return e.absences.SickReports.CancelAbsence(ctx, requester.staffID, requester.accountID, absenceID)
	})

	restored := e.reloadShift(t, shift.ID)
	assert.False(t, restored.Cancelled, "the withdrawal plans the shift again")
	assert.Nil(t, restored.ChangeReason)
	assert.Nil(t, restored.SickAbsenceID)
	assertPresent(t, e.reloadRow(t, row.ID), "withdrawn vacation day")
	assert.Empty(t, e.eventsFor(t, requester.staffID, vacationDay, eventSickCleared), "a vacation is no Krankmeldung")
	assert.Len(t, e.eventsFor(t, requester.staffID, vacationDay, "return_to_presence"), 1)
}

// A2: the Dienstplan written after the approval agrees with the one written
// before it.
func TestAbsenceCascade_ShiftPlannedOnApprovedVacationIsCancelled(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	requester := e.newVacationRequester(t)
	absenceID := e.requestVacation(t, requester.staffID, vacationDay)
	e.inTx(t, func(ctx context.Context) error {
		_, err := e.absences.SickReports.ApproveAbsence(ctx, absenceID, requester.accountID, e.admin.ID, "")
		return err
	})

	planned := e.mustPlanShift(t, requester.staffID, vacationDay, "13:00", "15:00")

	assertAbsenceCancelled(t, e.reloadShift(t, planned.ID), absenceID, "shift planned after the approval")
}

// A3: a training an admin reports is in force at once; a single half day of
// it is not, the person works the other half.
func TestAbsenceCascade_ReportedTrainingCascadesButAHalfDayDoesNot(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	today := calendar.NewDate(2026, time.August, 24)
	fullDay, halfDay := today.AddDays(1), today.AddDays(2)
	fullShift := e.createShift(t, e.subject.ID, fullDay, "08:00", "12:00", nil)
	halfShift := e.createShift(t, e.subject.ID, halfDay, "08:00", "12:00", nil)
	_, fullRow := e.createBlock(t, fullDay, timetable.InstanceStatusPlanned, "09:00", e.subject.ID, testpkg.InstanceStaffOpts{})

	var trainingID int64
	e.inTx(t, func(ctx context.Context) error {
		created, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
			AbsenceType: workforce.AbsenceTypeTraining, DateStart: fullDay.String(), DateEnd: fullDay.String(),
		})
		if err == nil {
			trainingID = created.ID
		}
		return err
	})
	e.inTx(t, func(ctx context.Context) error {
		_, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
			AbsenceType: workforce.AbsenceTypeTraining, DateStart: halfDay.String(), DateEnd: halfDay.String(), HalfDay: true,
		})
		return err
	})

	assertAbsenceCancelled(t, e.reloadShift(t, fullShift.ID), trainingID, "full training day")
	assertAbsentStamp(t, e.reloadRow(t, fullRow.ID), trainingID, "full training day")
	assert.False(t, e.reloadShift(t, halfShift.ID).Cancelled, "a half training day leaves the shift to the planner")
}

// A4: an open-ended "Sonstige" (an Elternzeit, say) cascades over a bounded
// window: today's plans leave, and the write neither scans nor locks years.
func TestAbsenceCascade_OpenEndedAbsenceCascadesOverABoundedWindow(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	tomorrow := calendar.NewDate(2026, time.August, 25)
	shift := e.createShift(t, e.subject.ID, tomorrow, "08:00", "12:00", nil)

	var absenceID int64
	e.inTx(t, func(ctx context.Context) error {
		created, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
			AbsenceType: workforce.AbsenceTypeOther, DateStart: tomorrow.String(), DateEnd: "9999-12-31",
		})
		if err == nil {
			absenceID = created.ID
		}
		return err
	})

	assertAbsenceCancelled(t, e.reloadShift(t, shift.ID), absenceID, "first day of the open-ended absence")
}
