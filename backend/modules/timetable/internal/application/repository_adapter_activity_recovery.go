package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// AssignmentRecovery is the Timetable side of a reopen: the planned
// participants of the block are locked so a concurrent roster edit cannot
// hide from the unchanged-row check.
type AssignmentRecovery interface {
	LockAttendance(context.Context, int64) error
}

// PresenceRecovery is the Student Presence capability the recovery needs:
// the live group, its supervisors, its visits, the activity session and the
// session attendance are all owned there (#2762).
type PresenceRecovery interface {
	LockOpenVisits(context.Context, int64) error
	RestoreVisits(context.Context, []int64) error
	LockOpenSupervisors(context.Context, int64) error
	LockSupervisors(context.Context, []int64) error
	RestoreGroup(context.Context, int64, time.Time) error
	RestoreSupervisors(context.Context, []int64) error
	RestoreAttendance(context.Context, []scheduleModels.CompletionAttendanceSnapshot) error
	ReopenSession(context.Context, int64, int64) error
}

// RetainedActivityRecovery returns the completion and reopen support of a
// block from its two owners: Timetable locks the participants, Student
// Presence locks and restores the live group, the visits, the supervisors,
// the attendance and the session (#2762).
func RetainedActivityRecovery(assignments ports.ParticipantRows) (AssignmentRecovery, PresenceRecovery) {
	if assignments.Plans == nil {
		panic("activity recovery: timetable assignment adapter is required")
	}
	// The recovery locks and restores through the same Student Presence
	// module the participants compose.
	presence, ok := assignments.Presence.(studentpresence.Capability)
	if !ok {
		panic("activity recovery: the assignment adapter must compose the Student Presence module")
	}
	return recoveryAssignments{capability: assignments.Plans, presence: presence}, recoveryPresence{Capability: presence}
}

// recoveryAssignments locks the attendance of one block in both owners so a
// completion, a cancellation and a reopen serialize with attendance edits.
type recoveryAssignments struct {
	capability timetable.InstanceStudentCapability
	presence   interface {
		LockSessionAttendance(context.Context, []int64) error
	}
}

func (r recoveryAssignments) LockAttendance(ctx context.Context, instanceID int64) error {
	if err := r.capability.LockInstanceStudentAssignments(ctx, instanceID); err != nil {
		return err
	}
	participants, err := r.capability.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instanceID}})
	if err != nil {
		return err
	}
	if len(participants) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		ids = append(ids, participant.ID)
	}
	return r.presence.LockSessionAttendance(ctx, ids)
}

// recoveryPresence serves the recovery's Student Presence port from the
// owner capability.
type recoveryPresence struct{ studentpresence.Capability }

// RestoreAttendance writes the completion snapshot back. The snapshot never
// carried the walk-in and manual markers, so the rows keep the ones they
// have now.
func (p recoveryPresence) RestoreAttendance(ctx context.Context, rows []scheduleModels.CompletionAttendanceSnapshot) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.RowID)
	}
	current, err := p.ListSessionAttendance(ctx, ids)
	if err != nil {
		return err
	}
	markers := make(map[int64]studentpresence.SessionAttendance, len(current))
	for _, row := range current {
		markers[row.ParticipantID] = row
	}
	values := make([]studentpresence.SessionAttendanceRestore, 0, len(rows))
	for _, row := range rows {
		marker := markers[row.RowID]
		values = append(values, studentpresence.SessionAttendanceRestore{
			ParticipantID: row.RowID, Status: row.Status, Substatus: row.Substatus, Note: row.Note,
			CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt, IsUnplanned: marker.IsUnplanned,
			NotScheduled: row.NotScheduled, ManualStatusAt: marker.ManualStatusAt,
			StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
		})
	}
	return p.RestoreSessionAttendance(ctx, values)
}

func (p recoveryPresence) ReopenSession(ctx context.Context, instanceID, activeGroupID int64) error {
	_, err := p.ReopenActivitySession(ctx, instanceID, activeGroupID)
	return err
}
