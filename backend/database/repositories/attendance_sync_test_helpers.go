package repositories

import (
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/uptrace/bun"
)

// NewAttendanceSyncTestRepositories adapts one timetable capability for tests of
// the attendance sync bridge without constructing the legacy repository factory.
func NewAttendanceSyncTestRepositories(db *bun.DB, capability timetable.Capability) (scheduleModels.ActivityInstanceRepository, scheduleModels.InstanceStudentRepository) {
	presence := newStudentPresence(db)
	return newTimetableActivityInstanceRepository(db, capability, presence), newTimetableInstanceStudentRepository(db, capability, presence)
}
