//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewAttendanceSyncTestRepositories adapts one timetable capability for tests of
// the attendance sync bridge without constructing the legacy repository
// factory: the instance and participant rows.
func NewAttendanceSyncTestRepositories(db transactionruntime.Database, capability timetable.Capability) timetableCompose.OwnerRows {
	presence := NewUnloggedStudentPresence(db)
	return timetableCompose.OwnerRows{
		Instances:    timetableCompose.NewActivityInstanceRepository(db.Pool(), capability, presence),
		Participants: timetableCompose.NewInstanceStudentRepository(db.Pool(), capability, presence, NewPartialAbsencePreview(db)),
	}
}
