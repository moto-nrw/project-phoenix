package ports

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Änderungsprotokoll event types the deviation writes record; they match
// the Audit Platform's deviation event vocabulary.
const (
	DeviationEventAbsence           = "absence"
	DeviationEventReturnToPresence  = "return_to_presence"
	DeviationEventSubstitution      = "substitution"
	DeviationEventSubstituteRemoved = "substitute_removed"
	DeviationEventSickReported      = "sick_reported"
	DeviationEventSickCleared       = "sick_cleared"
)

// DeviationEventRecord is one Änderungsprotokoll entry before it is stored.
type DeviationEventRecord struct {
	ActivityGroupID *int64
	OccurrenceDate  calendar.Date
	StartTime       time.Time
	InstanceID      *int64
	SubjectStaffID  *int64
	RelatedStaffID  *int64
	EventType       string
	OldValue        json.RawMessage
	NewValue        json.RawMessage
	Reason          *string
	ActorAccountID  *int64
}

// DeviationProtocol appends Änderungsprotokoll entries in the caller's
// transaction; the composition root binds the Audit Platform's repository.
type DeviationProtocol interface {
	RecordDeviationEvent(ctx context.Context, event DeviationEventRecord) error
}

// DeviationCancellation is the exclusive cancel branch of a deviations save.
type DeviationCancellation struct {
	InstanceID     int64
	Reason         *string
	ActorAccountID *int64
	GuardianNotice *timetable.GuardianNoticeInput
}

// CancelledBlock is what the lifecycle reports after a cancellation.
type CancelledBlock struct {
	InstanceID      int64
	UnderstaffedAck bool
	GuardianNotice  *timetable.GuardianNoticeResult
}

// DeviationLifecycle is the instance lifecycle the deviation saves hand the
// cancellation and the "deliberately unstaffed" acknowledgement to. Its
// errors pass through unchanged, so the handlers keep their lifecycle
// mapping.
type DeviationLifecycle interface {
	CancelBlock(ctx context.Context, in DeviationCancellation) (CancelledBlock, error)
	SetUnderstaffedAck(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) error
	ClearUnderstaffedAckIfStaffed(ctx context.Context, instanceID int64, actorAccountID *int64) error
}

// ActivityUpdateAnnouncer delivers one activity update of a running block to
// the session's subscribers; the composition root binds the realtime hub.
type ActivityUpdateAnnouncer interface {
	AnnounceActivityUpdate(tenantID, activeGroupID int64, update timetable.TouchedActivity) error
}

// DeviationStaff are the staff rows the deviation writes, the staff pool and
// the shift coverage read.
type DeviationStaff interface {
	FindByID(ctx context.Context, id any) (*timetable.StaffMember, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error)
	ListAllWithPerson(ctx context.Context) ([]*timetable.StaffMember, error)
}

// StaffDeviationDependencies wires the deviation writes. Every field but
// Logger and Now is required.
type StaffDeviationDependencies struct {
	Instances       ActivityInstanceRecords
	InstanceStaff   InstanceStaffRecords
	Staff           DeviationStaff
	Supervisions    studentpresence.SupervisionRecords
	Lifecycle       DeviationLifecycle
	Protocol        DeviationProtocol
	ActivityUpdates ActivityUpdateAnnouncer
	Locks           TransactionLocks
	Logger          *slog.Logger
	Now             func() time.Time
}
