//go:build testhooks

package api

import (
	"context"
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewAttendanceSyncTestRepositories adapts one timetable capability for tests of
// the attendance sync bridge without constructing the legacy repository
// factory: the instance and participant rows.
func NewAttendanceSyncTestRepositories(db transactionruntime.Database, capability timetable.Capability) TimetableOwnerRows {
	presence := owners.NewUnloggedStudentPresence(db)
	return timetableCompose.NewNativeMirrorRows(timetableCompose.OwnerRows{
		Instances:    timetablePorts.InstanceRows{Plans: capability, Sessions: presence, Reads: owners.MustPresenceReads(db)},
		Participants: timetableCompose.NewInstanceStudentRepository(owners.MustPresenceReads(db), capability, presence, owners.NewPartialAbsencePreview(db)),
	})
}

func NewAttendanceSyncTestMirror(rows TimetableOwnerRows, logger *slog.Logger) (studentpresence.AttendanceSyncer, error) {
	return owners.NewNativeAttendanceMirror(rows, logger)
}
func NewAttendanceMirrorParticipant() *timetable.ScheduledParticipant {
	return &timetable.ScheduledParticipant{}
}
func NewAttendanceMirrorInstance() *timetable.ScheduledInstance {
	return &timetable.ScheduledInstance{Status: timetable.InstanceStatusPlanned}
}

func AfterAttendanceMirrorWrite(rows TimetableOwnerRows, after func(string, bool, error) error) TimetableOwnerRows {
	return timetableCompose.WithMirrorAfterWrite(rows, after)
}
func SeedAttendanceMirrorParticipant(check func(error), ctx context.Context, rows TimetableOwnerRows, instanceID, studentID, tenantID int64, status string) *timetable.ScheduledParticipant {
	row := &timetable.ScheduledParticipant{InstanceID: instanceID, StudentID: studentID, TenantID: tenantID, Status: status}
	check(rows.Participants.Create(ctx, row))
	return row
}
