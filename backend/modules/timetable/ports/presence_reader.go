package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PresenceReader reads activity instances and participant attendance in the
// caller's tenant transaction. The root binds the cross-owner read.
type PresenceReader interface {
	CountNonAbsentParticipants(context.Context, []int64) (map[int64]int, error)
	ListParallelPresence(context.Context, int64, calendar.Date, []int64) ([]ParallelPresence, error)
	ListPartialAbsenceBlocks(context.Context, int64, calendar.Date, time.Time, bool, []int64) ([]PartialAbsenceBlock, error)
	ListScheduledInstancesForStudent(context.Context, int64, calendar.Date, calendar.Date) ([]ScheduledInstance, error)
	HasPlannedSlotsInRange(context.Context, calendar.Date, calendar.Date) (bool, error)
	ListInstances(context.Context, LegacyInstanceFilter) ([]ActivityInstance, error)
	ListParticipants(context.Context, LegacyParticipantFilter) ([]ParticipantAttendance, error)
}

// ActivityInstance is an activity block with its execution state.
type ActivityInstance struct {
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

// ParticipantAttendance is a planned participant with attendance state.
type ParticipantAttendance struct {
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
type ScheduledInstance struct {
	Instance    ActivityInstance
	Participant ParticipantAttendance
}
type ParallelPresence struct {
	StudentID  int64
	InstanceID int64
	Title      string
	StartTime  time.Time
	EndTime    time.Time
}
type PartialAbsenceBlock struct {
	ID        int64
	Title     string
	StartTime time.Time
	EndTime   time.Time
}
