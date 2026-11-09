// Hermetic integration tests for single shift writes after a sick report
// (#6302), the Dienstplan side of #6300's precedent.
//
// Invariants:
//   - W1: a shift created for a full sick day of its staff member is in the
//     state the cascade gives a shift created before the report, and the
//     report's deletion reactivates it alike. Past days stay history; a
//     shift the planner cancels herself keeps her reason.
//   - W2: a shift moved to a sick person or onto a sick day takes that day's
//     sick state; a time change on the same day keeps the row's state.
//   - W3: a sick person cannot cover a cancelled shift
//     (workforce.ErrReplacementStaffAbsent).
//   - W4: a shift a sick report cancels is checked for overlap as planned,
//     so the report's deletion can reactivate every shift it cancelled.
//
// No lock is added: every read runs under the staff member's shift lock the
// write already holds.
package compose_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// planShift creates one shift through the planning service.
func (e *sickCascadeEnv) planShift(t *testing.T, staffID int64, date calendar.Date, start, end string, mutate func(*workforce.StaffShiftInput)) (workforce.StaffShift, error) {
	t.Helper()
	input := workforce.StaffShiftInput{StaffID: staffID, Date: date.String(), StartTime: clock(start), EndTime: clock(end), ActorStaffID: e.admin.ID}
	if mutate != nil {
		mutate(&input)
	}
	var created workforce.PlannedShift
	var createErr error
	e.inTx(t, func(ctx context.Context) error {
		created, createErr = e.Planning.CreateShift(ctx, workforce.CreateStaffShift{StaffShiftInput: input})
		return nil
	})
	return created.StaffShift, createErr
}

func (e *sickCascadeEnv) mustPlanShift(t *testing.T, staffID int64, date calendar.Date, start, end string) workforce.StaffShift {
	t.Helper()
	shift, err := e.planShift(t, staffID, date, start, end, nil)
	require.NoError(t, err)
	return shift
}

func (e *sickCascadeEnv) moveShift(t *testing.T, shift workforce.StaffShift, targetStaffID int64, date calendar.Date, start, end string) workforce.StaffShift {
	t.Helper()
	moved, err := e.tryMoveShift(t, shift, targetStaffID, date, start, end)
	require.NoError(t, err)
	return moved
}

func (e *sickCascadeEnv) tryMoveShift(t *testing.T, shift workforce.StaffShift, targetStaffID int64, date calendar.Date, start, end string) (workforce.StaffShift, error) {
	t.Helper()
	var moved workforce.PlannedShift
	var moveErr error
	e.inTx(t, func(ctx context.Context) error {
		moved, moveErr = e.Planning.MoveShift(ctx, workforce.MoveStaffShift{
			ShiftID: shift.ID, SourceStaffID: shift.StaffID, TargetStaffID: targetStaffID,
			Date: date.String(), StartTime: clock(start), EndTime: clock(end), ActorStaffID: e.admin.ID,
		})
		return nil
	})
	return moved.StaffShift, moveErr
}

// W1: created before and after the report, the shifts end up alike.
func TestSickReport_CreatedShiftMatchesShiftPlannedBefore(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	before := e.mustPlanShift(t, e.subject.ID, stampWeekTwo, "08:00", "10:00")
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)

	after := e.mustPlanShift(t, e.subject.ID, stampWeekTwo, "13:00", "15:00")
	healthy := e.mustPlanShift(t, e.subject.ID, stampWeekOne, "08:00", "10:00")

	assertSickCancelled(t, after, absenceID, "created on the sick day")
	assert.Equal(t, sickStateOf(e.reloadShift(t, before.ID)), sickStateOf(e.reloadShift(t, after.ID)), "same state as the shift planned before")
	assertPlanned(t, healthy, "created on a healthy day")

	e.deleteSickAbsence(t, absenceID)
	assertPlanned(t, e.reloadShift(t, after.ID), "deleted report")
	assert.Equal(t, sickStateOf(e.reloadShift(t, before.ID)), sickStateOf(e.reloadShift(t, after.ID)), "same state after the deletion")
}

// W1: a past day is history, as the cascade leaves it.
func TestSickReport_CreatedShiftOnPastSickDayStaysPlanned(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	yesterday := calendar.NewDate(2026, 8, 23)
	absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekOne)
	_, err := e.db.NewRaw(`UPDATE active.staff_absences SET date_start = ? WHERE id = ? AND tenant_id = ?`, yesterday, absenceID, e.tenantID).Exec(e.ctx)
	require.NoError(t, err)

	assertPlanned(t, e.mustPlanShift(t, e.subject.ID, yesterday, "08:00", "10:00"), "past sick day")
	assertSickCancelled(t, e.mustPlanShift(t, e.subject.ID, stampWeekOne, "08:00", "10:00"), absenceID, "future sick day")
}

// W1: a planner's own cancellation keeps its reason and takes no stamp.
func TestSickReport_CreatedCancelledShiftKeepsPlannersReason(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	reason := "Fortbildung"

	shift, err := e.planShift(t, e.subject.ID, stampWeekTwo, "08:00", "10:00", func(in *workforce.StaffShiftInput) {
		in.Cancelled, in.ChangeReason = true, &reason
	})
	require.NoError(t, err)

	assert.True(t, shift.Cancelled)
	assert.Nil(t, shift.SickAbsenceID, "no stamp on the planner's cancellation")
	if assert.NotNil(t, shift.ChangeReason) {
		assert.Equal(t, reason, *shift.ChangeReason)
	}
}

// W2: moving a shift to a sick person, or the sick person's shift onto a
// sick day, cancels it; a time change on the sick day keeps a presence the
// planner restored.
func TestSickReport_MovedShiftTakesSickStateOfItsNewDay(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	restored := e.mustPlanShift(t, e.subject.ID, stampWeekTwo, "13:00", "15:00")
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	e.inTx(t, func(ctx context.Context) error {
		_, err := e.Planning.ApplyCancellation(ctx, workforce.CancelStaffShift{ShiftID: restored.ID, Cancelled: false, ActorStaffID: e.admin.ID})
		return err
	})
	require.False(t, e.reloadShift(t, restored.ID).Cancelled, "planner restored the presence")

	colleagues := e.mustPlanShift(t, e.admin.ID, stampWeekTwo, "08:00", "10:00")
	ownHealthy := e.mustPlanShift(t, e.subject.ID, stampWeekOne, "16:00", "17:00")

	assertSickCancelled(t, e.moveShift(t, colleagues, e.subject.ID, stampWeekTwo, "08:00", "10:00"), absenceID, "moved to the sick person")
	assertSickCancelled(t, e.moveShift(t, ownHealthy, e.subject.ID, stampWeekTwo, "16:00", "17:00"), absenceID, "moved onto the sick day")
	assertPlanned(t, e.moveShift(t, e.reloadShift(t, restored.ID), e.subject.ID, stampWeekTwo, "13:30", "15:00"), "time change on the same day")
}

// W2: a move onto a sick day cancels the row; repeating the identical
// request (a lost response) returns the stored row instead of failing.
func TestSickReport_MoveOntoSickDayStaysIdempotent(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	own := e.mustPlanShift(t, e.subject.ID, stampWeekOne, "08:00", "10:00")
	colleagues := e.mustPlanShift(t, e.admin.ID, stampWeekTwo, "13:00", "15:00")

	for _, attempt := range []string{"first", "retry"} {
		assertSickCancelled(t, e.moveShift(t, own, e.subject.ID, stampWeekTwo, "08:00", "10:00"), absenceID, attempt+" same-person move")
		assertSickCancelled(t, e.moveShift(t, colleagues, e.subject.ID, stampWeekTwo, "13:00", "15:00"), absenceID, attempt+" cross-person move")
	}

	_, err := e.tryMoveShift(t, own, e.subject.ID, stampWeekThree, "08:00", "10:00")
	require.Error(t, err, "a cancelled shift still cannot be moved elsewhere")
	assert.True(t, errors.Is(err, workforce.ErrInvalidStaffShift), "input refusal: %v", err)
	assertSickCancelled(t, e.reloadShift(t, own.ID), absenceID, "row unchanged by the refused move")
}

// editShift rewrites one shift through the planning service, keeping its
// cancellation, reason and notes as the Dienstplan edit does.
func (e *sickCascadeEnv) editShift(t *testing.T, shift workforce.StaffShift, date calendar.Date, start, end string) workforce.StaffShift {
	t.Helper()
	return e.editShiftWith(t, shift, date, start, end, nil)
}

// editShiftWith edits with an explicit change reason when reason is set.
func (e *sickCascadeEnv) editShiftWith(t *testing.T, shift workforce.StaffShift, date calendar.Date, start, end string, reason *string) workforce.StaffShift {
	t.Helper()
	var edited workforce.PlannedShift
	e.inTx(t, func(ctx context.Context) error {
		var err error
		edited, err = e.Planning.UpdateShift(ctx, workforce.UpdateStaffShift{
			ID: shift.ID,
			StaffShiftInput: workforce.StaffShiftInput{
				StaffID: shift.StaffID, Date: date.String(), StartTime: clock(start), EndTime: clock(end),
				ChangeReason: reason, ActorStaffID: e.admin.ID,
			},
			PreserveExistingNotes: true, PreserveExistingShiftType: true, PreserveExistingChangeReason: reason == nil,
		})
		return err
	})
	return edited.StaffShift
}

// W2 for an edit: a new day brings its sick state, a healthy new day takes
// the cascade's away; a planner's cancellation and a same-day edit keep the
// row's state.
func TestSickReport_EditedShiftTakesSickStateOfItsNewDay(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	sickBefore := e.mustPlanShift(t, e.subject.ID, stampWeekTwo, "08:00", "10:00")
	sameDay := e.mustPlanShift(t, e.subject.ID, stampWeekTwo, "13:00", "14:00")
	withOwnReason := e.mustPlanShift(t, e.subject.ID, stampWeekTwo, "16:00", "17:00")
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	healthy := e.mustPlanShift(t, e.subject.ID, stampWeekOne, "16:00", "17:00")
	reason := "Fortbildung"
	plannerCancelled, err := e.planShift(t, e.subject.ID, stampWeekOne, "11:00", "12:00", func(in *workforce.StaffShiftInput) {
		in.Cancelled, in.ChangeReason = true, &reason
	})
	require.NoError(t, err)

	assertSickCancelled(t, e.editShift(t, healthy, stampWeekTwo, "16:00", "17:00"), absenceID, "edited onto the sick day")

	toHealthy := e.editShift(t, e.reloadShift(t, sickBefore.ID), stampWeekThree, "08:00", "10:00")
	assertPlanned(t, toHealthy, "edited away from the sick day")
	assert.Nil(t, toHealthy.ChangeReason, "no sick label on the healthy day")

	swap := "Tausch"
	ownReason := e.editShiftWith(t, e.reloadShift(t, withOwnReason.ID), stampWeekThree, "16:00", "17:00", &swap)
	assertPlanned(t, ownReason, "edited away with a reason of its own")
	if assert.NotNil(t, ownReason.ChangeReason, "the edit's own reason stays") {
		assert.Equal(t, swap, *ownReason.ChangeReason)
	}

	keptCancellation := e.editShift(t, plannerCancelled, stampWeekTwo, "11:00", "12:00")
	assert.True(t, keptCancellation.Cancelled, "planner's cancellation stays")
	assert.Nil(t, keptCancellation.SickAbsenceID, "planner's cancellation takes no stamp")
	if assert.NotNil(t, keptCancellation.ChangeReason) {
		assert.Equal(t, reason, *keptCancellation.ChangeReason)
	}

	sameDayEdit := e.editShift(t, e.reloadShift(t, sameDay.ID), stampWeekTwo, "13:00", "14:30")
	assert.True(t, sameDayEdit.Cancelled, "a same-day edit keeps the cancellation")
	assert.Nil(t, sameDayEdit.SickAbsenceID, "a same-day edit takes ownership, as before #6302")
}

// W3: a sick person cannot cover a gap; a healthy day can.
func TestSickReport_SickPersonCannotCoverAShift(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	cover := func(origin workforce.StaffShift) error {
		var err error
		e.inTx(t, func(ctx context.Context) error {
			_, err = e.Planning.ApplyCancellation(ctx, workforce.CancelStaffShift{
				ShiftID: origin.ID, Cancelled: true, ActorStaffID: e.admin.ID,
				Replacements: []workforce.ShiftReplacement{{StaffID: e.subject.ID, StartTime: clock("08:00"), EndTime: clock("10:00")}},
			})
			return nil
		})
		return err
	}

	err := cover(e.mustPlanShift(t, e.admin.ID, stampWeekTwo, "08:00", "10:00"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, workforce.ErrReplacementStaffAbsent), "refusal names the absent cover: %v", err)
	assert.True(t, errors.Is(err, workforce.ErrInvalidStaffShift), "refusal is an input error: %v", err)

	require.NoError(t, cover(e.mustPlanShift(t, e.admin.ID, stampWeekOne, "08:00", "10:00")), "healthy day")
}

// W4: a second shift in the window of a shift the report cancelled is refused
// like an overlap with a planned shift, so the report can still be deleted.
func TestSickReport_CreatedSickShiftCannotOverlapAnotherSickShift(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	before := e.mustPlanShift(t, e.subject.ID, stampWeekTwo, "08:00", "12:00")
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)

	_, err := e.planShift(t, e.subject.ID, stampWeekTwo, "10:00", "14:00", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, workforce.ErrStaffShiftOverlap), "overlap refusal: %v", err)
	assertSickCancelled(t, e.mustPlanShift(t, e.subject.ID, stampWeekTwo, "13:00", "15:00"), absenceID, "free window")

	e.deleteSickAbsence(t, absenceID)
	assertPlanned(t, e.reloadShift(t, before.ID), "deleted report")
}
