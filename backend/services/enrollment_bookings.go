package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// NewEnrollmentBookings is the Enrollment booking projection the serving
// root's enrollment and settings routes read, over the ambient tenant
// transaction (#2743).
func NewEnrollmentBookings() repositories.EnrollmentBookingProjection {
	return repositories.NewEnrollmentBookingProjection(enrollmentCompose.New())
}

// NewCalendarPeriodUsage answers the School Calendar deletion preview's
// per-period reference counts from Enrollment's phases and the planning
// owner's tables, one statement each (#3124).
func NewCalendarPeriodUsage(planning timetable.Capability) *timetableCompose.CalendarPeriodUsageRepository {
	return timetableCompose.NewPlanningCalendarPeriodUsage(enrollmentCompose.New(), planning)
}
