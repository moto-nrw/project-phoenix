package repositories

import (
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// NewAttendanceSyncTestRepositories adapts one timetable capability for tests of
// the attendance sync bridge without constructing the legacy repository factory.
func NewAttendanceSyncTestRepositories(capability timetable.Capability) (scheduleModels.ActivityInstanceRepository, scheduleModels.InstanceStudentRepository) {
	return timetableActivityInstanceRepository{timetable: capability}, timetableInstanceStudentRepository{timetable: capability}
}
