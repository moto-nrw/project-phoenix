package api

import (
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// newCalendarPeriodUsage answers the School Calendar deletion preview's
// per-period reference counts from Enrollment's phases and the planning
// owner's tables, one statement each (#3124).
func newCalendarPeriodUsage(planning timetable.Capability) timetablePorts.CalendarPeriodUsageRepository {
	return timetableCompose.NewPlanningCalendarPeriodUsage(enrollmentCompose.New(), planning)
}
