package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedCareParticipation map[int64]bool

func (f fixedCareParticipation) ParticipatingStudentIDsByDate(
	_ context.Context, _ []int64, from, to timezone.Date,
) (map[timezone.Date]map[int64]bool, error) {
	result := make(map[timezone.Date]map[int64]bool)
	for day := from; !day.After(to); day = day.AddDays(1) {
		result[day] = f
	}
	return result, nil
}

type countingCareParticipation struct {
	calls int
}

func (f *countingCareParticipation) ParticipatingStudentIDsByDate(
	_ context.Context, studentIDs []int64, from, to timezone.Date,
) (map[timezone.Date]map[int64]bool, error) {
	f.calls++
	result := make(map[timezone.Date]map[int64]bool)
	for day := from; !day.After(to); day = day.AddDays(1) {
		result[day] = map[int64]bool{studentIDs[0]: true}
	}
	return result, nil
}

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
		arrivalByStudentWeekday: map[int64]map[int]*careplan.ArrivalSchedule{},
		arrivalByStudentDate:    map[int64]map[timezone.Date]*careplan.ArrivalSchedule{},
		pickupByStudentDate:     map[int64]map[timezone.Date]*careplan.PickupSchedule{},
		hasPlan:                 map[int64]map[timezone.Date]bool{},
		arrivalExceptions:       map[int64]map[timezone.Date]*careplan.ArrivalException{},
		pickupExceptions:        map[int64]map[timezone.Date]*careplan.PickupException{},
	}
	if len(arrivalWeekdays) > 0 {
		byWeekday := map[int]*careplan.ArrivalSchedule{}
		for _, weekday := range arrivalWeekdays {
			byWeekday[weekday] = &careplan.ArrivalSchedule{
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

		assert.Equal(t, careplan.CareDayUnknown, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)))
		assert.True(t, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)).Expected(),
			"a child without a care plan must keep counting as expected")
	})

	t.Run("plan covers another weekday only", func(t *testing.T) {
		plans := plansForStudent(studentID, 4)

		assert.Equal(t, careplan.CareDayNotScheduled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)))
		assert.False(t, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)).Expected())
	})

	t.Run("plan covers this weekday", func(t *testing.T) {
		plans := plansForStudent(studentID, 1, 4)

		assert.Equal(t, careplan.CareDayScheduled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)))
		assert.Equal(t, careplan.CareDayScheduled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayThursday)))
	})
}

func TestCareDayStatusFor_Exceptions(t *testing.T) {
	t.Parallel()

	const studentID int64 = 43

	// A cancellation is NOT a non-booking: the child is not expected, but the
	// day was booked and has to end as a recorded absence (#1747 review).
	t.Run("exception without a time cancels an otherwise booked day", func(t *testing.T) {
		plans := plansForStudent(studentID, 1)
		plans.arrivalExceptions[studentID] = map[timezone.Date]*careplan.ArrivalException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careplan.Date(careDayMonday)},
		}

		status := careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday))
		assert.Equal(t, careplan.CareDayCancelled, status)
		assert.False(t, status.Expected())
		assert.False(t, status.ExemptFromAbsence(),
			"a cancelled day must still be stamped absent, or it vanishes from the history")
	})

	t.Run("exception with a time books a day the weekly plan skips", func(t *testing.T) {
		arrival := careDayClock("09:30")
		plans := plansForStudent(studentID, 1)
		plans.arrivalExceptions[studentID] = map[timezone.Date]*careplan.ArrivalException{
			careDayThursday: {StudentID: studentID, ExceptionDate: careplan.Date(careDayThursday), ExpectedArrival: &arrival},
		}

		assert.Equal(t, careplan.CareDayScheduled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayThursday)))
	})

	// One-leg cancellations: staff often mark only the leg they are looking
	// at as "Kommt heute nicht". The other leg's regular schedule time must
	// not resurrect the day (#1725 contract in mergeCareExceptions).
	t.Run("timeless arrival exception cancels the day despite a pickup schedule", func(t *testing.T) {
		plans := plansForStudent(studentID, 1)
		plans.pickupByStudentDate[studentID] = map[timezone.Date]*careplan.PickupSchedule{
			careDayMonday: {StudentID: studentID, Weekday: 1, PickupTime: careDayClock("16:00")},
		}
		plans.arrivalExceptions[studentID] = map[timezone.Date]*careplan.ArrivalException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careplan.Date(careDayMonday)},
		}

		assert.Equal(t, careplan.CareDayCancelled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)))
	})

	t.Run("timeless pickup exception cancels the day despite an arrival schedule", func(t *testing.T) {
		plans := plansForStudent(studentID, 1)
		plans.pickupExceptions[studentID] = map[timezone.Date]*careplan.PickupException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careplan.Date(careDayMonday)},
		}

		assert.Equal(t, careplan.CareDayCancelled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)))
	})

	// The parent portal resolves arrival_absent || pickup_absent to an absence
	// before it looks at any time, so a timed exception on the other leg must
	// not book the day here either — the guardian tile and the staff-side
	// counts would disagree about the same child (#1747 review).
	t.Run("a timed exception on the other leg does not beat the cancellation", func(t *testing.T) {
		pickup := careDayClock("15:00")
		plans := plansForStudent(studentID, 1)
		plans.arrivalExceptions[studentID] = map[timezone.Date]*careplan.ArrivalException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careplan.Date(careDayMonday)},
		}
		plans.pickupExceptions[studentID] = map[timezone.Date]*careplan.PickupException{
			careDayMonday: {StudentID: studentID, ExceptionDate: careplan.Date(careDayMonday), PickupTime: &pickup},
		}

		assert.Equal(t, careplan.CareDayCancelled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)),
			"the timeless leg is the 'kommt heute nicht' marker and wins")
	})

	t.Run("exception outranks the weekly plan on a weekend", func(t *testing.T) {
		arrival := careDayClock("09:30")
		plans := plansForStudent(studentID, 1)
		assert.Equal(t, careplan.CareDayNotScheduled, careplan.ResolveCareDay(plans.factsFor(studentID, careDaySaturday)),
			"weekends carry no weekly rows")

		plans.arrivalExceptions[studentID] = map[timezone.Date]*careplan.ArrivalException{
			careDaySaturday: {StudentID: studentID, ExceptionDate: careplan.Date(careDaySaturday), ExpectedArrival: &arrival},
		}
		assert.Equal(t, careplan.CareDayScheduled, careplan.ResolveCareDay(plans.factsFor(studentID, careDaySaturday)))
	})
}

// A pickup-only plan is still a plan: schools that record only the going-home
// side must not have every child collapse to "not scheduled".
func TestCareDayStatusFor_PickupOnlyPlan(t *testing.T) {
	t.Parallel()

	const studentID int64 = 44
	pickup := careDayClock("16:00")

	plans := plansForStudent(studentID)
	plans.pickupByStudentDate[studentID] = map[timezone.Date]*careplan.PickupSchedule{
		careDayMonday: {StudentID: studentID, Weekday: 1, PickupTime: pickup},
	}
	plans.hasPlan[studentID] = map[timezone.Date]bool{
		careDayMonday: true, careDayThursday: true, careDaySaturday: true,
	}

	assert.Equal(t, careplan.CareDayScheduled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)))
	assert.Equal(t, careplan.CareDayNotScheduled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayThursday)))
}

// In booking mode a pickup time describes a booked day; it must never create
// the care day itself. Otherwise a stale/manual pickup row can bypass the
// tenant's authoritative booking boundary and put an unbooked child back into
// expected counts.
func TestCareDayStatusFor_AuthoritativeBookingOutranksPickupPlan(t *testing.T) {
	t.Parallel()

	const studentID int64 = 46
	plans := plansForStudent(studentID)
	plans.bookingsAuthoritative = true
	plans.pickupByStudentDate[studentID] = map[timezone.Date]*careplan.PickupSchedule{
		careDayMonday: {StudentID: studentID, Weekday: 1, PickupTime: careDayClock("16:00")},
	}
	plans.hasPlan[studentID] = map[timezone.Date]bool{careDayMonday: true}

	assert.Equal(t, careplan.CareDayNotScheduled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)),
		"a pickup row must not add a care day when bookings are authoritative")

	// The arrival projection carries a placeholder row even when neither the
	// child nor the class has an arrival time. That row is the positive booking
	// signal; pickup details may enrich the day once it exists.
	plans.arrivalByStudentDate[studentID] = map[timezone.Date]*careplan.ArrivalSchedule{
		careDayMonday: {StudentID: studentID, Weekday: 1},
	}
	assert.Equal(t, careplan.CareDayScheduled, careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday)))
}

type careDayQueryFactsFixture struct {
	CareParticipationResolver
	facts map[int64]map[timezone.Date]careplan.CareDayFacts
}

func (f careDayQueryFactsFixture) LoadCareDayFacts(context.Context, []int64, timezone.Date, timezone.Date) (map[int64]map[timezone.Date]careplan.CareDayFacts, error) {
	return f.facts, nil
}

func TestCareParticipationBoundaryOverridesAStaleCarePlan(t *testing.T) {
	t.Parallel()
	const participatingID int64 = 46
	const withdrawnID int64 = 47
	var query careplan.CareDayQuery = NewCareDayQueries(careDayQueryFactsFixture{
		CareParticipationResolver: fixedCareParticipation{participatingID: true},
		facts: map[int64]map[timezone.Date]careplan.CareDayFacts{
			participatingID: {careDayMonday: {HasPlan: true, HasArrivalSchedule: true}},
			withdrawnID:     {careDayMonday: {HasPlan: true, HasArrivalSchedule: true}},
		},
	})
	out, err := query.ResolveForRange(context.Background(), []int64{participatingID, withdrawnID}, careDayMonday, careDayMonday)
	require.NoError(t, err)
	assert.Equal(t, careplan.CareDayScheduled, out[participatingID][careDayMonday])
	assert.Equal(t, careplan.CareDayNotScheduled, out[withdrawnID][careDayMonday])
}

func TestCareParticipationRangeLoadsBoundariesOnce(t *testing.T) {
	t.Parallel()
	resolver := &countingCareParticipation{}
	var query careplan.CareDayQuery = NewCareDayQueries(careDayQueryFactsFixture{
		CareParticipationResolver: resolver,
	})
	from := timezone.NewDate(2026, time.August, 24)
	out, err := query.ResolveForRange(context.Background(), []int64{41}, from, from.AddDays(20))
	require.NoError(t, err)
	require.Len(t, out[41], 21)
	assert.Equal(t, 1, resolver.calls)
}

func TestCareDayStatusFor_ArrivalPlanWithoutTime(t *testing.T) {
	t.Parallel()

	const studentID int64 = 45
	plans := plansForStudent(studentID)
	plans.arrivalByStudentWeekday[studentID] = map[int]*careplan.ArrivalSchedule{
		1: {StudentID: studentID, Weekday: 1},
	}
	plans.hasPlan[studentID] = map[timezone.Date]bool{careDayMonday: true}

	status := careplan.ResolveCareDay(plans.factsFor(studentID, careDayMonday))
	assert.Equal(t, careplan.CareDayScheduled, status)
	assert.True(t, status.Expected(), "a booked care day stays expected when its class time is unknown")
}

// Children never resolved (walk-ins, unwired service) must fall through as
// expected rather than silently vanishing from a roster.
func TestCareDayStatusExpected_DefaultsToVisible(t *testing.T) {
	t.Parallel()

	assert.True(t, careplan.CareDayUnknown.Expected())
	assert.True(t, careplan.CareDayScheduled.Expected())
	assert.True(t, careplan.CareDayStatus("").Expected())
	assert.False(t, careplan.CareDayNotScheduled.Expected())
	assert.False(t, careplan.CareDayCancelled.Expected())
}

// Only a genuine non-booking may skip the expected → absent stamp. Exempting a
// cancellation would drop the row from the attendance history and the exports,
// because the completed-instance filter hides surviving 'expected' rows.
func TestCareDayStatusExemptFromAbsence_OnlyNonBookings(t *testing.T) {
	t.Parallel()

	assert.True(t, careplan.CareDayNotScheduled.ExemptFromAbsence())
	assert.False(t, careplan.CareDayCancelled.ExemptFromAbsence())
	assert.False(t, careplan.CareDayScheduled.ExemptFromAbsence())
	assert.False(t, careplan.CareDayUnknown.ExemptFromAbsence())
	assert.False(t, careplan.CareDayStatus("").ExemptFromAbsence())
}
