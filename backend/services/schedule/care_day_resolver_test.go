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
		pickupByStudentWeekday:  map[int64]map[int]*schedule.StudentPickupSchedule{},
		hasPlan:                 map[int64]bool{},
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
		plans.hasPlan[studentID] = true
	}
	return plans
}

// The distinction this whole feature turns on: a child with no care plan at all
// and a child whose plan simply does not cover today both produce the same
// day-planning reason ("no_plan"), and must NOT be treated the same. Collapsing
// them either hides children at schools that keep no plans, or never filters
// anything at schools that do.
func TestCareDayStatusFor_NoPlanVersusNotBookedToday(t *testing.T) {
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
	const studentID int64 = 43

	t.Run("exception without a time cancels an otherwise booked day", func(t *testing.T) {
		plans := plansForStudent(studentID, schedule.WeekdayMonday)
		plans.arrivalExceptions[studentID] = map[timezone.Date]*schedule.StudentArrivalException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careDayMonday},
		}

		assert.Equal(t, CareDayNotScheduled, plans.statusFor(studentID, careDayMonday))
	})

	t.Run("exception with a time books a day the weekly plan skips", func(t *testing.T) {
		arrival := careDayClock("09:30")
		plans := plansForStudent(studentID, schedule.WeekdayMonday)
		plans.arrivalExceptions[studentID] = map[timezone.Date]*schedule.StudentArrivalException{
			careDayThursday: {StudentID: studentID, ExceptionDate: careDayThursday, ExpectedArrival: &arrival},
		}

		assert.Equal(t, CareDayScheduled, plans.statusFor(studentID, careDayThursday))
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
	const studentID int64 = 44
	pickup := careDayClock("16:00")

	plans := plansForStudent(studentID)
	plans.pickupByStudentWeekday[studentID] = map[int]*schedule.StudentPickupSchedule{
		schedule.WeekdayMonday: {StudentID: studentID, Weekday: schedule.WeekdayMonday, PickupTime: pickup},
	}
	plans.hasPlan[studentID] = true

	assert.Equal(t, CareDayScheduled, plans.statusFor(studentID, careDayMonday))
	assert.Equal(t, CareDayNotScheduled, plans.statusFor(studentID, careDayThursday))
}

// Children never resolved (walk-ins, unwired service) must fall through as
// expected rather than silently vanishing from a roster.
func TestCareDayStatusExpected_DefaultsToVisible(t *testing.T) {
	assert.True(t, CareDayUnknown.Expected())
	assert.True(t, CareDayScheduled.Expected())
	assert.True(t, CareDayStatus("").Expected())
	assert.False(t, CareDayNotScheduled.Expected())
}
