package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Activity instance status constants. Stored as TEXT with a CHECK constraint
// rather than a DB enum so adding states later does not require an ALTER TYPE.
const (
	InstanceStatusPlanned   = "planned"
	InstanceStatusActive    = "active"
	InstanceStatusCompleted = "completed"
	InstanceStatusCancelled = "cancelled"
)

// Sentinel errors for the activity instance domain. Callers should compare via
// errors.Is rather than matching on error strings.
var (
	// ErrActivityInstanceTimeConflict is returned when an attempted create or
	// update collides with an existing template-backed instance on the same
	// (tenant, date, activity_group, start_time).
	ErrActivityInstanceTimeConflict = errors.New("activity instance time conflict")
)

// tableActivityInstances is the schema-qualified table name.
const tableActivityInstances = "schedule.activity_instances"

// ActivityInstanceTitleMaxLength is the maximum length of the title field.
const ActivityInstanceTitleMaxLength = 255

// ActivityInstance is the concrete materialized occurrence of a template on a
// given date (or a spontaneous instance created without a template). It lives
// in the "instance layer" between the template layer (activities.*) and the
// live layer (active.*) — see docs/timetable-system-plan.md §5.1.
type ActivityInstance struct {
	base.Model `bun:"schema:schedule,table:activity_instances"`
	base.TenantModel

	Date             time.Time  `bun:"date,notnull" json:"date"`
	ActivityGroupID  *int64     `bun:"activity_group_id" json:"activity_group_id,omitempty"`
	CalendarPeriodID *int64     `bun:"calendar_period_id" json:"calendar_period_id,omitempty"`
	Title            string     `bun:"title,notnull" json:"title"`
	Description      *string    `bun:"description" json:"description,omitempty"`
	StartTime        time.Time  `bun:"start_time,notnull" json:"start_time"`
	EndTime          time.Time  `bun:"end_time,notnull" json:"end_time"`
	RoomID           int64      `bun:"room_id,notnull" json:"room_id"`
	Status           string     `bun:"status,notnull,default:'planned'" json:"status"`
	ActiveGroupID    *int64     `bun:"active_group_id" json:"active_group_id,omitempty"`
	IsSpontaneous    bool       `bun:"is_spontaneous,notnull,default:false" json:"is_spontaneous"`
	Notes            *string    `bun:"notes" json:"notes,omitempty"`
	CreatedBy        *int64     `bun:"created_by" json:"created_by,omitempty"`
	StartedBy        *int64     `bun:"started_by" json:"started_by,omitempty"`
	StartedAt        *time.Time `bun:"started_at" json:"started_at,omitempty"`
	CompletedAt      *time.Time `bun:"completed_at" json:"completed_at,omitempty"`
}

func (i *ActivityInstance) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`schedule.activity_instances AS "activity_instance"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`schedule.activity_instances AS "activity_instance"`)
	}
	return nil
}

// TableName returns the database table name.
func (i *ActivityInstance) TableName() string {
	return tableActivityInstances
}

// GetID implements the Entity interface.
func (i *ActivityInstance) GetID() any { return i.ID }

// GetCreatedAt implements the Entity interface.
func (i *ActivityInstance) GetCreatedAt() time.Time { return i.CreatedAt }

// GetUpdatedAt implements the Entity interface.
func (i *ActivityInstance) GetUpdatedAt() time.Time { return i.UpdatedAt }

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
	FindByTenantAndDate(ctx context.Context, date time.Time) ([]*ActivityInstance, error)

	// FindByTenantAndDateRange returns all instances within an inclusive date range.
	FindByTenantAndDateRange(ctx context.Context, from, to time.Time) ([]*ActivityInstance, error)

	// FindByActivityGroupAndDate returns instances for a specific template on a date.
	// There can be multiple rows when a template schedule defines several start
	// times on the same weekday.
	FindByActivityGroupAndDate(ctx context.Context, activityGroupID int64, date time.Time) ([]*ActivityInstance, error)

	// FindByActiveGroupID returns the instance that is currently bridged to the
	// given active.group, or nil if none.
	FindByActiveGroupID(ctx context.Context, activeGroupID int64) (*ActivityInstance, error)

	// MarkCompleted updates only lifecycle columns. Do not use a full-row
	// Update for DB-loaded instances because SQL TIME columns do not round-trip
	// safely through Bun.
	MarkCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error

	// CompleteActiveByActiveGroupIDs marks every still-active instance bridged
	// to one of the given active.groups as completed and returns the number of
	// rows changed. Used by the scheduler's daily session-end bridge.
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)

	// DeletePlannedNonSpontaneousInWindow removes still-planned,
	// template-backed instances in the inclusive [from, to] window and
	// returns the number of rows deleted. Used by ReplanWeek.
	DeletePlannedNonSpontaneousInWindow(ctx context.Context, from, to time.Time) (int64, error)

	// UpdateColumns is the generic partial-update helper promoted from the
	// embedded base repository: updates only the named columns by primary
	// key. Lifecycle transitions use it because SQL TIME columns do not
	// round-trip safely through a full-row Update.
	UpdateColumns(ctx context.Context, instance *ActivityInstance, columns ...string) (int64, error)

	// Generic query helpers promoted from the embedded base repository.
	// Used by the timetable retention cleanup.
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)
	OldestBefore(ctx context.Context, dateColumn string, cutoff *time.Time) (*time.Time, error)
	DeleteOlderThan(ctx context.Context, dateColumn string, cutoff time.Time) (int64, error)
}
