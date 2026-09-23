package repositories

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// careExitRoster serves the lifecycle's roster port over the two owners of a
// planned participant since the presence cutover (#2762): Timetable removes
// and restores the participant, Student Presence carries its attendance.
// Removed rows travel as roster-kind ledger entries that keep both halves.
type careExitRoster struct {
	reads    timetableCompose.PresenceReads
	roster   timetable.Capability
	presence careExitRosterPresence
}

type careExitRosterPresence interface {
	studentpresence.SessionAttendanceQuery
	studentpresence.SessionAttendanceCommand
}

func newCareExitRoster(db *bun.DB, roster timetable.Capability, presence careExitRosterPresence) careExitRoster {
	if db == nil || roster == nil || presence == nil {
		panic("care exit roster: database, timetable and student presence are required")
	}
	return careExitRoster{reads: mustPresenceReads(db), roster: roster, presence: presence}
}

func (r careExitRoster) LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after users.CalendarDate) error {
	return r.roster.LockPlannedRosterForCareExit(ctx, studentIDs, after.String())
}

// RemovePlannedRosterForCareExit reads the attendance of the rows the
// removal takes while they still exist, removes them, and hands both halves
// to the ledger.
func (r careExitRoster) RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after users.CalendarDate) ([]careplan.CareExitRemoval, error) {
	planned, err := r.roster.PreviewPlannedRosterForCareExit(ctx, studentIDs, after.String())
	if err != nil {
		return nil, err
	}
	attendance, err := r.attendanceOf(ctx, planned)
	if err != nil {
		return nil, err
	}
	rows, err := r.roster.RemovePlannedRosterForCareExit(ctx, studentIDs, after.String())
	if err != nil {
		return nil, err
	}
	result := make([]careplan.CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		record, ok := attendance[row.ParticipantID]
		if !ok {
			record = studentpresence.ExpectedSessionAttendance(row.ParticipantID)
		}
		status, unplanned, notScheduled := record.Status, record.IsUnplanned, record.NotScheduled
		result = append(result, careplan.CareExitRemoval{
			TenantID: row.TenantID, StudentID: row.StudentID, Kind: careplan.CareExitRemovalRoster,
			InstanceID: &row.InstanceID, RoomID: row.RoomID, Status: &status,
			Substatus: record.Substatus, Note: record.Note, IsUnplanned: &unplanned,
			NotScheduled: &notScheduled, ManualStatusAt: record.ManualStatusAt,
			StudentStatusDayID: record.StudentStatusDayID, PickupExceptionID: record.PickupExceptionID,
		})
	}
	return result, nil
}

// RestoreRosterForCareExit puts the participants back and replays the
// attendance they carried. The pickup exception link is reconnected by the
// lifecycle once the weekly plans are back.
func (r careExitRoster) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, removals []careplan.CareExitRemoval) (int, error) {
	restored, err := r.roster.RestoreRosterForCareExit(ctx, studentIDs, careExitRosterRows(removals))
	if err != nil {
		return 0, err
	}
	if len(restored) == 0 {
		return 0, nil
	}
	archived := make(map[careExitRosterKey]careplan.CareExitRemoval, len(removals))
	for _, removal := range removals {
		if removal.Kind == careplan.CareExitRemovalRoster && removal.InstanceID != nil {
			archived[careExitRosterKey{StudentID: removal.StudentID, InstanceID: *removal.InstanceID}] = removal
		}
	}
	rows := make([]studentpresence.SessionAttendanceRestore, 0, len(restored))
	for _, row := range restored {
		removal, ok := archived[careExitRosterKey{StudentID: row.StudentID, InstanceID: row.InstanceID}]
		if !ok || !careExitAttendanceDiffers(removal) {
			continue
		}
		restore := studentpresence.SessionAttendanceRestore{
			ParticipantID: row.ParticipantID, Status: studentpresence.SessionAttendanceExpected,
			Substatus: removal.Substatus, Note: removal.Note, ManualStatusAt: removal.ManualStatusAt,
			StudentStatusDayID: removal.StudentStatusDayID,
		}
		if removal.Status != nil {
			restore.Status = *removal.Status
		}
		if removal.IsUnplanned != nil {
			restore.IsUnplanned = *removal.IsUnplanned
		}
		if removal.NotScheduled != nil {
			restore.NotScheduled = *removal.NotScheduled
		}
		rows = append(rows, restore)
	}
	if len(rows) > 0 {
		if err := r.presence.RestoreSessionAttendance(ctx, rows); err != nil {
			return 0, err
		}
	}
	return len(restored), nil
}

func (r careExitRoster) CountPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after users.CalendarDate, restorable []careplan.CareExitRemoval) (map[int64]int, error) {
	return r.roster.CountPlannedRosterForCareExit(ctx, studentIDs, after.String(), careExitRosterRows(restorable))
}

func (r careExitRoster) ListOpenStudentAssignments(ctx context.Context, studentIDs []int64) ([]int64, error) {
	open, err := r.openParticipants(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make([]int64, 0, len(open))
	seen := make(map[int64]bool, len(open))
	for _, row := range open {
		if !seen[row.StudentID] {
			seen[row.StudentID] = true
			result = append(result, row.StudentID)
		}
	}
	return result, nil
}

func (r careExitRoster) LatestStudentAssignmentAttendanceDate(ctx context.Context, studentID int64) (*users.CalendarDate, error) {
	day, err := r.reads.LatestAttendedBlockDate(ctx, studentID)
	if err != nil || day == nil {
		return nil, err
	}
	return parseCareExitDay(day)
}

func (r careExitRoster) LockOpenStudentAssignments(ctx context.Context, studentIDs []int64) error {
	ids, err := r.openParticipantIDs(ctx, studentIDs)
	if err != nil || len(ids) == 0 {
		return err
	}
	return r.presence.LockSessionAttendance(ctx, ids)
}

func (r careExitRoster) CloseOpenStudentAssignments(ctx context.Context, studentIDs []int64, at time.Time) (int64, error) {
	ids, err := r.openParticipantIDs(ctx, studentIDs)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	return r.presence.CloseOpenParticipants(ctx, ids, at)
}

// ReconnectCareExitAssignmentPickupExceptions re-links the restored
// participants to the still existing partial excusals the ledger names.
func (r careExitRoster) ReconnectCareExitAssignmentPickupExceptions(ctx context.Context, studentIDs, pickupExceptionIDs []int64, removals []careplan.CareExitRemoval) error {
	valid := make(map[int64]bool, len(pickupExceptionIDs))
	for _, id := range pickupExceptionIDs {
		valid[id] = true
	}
	wanted := make(map[careExitRosterKey]int64)
	instanceIDs := make([]int64, 0, len(removals))
	for _, removal := range removals {
		if removal.Kind != careplan.CareExitRemovalRoster || removal.InstanceID == nil || removal.PickupExceptionID == nil || !valid[*removal.PickupExceptionID] {
			continue
		}
		wanted[careExitRosterKey{StudentID: removal.StudentID, InstanceID: *removal.InstanceID}] = *removal.PickupExceptionID
		instanceIDs = append(instanceIDs, *removal.InstanceID)
	}
	if len(wanted) == 0 {
		return nil
	}
	participants, err := r.roster.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: uniqueInt64s(instanceIDs), StudentIDs: studentIDs})
	if err != nil {
		return err
	}
	links := make([]studentpresence.ParticipantPickupException, 0, len(participants))
	for _, participant := range participants {
		if exceptionID, ok := wanted[careExitRosterKey{StudentID: participant.StudentID, InstanceID: participant.InstanceID}]; ok {
			links = append(links, studentpresence.ParticipantPickupException{ParticipantID: participant.ID, PickupExceptionID: exceptionID})
		}
	}
	if len(links) == 0 {
		return nil
	}
	return r.presence.ReconnectParticipantPickupExceptions(ctx, links)
}

type careExitRosterKey struct {
	StudentID  int64
	InstanceID int64
}

func (r careExitRoster) attendanceOf(ctx context.Context, rows []timetable.CareExitRosterRow) (map[int64]studentpresence.SessionAttendance, error) {
	result := make(map[int64]studentpresence.SessionAttendance, len(rows))
	if len(rows) == 0 {
		return result, nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ParticipantID)
	}
	values, err := r.presence.ListSessionAttendance(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		result[value.ParticipantID] = value
	}
	return result, nil
}

func (r careExitRoster) openParticipants(ctx context.Context, studentIDs []int64) ([]timetableCompose.OpenParticipant, error) {
	return r.reads.ListOpenParticipants(ctx, studentIDs)
}

func (r careExitRoster) openParticipantIDs(ctx context.Context, studentIDs []int64) ([]int64, error) {
	open, err := r.openParticipants(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(open))
	for _, row := range open {
		ids = append(ids, row.ParticipantID)
	}
	return ids, nil
}

// careExitAttendanceDiffers reports whether the ledger entry carries more
// than expected attendance.
func careExitAttendanceDiffers(removal careplan.CareExitRemoval) bool {
	return (removal.Status != nil && *removal.Status != studentpresence.SessionAttendanceExpected) ||
		removal.Substatus != nil || removal.Note != nil || (removal.IsUnplanned != nil && *removal.IsUnplanned) ||
		(removal.NotScheduled != nil && *removal.NotScheduled) || removal.ManualStatusAt != nil || removal.StudentStatusDayID != nil
}

// careExitRosterRows keeps the roster entries of the ledger as Timetable rows.
func careExitRosterRows(removals []careplan.CareExitRemoval) []timetable.CareExitRosterRow {
	rows := make([]timetable.CareExitRosterRow, 0, len(removals))
	for _, removal := range removals {
		if removal.Kind != careplan.CareExitRemovalRoster || removal.InstanceID == nil {
			continue
		}
		rows = append(rows, timetable.CareExitRosterRow{
			TenantID: removal.TenantID, StudentID: removal.StudentID, InstanceID: *removal.InstanceID, RoomID: removal.RoomID,
		})
	}
	return rows
}
