package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// Monday 2026-04-20 / Thursday 2026-04-23 — fixed dates, so the matrix below
// never depends on when the suite runs.
var (
	careDayMonday   = timezone.NewDate(2026, 4, 20)
	careDayThursday = timezone.NewDate(2026, 4, 23)
	careDaySaturday = timezone.NewDate(2026, 4, 25)
)

func careDayClock(hhmm string) time.Time {
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		panic(err)
	}
	return parsed
}

// plansForStudent builds the in-memory projection ResolveForRange produces,
// for one student, from the weekdays their care plan covers.
func plansForStudent(studentID int64, arrivalWeekdays ...int) *carePlans {
	plans := &carePlans{
		arrivalByStudentWeekday: map[int64]map[int]*schedule.StudentArrivalSchedule{},
		pickupByStudentDate:     map[int64]map[timezone.Date]*schedule.StudentPickupSchedule{},
		hasPlan:                 map[int64]map[timezone.Date]bool{},
		arrivalExceptions:       map[int64]map[timezone.Date]*schedule.StudentArrivalException{},
		pickupExceptions:        map[int64]map[timezone.Date]*schedule.StudentPickupException{},
	}
	if len(arrivalWeekdays) > 0 {
		byWeekday := map[int]*schedule.StudentArrivalSchedule{}
		for _, weekday := range arrivalWeekdays {
			byWeekday[weekday] = &schedule.StudentArrivalSchedule{
				StudentID:       studentID,
				Weekday:         weekday,
				ExpectedArrival: careDayClock("08:00"),
			}
		}
		plans.arrivalByStudentWeekday[studentID] = byWeekday
		plans.hasPlan[studentID] = map[timezone.Date]bool{
			careDayMonday: true, careDayThursday: true, careDaySaturday: true,
		}
	}
	return plans
}

// The distinction this whole feature turns on: a child with no care plan at all
// and a child whose plan simply does not cover today both produce the same
// day-planning reason ("no_plan"), and must NOT be treated the same. Collapsing
// them either hides children at schools that keep no plans, or never filters
// anything at schools that do.
func TestCareDayStatusFor_NoPlanVersusNotBookedToday(t *testing.T) {
	t.Parallel()

	const studentID int64 = 42

	t.Run("no care plan at all stays unknown", func(t *testing.T) {
		plans := plansForStudent(studentID)

		assert.Equal(t, CareDayUnknown, plans.statusFor(studentID, careDayMonday))
		assert.True(t, plans.statusFor(studentID, careDayMonday).Expected(),
			"a child without a care plan must keep counting as expected")
	})

	t.Run("plan covers another weekday only", func(t *testing.T) {
		plans := plansForStudent(studentID, schedule.WeekdayThursday)

		assert.Equal(t, CareDayNotScheduled, plans.statusFor(studentID, careDayMonday))
		assert.False(t, plans.statusFor(studentID, careDayMonday).Expected())
	})

	t.Run("plan covers this weekday", func(t *testing.T) {
		plans := plansForStudent(studentID, schedule.WeekdayMonday, schedule.WeekdayThursday)

		assert.Equal(t, CareDayScheduled, plans.statusFor(studentID, careDayMonday))
		assert.Equal(t, CareDayScheduled, plans.statusFor(studentID, careDayThursday))
	})
}

func TestCareDayStatusFor_Exceptions(t *testing.T) {
	t.Parallel()

	const studentID int64 = 43

	// A cancellation is NOT a non-booking: the child is not expected, but the
	// day was booked and has to end as a recorded absence (#1747 review).
	t.Run("exception without a time cancels an otherwise booked day", func(t *testing.T) {
		plans := plansForStudent(studentID, schedule.WeekdayMonday)
		plans.arrivalExceptions[studentID] = map[timezone.Date]*schedule.StudentArrivalException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careDayMonday},
		}

		status := plans.statusFor(studentID, careDayMonday)
		assert.Equal(t, CareDayCancelled, status)
		assert.False(t, status.Expected())
		assert.False(t, status.ExemptFromAbsence(),
			"a cancelled day must still be stamped absent, or it vanishes from the history")
	})

	t.Run("exception with a time books a day the weekly plan skips", func(t *testing.T) {
		arrival := careDayClock("09:30")
		plans := plansForStudent(studentID, schedule.WeekdayMonday)
		plans.arrivalExceptions[studentID] = map[timezone.Date]*schedule.StudentArrivalException{
			careDayThursday: {StudentID: studentID, ExceptionDate: careDayThursday, ExpectedArrival: &arrival},
		}

		assert.Equal(t, CareDayScheduled, plans.statusFor(studentID, careDayThursday))
	})

	// One-leg cancellations: staff often mark only the leg they are looking
	// at as "Kommt heute nicht". The other leg's regular schedule time must
	// not resurrect the day (#1725 contract in mergeCareExceptions).
	t.Run("timeless arrival exception cancels the day despite a pickup schedule", func(t *testing.T) {
		plans := plansForStudent(studentID, schedule.WeekdayMonday)
		plans.pickupByStudentDate[studentID] = map[timezone.Date]*schedule.StudentPickupSchedule{
			careDayMonday: {StudentID: studentID, Weekday: schedule.WeekdayMonday, PickupTime: careDayClock("16:00")},
		}
		plans.arrivalExceptions[studentID] = map[timezone.Date]*schedule.StudentArrivalException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careDayMonday},
		}

		assert.Equal(t, CareDayCancelled, plans.statusFor(studentID, careDayMonday))
	})

	t.Run("timeless pickup exception cancels the day despite an arrival schedule", func(t *testing.T) {
		plans := plansForStudent(studentID, schedule.WeekdayMonday)
		plans.pickupExceptions[studentID] = map[timezone.Date]*schedule.StudentPickupException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careDayMonday},
		}

		assert.Equal(t, CareDayCancelled, plans.statusFor(studentID, careDayMonday))
	})

	// The parent portal resolves arrival_absent || pickup_absent to an absence
	// before it looks at any time, so a timed exception on the other leg must
	// not book the day here either — the guardian tile and the staff-side
	// counts would disagree about the same child (#1747 review).
	t.Run("a timed exception on the other leg does not beat the cancellation", func(t *testing.T) {
		pickup := careDayClock("15:00")
		plans := plansForStudent(studentID, schedule.WeekdayMonday)
		plans.arrivalExceptions[studentID] = map[timezone.Date]*schedule.StudentArrivalException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careDayMonday},
		}
		plans.pickupExceptions[studentID] = map[timezone.Date]*schedule.StudentPickupException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careDayMonday, PickupTime: &pickup},
		}

		assert.Equal(t, CareDayCancelled, plans.statusFor(studentID, careDayMonday),
			"the timeless leg is the 'kommt heute nicht' marker and wins")
	})

	t.Run("exception outranks the weekly plan on a weekend", func(t *testing.T) {
		arrival := careDayClock("09:30")
		plans := plansForStudent(studentID, schedule.WeekdayMonday)
		assert.Equal(t, CareDayNotScheduled, plans.statusFor(studentID, careDaySaturday),
			"weekends carry no weekly rows")

		plans.arrivalExceptions[studentID] = map[timezone.Date]*schedule.StudentArrivalException{
			careDaySaturday: {StudentID: studentID, ExceptionDate: careDaySaturday, ExpectedArrival: &arrival},
		}
		assert.Equal(t, CareDayScheduled, plans.statusFor(studentID, careDaySaturday))
	})
}

// A pickup-only plan is still a plan: schools that record only the going-home
// side must not have every child collapse to "not scheduled".
func TestCareDayStatusFor_PickupOnlyPlan(t *testing.T) {
	t.Parallel()

	const studentID int64 = 44
	pickup := careDayClock("16:00")

	plans := plansForStudent(studentID)
	plans.pickupByStudentDate[studentID] = map[timezone.Date]*schedule.StudentPickupSchedule{
		careDayMonday: {StudentID: studentID, Weekday: schedule.WeekdayMonday, PickupTime: pickup},
	}
	plans.hasPlan[studentID] = map[timezone.Date]bool{
		careDayMonday: true, careDayThursday: true, careDaySaturday: true,
	}

	assert.Equal(t, CareDayScheduled, plans.statusFor(studentID, careDayMonday))
	assert.Equal(t, CareDayNotScheduled, plans.statusFor(studentID, careDayThursday))
}

func TestCareDayStatusFor_ArrivalPlanWithoutTime(t *testing.T) {
	t.Parallel()

	const studentID int64 = 45
	plans := plansForStudent(studentID)
	plans.arrivalByStudentWeekday[studentID] = map[int]*schedule.StudentArrivalSchedule{
		schedule.WeekdayMonday: {StudentID: studentID, Weekday: schedule.WeekdayMonday},
	}
	plans.hasPlan[studentID] = map[timezone.Date]bool{careDayMonday: true}

	status := plans.statusFor(studentID, careDayMonday)
	assert.Equal(t, CareDayScheduled, status)
	assert.True(t, status.Expected(), "a booked care day stays expected when its class time is unknown")
}

// Children never resolved (walk-ins, unwired service) must fall through as
// expected rather than silently vanishing from a roster.
func TestCareDayStatusExpected_DefaultsToVisible(t *testing.T) {
	t.Parallel()

	assert.True(t, CareDayUnknown.Expected())
	assert.True(t, CareDayScheduled.Expected())
	assert.True(t, CareDayStatus("").Expected())
	assert.False(t, CareDayNotScheduled.Expected())
	assert.False(t, CareDayCancelled.Expected())
}

// Only a genuine non-booking may skip the expected → absent stamp. Exempting a
// cancellation would drop the row from the attendance history and the exports,
// because the completed-instance filter hides surviving 'expected' rows.
func TestCareDayStatusExemptFromAbsence_OnlyNonBookings(t *testing.T) {
	t.Parallel()

	assert.True(t, CareDayNotScheduled.ExemptFromAbsence())
	assert.False(t, CareDayCancelled.ExemptFromAbsence())
	assert.False(t, CareDayScheduled.ExemptFromAbsence())
	assert.False(t, CareDayUnknown.ExemptFromAbsence())
	assert.False(t, CareDayStatus("").ExemptFromAbsence())
}

// Staff setting an unbooked slot back to 'expected' means "the plan is wrong,
// this child is coming". The row then looks exactly like the automatic state,
// so the decision is recognized by its own stamp — and it outranks the
// derivation both while the block runs and once it is over (#1747 review).
func TestAttendanceRowCareDay_ManualDecisionOutranksThePlan(t *testing.T) {
	t.Parallel()

	stamp := time.Date(2026, 4, 20, 13, 0, 0, 0, time.UTC)

	manual := &schedule.InstanceStudent{
		Status:         schedule.AttendanceStatusExpected,
		ManualStatusAt: &stamp,
	}
	assert.Equal(t, CareDayUnknown, AttendanceRowCareDay(false, manual, CareDayNotScheduled),
		"a running block must show the hand-set expectation, not the plan it overrides")

	// The same row after completion: it was never stamped as a non-booking, so
	// it stays an ordinary expectation in the history.
	marked := &schedule.InstanceStudent{
		Status:         schedule.AttendanceStatusExpected,
		NotScheduled:   true,
		ManualStatusAt: &stamp,
	}
	assert.Equal(t, CareDayUnknown, AttendanceRowCareDay(true, marked, CareDayNotScheduled),
		"a hand-decided row is never read back as a non-booking")

	// Without the stamp nothing changes for the automatic rows.
	auto := &schedule.InstanceStudent{Status: schedule.AttendanceStatusExpected}
	assert.Equal(t, CareDayNotScheduled, AttendanceRowCareDay(false, auto, CareDayNotScheduled))
}

// A partial-day excusal rewrites unbooked slots to absent with
// pickup_exception_id provenance. While the instance is still active the
// planner must keep counting those children as non-scheduled — the same
// treatment a broad day status gets via student_status_day_id.
func TestAttendanceRowCareDay_PartialExcusalProvenanceIsNotScheduled(t *testing.T) {
	t.Parallel()

	exceptionID := int64(90)
	statusDayID := int64(30)
	stamp := time.Date(2026, 4, 20, 13, 0, 0, 0, time.UTC)

	partial := &schedule.InstanceStudent{
		Status:            schedule.AttendanceStatusAbsent,
		PickupExceptionID: &exceptionID,
	}
	assert.Equal(t, CareDayNotScheduled, AttendanceRowCareDay(false, partial, CareDayNotScheduled),
		"active partial-excused absence on a not-scheduled plan must stay a non-booking")
	assert.Equal(t, CareDayUnknown, AttendanceRowCareDay(false, partial, CareDayScheduled),
		"a partial-excused absence on a day that was booked is a real absence")

	// Manual decisions still outrank provenance of either kind.
	manualPartial := &schedule.InstanceStudent{
		Status:            schedule.AttendanceStatusAbsent,
		PickupExceptionID: &exceptionID,
		ManualStatusAt:    &stamp,
	}
	assert.Equal(t, CareDayUnknown, AttendanceRowCareDay(false, manualPartial, CareDayNotScheduled),
		"a hand-set absence is never relabelled as non-booking")

	// Broad day-status provenance keeps its existing behaviour.
	dayStatus := &schedule.InstanceStudent{
		Status:             schedule.AttendanceStatusAbsent,
		StudentStatusDayID: &statusDayID,
	}
	assert.Equal(t, CareDayNotScheduled, AttendanceRowCareDay(false, dayStatus, CareDayNotScheduled))
}
