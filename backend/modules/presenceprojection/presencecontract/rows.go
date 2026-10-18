package presencecontract

import (
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

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
type ScheduledInstance struct {
	Instance    Instance
	Participant Participant
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
type LegacyInstanceFilter struct {
	IDs                 []int64
	Date                *string
	Dates               []string
	FromDate            *string
	ToDate              *string
	ActivityGroupID     *int64
	ActivityGroupIDs    []int64
	ActiveGroupIDs      []int64
	Status              string
	IsSpontaneous       *bool
	IdempotencyKey      string
	MaterializedPlanned bool
	OrderByDateAndTime  bool
	Limit               int
	Offset              int
}
type LegacyParticipantFilter struct {
	IDs                        []int64
	InstanceIDs                []int64
	StudentIDs                 []int64
	Status                     *string
	Date                       *string
	FromDate                   *string
	ToDate                     *string
	CurrentTime                *string
	ExcludeFinished            bool
	OrderByCreated             bool
	OrderByInstanceStudent     bool
	OrderByStudentActivityTime bool
	OrderByActivityDateTime    bool
	Limit                      int
	Offset                     int
}
