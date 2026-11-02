package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

type staffDeviations struct {
	substituteConflicts
	deps ports.StaffDeviationDependencies
}

var _ timetable.StaffDeviations = (*staffDeviations)(nil)

// NewStaffDeviations composes the Timetable owner's deviation writes.
func NewStaffDeviations(deps ports.StaffDeviationDependencies) (timetable.StaffDeviations, error) {
	if noInstanceRows(deps.Instances) || deps.InstanceStaff == nil || deps.Staff == nil || deps.Supervisions == nil ||
		deps.Lifecycle == nil || deps.Protocol == nil || deps.ActivityUpdates == nil || deps.Locks == nil {
		return nil, errors.New("timetable staff deviations: required dependency is nil")
	}
	return &staffDeviations{
		substituteConflicts: substituteConflicts{instances: deps.Instances.Reads, staff: deps.InstanceStaff},
		deps:                deps,
	}, nil
}

func (s *staffDeviations) now() time.Time {
	if s.deps.Now != nil {
		return s.deps.Now()
	}
	return time.Now()
}

func (s *staffDeviations) getLogger() *slog.Logger {
	return OrDefaultLogger(s.deps.Logger)
}

// acquireSubstituteDayLock takes the shared (tenant, date) advisory lock that
// serializes every same-day staffing mutation, within the caller's tx.
// Take it before instance/participant row locks; if recurrence is needed,
// take recurrence first (see InstanceLifecycleService.LockDeviationDays).
func (s *staffDeviations) acquireSubstituteDayLock(ctx context.Context, date calendar.Date) error {
	return s.deps.Locks.AcquireTransactionLock(ctx, timetable.SubstituteDayLockKey(tenant.FromContext(ctx), date))
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
		Date:       calendar.Date(instance.Date),
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
// tenant tx.
func (s *staffDeviations) logDeviationEvent(ctx context.Context, in deviationEventInput) error {
	return recordDeviationEvent(ctx, s.deps.Protocol, in)
}

// recordDeviationEvent appends one Änderungsprotokoll entry inside the
// caller's tenant tx. Fail closed: an error here must abort the surrounding
// mutation — the protocol is the compliance artifact, not a best-effort side
// channel.
func recordDeviationEvent(ctx context.Context, protocol ports.DeviationProtocol, in deviationEventInput) error {
	event := ports.DeviationEventRecord{
		ActivityGroupID: in.instance.ActivityGroupID,
		OccurrenceDate:  calendar.Date(in.instance.Date),
		StartTime:       calendar.NormalizeWallClock(in.instance.StartTime),
		InstanceID:      &in.instance.ID,
		SubjectStaffID:  in.subjectStaffID,
		RelatedStaffID:  in.relatedStaffID,
		EventType:       in.eventType,
		Reason:          in.reason,
		ActorAccountID:  normalizeActor(in.actorAccountID),
	}
	var err error
	if event.OldValue, err = marshalDeviationValue(in.oldValue); err != nil {
		return &ports.ScheduleError{Op: "log deviation event: marshal old value", Err: err}
	}
	if event.NewValue, err = marshalDeviationValue(in.newValue); err != nil {
		return &ports.ScheduleError{Op: "log deviation event: marshal new value", Err: err}
	}
	if err := protocol.RecordDeviationEvent(ctx, event); err != nil {
		return &ports.ScheduleError{Op: "log deviation event", Err: fmt.Errorf("event_type %s: %w", in.eventType, err)}
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
