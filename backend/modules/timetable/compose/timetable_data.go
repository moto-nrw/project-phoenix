package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The planner's reads of the Timetable owner (#3424 slice S5) behind
// timetable.TimetableDataCapability: the blocks with the state of their
// sessions, their rows, the staffing gaps, a child's week, the template list,
// the Änderungsprotokoll and the preparation of a spontaneous start. The
// reads use the retained repository rows, whose status carries the Student
// Presence session state the owner's own rows do not. The conflict
// acknowledgements are the owner's own capability.

// DataInstances reads blocks from the retained activity instance repository.
type DataInstances interface {
	FindByID(ctx context.Context, id any) (*scheduleModels.ActivityInstance, error)
	FindByTenantAndDateRange(ctx context.Context, from, to scheduleModels.Date) ([]*scheduleModels.ActivityInstance, error)
}

// DataParticipants reads and patches the participant rows of blocks.
type DataParticipants interface {
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStudent, error)
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error)
	FindNotScheduledCandidatesByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error)
	FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*scheduleModels.InstanceStudent, error)
	FindInstancesWithAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, from, to scheduleModels.Date) ([]*scheduleModels.ScheduledInstanceRow, error)
	UpdateAttendanceFields(ctx context.Context, id int64, patch scheduleModels.AttendanceFieldPatch) error
}

// DataPickupExceptions reads the dated pickup exceptions of children.
type DataPickupExceptions interface {
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date scheduleModels.Date) ([]*scheduleModels.StudentPickupException, error)
	FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to scheduleModels.Date) ([]*scheduleModels.StudentPickupException, error)
}

// DataArrivalExceptions reads the dated arrival exceptions of a child.
type DataArrivalExceptions interface {
	FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to scheduleModels.Date) ([]*scheduleModels.StudentArrivalException, error)
}

// PickupBaselineProjector is the consumer-owned port to Care Plan's regular
// pickup plan over an inclusive window (#2414, ADR 0005).
type PickupBaselineProjector interface {
	ProjectPickups(ctx context.Context, studentIDs []int64, from, to timezone.Date) (PickupBaselines, error)
}

// PickupBaselines answers the regular pickup of a student on a date; ok is
// false when no recurring pickup applies.
type PickupBaselines interface {
	RegularPickup(studentID int64, date timezone.Date) (pickup time.Time, ok bool)
}

// DataTemplates reads the Vorlagen list and writes the activity a
// spontaneous start needs, through the retained activity group repository.
type DataTemplates interface {
	ListTemplateRows(ctx context.Context, templateID *int64) ([]activitiesModels.TemplateListRow, error)
	ListTemplateRowsForTemplatePeriod(ctx context.Context, templateID, periodID int64) ([]activitiesModels.TemplateListRow, error)
	ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]activitiesModels.TemplateListRow, error)
	ListTemplateWeekdayRoster(ctx context.Context, templateID, calendarPeriodID *int64) ([]activitiesModels.TemplateWeekdayRosterRow, error)
	ListTemplateCapacityOccurrences(ctx context.Context, periodID *int64, templateIDs []int64) ([]activitiesModels.TemplateCapacityOccurrence, error)
	FindTargetsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]*activitiesModels.GroupTarget, error)
	FindTargetStudentIDsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]int64, error)
	FindByName(ctx context.Context, name string) (*activitiesModels.Group, error)
	Create(ctx context.Context, group *activitiesModels.Group) error
}

// DataGroups reads the template a block was materialized from through the
// owner's group capability.
type DataGroups interface {
	FindGroup(ctx context.Context, id int64) (timetable.Group, error)
}

// DataCategories reads and creates the activity categories.
type DataCategories interface {
	FindByNameIncludingArchivedForShare(ctx context.Context, name string) (*activitiesModels.Category, error)
	Create(ctx context.Context, category *activitiesModels.Category) error
}

// RoomOccupancy reports another open Student Presence session in a room.
type RoomOccupancy interface {
	RoomOccupied(ctx context.Context, roomID int64) (bool, error)
}

// DeviationEventReader is the consumer-owned port to the Audit Platform's
// Änderungsprotokoll (#1886).
type DeviationEventReader interface {
	ListDeviationEvents(ctx context.Context, from, to timezone.Date, activityGroupID *int64, startTime *string) ([]timetable.DeviationEvent, error)
}

// AdvisoryLocks takes transaction-scoped advisory locks through the
// Transaction Runtime.
type AdvisoryLocks interface {
	AcquireXactLock(ctx context.Context, key string) error
}

// TimetableDataDependencies wires the planner's reads. Locks and Advisory
// are optional: without them attendance patches skip the completion lock and
// spontaneous-start locks are skipped outside a transaction only. Every other
// collaborator is required.
type TimetableDataDependencies struct {
	Instances         DataInstances
	InstanceStaff     OperationInstanceStaff
	Participants      DataParticipants
	PickupExceptions  DataPickupExceptions
	ArrivalExceptions DataArrivalExceptions
	ArrivalBaselines  ArrivalBaselineProjector
	PickupBaselines   PickupBaselineProjector
	Visits            OperationVisits
	Templates         DataTemplates
	Groups            DataGroups
	Categories        DataCategories
	Rooms             RoomNames
	RoomOccupancy     RoomOccupancy
	DeviationEvents   DeviationEventReader
	ConflictAcks      timetable.ConflictAckCapability
	Locks             AttendanceLocks
	Advisory          AdvisoryLocks
	Logger            *slog.Logger
}

type timetableData struct {
	timetable.ConflictAckCapability
	deps TimetableDataDependencies
}

// NewTimetableData composes the planner's reads of the Timetable owner.
func NewTimetableData(deps TimetableDataDependencies) (timetable.TimetableDataCapability, error) {
	if deps.Instances == nil || deps.InstanceStaff == nil || deps.Participants == nil || deps.PickupExceptions == nil ||
		deps.ArrivalExceptions == nil || deps.Visits == nil || deps.Templates == nil || deps.Groups == nil || deps.Categories == nil ||
		deps.Rooms == nil || deps.RoomOccupancy == nil || deps.DeviationEvents == nil || deps.ConflictAcks == nil {
		return nil, errors.New("timetable data: required dependency is nil")
	}
	return &timetableData{ConflictAckCapability: deps.ConflictAcks, deps: deps}, nil
}

func (d *timetableData) FindScheduledInstance(ctx context.Context, id int64) (timetable.ScheduledInstance, error) {
	row, err := d.deps.Instances.FindByID(ctx, id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return timetable.ScheduledInstance{}, fmt.Errorf("%w: %w", timetable.ErrActivityInstanceNotFound, err)
		}
		return timetable.ScheduledInstance{}, err
	}
	if row == nil {
		return timetable.ScheduledInstance{}, timetable.ErrActivityInstanceNotFound
	}
	return ScheduledInstanceOf(row), nil
}

func (d *timetableData) ListScheduledInstances(ctx context.Context, from, to timezone.Date) ([]timetable.ScheduledInstance, error) {
	rows, err := d.deps.Instances.FindByTenantAndDateRange(ctx, scheduleModels.Date(from), scheduleModels.Date(to))
	if err != nil {
		return nil, err
	}
	return scheduledInstancesOf(rows), nil
}

// ListScheduledInstanceRows loads the rows of the listed blocks with one
// batched read per kind plus one cutoff read per distinct date, instead of
// three reads per block (#2940). Participant rows keep the created_at order
// of the single-block read.
func (d *timetableData) ListScheduledInstanceRows(ctx context.Context, instances []timetable.ScheduledInstance) (*timetable.ScheduledInstanceRows, error) {
	rows := &timetable.ScheduledInstanceRows{
		Staff:        make(map[int64][]timetable.InstanceStaff, len(instances)),
		Participants: make(map[int64][]timetable.ScheduledParticipant, len(instances)),
		Cutoffs:      make(map[timezone.Date]map[int64]time.Time),
	}
	if len(instances) == 0 {
		return rows, nil
	}
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID)
	}
	staffRows, err := d.deps.InstanceStaff.FindByInstanceIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load staff for instances: %w", err)
	}
	for _, row := range staffRows {
		rows.Staff[row.InstanceID] = append(rows.Staff[row.InstanceID], instanceStaffOf(row))
	}
	participantRows, err := d.deps.Participants.FindByInstanceIDs(ctx, ids)
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

func studentIDsByDate(instances []timetable.ScheduledInstance, participants map[int64][]timetable.ScheduledParticipant) map[timezone.Date][]int64 {
	seen := make(map[timezone.Date]map[int64]struct{})
	out := make(map[timezone.Date][]int64)
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
func (d *timetableData) partialAbsenceCutoffs(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]time.Time, error) {
	if len(studentIDs) == 0 {
		return map[int64]time.Time{}, nil
	}
	rows, err := d.deps.PickupExceptions.FindByStudentIDsAndDate(ctx, studentIDs, scheduleModels.Date(date))
	if err != nil {
		return nil, err
	}
	cutoffs := make(map[int64]time.Time, len(rows))
	for _, row := range rows {
		if row != nil && row.ExcusedAuto && row.ExcusedFrom != nil {
			cutoffs[row.StudentID] = timezone.NormalizeWallClock(*row.ExcusedFrom)
		}
	}
	return cutoffs, nil
}

func (d *timetableData) ListBlockStaff(ctx context.Context, instanceID int64) ([]timetable.InstanceStaff, error) {
	rows, err := d.deps.InstanceStaff.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	staff := make([]timetable.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		staff = append(staff, instanceStaffOf(row))
	}
	return staff, nil
}

func (d *timetableData) ListBlockParticipants(ctx context.Context, instanceID int64) ([]timetable.ScheduledParticipant, error) {
	rows, err := d.deps.Participants.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return scheduledParticipantsOf(rows), nil
}

// ListCareDayCandidates returns the rows whose care-day verdict can still
// change something: still 'expected', plus rows a broad day status (sick,
// excused, class trip) flipped to 'absent' and still owns. The status day
// is written before anything knows whether the child was booked, so an
// unbooked child can already sit there as absent (#1747).
func (d *timetableData) ListCareDayCandidates(ctx context.Context, instanceIDs []int64) ([]timetable.ScheduledParticipant, error) {
	rows, err := d.deps.Participants.FindNotScheduledCandidatesByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, err
	}
	return scheduledParticipantsOf(rows), nil
}

func (d *timetableData) FindBlockParticipant(ctx context.Context, instanceID, studentID int64) (*timetable.ScheduledParticipant, error) {
	row, err := d.deps.Participants.FindByInstanceAndStudent(ctx, instanceID, studentID)
	if modelBase.IsNoRows(err) {
		return nil, nil
	}
	if err != nil || row == nil {
		return nil, err
	}
	participant := ScheduledParticipantOf(row)
	return &participant, nil
}

func (d *timetableData) FindBlockTemplate(ctx context.Context, groupID int64) (timetable.Group, error) {
	return d.deps.Groups.FindGroup(ctx, groupID)
}

func (d *timetableData) BlockRoomName(ctx context.Context, roomID int64) (string, bool, error) {
	return d.deps.Rooms.RoomName(ctx, roomID)
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
	return d.deps.Participants.UpdateAttendanceFields(ctx, participantID, attendanceFieldPatch(patch))
}

func (d *timetableData) ListDeviationEvents(ctx context.Context, from, to timezone.Date, activityGroupID *int64, startTime *string) ([]timetable.DeviationEvent, error) {
	return d.deps.DeviationEvents.ListDeviationEvents(ctx, from, to, activityGroupID, startTime)
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
