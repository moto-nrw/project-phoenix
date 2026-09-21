package careplan_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
)

// Staff setting an unbooked slot back to 'expected' means "the plan is wrong,
// this child is coming". The row then looks exactly like the automatic state,
// so the decision is recognized by its own stamp — and it outranks the
// derivation both while the block runs and once it is over (#1747 review).
func TestAttendanceRowCareDay_ManualDecisionOutranksThePlan(t *testing.T) {
	t.Parallel()

	manual := &careplan.CareDayAttendance{
		Expected:        true,
		ManuallyDecided: true,
	}
	assert.Equal(t, careplan.CareDayUnknown, careplan.AttendanceRowCareDay(false, manual, careplan.CareDayNotScheduled),
		"a running block must show the hand-set expectation, not the plan it overrides")

	// The same row after completion: it was never stamped as a non-booking, so
	// it stays an ordinary expectation in the history.
	marked := &careplan.CareDayAttendance{
		Expected:        true,
		NotScheduled:    true,
		ManuallyDecided: true,
	}
	assert.Equal(t, careplan.CareDayUnknown, careplan.AttendanceRowCareDay(true, marked, careplan.CareDayNotScheduled),
		"a hand-decided row is never read back as a non-booking")

	// Without the stamp nothing changes for the automatic rows.
	auto := &careplan.CareDayAttendance{Expected: true}
	assert.Equal(t, careplan.CareDayNotScheduled, careplan.AttendanceRowCareDay(false, auto, careplan.CareDayNotScheduled))
}

// A partial-day excusal rewrites unbooked slots to absent with
// pickup_exception_id provenance. While the instance is still active the
// planner must keep counting those children as non-scheduled — the same
// treatment a broad day status gets via student_status_day_id.
func TestAttendanceRowCareDay_PartialExcusalProvenanceIsNotScheduled(t *testing.T) {
	t.Parallel()

	partial := &careplan.CareDayAttendance{
		Expected:         false,
		PlanOwnedAbsence: true,
	}
	assert.Equal(t, careplan.CareDayNotScheduled, careplan.AttendanceRowCareDay(false, partial, careplan.CareDayNotScheduled),
		"active partial-excused absence on a not-scheduled plan must stay a non-booking")
	assert.Equal(t, careplan.CareDayUnknown, careplan.AttendanceRowCareDay(false, partial, careplan.CareDayScheduled),
		"a partial-excused absence on a day that was booked is a real absence")

	// Manual decisions still outrank provenance of either kind.
	manualPartial := &careplan.CareDayAttendance{
		Expected:         false,
		PlanOwnedAbsence: true,
		ManuallyDecided:  true,
	}
	assert.Equal(t, careplan.CareDayUnknown, careplan.AttendanceRowCareDay(false, manualPartial, careplan.CareDayNotScheduled),
		"a hand-set absence is never relabelled as non-booking")

	// Broad day-status provenance keeps its existing behaviour.
	dayStatus := &careplan.CareDayAttendance{
		Expected:         false,
		PlanOwnedAbsence: true,
	}
	assert.Equal(t, careplan.CareDayNotScheduled, careplan.AttendanceRowCareDay(false, dayStatus, careplan.CareDayNotScheduled))
}
