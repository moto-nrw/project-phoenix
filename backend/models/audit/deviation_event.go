package audit

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Event types for audit.deviation_events. Open vocabulary: the column has no
// CHECK constraint, so follow-up features (#1841 day changes, #1843 sickness
// lifecycle, #1884 staff moves) add constants here without a migration.
const (
	DeviationEventAbsence           = "absence"
	DeviationEventReturnToPresence  = "return_to_presence"
	DeviationEventSubstitution      = "substitution"
	DeviationEventSubstituteRemoved = "substitute_removed"
	DeviationEventCancellation      = "cancellation"
	DeviationEventUnderstaffedAck   = "understaffed_ack"
	DeviationEventUnderstaffedUnack = "understaffed_unack"
	// A sick report (#1843) marked this staff row absent / cleared that mark
	// again when the report was deleted. Distinct from the manual "absence" so
	// history separates Krankmeldung from an ordinary Vertretungsplan edit;
	// new_value carries {"cause":"sick","absence_id":<id>}.
	DeviationEventSickReported = "sick_reported"
	DeviationEventSickCleared  = "sick_cleared"
	// A re-plan or template split could not re-attach a snapshotted deviation
	// because its slot no longer regenerates.
	DeviationEventDroppedByReplan = "deviation_dropped_by_replan"
	// A planned-instance roster edit discarded existing deviation flags.
	DeviationEventDroppedByEdit = "deviation_dropped_by_edit"
	// A staff member was moved between two same-day blocks in one atomic save
	// (#1884). Anchored on the TARGET instance; old_value names the source
	// block, new_value the target. An assign-from-pool (no source block) omits
	// old_value.
	DeviationEventStaffMoved = "staff_moved"
	// A Dienstplan shift was moved to another person and/or slot (#1884).
	// Anchored via StaffShiftID (no activity slot exists for a shift);
	// old_value/new_value carry {staff_id, date, start_time, end_time}.
	DeviationEventShiftMoved = "shift_moved"
)

// DeviationEvent is one append-only entry in the planning Änderungsprotokoll
// (#1886). Rows are never updated or deleted by application code; only the
// GDPR retention job removes old rows. The slot anchor (ActivityGroupID,
// OccurrenceDate, StartTime) is the identity that survives re-plans —
// InstanceID is a convenience pointer that a re-plan nulls out. No names or
// other PII beyond staff/account IDs are stored; display names resolve at
// read time.
type DeviationEvent struct {
	ID               int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel `bun:"schema:audit,table:deviation_events"`

	// ActivityGroupID is nil for spontaneous instances (no template); those
	// anchor via InstanceID, which re-plans never delete.
	ActivityGroupID *int64        `bun:"activity_group_id" json:"activity_group_id,omitempty"`
	OccurrenceDate  timezone.Date `bun:"occurrence_date,notnull,type:date" json:"occurrence_date"`
	// StartTime maps a TIME column; only the wall-clock portion is meaningful
	// (same convention as schedule.activity_instances.start_time).
	StartTime time.Time `bun:"start_time,notnull" json:"start_time"`

	InstanceID *int64 `bun:"instance_id" json:"instance_id,omitempty"`

	// StaffShiftID anchors events on a schedule.staff_shifts row (#1884 shift
	// moves). A shift is not an activity slot, so neither ActivityGroupID nor
	// InstanceID can identify it; shift-anchored rows are excluded from the
	// slot-scoped Betreuungsplan history read.
	StaffShiftID *int64 `bun:"staff_shift_id" json:"staff_shift_id,omitempty"`

	SubjectStaffID *int64 `bun:"subject_staff_id" json:"subject_staff_id,omitempty"`
	RelatedStaffID *int64 `bun:"related_staff_id" json:"related_staff_id,omitempty"`

	ActorAccountID *int64 `bun:"actor_account_id" json:"actor_account_id,omitempty"`

	EventType string `bun:"event_type,notnull" json:"event_type"`

	OldValue json.RawMessage `bun:"old_value,type:jsonb" json:"old_value,omitempty"`
	NewValue json.RawMessage `bun:"new_value,type:jsonb" json:"new_value,omitempty"`

	Reason *string `bun:"reason" json:"reason,omitempty"`

	OccurredAt time.Time `bun:"occurred_at,notnull,default:now()" json:"occurred_at"`
}

func (e *DeviationEvent) GetID() interface{} {
	return e.ID
}

func (e *DeviationEvent) GetCreatedAt() time.Time {
	return e.OccurredAt
}

func (e *DeviationEvent) GetUpdatedAt() time.Time {
	return e.OccurredAt
}

// Validate checks the minimal invariants before an insert.
func (e *DeviationEvent) Validate() error {
	if (e.ActivityGroupID == nil || *e.ActivityGroupID <= 0) &&
		(e.InstanceID == nil || *e.InstanceID <= 0) &&
		(e.StaffShiftID == nil || *e.StaffShiftID <= 0) {
		return errors.New("activity_group_id, instance_id or staff_shift_id is required")
	}
	if e.OccurrenceDate.IsZero() {
		return errors.New("occurrence_date is required")
	}
	if e.EventType == "" {
		return errors.New("event_type is required")
	}
	return nil
}

// DeviationEventRepository is deliberately minimal: append + one filtered
// read + the retention delete.
type DeviationEventRepository interface {
	Create(ctx context.Context, event *DeviationEvent) error
	// ListByRange returns events with occurrence_date in [from, to], newest
	// first, optionally narrowed to one slot via activityGroupID and
	// startTime ("HH:MM").
	ListByRange(ctx context.Context, from, to timezone.Date, activityGroupID *int64, startTime *string) ([]*DeviationEvent, error)
	// DeleteOlderThan removes events whose occurrence_date is before the
	// cutoff (GDPR retention, rides the timetable retention window).
	DeleteOlderThan(ctx context.Context, cutoff timezone.Date) (int64, error)
}
