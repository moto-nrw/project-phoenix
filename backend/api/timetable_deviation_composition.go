package api

import (
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The Timetable owner's deviation writes, attendance corrections and
// attendance mirror (#3424 slice S3), bound to the collaborators the owner
// may not name: the Audit Platform's protocol and correction trail
// (TimetableDeviationProtocol and
// TimetableAttendanceCorrectionTrail), the realtime hub and
// Student Presence's attendance syncer port.

// NewTimetableAttendanceCorrections composes the attendance correction over
// the retained rows (Participants, Instances and the optional completion
// Locks) with the Audit Platform's correction trail. trail is optional:
// without it every correction fails closed.
func NewTimetableAttendanceCorrections(rows timetableCompose.OwnerRows, trail attendanceCorrectionLedger, people timetablePorts.CorrectionPeople, logger *slog.Logger) (timetable.AttendanceCorrections, error) {
	return timetableCompose.NewAttendanceCorrections(timetablePorts.AttendanceCorrectionDependencies{
		Participants: rows.Participants,
		Instances:    rows.Instances,
		Trail:        TimetableAttendanceCorrectionTrail(trail),
		People:       people,
		Locks:        rows.Locks,
		Logger:       logger,
	})
}

var _ studentpresence.AttendanceSyncer = owners.PresenceAttendanceMirror{}
