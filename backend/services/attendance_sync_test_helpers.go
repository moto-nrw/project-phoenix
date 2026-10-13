package services

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// NewAttendanceSyncTestRepositories adapts one timetable capability for tests of
// the attendance sync bridge without constructing the legacy repository
// factory: the instance and participant rows.
func NewAttendanceSyncTestRepositories(db *bun.DB, capability timetable.Capability) timetableCompose.OwnerRows {
	presence := newUnloggedStudentPresence(db)
	return timetableCompose.OwnerRows{
		Instances:    timetableCompose.NewActivityInstanceRepository(db, capability, presence),
		Participants: timetableCompose.NewInstanceStudentRepository(db, capability, presence, NewPartialAbsencePreview(db)),
	}
}
