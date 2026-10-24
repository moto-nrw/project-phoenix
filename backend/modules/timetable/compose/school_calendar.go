package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The legacy School Calendar repositories the Timetable planner reads are
// served by the application over the School Calendar owner (#2666, #4084).

// NewPlanningCalendarPeriodUsage composes the School Calendar usage read from
// the two owners that hold calendar-period references: Enrollment (phases)
// and Timetable (planning tables), one statement each (#3124).
func NewPlanningCalendarPeriodUsage(enrollment application.EnrollmentPhaseQueries, planning timetable.Capability) ports.CalendarPeriodUsageRepository {
	return application.NewPlanningCalendarPeriodUsage(enrollment, planning)
}

// NewCalendarPeriodRepository serves the legacy calendar period repository
// over the School Calendar owner; the reference counts come from usage
// (#2666, #3124).
func NewCalendarPeriodRepository(calendar schoolcalendar.Capability, usage ports.CalendarPeriodUsageRepository) ports.CalendarPeriodRecords {
	return application.NewCalendarPeriodRepository(calendar, usage)
}

// CalendarRows are the retained School Calendar rows the Timetable planner
// reads (periods, closing days, date frames) and the class arrival
// exceptions, as the test compositions hand them on.
type CalendarRows struct {
	CalendarPeriod        ports.CalendarPeriodRecords
	ClosingDay            ports.ClosingDayRecords
	Dateframe             ports.DateframeRecords
	ClassArrivalException ports.ClassArrivalExceptionRecords
}
