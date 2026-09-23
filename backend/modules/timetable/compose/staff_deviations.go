package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Staff deviations (Vertretungsplan, #1840/#1886, #3424 slice S3) behind
// timetable.StaffDeviations: the day-wide absence, presence and substitution
// writes, the atomic single-day save (deviation_apply.go), the
// Sammel-Vertretung (bulk_substitution.go) and the #1843 sick-report writes.
// Every write appends its Änderungsprotokoll entry in the caller's tenant
// transaction; a protocol failure aborts the mutation (fail closed).
//
// Collaborators the owner may not name are consumer-owned ports the
// composition root binds: the Audit Platform's protocol, the retained
// instance lifecycle (cancellation and the "deliberately unstaffed"
// acknowledgement) and the realtime activity update.

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
	OccurrenceDate  timezone.Date
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

// StaffDeviationDependencies wires the deviation writes. Every field but
// Logger and Now is required.
type StaffDeviationDependencies struct {
	Instances       scheduleModel.ActivityInstanceRepository
	InstanceStaff   scheduleModel.InstanceStaffRepository
	Staff           usersModel.StaffRepository
	Supervisions    studentpresence.SupervisionRecords
	Lifecycle       DeviationLifecycle
	Protocol        DeviationProtocol
	ActivityUpdates ActivityUpdateAnnouncer
	DB              *bun.DB
	Logger          *slog.Logger
	Now             func() time.Time
}

type staffDeviations struct {
	substituteConflicts
	deps  StaffDeviationDependencies
	store *postgres.Store
}

var _ timetable.StaffDeviations = (*staffDeviations)(nil)

// NewStaffDeviations composes the Timetable owner's deviation writes.
func NewStaffDeviations(deps StaffDeviationDependencies) (timetable.StaffDeviations, error) {
	if deps.Instances == nil || deps.InstanceStaff == nil || deps.Staff == nil || deps.Supervisions == nil ||
		deps.Lifecycle == nil || deps.Protocol == nil || deps.ActivityUpdates == nil || deps.DB == nil {
		return nil, errors.New("timetable staff deviations: required dependency is nil")
	}
	return &staffDeviations{
		substituteConflicts: substituteConflicts{instances: deps.Instances, staff: deps.InstanceStaff},
		deps:                deps,
		store:               postgres.New(databaseRuntime(deps.DB)),
	}, nil
}

func (s *staffDeviations) now() time.Time {
	if s.deps.Now != nil {
		return s.deps.Now()
	}
	return time.Now()
}

func (s *staffDeviations) getLogger() *slog.Logger {
	return orDefaultLogger(s.deps.Logger)
}

// acquireSubstituteDayLock takes the shared (tenant, date) advisory lock that
// serializes every same-day staffing mutation, within the caller's tx.
func (s *staffDeviations) acquireSubstituteDayLock(ctx context.Context, date timezone.Date) error {
	return s.store.AcquireTransactionLock(ctx, timetable.SubstituteDayLockKey(tenant.FromContext(ctx), date))
}

// QueueActivityUpdates queues one activity update per touched running block
// until the surrounding tenant transaction commits. The update is a refetch
// trigger and carries only the slot identity.
func (s *staffDeviations) QueueActivityUpdates(ctx context.Context, touched timetable.TouchedActivities) {
	if len(touched) == 0 {
		return
	}
	tenantID := tenant.FromContext(ctx)
	for activeGroupID, update := range touched {
		tenant.RegisterAfterCommit(ctx, func() {
			if err := s.deps.ActivityUpdates.AnnounceActivityUpdate(tenantID, activeGroupID, update); err != nil {
				s.getLogger().Warn("SSE activity update broadcast failed",
					slog.Int64("active_group_id", activeGroupID),
					slog.String("error", err.Error()),
				)
			}
		})
	}
}

// touch records a running block in touched.
func touch(touched timetable.TouchedActivities, instance *scheduleModel.ActivityInstance) {
	if instance.Status != scheduleModel.InstanceStatusActive || instance.ActiveGroupID == nil {
		return
	}
	touched[*instance.ActiveGroupID] = timetable.TouchedActivity{
		InstanceID: instance.ID,
		Date:       timezone.Date(instance.Date),
		StartTime:  instance.StartTime,
	}
}

// deviationEventInput bundles one protocol entry before marshalling.
type deviationEventInput struct {
	instance       *scheduleModel.ActivityInstance
	eventType      string
	subjectStaffID *int64
	relatedStaffID *int64
	oldValue       any
	newValue       any
	reason         *string
	actorAccountID *int64
}

// logDeviationEvent appends one Änderungsprotokoll entry inside the caller's
// tenant tx. Fail closed: an error here must abort the surrounding mutation —
// the protocol is the compliance artifact, not a best-effort side channel.
func (s *staffDeviations) logDeviationEvent(ctx context.Context, in deviationEventInput) error {
	event := DeviationEventRecord{
		ActivityGroupID: in.instance.ActivityGroupID,
		OccurrenceDate:  timezone.Date(in.instance.Date),
		StartTime:       timezone.NormalizeWallClock(in.instance.StartTime),
		InstanceID:      &in.instance.ID,
		SubjectStaffID:  in.subjectStaffID,
		RelatedStaffID:  in.relatedStaffID,
		EventType:       in.eventType,
		Reason:          in.reason,
		ActorAccountID:  normalizeActor(in.actorAccountID),
	}
	var err error
	if event.OldValue, err = marshalDeviationValue(in.oldValue); err != nil {
		return &ScheduleError{Op: "log deviation event: marshal old value", Err: err}
	}
	if event.NewValue, err = marshalDeviationValue(in.newValue); err != nil {
		return &ScheduleError{Op: "log deviation event: marshal new value", Err: err}
	}
	if err := s.deps.Protocol.RecordDeviationEvent(ctx, event); err != nil {
		return &ScheduleError{Op: "log deviation event", Err: fmt.Errorf("event_type %s: %w", in.eventType, err)}
	}
	return nil
}

func marshalDeviationValue(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// normalizeActor maps a missing/zero actor to nil so the row stores NULL
// instead of a fabricated id.
func normalizeActor(actorAccountID *int64) *int64 {
	if actorAccountID == nil || *actorAccountID <= 0 {
		return nil
	}
	return actorAccountID
}

// affectedOf builds a neutral affected-instance row.
func affectedOf(inst *scheduleModel.ActivityInstance, action string) timetable.DeviationAffected {
	return timetable.DeviationAffected{
		InstanceID: inst.ID,
		Title:      inst.Title,
		StartTime:  inst.StartTime,
		Action:     action,
	}
}

// sameNote reports whether two optional notes carry the same text.
func sameNote(a, b *string) bool {
	av := ""
	if a != nil {
		av = *a
	}
	bv := ""
	if b != nil {
		bv = *b
	}
	return av == bv
}

// trimDeviationReason normalizes an optional deviation reason: nil/blank becomes
// nil, and an over-long value is truncated to the shared note ceiling (rune-
// based so a multi-byte rune is never split).
func trimDeviationReason(reason *string) *string {
	if reason == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*reason)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > scheduleModel.ActivityExceptionReasonMaxLength {
		trimmed = string([]rune(trimmed)[:scheduleModel.ActivityExceptionReasonMaxLength])
	}
	return &trimmed
}
