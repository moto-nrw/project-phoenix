package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The planner's reads of the Timetable owner (#3424 slice S5) behind
// timetable.TimetableDataCapability: the blocks with the state of their
// sessions, their rows, the staffing gaps, a child's week, the template list,
// the Änderungsprotokoll and the preparation of a spontaneous start. The
// reads use the retained repository rows, whose status carries the Student
// Presence session state the owner's own rows do not. The conflict
// acknowledgements are the owner's own capability.

type timetableData struct {
	timetable.ConflictAckCapability
	ports.DeviationEventReader
	ports.RoomOccupancy
	deps ports.TimetableDataDependencies
}

// NewTimetableData composes the planner's reads of the Timetable owner.
func NewTimetableData(deps ports.TimetableDataDependencies) (timetable.TimetableDataCapability, error) {
	if deps.Instances == nil || deps.InstanceStaff == nil || noParticipantRows(deps.Participants) || deps.PickupExceptions == nil ||
		deps.ArrivalExceptions == nil || deps.Visits == nil || deps.Templates == nil || deps.Groups == nil || deps.Categories == nil ||
		deps.Rooms == nil || deps.RoomOccupancy == nil || deps.DeviationEvents == nil || deps.ConflictAcks == nil {
		return nil, errors.New("timetable data: required dependency is nil")
	}
	records := &timetableData{ConflictAckCapability: deps.ConflictAcks, DeviationEventReader: deps.DeviationEvents, RoomOccupancy: deps.RoomOccupancy, deps: deps}
	return &timetableCommands{records, records}, nil
}

func (d *timetableData) FindScheduledInstance(ctx context.Context, id int64) (timetable.ScheduledInstance, error) {
	notFound := fmt.Errorf("%w: %w", timetable.ErrActivityInstanceNotFound, WrapNotFoundDatabaseError("find by id"))
	if id <= 0 {
		return timetable.ScheduledInstance{}, notFound
	}
	rows, err := d.deps.Instances.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{IDs: []int64{id}})
	if err != nil {
		return timetable.ScheduledInstance{}, WrapDatabaseError("find by id", err)
	}
	if len(rows) == 0 || rows[0] == nil {
		return timetable.ScheduledInstance{}, notFound
	}
	return ScheduledInstanceOf(rows[0]), nil
}

func (d *timetableData) ListScheduledInstances(ctx context.Context, from, to calendar.Date) ([]timetable.ScheduledInstance, error) {
	fromDay, toDay := from.String(), to.String()
	rows, err := d.deps.Instances.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{FromDate: &fromDay, ToDate: &toDay, OrderByDateAndTime: true})
	if err != nil {
		return nil, WrapDatabaseError("find by tenant and date range", err)
	}
	return scheduledInstancesOf(rows), nil
}

func (d *timetableData) ListScheduledInstanceRows(ctx context.Context, instances []timetable.ScheduledInstance) (*timetable.ScheduledInstanceRows, error) {
	rows := &timetable.ScheduledInstanceRows{
		Staff:        make(map[int64][]timetable.InstanceStaff, len(instances)),
		Participants: make(map[int64][]timetable.ScheduledParticipant, len(instances)),
		Cutoffs:      make(map[calendar.Date]map[int64]time.Time),
	}
	if len(instances) == 0 {
		return rows, nil
	}
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID)
	}
	staffRows, err := staffOfInstances(ctx, d.deps.InstanceStaff, ids)
	if err != nil {
		return nil, fmt.Errorf("load staff for instances: %w", err)
	}
	for _, row := range staffRows {
		rows.Staff[row.InstanceID] = append(rows.Staff[row.InstanceID], instanceStaffOf(row))
	}
	participantRows, err := participantsOfInstances(ctx, d.deps.Participants.Reads, ids)
	if err != nil {
		return nil, fmt.Errorf("load students for instances: %w", err)
	}
	slices.SortStableFunc(participantRows, func(a, b *scheduleModels.InstanceStudent) int {
		if byCreation := a.CreatedAt.Compare(b.CreatedAt); byCreation != 0 {
			return byCreation
		}
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	for _, row := range participantRows {
		rows.Participants[row.InstanceID] = append(rows.Participants[row.InstanceID], ScheduledParticipantOf(row))
	}
	for date, studentIDs := range studentIDsByDate(instances, rows.Participants) {
		cutoffs, err := d.partialAbsenceCutoffs(ctx, studentIDs, date)
		if err != nil {
			return nil, fmt.Errorf("load pickup cutoffs for %s: %w", date, err)
		}
		rows.Cutoffs[date] = cutoffs
	}
	return rows, nil
}

func studentIDsByDate(instances []timetable.ScheduledInstance, participants map[int64][]timetable.ScheduledParticipant) map[calendar.Date][]int64 {
	seen := make(map[calendar.Date]map[int64]struct{})
	out := make(map[calendar.Date][]int64)
	for _, instance := range instances {
		for _, row := range participants[instance.ID] {
			if seen[instance.Date] == nil {
				seen[instance.Date] = make(map[int64]struct{})
			}
			if _, dup := seen[instance.Date][row.StudentID]; dup {
				continue
			}
			seen[instance.Date][row.StudentID] = struct{}{}
			out[instance.Date] = append(out[instance.Date], row.StudentID)
		}
	}
	return out
}

// partialAbsenceCutoffs returns, per child, the wall-clock time from which
// the child is auto-excused on the date. Only auto-derived rows qualify:
// their excused_from IS the day's pickup time, so the planner may show it as
// "Abholung 14:45" on the block the time falls into (#2360). A manual
// partial absence can differ from the pickup time and never feeds the
// marker.
func (d *timetableData) partialAbsenceCutoffs(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]time.Time, error) {
	if len(studentIDs) == 0 {
		return map[int64]time.Time{}, nil
	}
	rows, err := d.deps.PickupExceptions.FindByStudentIDsAndDate(ctx, studentIDs, calendar.Date(date))
	if err != nil {
		return nil, err
	}
	cutoffs := make(map[int64]time.Time, len(rows))
	for _, row := range rows {
		if row != nil && row.ExcusedAuto && row.ExcusedFrom != nil {
			cutoffs[row.StudentID] = calendar.NormalizeWallClock(*row.ExcusedFrom)
		}
	}
	return cutoffs, nil
}

func (d *timetableData) ListBlockStaff(ctx context.Context, instanceID int64) ([]timetable.InstanceStaff, error) {
	staff, err := d.deps.InstanceStaff.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: []int64{instanceID}, OrderByCreated: true})
	if err != nil {
		return nil, WrapDatabaseError("find by instance id", err)
	}
	if staff == nil {
		staff = []timetable.InstanceStaff{}
	}
	return staff, nil
}

func (d *timetableData) ListBlockParticipants(ctx context.Context, instanceID int64) ([]timetable.ScheduledParticipant, error) {
	rows, err := participantsOfInstance(ctx, d.deps.Participants.Reads, instanceID)
	if err != nil {
		return nil, err
	}
	return scheduledParticipantsOf(rows), nil
}

func (d *timetableData) ListCareDayCandidates(ctx context.Context, instanceIDs []int64) ([]timetable.ScheduledParticipant, error) {
	rows, err := notScheduledCandidatesOf(ctx, d.deps.Participants.Reads, instanceIDs)
	if err != nil {
		return nil, err
	}
	return scheduledParticipantsOf(rows), nil
}

func (d *timetableData) FindBlockParticipant(ctx context.Context, instanceID, studentID int64) (*timetable.ScheduledParticipant, error) {
	row, err := participantOf(ctx, d.deps.Participants.Reads, instanceID, studentID)
	if domain.IsRecordNotFound(err) {
		return nil, nil
	}
	if err != nil || row == nil {
		return nil, err
	}
	participant := ScheduledParticipantOf(row)
	return &participant, nil
}

func (d *timetableData) BlockDisplayMetadata(ctx context.Context, instanceIDs []int64) (map[int64]timetable.BlockDisplayMetadata, error) {
	if d.deps.BlockMetadata == nil {
		return map[int64]timetable.BlockDisplayMetadata{}, nil
	}
	return d.deps.BlockMetadata.ListBlockDisplayMetadata(ctx, instanceIDs)
}

// LockBlockAttendance takes FOR UPDATE on every participant row, so a
// concurrent Complete cannot flip the block between the status check and the
// attendance write. A composition without locks skips it.
func (d *timetableData) LockBlockAttendance(ctx context.Context, instanceID int64) error {
	if d.deps.Locks == nil {
		return nil
	}
	return d.deps.Locks.LockAttendance(ctx, instanceID)
}

func (d *timetableData) PatchSlotAttendance(ctx context.Context, participantID int64, patch timetable.AttendancePatch) error {
	return patchAttendanceFields(ctx, d.deps.Participants.Presence, participantID, attendanceFieldPatch(patch))
}

func scheduledInstancesOf(rows []*scheduleModels.ActivityInstance) []timetable.ScheduledInstance {
	instances := make([]timetable.ScheduledInstance, 0, len(rows))
	for _, row := range rows {
		instances = append(instances, ScheduledInstanceOf(row))
	}
	return instances
}

// ScheduledParticipantOf maps a retained participant row, which carries the
// Student Presence attendance of its slot, onto the owner's view.
func ScheduledParticipantOf(row *scheduleModels.InstanceStudent) timetable.ScheduledParticipant {
	return timetable.ScheduledParticipant{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		InstanceID: row.InstanceID, StudentID: row.StudentID, RoomID: row.RoomID,
		Status: row.Status, Substatus: row.Substatus, Note: row.Note,
		CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt, IsUnplanned: row.IsUnplanned,
		NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
	}
}

func scheduledParticipantsOf(rows []*scheduleModels.InstanceStudent) []timetable.ScheduledParticipant {
	participants := make([]timetable.ScheduledParticipant, 0, len(rows))
	for _, row := range rows {
		participants = append(participants, ScheduledParticipantOf(row))
	}
	return participants
}

func instanceStaffOf(row *scheduleModels.InstanceStaff) timetable.InstanceStaff {
	return timetable.InstanceStaff{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		InstanceID: row.InstanceID, StaffID: row.StaffID, RoomID: row.RoomID,
		IsPrimary: row.IsPrimary, IsSubstitute: row.IsSubstitute, IsAbsent: row.IsAbsent,
		AbsenceReason: row.AbsenceReason, SickAbsenceID: row.SickAbsenceID,
	}
}

type timetableCommands struct {
	timetable.TimetableDataCapability
	commands *timetableData
}
