package timetable

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The planner's reads of the Timetable owner beside the instance lifecycle
// (#3424 slice S5): the blocks with the state of their sessions, their staff
// and participant rows, the staffing gaps, a child's week, the template
// list, the Änderungsprotokoll and the preparation of a spontaneous start.

// ErrSpontaneousCategoryArchived rejects a spontaneous start while the
// "Spontan" category is archived.
var ErrSpontaneousCategoryArchived = errors.New("spontaneous activity category is archived")

// ScheduledParticipant is one planned child of a block with the Student
// Presence attendance of its slot.
type ScheduledParticipant struct {
	ID           int64
	TenantID     int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	InstanceID   int64
	StudentID    int64
	RoomID       *int64
	Status       string
	Substatus    *string
	Note         *string
	CheckedInAt  *time.Time
	CheckedOutAt *time.Time
	IsUnplanned  bool
	// NotScheduled is the frozen non-booking marker of an ended block
	// (#1747); ManualStatusAt records a status set by hand.
	NotScheduled   bool
	ManualStatusAt *time.Time
	// StudentStatusDayID and PickupExceptionID name the care-plan record
	// that owns an absence.
	StudentStatusDayID *int64
	PickupExceptionID  *int64
}

// ScheduledInstanceRows are the rows of a list window: staff and
// participants keyed by block, the auto-excusal pickup cutoffs keyed by date
// and child.
type ScheduledInstanceRows struct {
	Staff        map[int64][]InstanceStaff
	Participants map[int64][]ScheduledParticipant
	Cutoffs      map[calendar.Date]map[int64]time.Time
}

// ScheduledBlockQuery reads the planner's blocks with the state of their
// sessions and their rows.
type ScheduledBlockQuery interface {
	// FindScheduledInstance reads one block; a missing one is
	// ErrActivityInstanceNotFound.
	FindScheduledInstance(ctx context.Context, id int64) (ScheduledInstance, error)
	// ListScheduledInstances reads the blocks of an inclusive date window.
	ListScheduledInstances(ctx context.Context, from, to calendar.Date) ([]ScheduledInstance, error)
	// ListScheduledInstanceRows loads the rows of the listed blocks with one
	// read per kind plus one cutoff read per distinct date (#2940).
	// Participants keep their creation order.
	ListScheduledInstanceRows(ctx context.Context, instances []ScheduledInstance) (*ScheduledInstanceRows, error)
	ListBlockStaff(ctx context.Context, instanceID int64) ([]InstanceStaff, error)
	ListBlockParticipants(ctx context.Context, instanceID int64) ([]ScheduledParticipant, error)
	// ListCareDayCandidates returns the participants whose care-day verdict
	// can still change something: still expected, or absent through a broad
	// day status that still owns the row (#1747).
	ListCareDayCandidates(ctx context.Context, instanceIDs []int64) ([]ScheduledParticipant, error)
	// FindBlockParticipant reads one child's slot; nil when the child has
	// none on the block.
	FindBlockParticipant(ctx context.Context, instanceID, studentID int64) (*ScheduledParticipant, error)
	// FindBlockTemplate reads the template a block was materialized from.
	FindBlockTemplate(ctx context.Context, groupID int64) (Group, error)
	// BlockRoomName names a block's room; ok is false when it does not exist.
	BlockRoomName(ctx context.Context, roomID int64) (name string, ok bool, err error)
}

// SlotAttendanceCommand edits a planned child's attendance from the planner.
type SlotAttendanceCommand interface {
	// LockBlockAttendance serializes an attendance write with a concurrent
	// completion of the block.
	LockBlockAttendance(ctx context.Context, instanceID int64) error
	PatchSlotAttendance(ctx context.Context, participantID int64, patch AttendancePatch) error
}

// UnderstaffedInstance is one understaffed block of a window with its staff
// counts (see IsUnderstaffed). The block carries its own acknowledgement.
type UnderstaffedInstance struct {
	Instance           ScheduledInstance
	AssignedStaffCount int
	AbsentStaffCount   int
	PresentStaffCount  int
	PlannedStaffCount  int
}

// StaffingGapQuery reports the staffing gaps of the Vertretungsplan (#1840).
type StaffingGapQuery interface {
	// ListUnderstaffedInstances returns the planned and running blocks of the
	// inclusive window that are understaffed, with one block read and one
	// staff read.
	ListUnderstaffedInstances(ctx context.Context, from, to calendar.Date) ([]UnderstaffedInstance, error)
}

// StudentWeekEntry is one block a child is planned into, with the child's
// slot.
type StudentWeekEntry struct {
	Instance   ScheduledInstance
	Attendance ScheduledParticipant
}

// StudentWeekVisit is one visit of the child in a block's session.
type StudentWeekVisit struct {
	ID        int64
	EntryTime time.Time
	ExitTime  *time.Time
}

// StudentWeekTimes are the arrival or pickup of a child on a date as the
// care plan has them: the recurring plan and the day's exception.
type StudentWeekTimes struct {
	// HasSchedule reports a recurring plan for the date; Time is its time,
	// zero when a care day carries no time (#2414).
	HasSchedule bool
	Time        time.Time
	// Exception is the day's exception, when one exists.
	Exception *StudentDayException
}

// StudentDayException is a dated arrival or pickup exception of a child; a
// nil time is an absence on the date.
type StudentDayException struct {
	Time   *time.Time
	Reason *string
}

// StudentWeek is everything a child's day or week view is built from, keyed
// by YYYY-MM-DD, with the visits keyed by live session.
type StudentWeek struct {
	EnrolledByDate      map[string][]StudentWeekEntry
	InstancesByDate     map[string][]ScheduledInstance
	VisitsByActiveGroup map[int64][]StudentWeekVisit
	ArrivalByDate       map[string]StudentWeekTimes
	PickupByDate        map[string]StudentWeekTimes
}

// StudentWeekQuery reads a child's timetable over a window with a fixed
// number of statements.
type StudentWeekQuery interface {
	StudentWeek(ctx context.Context, studentID int64, from, to calendar.Date) (*StudentWeek, error)
}

// TemplateListEntry is one template schedule of the Vorlagen list with the
// template's dynamic targets. Its capacity counts come from the template's
// worst occurrence.
type TemplateListEntry struct {
	TemplateListRow
	Targets []GroupTarget
	// SourceCareOfferingIDs, SourceGradeLevels and SourceSchoolClasses are
	// the decoded offering-source rule (#2137, #2482); a corrupt stored value
	// reads as no filter instead of failing the list.
	SourceCareOfferingIDs []int64
	SourceGradeLevels     []int
	SourceSchoolClasses   []string
}

// TemplateListing reads the Vorlagen list with its capacity (#1839).
type TemplateListing interface {
	ListTemplateEntries(ctx context.Context, templateID *int64, childrenPerStaffRatio int) ([]TemplateListEntry, error)
	ListTemplateEntriesForTemplatePeriod(ctx context.Context, templateID, periodID int64, childrenPerStaffRatio int) ([]TemplateListEntry, error)
	ListTemplateEntriesForPeriod(ctx context.Context, periodID *int64, childrenPerStaffRatio int) ([]TemplateListEntry, error)
	// ListTemplateWeekdayRoster returns the weekday-scoped roster
	// memberships (#2129); a nil period selects only unscoped rows.
	ListTemplateWeekdayRoster(ctx context.Context, templateID, calendarPeriodID *int64) ([]TemplateWeekdayRosterRow, error)
}

// DeviationEvent is one entry of the Änderungsprotokoll (#1886); it stores
// ids only, display names resolve at read time.
type DeviationEvent struct {
	ID              int64
	ActivityGroupID *int64
	OccurrenceDate  calendar.Date
	StartTime       time.Time
	InstanceID      *int64
	EventType       string
	SubjectStaffID  *int64
	RelatedStaffID  *int64
	ActorAccountID  *int64
	OldValue        json.RawMessage
	NewValue        json.RawMessage
	Reason          *string
	OccurredAt      time.Time
}

// DeviationHistoryQuery reads the Änderungsprotokoll of a window, newest
// first, optionally narrowed to one slot.
type DeviationHistoryQuery interface {
	ListDeviationEvents(ctx context.Context, from, to calendar.Date, activityGroupID *int64, startTime *string) ([]DeviationEvent, error)
}

// SpontaneousStartPreparation readies a spontaneous start before its block
// is created. Every lock is transaction-scoped.
type SpontaneousStartPreparation interface {
	SpontaneousRoomExists(ctx context.Context, roomID int64) (bool, error)
	// LockSpontaneousStartRoom serializes starts into the same room.
	LockSpontaneousStartRoom(ctx context.Context, roomID int64) error
	// SpontaneousRoomOccupied reports another open session in the room.
	SpontaneousRoomOccupied(ctx context.Context, roomID int64) (bool, error)
	// ResolveSpontaneousActivity returns the requested activity, else the
	// activity named after the title, creating it in the "Spontan"
	// category (created on first use) when it does not exist.
	ResolveSpontaneousActivity(ctx context.Context, title string, requestedID *int64, createdBy int64) (*int64, error)
}

// TimetableDataCapability is what the planner and the operational views
// read and edit beside the lifecycle.
type TimetableDataCapability interface {
	ScheduledBlockQuery
	SlotAttendanceCommand
	StaffingGapQuery
	StudentWeekQuery
	TemplateListing
	DeviationHistoryQuery
	SpontaneousStartPreparation
	ConflictAckCapability
}

// CanReopenInstance is the reopen gate a list payload announces: a completed
// block inside its reopen window whose completion kept a snapshot, for an
// admin or the account that completed it. Session-end completions write no
// snapshot and stay closed.
func CanReopenInstance(instance ScheduledInstance, accountID int64, isAdmin bool, now time.Time) bool {
	if instance.Status != InstanceStatusCompleted {
		return false
	}
	if instance.ReopenUntil == nil || now.After(*instance.ReopenUntil) || !instance.HasCompletionSnapshot {
		return false
	}
	return CanReopenAsActor(true, instance.CompletedBy, accountID, isAdmin)
}

// AttendanceUnchangedSinceCompletion reports whether no slot of the block was
// written after its completion; a later attendance edit makes the snapshot
// restore unsafe.
func AttendanceUnchangedSinceCompletion(instance ScheduledInstance, participants []ScheduledParticipant) bool {
	if instance.CompletedAt == nil {
		return true
	}
	for _, participant := range participants {
		if participant.UpdatedAt.After(*instance.CompletedAt) {
			return false
		}
	}
	return true
}
