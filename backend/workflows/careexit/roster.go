package careexit

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RosterTimetable is the Timetable half of a planned participant since the
// presence cutover (#2762): the roster rows a care exit removes and restores,
// the baseline its preview counts and the participants of the restored blocks.
type RosterTimetable interface {
	timetable.CareExitRosterCommand
	CountPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string, restorable []timetable.CareExitRosterRow) (map[int64]int, error)
	ListInstanceStudents(ctx context.Context, filter timetable.InstanceStudentFilter) ([]timetable.InstanceStudent, error)
}

// RosterPresence is the Student Presence half: the attendance a planned
// participant carries.
type RosterPresence interface {
	studentpresence.SessionAttendanceQuery
	RestoreSessionAttendance(ctx context.Context, rows []studentpresence.SessionAttendanceRestore) error
	LockSessionAttendance(ctx context.Context, participantIDs []int64) error
	CloseOpenParticipants(ctx context.Context, participantIDs []int64, at time.Time) (int64, error)
	ReconnectParticipantPickupExceptions(ctx context.Context, links []studentpresence.ParticipantPickupException) error
}

// OpenParticipant is a planned participant of a child with an open block
// presence.
type OpenParticipant struct {
	ParticipantID int64
	StudentID     int64
}

// RosterAttendance reads the block presence of the children across both
// owners, in the caller's tenant.
type RosterAttendance interface {
	// OpenParticipants lists the participants of the students with an open
	// block presence, ordered by participant id.
	OpenParticipants(ctx context.Context, studentIDs []int64) ([]OpenParticipant, error)
	// LatestAttendedBlockDate is the day of the last block the child
	// attended, nil when there is none.
	LatestAttendedBlockDate(ctx context.Context, studentID int64) (*calendar.Date, error)
}

// Roster serves the Care Plan lifecycle's roster port over Timetable and
// Student Presence. Removed rows travel as roster-kind ledger entries that
// keep both halves.
type Roster struct {
	timetable  RosterTimetable
	presence   RosterPresence
	attendance RosterAttendance
}

// NewRoster binds the roster to its owners. Every dependency is required; a
// missing one is a composition error.
func NewRoster(timetable RosterTimetable, presence RosterPresence, attendance RosterAttendance) Roster {
	if timetable == nil || presence == nil || attendance == nil {
		panic("care exit roster: timetable and student presence are required")
	}
	return Roster{timetable: timetable, presence: presence, attendance: attendance}
}

func (r Roster) LockPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after calendar.Date) error {
	return r.timetable.LockPlannedRosterForCareExit(ctx, studentIDs, after.String())
}

// RemovePlannedRosterForCareExit reads the attendance of the rows the
// removal takes while they still exist, removes them, and hands both halves
// to the ledger. A participant without an attendance row was expected.
func (r Roster) RemovePlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after calendar.Date) ([]careplan.CareExitRemoval, error) {
	planned, err := r.timetable.PreviewPlannedRosterForCareExit(ctx, studentIDs, after.String())
	if err != nil {
		return nil, err
	}
	attendance, err := r.attendanceOf(ctx, planned)
	if err != nil {
		return nil, err
	}
	rows, err := r.timetable.RemovePlannedRosterForCareExit(ctx, studentIDs, after.String())
	if err != nil {
		return nil, err
	}
	result := make([]careplan.CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		record, ok := attendance[row.ParticipantID]
		if !ok {
			record = studentpresence.ExpectedSessionAttendance(row.ParticipantID)
		}
		result = append(result, rosterRemoval(row, record))
	}
	return result, nil
}

func rosterRemoval(row timetable.CareExitRosterRow, record studentpresence.SessionAttendance) careplan.CareExitRemoval {
	status, unplanned, notScheduled := record.Status, record.IsUnplanned, record.NotScheduled
	return careplan.CareExitRemoval{
		TenantID: row.TenantID, StudentID: row.StudentID, Kind: careplan.CareExitRemovalRoster,
		InstanceID: &row.InstanceID, RoomID: row.RoomID, Status: &status,
		Substatus: record.Substatus, Note: record.Note, IsUnplanned: &unplanned,
		NotScheduled: &notScheduled, ManualStatusAt: record.ManualStatusAt,
		StudentStatusDayID: record.StudentStatusDayID, PickupExceptionID: record.PickupExceptionID,
	}
}

// RestoreRosterForCareExit puts the participants back and replays the
// attendance they carried, but only where it differs from expected. The
// pickup exception link is reconnected by the lifecycle once the weekly plans
// are back.
func (r Roster) RestoreRosterForCareExit(ctx context.Context, studentIDs []int64, removals []careplan.CareExitRemoval) (int, error) {
	restored, err := r.timetable.RestoreRosterForCareExit(ctx, studentIDs, rosterRows(removals))
	if err != nil {
		return 0, err
	}
	if len(restored) == 0 {
		return 0, nil
	}
	archived := archivedRoster(removals)
	rows := make([]studentpresence.SessionAttendanceRestore, 0, len(restored))
	for _, row := range restored {
		removal, ok := archived[rosterKey{StudentID: row.StudentID, InstanceID: row.InstanceID}]
		if !ok || !attendanceDiffers(removal) {
			continue
		}
		rows = append(rows, attendanceRestore(row.ParticipantID, removal))
	}
	if len(rows) > 0 {
		if err := r.presence.RestoreSessionAttendance(ctx, rows); err != nil {
			return 0, err
		}
	}
	return len(restored), nil
}

func (r Roster) CountPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after calendar.Date, restorable []careplan.CareExitRemoval) (map[int64]int, error) {
	return r.timetable.CountPlannedRosterForCareExit(ctx, studentIDs, after.String(), rosterRows(restorable))
}

// ListOpenStudentAssignments lists every child with an open block presence
// once, in participant order.
func (r Roster) ListOpenStudentAssignments(ctx context.Context, studentIDs []int64) ([]int64, error) {
	open, err := r.attendance.OpenParticipants(ctx, studentIDs)
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

func (r Roster) LatestStudentAssignmentAttendanceDate(ctx context.Context, studentID int64) (*calendar.Date, error) {
	day, err := r.attendance.LatestAttendedBlockDate(ctx, studentID)
	if err != nil || day == nil {
		return nil, err
	}
	parsed, err := calendar.ParseDate(day.String())
	if err != nil {
		return nil, fmt.Errorf("care exit: invalid calendar day %q", day.String())
	}
	return &parsed, nil
}

func (r Roster) LockOpenStudentAssignments(ctx context.Context, studentIDs []int64) error {
	ids, err := r.openParticipantIDs(ctx, studentIDs)
	if err != nil || len(ids) == 0 {
		return err
	}
	return r.presence.LockSessionAttendance(ctx, ids)
}

func (r Roster) CloseOpenStudentAssignments(ctx context.Context, studentIDs []int64, at time.Time) (int64, error) {
	ids, err := r.openParticipantIDs(ctx, studentIDs)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	return r.presence.CloseOpenParticipants(ctx, ids, at)
}

// ReconnectCareExitAssignmentPickupExceptions re-links the restored
// participants to the still existing partial excusals the ledger names. A
// link to an excusal that no longer exists is dropped.
func (r Roster) ReconnectCareExitAssignmentPickupExceptions(ctx context.Context, studentIDs, pickupExceptionIDs []int64, removals []careplan.CareExitRemoval) error {
	wanted, instanceIDs := wantedPickupExceptions(pickupExceptionIDs, removals)
	if len(wanted) == 0 {
		return nil
	}
	participants, err := r.timetable.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: uniqueSortedIDs(instanceIDs), StudentIDs: studentIDs})
	if err != nil {
		return err
	}
	links := make([]studentpresence.ParticipantPickupException, 0, len(participants))
	for _, participant := range participants {
		if exceptionID, ok := wanted[rosterKey{StudentID: participant.StudentID, InstanceID: participant.InstanceID}]; ok {
			links = append(links, studentpresence.ParticipantPickupException{ParticipantID: participant.ID, PickupExceptionID: exceptionID})
		}
	}
	if len(links) == 0 {
		return nil
	}
	return r.presence.ReconnectParticipantPickupExceptions(ctx, links)
}

// wantedPickupExceptions keys the roster removals that carried one of the
// still valid partial excusals by student and instance.
func wantedPickupExceptions(pickupExceptionIDs []int64, removals []careplan.CareExitRemoval) (map[rosterKey]int64, []int64) {
	valid := make(map[int64]bool, len(pickupExceptionIDs))
	for _, id := range pickupExceptionIDs {
		valid[id] = true
	}
	wanted := make(map[rosterKey]int64)
	instanceIDs := make([]int64, 0, len(removals))
	for _, removal := range removals {
		if removal.Kind != careplan.CareExitRemovalRoster || removal.InstanceID == nil || removal.PickupExceptionID == nil || !valid[*removal.PickupExceptionID] {
			continue
		}
		wanted[rosterKey{StudentID: removal.StudentID, InstanceID: *removal.InstanceID}] = *removal.PickupExceptionID
		instanceIDs = append(instanceIDs, *removal.InstanceID)
	}
	return wanted, instanceIDs
}

type rosterKey struct {
	StudentID  int64
	InstanceID int64
}

func (r Roster) attendanceOf(ctx context.Context, rows []timetable.CareExitRosterRow) (map[int64]studentpresence.SessionAttendance, error) {
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

func (r Roster) openParticipantIDs(ctx context.Context, studentIDs []int64) ([]int64, error) {
	open, err := r.attendance.OpenParticipants(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(open))
	for _, row := range open {
		ids = append(ids, row.ParticipantID)
	}
	return ids, nil
}

// archivedRoster indexes the archived roster removals by student and
// instance.
func archivedRoster(removals []careplan.CareExitRemoval) map[rosterKey]careplan.CareExitRemoval {
	archived := make(map[rosterKey]careplan.CareExitRemoval, len(removals))
	for _, removal := range removals {
		if removal.Kind == careplan.CareExitRemovalRoster && removal.InstanceID != nil {
			archived[rosterKey{StudentID: removal.StudentID, InstanceID: *removal.InstanceID}] = removal
		}
	}
	return archived
}

// attendanceRestore replays the attendance an archived removal carried onto
// the restored participant.
func attendanceRestore(participantID int64, removal careplan.CareExitRemoval) studentpresence.SessionAttendanceRestore {
	restore := studentpresence.SessionAttendanceRestore{
		ParticipantID: participantID, Status: studentpresence.SessionAttendanceExpected,
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
	return restore
}

// attendanceDiffers reports whether the ledger entry carries more than
// expected attendance.
func attendanceDiffers(removal careplan.CareExitRemoval) bool {
	return (removal.Status != nil && *removal.Status != studentpresence.SessionAttendanceExpected) ||
		removal.Substatus != nil || removal.Note != nil || (removal.IsUnplanned != nil && *removal.IsUnplanned) ||
		(removal.NotScheduled != nil && *removal.NotScheduled) || removal.ManualStatusAt != nil || removal.StudentStatusDayID != nil
}

// rosterRows keeps the roster entries of the ledger as Timetable rows.
func rosterRows(removals []careplan.CareExitRemoval) []timetable.CareExitRosterRow {
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
