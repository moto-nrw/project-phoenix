package repositories

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/carelifecycle"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/uptrace/bun"
)

// The care-exit repositories are constructed before the owners they read.
// Care Plan needs the instance-student repository this factory holds, and the
// School Calendar is bound later still, so both arrive as resolvers over the
// factory's own fields: the bind methods fill those fields, and the resolvers
// see the result. The alternative — a setter per owner on each repository —
// is mutable wiring the composition-surface guard records per package.

func careExitReasonsOf(factory **Factory) func() carelifecycle.CareExitReasonDirectory {
	return func() carelifecycle.CareExitReasonDirectory {
		if *factory == nil || (*factory).carePlan == nil {
			return nil
		}
		return careExitDirectory{capability: (*factory).carePlan}
	}
}

func careExitCarePlanOf(factory **Factory) func() carelifecycle.CarePlanDirectory {
	return func() carelifecycle.CarePlanDirectory {
		if *factory == nil || (*factory).carePlan == nil {
			return nil
		}
		return careExitCarePlanDirectory{capability: (*factory).carePlan}
	}
}

func careExitCalendarOf(factory **Factory) func() carelifecycle.CalendarPeriodDirectory {
	return func() carelifecycle.CalendarPeriodDirectory {
		if *factory == nil || (*factory).schoolCalendar == nil {
			return nil
		}
		return careExitCalendarPeriods{calendar: (*factory).schoolCalendar}
	}
}

func careExitBookingsOf(capability timetable.Capability) func() carelifecycle.ActivityBookingDirectory {
	return func() carelifecycle.ActivityBookingDirectory {
		if capability == nil {
			return nil
		}
		return activityBookingDirectory{capability: capability}
	}
}

// newCareExitCleanup assembles the cleanup repository for a factory that is
// still being built. bookings is the Timetable capability the same graph uses;
// enrollment and presence exist before the factory does.
func newCareExitCleanup(
	db *bun.DB,
	factory **Factory,
	enrollment carelifecycle.CareExitEnrollmentQueries,
	presence carelifecycle.CareExitPresence,
	bookings timetable.Capability,
) carelifecycle.CareExitCleanupDependencies {
	return carelifecycle.CareExitCleanupDependencies{
		Enrollment:       enrollment,
		Assignments:      careExitAssignments{capability: bookings},
		Presence:         presence,
		CarePlan:         careExitCarePlanOf(factory),
		ActivityBookings: careExitBookingsOf(bookings),
		CalendarPeriods:  careExitCalendarOf(factory),
	}
}
