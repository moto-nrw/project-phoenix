package repositories

import (
	"context"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// NewActivityRecoveryRepository composes the completion and reopen support of
// a block from its two owners: Timetable locks the participants, Student
// Presence locks and restores the live group, the visits, the supervisors,
// the attendance and the session (#2762).
func NewActivityRecoveryRepository(db *bun.DB, assignments scheduleModels.InstanceStudentRepository) scheduleModels.ActivityRecoveryRepository {
	owner, ok := assignments.(timetableInstanceStudentRepository)
	if !ok {
		panic("activity recovery: timetable assignment adapter is required")
	}
	presence := newStudentPresence(db)
	return timetableCompose.NewActivityRecoveryRepository(db,
		recoveryAssignments{capability: owner.timetable, presence: presence},
		recoveryPresence{Module: presence})
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
// owner module.
type recoveryPresence struct{ *studentpresence.Module }

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

func (p recoveryPresence) RestoreGroup(ctx context.Context, activeGroupID int64, now time.Time) error {
	return p.Module.RestoreGroup(ctx, activeGroupID, now)
}
