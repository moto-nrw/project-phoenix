package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Activity instance status constants. Stored as TEXT with a CHECK constraint
// rather than a DB enum so adding states later does not require an ALTER TYPE.
const (
	InstanceStatusPlanned   = "planned"
	InstanceStatusActive    = "active"
	InstanceStatusCompleted = "completed"
	InstanceStatusCancelled = "cancelled"
)

// ActivityInstanceTitleMaxLength is the maximum length of the title field.
const ActivityInstanceTitleMaxLength = 255

// ActivityInstance is the concrete materialized occurrence of a template on a
// given date (or a spontaneous instance created without a template). It lives
// in the "instance layer" between the template layer (activities.*) and the
// live layer (active.*) — see docs/timetable-system-plan.md §5.1.
type ActivityInstance struct {
	base.Model `bun:"schema:schedule,table:activity_instances"`
	base.TenantModel

	Date             timezone.Date `bun:"date,notnull" json:"date"`
	ActivityGroupID  *int64        `bun:"activity_group_id" json:"activity_group_id,omitempty"`
	CalendarPeriodID *int64        `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`
	Title            string        `bun:"title,notnull" json:"title"`
	Description      *string       `bun:"description" json:"description,omitempty"`
	StartTime        time.Time     `bun:"start_time,notnull" json:"start_time"`
	EndTime          time.Time     `bun:"end_time,notnull" json:"end_time"`
	RoomID           int64         `bun:"room_id,notnull" json:"room_id"`
	// RequiredStaff is the per-occurrence Personalbedarf pin (issue #1839).
	// NULL means "inherit": template-backed instances fall back to the
	// template's override, then to the Betreuungsschlüssel (issue #1869).
	// Materialization deliberately leaves this NULL; a set value (>= 0) is
	// always a single-occurrence pin, which is what lets ReplanWeek preserve
	// it while template edits still propagate. See
	// services/schedule/capacity_service.go EffectiveRequiredStaff.
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
	CancelReason       *string         `bun:"cancel_reason" json:"cancel_reason,omitempty"`
	Notes              *string         `bun:"notes" json:"notes,omitempty"`
	CreatedBy          *int64          `bun:"created_by" json:"created_by,omitempty"`
	StartedBy          *int64          `bun:"started_by" json:"started_by,omitempty"`
	StartedAt          *time.Time      `bun:"started_at" json:"started_at,omitempty"`
	CompletedAt        *time.Time      `bun:"completed_at" json:"completed_at,omitempty"`
	CompletedBy        *int64          `bun:"completed_by" json:"completed_by,omitempty"`
	ReopenUntil        *time.Time      `bun:"reopen_until" json:"reopen_until,omitempty"`
	CompletionSnapshot json.RawMessage `bun:"completion_snapshot,type:jsonb" json:"-"`
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

type ActivityRecoveryRepository interface {
	// LockOpenVisits takes FOR UPDATE locks on every still-open visit of the
	// group so completion can snapshot the same rows EndActivitySession closes.
	LockOpenVisits(ctx context.Context, activeGroupID int64) error
	// LockOpenSupervisors locks every still-open supervisor of the group so
	// completion snapshots the same rows EndActivitySession closes.
	LockOpenSupervisors(ctx context.Context, activeGroupID int64) error
	// LockSupervisors locks the snapshot supervisor rows during reopen so a
	// concurrent staffing change cannot hide from the unchanged-row check.
	LockSupervisors(ctx context.Context, supervisorIDs []int64) error
	// LockAttendance locks every instance_students row of the instance so
	// reopen can refuse a restore after a post-completion attendance edit.
	LockAttendance(ctx context.Context, instanceID int64) error
	Restore(ctx context.Context, instanceID int64, snapshot ActivityCompletionSnapshot, now time.Time) error
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
	if i.RoomID <= 0 {
		return errors.New("room_id is required")
	}
	if !IsValidInstanceStatus(i.Status) {
		return errors.New("invalid instance status")
	}
	if i.RequiredStaff != nil && *i.RequiredStaff < 0 {
		return errors.New("required_staff cannot be negative")
	}
	// Canonicalize a non-nil pointer to "" to NULL so it satisfies the DB's
	// `list_kind IS NULL OR list_kind IN (...)` CHECK instead of hitting a
	// constraint error (IsValidListKind("") stays true for the slot-list
	// filter, where empty means "any kind").
	if i.ListKind != nil && *i.ListKind == "" {
		i.ListKind = nil
	}
	if i.ListKind != nil && !activitiesModel.IsValidListKind(*i.ListKind) {
		return errors.New("invalid list kind")
	}
	return nil
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

// ActivityInstanceRepository defines operations for managing activity instances.
type ActivityInstanceRepository interface {
	base.Repository[*ActivityInstance]

	// CreateTemplateBackedIfAbsent inserts a template-backed instance and
	// returns inserted=false when the unique template slot already exists.
	CreateTemplateBackedIfAbsent(ctx context.Context, instance *ActivityInstance) (inserted bool, err error)

	// FindByTenantAndDate returns all instances for the current tenant on the given date.
	FindByTenantAndDate(ctx context.Context, date timezone.Date) ([]*ActivityInstance, error)

	// FindByTenantAndDateRange returns all instances within an inclusive date range.
	FindByTenantAndDateRange(ctx context.Context, from, to timezone.Date) ([]*ActivityInstance, error)

	// FindByIDs returns the instances matching the given IDs in one
	// tenant-scoped IN query, ordered by date then start time. Empty input
	// returns an empty slice without hitting the DB, matching the sibling
	// bulk helpers. Used by the self-service assignment read (#1844) to load
	// only the instances a staff member is actually assigned to.
	FindByIDs(ctx context.Context, ids []int64) ([]*ActivityInstance, error)

	// FindByActivityGroupAndDate returns instances for a specific template on a date.
	// There can be multiple rows when a template schedule defines several start
	// times on the same weekday.
	FindByActivityGroupAndDate(ctx context.Context, activityGroupID int64, date timezone.Date) ([]*ActivityInstance, error)

	// FindByActivityGroupAndDateRange returns one template's instances within
	// an inclusive date range in one tenant-scoped query.
	FindByActivityGroupAndDateRange(ctx context.Context, activityGroupID int64, from, to timezone.Date) ([]*ActivityInstance, error)

	// FindByActiveGroupID returns the instance that is currently bridged to the
	// given active.group, or nil if none.
	FindByActiveGroupID(ctx context.Context, activeGroupID int64) (*ActivityInstance, error)

	// FindPlannedTemplateBackedFrom returns every planned instance dated on or
	// after `from` that the MATERIALIZER produced (activity_group_id and
	// calendar_period_id both set, is_spontaneous false), tenant-scoped and
	// ordered by date then start time. Used to reconcile already-materialized
	// rosters when a reverted grade transition restores students the
	// insert-only materializer had skipped while they were alumni (#405
	// review).
	//
	// Hand-created blocks are excluded even when they link a template for its
	// metadata: their roster is the list of students the planner submitted, so
	// refilling it from the template's enrollments would add children nobody
	// assigned. Only the materializer stamps calendar_period_id (#405 review).
	//
	// The bound is INCLUSIVE of `from` (today, at revert time). An instance
	// materialized after the apply but dated today has no archive row — the
	// materializer skipped the alumnus outright — so excluding the boundary date
	// would leave a same-day apply-then-revert child permanently missing from
	// today's roster with nothing left to repair it. Today's instances that
	// already started or finished are excluded by the planned-status filter.
	FindPlannedTemplateBackedFrom(ctx context.Context, from timezone.Date) ([]*ActivityInstance, error)

	// MaxID returns the highest instance id currently visible to this tenant, or
	// 0 when it has none. It is an ORDERING MARKER, not a count: a grade
	// transition records it while applying so its revert can tell instances that
	// already existed from instances materialized afterwards, during the alumnus
	// window. created_at cannot answer that — it defaults to the transaction
	// start time, so a materialization that started earlier and then blocked on
	// the tenant transition lock backdates rows it inserts after the apply
	// commits. Sequence ids are assigned at INSERT and do reflect that order
	// (#405 review).
	MaxID(ctx context.Context) (int64, error)

	// MarkCompleted updates only lifecycle columns. Do not use a full-row
	// Update for DB-loaded instances because SQL TIME columns do not round-trip
	// safely through Bun.
	MarkCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error

	// CompleteActiveByActiveGroupIDs marks every still-active instance bridged
	// to one of the given active.groups as completed and returns the number of
	// rows changed. Used by the scheduler's daily session-end bridge and by
	// kiosk/session end. This path is outside the five-minute reopen window:
	// it does not write completed_by, reopen_until, or a completion snapshot,
	// so CanReopenInstance stays false.
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)

	// DeletePlannedNonSpontaneousInWindow removes still-planned,
	// template-backed instances in the inclusive [from, to] window and
	// returns the number of rows deleted. A nil `to` leaves the window
	// open-ended (everything from `from` onward) — the template split uses
	// that so no planned old-template instance can survive past the
	// effective date regardless of the materialization window.
	// activityGroupID narrows the delete to one template's instances; nil
	// deletes across all templates. Used by ReplanWeek and the template
	// split (WP-B3). preserveDeviations keeps Vertretungsplan overrides
	// (#1840): true for re-plan, false for the destructive template
	// split/end series operation — see the implementation for why.
	DeletePlannedNonSpontaneousInWindow(ctx context.Context, from timezone.Date, to *timezone.Date, activityGroupID *int64, preserveDeviations bool) (int64, error)

	// PropagateListKindToFutureInstances re-classifies future template-backed
	// planned instances of one template whose list_kind still equals the
	// series' previous value (NULL and '' treated alike), returning the number
	// of rows changed. It carries a series Listenart edit onto already
	// materialized future occurrences so the classified daily lists (#1565)
	// reflect it without a manual re-plan, while leaving today/past rows,
	// non-planned/spontaneous rows, and per-occurrence classification overrides
	// untouched. `after` is today; only rows dated strictly after it change.
	PropagateListKindToFutureInstances(ctx context.Context, activityGroupID int64, previousKind, newKind *string, after timezone.Date) (int64, error)

	// UpdateColumns is the generic partial-update helper promoted from the
	// embedded base repository: updates only the named columns by primary
	// key. Lifecycle transitions use it because SQL TIME columns do not
	// round-trip safely through a full-row Update.
	UpdateColumns(ctx context.Context, instance *ActivityInstance, columns ...string) (int64, error)

	// Generic query helpers promoted from the embedded base repository.
	// Used by the timetable retention cleanup.
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)
	OldestBefore(ctx context.Context, dateColumn string, cutoff *timezone.Date) (*timezone.Date, error)
	DeleteOlderThan(ctx context.Context, dateColumn string, cutoff timezone.Date) (int64, error)
}
