package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// NewService binds the activity application to the owner capabilities.
func NewService(
	capability timetable.Capability,
	schedules timetable.ScheduleCapability,
	directories ports.RepositoryDirectories,
	occupancy application.ActivityOccupancy,
	staff ports.LifecycleStaff,
	students RosterStudents,
) (activityContract.ActivityService, error) {
	return application.NewActivityService(capability, schedules, directories, occupancy, staff, students)
}
