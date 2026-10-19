package schedulerecords

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Activity instance status constants. Stored as TEXT with a CHECK constraint
// rather than a DB enum so adding states later does not require an ALTER TYPE.
const (
	InstanceStatusPlanned   = "planned"
	InstanceStatusActive    = "active"
	InstanceStatusCompleted = "completed"
	InstanceStatusCancelled = "cancelled"
)

const (
	// ActivityInstanceTitleMaxLength is the maximum length of the title field.
	ActivityInstanceTitleMaxLength = 255
	// ActivityInstanceIdempotencyKeyMaxLength bounds client-generated create keys.
	ActivityInstanceIdempotencyKeyMaxLength = 128
)

// templateTypeDuty mirrors activities.GroupTypeDuty: the duty block type
// (#3822), the one template type whose occurrences may lack a room.
const templateTypeDuty = "duty"

// ActivityInstance is the concrete materialized occurrence of a template on a
// given date (or a spontaneous instance created without a template). It lives
// in the "instance layer" between the template layer (activities.*) and the
// live layer (active.*).
//
// It is a composed legacy DTO. schedule.activity_instances stores the plan,
// whose status is only planned or cancelled; Student Presence stores the
// execution (status active or completed, ActiveGroupID, StartedBy through
// CompletionSnapshot) in active.activity_sessions (#2762). Since the Contract
// (#2763) the plan table has no execution columns: write and read those
// fields only through the owner commands or the presence projection.
type ActivityInstance struct {
	Model `bun:"schema:schedule,table:activity_instances"`
	TenantModel

	Date             calendar.Date `bun:"date,notnull" json:"date"`
	ActivityGroupID  *int64        `bun:"activity_group_id" json:"activity_group_id,omitempty"`
	CalendarPeriodID *int64        `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`
	Title            string        `bun:"title,notnull" json:"title"`
	Description      *string       `bun:"description" json:"description,omitempty"`
	StartTime        time.Time     `bun:"start_time,notnull" json:"start_time"`
	EndTime          time.Time     `bun:"end_time,notnull" json:"end_time"`
	// RoomID 0 binds NULL: only a duty occurrence has no room (#3822).
	RoomID int64 `bun:"room_id,nullzero" json:"room_id"`
	// RequiredStaff is the per-occurrence Personalbedarf pin (issue #1839).
	// NULL means "inherit": template-backed instances fall back to the
	// template's override, then to the Betreuungsschlüssel (issue #1869).
	// Materialization deliberately leaves this NULL; a set value (>= 0) is
	// always a single-occurrence pin, which is what lets ReplanWeek preserve
	// it while template edits still propagate. See
	// modules/timetable/staffing.go EffectiveRequiredStaff.
	RequiredStaff *int   `bun:"required_staff" json:"required_staff,omitempty"`
	Status        string `bun:"status,notnull,default:'planned'" json:"status"`
	ActiveGroupID *int64 `bun:"active_group_id" json:"active_group_id,omitempty"`
	// ListKind classifies the instance for printable daily lists (issue #1565).
	// Copied from the template at materialization time; NULL means no list kind.
	ListKind      *string `bun:"list_kind" json:"list_kind,omitempty"`
	IsSpontaneous bool    `bun:"is_spontaneous,notnull,default:false" json:"is_spontaneous"`
	// UnderstaffedAck records that an admin deliberately accepts this block
	// running with zero staff (Vertretungsplan, issue #1840). When true the gap
	// detector reports the block as an acknowledged shortfall instead of an open
	// gap — the staffing hole stays visible, it just stops nagging.
	UnderstaffedAck  bool    `bun:"understaffed_ack,notnull,default:false" json:"understaffed_ack"`
	UnderstaffedNote *string `bun:"understaffed_note" json:"understaffed_note,omitempty"`
	// CancelReason is an optional short "why" captured when a block is cancelled
	// (Vertretungsplan, issue #1840).
	CancelReason           *string `bun:"cancel_reason" json:"cancel_reason,omitempty"`
	Notes                  *string `bun:"notes" json:"notes,omitempty"`
	IdempotencyKey         *string `bun:"idempotency_key" json:"-"`
	IdempotencyFingerprint *string `bun:"idempotency_fingerprint" json:"-"`
	CreatedBy              *int64  `bun:"created_by" json:"created_by,omitempty"`
	// TemplateType is the block type of the linked template (#3822), read
	// along by the presence projection's instance list; empty without a
	// template or outside that read. Not a column of this table.
	TemplateType string `bun:"-" json:"-"`
	// TemplateRequiredStaff is the template's Personalbedarf override, read
	// along with TemplateType; a duty's gap rule needs it (#3822).
	TemplateRequiredStaff *int            `bun:"-" json:"-"`
	StartedBy             *int64          `bun:"started_by" json:"started_by,omitempty"`
	StartedAt             *time.Time      `bun:"started_at" json:"started_at,omitempty"`
	CompletedAt           *time.Time      `bun:"completed_at" json:"completed_at,omitempty"`
	CompletedBy           *int64          `bun:"completed_by" json:"completed_by,omitempty"`
	ReopenUntil           *time.Time      `bun:"reopen_until" json:"reopen_until,omitempty"`
	CompletionSnapshot    json.RawMessage `bun:"completion_snapshot,type:jsonb" json:"-"`
}

// MarkCompleted applies the persisted lifecycle state after the live session
// has ended. completedAt is supplied by the persistence layer because it is
// also the baseline for detecting later recovery conflicts.
func (i *ActivityInstance) MarkCompleted(completedAt, reopenUntil time.Time, completedBy int64) {
	i.Status = InstanceStatusCompleted
	i.CompletedAt = &completedAt
	i.ReopenUntil = &reopenUntil
	if completedBy > 0 {
		i.CompletedBy = &completedBy
	}
}

type CompletionAttendanceSnapshot struct {
	RowID              int64      `json:"row_id"`
	Status             string     `json:"status"`
	Substatus          *string    `json:"substatus,omitempty"`
	Note               *string    `json:"note,omitempty"`
	CheckedInAt        *time.Time `json:"checked_in_at,omitempty"`
	CheckedOutAt       *time.Time `json:"checked_out_at,omitempty"`
	NotScheduled       bool       `json:"not_scheduled"`
	StudentStatusDayID *int64     `json:"student_status_day_id,omitempty"`
	PickupExceptionID  *int64     `json:"pickup_exception_id,omitempty"`
}

type ActivityCompletionSnapshot struct {
	ActiveGroupID int64                          `json:"active_group_id"`
	VisitIDs      []int64                        `json:"visit_ids"`
	SupervisorIDs []int64                        `json:"supervisor_ids"`
	Attendance    []CompletionAttendanceSnapshot `json:"attendance"`
}

// Validate ensures activity instance data is valid for persistence.
func (i *ActivityInstance) Validate() error {
	if i.Title == "" {
		return errors.New("title is required")
	}
	if len(i.Title) > ActivityInstanceTitleMaxLength {
		return errors.New("title cannot exceed 255 characters")
	}
	if i.Date.IsZero() {
		return errors.New("date is required")
	}
	if i.StartTime.IsZero() {
		return errors.New("start_time is required")
	}
	if i.EndTime.IsZero() {
		return errors.New("end_time is required")
	}
	if !i.EndTime.After(i.StartTime) {
		return errors.New("end_time must be after start_time")
	}
	// Only a duty (#3822) may lack a room; the type comes from its template.
	if i.RoomID < 0 || (i.RoomID == 0 && i.TemplateType != templateTypeDuty) {
		return errors.New("room_id is required")
	}
	if !IsValidInstanceStatus(i.Status) {
		return errors.New("invalid instance status")
	}
	if i.RequiredStaff != nil && *i.RequiredStaff < 0 {
		return errors.New("required_staff cannot be negative")
	}
	return i.validateMetadata()
}

func (i *ActivityInstance) validateMetadata() error {
	if i.IdempotencyKey != nil {
		if strings.TrimSpace(*i.IdempotencyKey) == "" {
			return errors.New("idempotency_key cannot be blank")
		}
		if len(*i.IdempotencyKey) > ActivityInstanceIdempotencyKeyMaxLength {
			return errors.New("idempotency_key is too long")
		}
	}
	// Canonicalize a non-nil pointer to "" to NULL so it satisfies the DB's
	// `list_kind IS NULL OR list_kind IN (...)` CHECK instead of hitting a
	// constraint error (IsValidListKind("") stays true for the slot-list
	// filter, where empty means "any kind").
	if i.ListKind != nil && *i.ListKind == "" {
		i.ListKind = nil
	}
	if i.ListKind != nil && !isValidInstanceListKind(*i.ListKind) {
		return errors.New("invalid list kind")
	}
	return nil
}

func isValidInstanceListKind(kind string) bool {
	switch kind {
	case "", "edge_hours", "learning_time", "activity", "mensa":
		return true
	default:
		return false
	}
}

// IsValidInstanceStatus returns true if s is one of the four permitted
// activity instance statuses.
func IsValidInstanceStatus(s string) bool {
	switch s {
	case InstanceStatusPlanned, InstanceStatusActive, InstanceStatusCompleted, InstanceStatusCancelled:
		return true
	}
	return false
}

// IsTemplateBacked reports whether the instance was materialized from a template
// (as opposed to being created spontaneously by staff).
func (i *ActivityInstance) IsTemplateBacked() bool {
	return i.ActivityGroupID != nil
}

// IsLive reports whether the instance is currently running.
func (i *ActivityInstance) IsLive() bool {
	return i.Status == InstanceStatusActive
}

// ports.ActivityInstanceRecords defines operations for managing activity instances.

// ports.InstanceIdempotencyRecords owns the conflict-safe insert used by
// client-request replay protection. Keeping it separate avoids forcing every
// activity-instance reader or test fake to implement a create-only concern.
