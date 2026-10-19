package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The grade transition (#3424) parks and replays the planned participants of
// departing students; the attendance travels with the participant.

// ArchivePlannedByStudentIDsFrom parks the still-planned rows of departing
// children: a row stays when it records an observed presence, or a hand-set
// status on a block that already started. The attendance each parked row
// carried travels into the archive with it.
func (r timetableInstanceStudentRepository) ArchivePlannedByStudentIDsFrom(ctx context.Context, transitionID int64, studentIDs []int64, from calendar.Date, at time.Time) (int, error) {
	studentIDs = positiveInstanceStudentIDs(studentIDs)
	if len(studentIDs) == 0 {
		return 0, nil
	}
	fromText := from.String()
	participants, err := r.timetable.ListPlannedInstanceStudents(ctx, timetable.InstanceStudentFilter{StudentIDs: studentIDs, FromDate: &fromText, ExcludeCancelled: true})
	if err != nil {
		return 0, WrapDatabaseError("archive planned by student ids from", err)
	}
	if len(participants) == 0 {
		return 0, nil
	}
	ids, instanceIDs := make([]int64, 0, len(participants)), make([]int64, 0, len(participants))
	for _, participant := range participants {
		ids = append(ids, participant.ID)
		instanceIDs = append(instanceIDs, participant.InstanceID)
	}
	attendance, err := r.attendanceByParticipant(ctx, ids)
	if err != nil {
		return 0, WrapDatabaseError("archive planned by student ids from", err)
	}
	started, completed, err := r.sessionStates(ctx, uniqueInt64s(instanceIDs))
	if err != nil {
		return 0, WrapDatabaseError("archive planned by student ids from", err)
	}
	today, clock := InstanceStudentDay(at), InstanceStudentWallClock(at)
	entries := make([]timetable.RosterArchiveEntry, 0, len(participants))
	for _, participant := range participants {
		row := attendance[participant.ID]
		if archivesPlannedParticipant(participant, row, started[participant.InstanceID], completed[participant.InstanceID], today, clock) {
			entries = append(entries, timetable.RosterArchiveEntry{ParticipantID: participant.ID, Attendance: archivedAttendance(row)})
		}
	}
	rows, err := r.timetable.ArchivePlannedInstanceStudents(ctx, transitionID, entries)
	if err != nil {
		return 0, WrapDatabaseError("archive planned by student ids from", err)
	}
	return rows, nil
}

// RestoreArchivedByTransition replays the parked rows: the participant comes
// back through Timetable, a non-booking or hand-set status is restored as
// archived, and every other row takes today's day statuses and excusals.
func (r timetableInstanceStudentRepository) RestoreArchivedByTransition(ctx context.Context, transitionID int64, studentIDs []int64, from calendar.Date) (int, error) {
	restored, err := r.timetable.RestoreArchivedInstanceStudents(ctx, transitionID, positiveInstanceStudentIDs(studentIDs), from.String())
	if err != nil {
		return 0, WrapDatabaseError("restore archived rows by transition", err)
	}
	if len(restored) == 0 {
		return 0, nil
	}
	rows := make([]studentpresence.SessionAttendanceRestore, 0, len(restored))
	instances := make(map[int64]string, len(restored))
	for _, row := range restored {
		restore, dayStatuses := restoredAttendance(row)
		if restore != nil {
			rows = append(rows, *restore)
		}
		if dayStatuses {
			instances[row.InstanceID] = row.Date
		}
	}
	if len(rows) > 0 {
		if err := r.presence.RestoreSessionAttendance(ctx, rows); err != nil {
			return 0, WrapDatabaseError("restore archived rows by transition", err)
		}
	}
	if err := r.reapplyDayStatuses(ctx, instances); err != nil {
		return 0, WrapDatabaseError("restore archived rows by transition", err)
	}
	return len(restored), nil
}

func archivedAttendance(row studentpresence.SessionAttendance) timetable.ArchivedAttendance {
	status := row.Status
	if status == "" {
		status = studentpresence.SessionAttendanceExpected
	}
	return timetable.ArchivedAttendance{
		Status: status, Substatus: row.Substatus, Note: row.Note, IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled,
		ManualStatusAt: row.ManualStatusAt, StudentStatusDayID: row.StudentStatusDayID,
	}
}

// sessionStates reports which blocks Student Presence already started and
// which it completed.
func (r timetableInstanceStudentRepository) sessionStates(ctx context.Context, instanceIDs []int64) (map[int64]bool, map[int64]bool, error) {
	sessions, err := r.presence.ListActivitySessions(ctx, studentpresence.ActivitySessionFilter{InstanceIDs: instanceIDs})
	if err != nil {
		return nil, nil, err
	}
	started := make(map[int64]bool, len(sessions))
	completed := make(map[int64]bool, len(sessions))
	for _, session := range sessions {
		started[session.InstanceID] = true
		completed[session.InstanceID] = session.Status == studentpresence.ActivitySessionCompleted
	}
	return started, completed, nil
}

// archivesPlannedParticipant keeps a row on the roster when it records an
// observed presence, or a hand-set status on a block that already started.
func archivesPlannedParticipant(participant timetable.PlannedInstanceStudent, row studentpresence.SessionAttendance, started, completed bool, today, clock string) bool {
	if completed || row.CheckedInAt != nil || row.CheckedOutAt != nil {
		return false
	}
	undecided := row.ManualStatusAt == nil && row.Status != studentpresence.SessionAttendancePresent
	future := !started && (participant.Date > today || (participant.Date == today && participant.StartTime > clock))
	return undecided || future
}

// restoredAttendance restores a non-booking or hand-set status as archived;
// every other row keeps only its walk-in marker and note and takes today's
// day statuses and excusals.
func restoredAttendance(row timetable.RestoredInstanceStudent) (*studentpresence.SessionAttendanceRestore, bool) {
	archived := row.Attendance
	if archived.NotScheduled || (archived.ManualStatusAt != nil && archived.Status != studentpresence.SessionAttendanceExpected) {
		return &studentpresence.SessionAttendanceRestore{
			ParticipantID: row.ID, Status: archived.Status, Substatus: archived.Substatus, Note: archived.Note,
			IsUnplanned: archived.IsUnplanned, NotScheduled: archived.NotScheduled, ManualStatusAt: archived.ManualStatusAt,
		}, false
	}
	if archived.IsUnplanned || archived.Note != nil {
		return &studentpresence.SessionAttendanceRestore{
			ParticipantID: row.ID, Status: studentpresence.SessionAttendanceExpected, Note: archived.Note, IsUnplanned: archived.IsUnplanned,
		}, true
	}
	return nil, true
}

// reapplyDayStatuses applies the active day statuses and partial absences to
// the blocks of the restored rows, in block order.
func (r timetableInstanceStudentRepository) reapplyDayStatuses(ctx context.Context, instances map[int64]string) error {
	for _, instanceID := range sortedKeys(instances) {
		if _, err := r.presence.ApplyActiveStatusDaysForInstance(ctx, instanceID, instances[instanceID]); err != nil {
			return err
		}
		if _, err := r.presence.ApplyActivePartialAbsencesForInstance(ctx, instanceID, instances[instanceID]); err != nil {
			return err
		}
	}
	return nil
}
