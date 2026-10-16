package presenceprojection

import (
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The projection answers in its own vocabulary (ADR 0058); the Timetable
// composition maps the rows onto the retained schedule DTOs.

// The planning and execution states the legacy readers compare against.
const (
	statusPlanned   = "planned"
	statusActive    = "active"
	statusCompleted = "completed"
	statusCancelled = "cancelled"
)

// careGroupType is the activities.groups type of a care block template.
const careGroupType = "care"

// Instance is one activity instance with its execution folded onto the
// legacy status vocabulary: a started block that is not cancelled carries
// the session's status and execution fields.
type Instance struct {
	ID, TenantID                      int64
	CreatedAt, UpdatedAt              time.Time
	Date                              calendar.Date
	ActivityGroupID, CalendarPeriodID *int64
	Title                             string
	Description                       *string
	StartTime, EndTime                time.Time
	RoomID                            int64
	RequiredStaff                     *int
	Status                            string
	ListKind                          *string
	IsSpontaneous, UnderstaffedAck    bool
	UnderstaffedNote, CancelReason    *string
	Notes                             *string
	IdempotencyKey                    *string
	IdempotencyFingerprint            *string
	CreatedBy                         *int64
	TemplateType                      string
	TemplateRequiredStaff             *int
	ActiveGroupID, StartedBy          *int64
	StartedAt, CompletedAt            *time.Time
	CompletedBy                       *int64
	ReopenUntil                       *time.Time
	CompletionSnapshot                json.RawMessage
}

// Participant is one planned participant of a block with its attendance; a
// participant without an attendance row is expected.
type Participant struct {
	ID, TenantID                          int64
	CreatedAt, UpdatedAt                  time.Time
	InstanceID, StudentID                 int64
	RoomID                                *int64
	Status                                string
	Substatus, Note                       *string
	CheckedInAt, CheckedOutAt             *time.Time
	IsUnplanned, NotScheduled             bool
	ManualStatusAt                        *time.Time
	StudentStatusDayID, PickupExceptionID *int64
}

// ScheduledInstance is one planned block of a child with the attendance the
// child's participant row carries.
type ScheduledInstance struct {
	Instance    Instance
	Participant Participant
}

// ParallelPresence is another running block of the day a child is still
// present in.
type ParallelPresence struct {
	StudentID  int64
	InstanceID int64
	Title      string
	StartTime  time.Time
	EndTime    time.Time
}

// PartialAbsenceBlock is one block an earlier pickup can still excuse.
type PartialAbsenceBlock struct {
	ID        int64     `bun:"id"`
	Title     string    `bun:"title"`
	StartTime time.Time `bun:"start_time"`
	EndTime   time.Time `bun:"end_time"`
}
