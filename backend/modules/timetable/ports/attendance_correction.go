package ports

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// AttendanceCorrectionRecord is one field change of a correction before the
// trail stores it.
type AttendanceCorrectionRecord struct {
	InstanceID        int64
	StudentID         int64
	ActorAccountID    *int64
	ActorNameSnapshot *string
	FieldName         string
	OldValue          *string
	NewValue          *string
	Reason            string
}

// AttendanceCorrectionTrail is the Audit Platform's append-only correction
// trail, bound at the composition root. Recording runs in the caller's
// transaction, so a failing insert rolls the correction back with it.
type AttendanceCorrectionTrail interface {
	RecordAttendanceCorrections(ctx context.Context, records []AttendanceCorrectionRecord) error
	ListAttendanceCorrections(ctx context.Context, instanceID, studentID int64) ([]timetable.AttendanceCorrectionEntry, error)
}

// CorrectionPeople resolves the person of the correcting account for the
// trail's name snapshot.
type CorrectionPeople interface {
	FindByAccountID(ctx context.Context, accountID int64) (*timetable.Person, error)
}

// AttendanceCorrectionDependencies wires the correction. Participants and
// Instances are required. Without Trail every correction fails closed and
// the trail reads empty. People snapshots the actor's name and Locks
// serializes the write with the block's completion; both are optional in
// read-only test facades.
type AttendanceCorrectionDependencies struct {
	Participants ParticipantRows
	Instances    InstanceRows
	Trail        AttendanceCorrectionTrail
	People       CorrectionPeople
	Locks        AttendanceLocks
	Logger       *slog.Logger
}
