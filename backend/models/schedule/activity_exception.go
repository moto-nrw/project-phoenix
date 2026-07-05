package schedule

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Activity exception type constants.
const (
	ActivityExceptionCancelled = "cancelled"
	ActivityExceptionModified  = "modified"
)

// ActivityExceptionReasonMaxLength is the maximum length of the reason field.
const ActivityExceptionReasonMaxLength = 500

// Sentinel errors for the activity exception domain.
var (
	// ErrActivityExceptionDateConflict is returned when a create or update
	// would violate the per-template per-date uniqueness rule.
	ErrActivityExceptionDateConflict = errors.New("activity exception for this date already exists")
)

// ActivityException records a per-date deviation from a template's normal
// schedule: either a cancellation or a modification (time/room override).
// Consumed by the materialization service (E8).
//
// When ExceptionType is "cancelled", StartTime / EndTime / RoomID must be nil.
// When "modified", at least one of them should be non-nil (otherwise the
// exception expresses nothing).
type ActivityException struct {
	base.Model `bun:"schema:schedule,table:activity_exceptions"`
	base.TenantModel

	ActivityGroupID int64         `bun:"activity_group_id,notnull" json:"activity_group_id"`
	ExceptionDate   timezone.Date `bun:"exception_date,notnull" json:"exception_date"`
	ExceptionType   string        `bun:"exception_type,notnull" json:"exception_type"`
	StartTime       *time.Time    `bun:"start_time" json:"start_time,omitempty"`
	EndTime         *time.Time    `bun:"end_time" json:"end_time,omitempty"`
	RoomID          *int64        `bun:"room_id" json:"room_id,omitempty"`
	Reason          *string       `bun:"reason" json:"reason,omitempty"`
	CreatedBy       *int64        `bun:"created_by" json:"created_by,omitempty"`
}

func (e *ActivityException) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`schedule.activity_exceptions AS "activity_exception"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`schedule.activity_exceptions AS "activity_exception"`)
	}
	return nil
}

// Validate ensures the exception is well-formed for persistence.
func (e *ActivityException) Validate() error {
	if e.ActivityGroupID <= 0 {
		return errors.New("activity_group_id is required")
	}
	if e.ExceptionDate.IsZero() {
		return errors.New("exception_date is required")
	}
	if !IsValidExceptionType(e.ExceptionType) {
		return errors.New("invalid exception type")
	}

	switch e.ExceptionType {
	case ActivityExceptionCancelled:
		if e.StartTime != nil || e.EndTime != nil || e.RoomID != nil {
			return errors.New("cancelled exception must not set start_time, end_time, or room_id")
		}
	case ActivityExceptionModified:
		if e.StartTime == nil && e.EndTime == nil && e.RoomID == nil {
			return errors.New("modified exception must override at least one of start_time, end_time, or room_id")
		}
		if e.StartTime != nil && e.EndTime != nil && !e.EndTime.After(*e.StartTime) {
			return errors.New("end_time must be after start_time")
		}
	}

	if e.Reason != nil && len(*e.Reason) > ActivityExceptionReasonMaxLength {
		return fmt.Errorf("reason cannot exceed %d characters", ActivityExceptionReasonMaxLength)
	}

	return nil
}

// IsValidExceptionType reports whether t is a permitted exception type.
func IsValidExceptionType(t string) bool {
	switch t {
	case ActivityExceptionCancelled, ActivityExceptionModified:
		return true
	}
	return false
}

// IsCancellation reports whether this exception cancels the occurrence.
func (e *ActivityException) IsCancellation() bool {
	return e.ExceptionType == ActivityExceptionCancelled
}

// ActivityExceptionRepository defines operations for managing activity
// exceptions.
type ActivityExceptionRepository interface {
	base.Repository[*ActivityException]

	// FindByActivityGroupID returns all exceptions for a given template.
	FindByActivityGroupID(ctx context.Context, activityGroupID int64) ([]*ActivityException, error)

	// FindByActivityGroupAndDate returns the exception for a specific
	// (template, date) pair, or nil if none exists.
	FindByActivityGroupAndDate(ctx context.Context, activityGroupID int64, date timezone.Date) (*ActivityException, error)

	// FindByDateRange returns all exceptions across all templates within the
	// inclusive range. Used by the materialization service to apply exceptions
	// efficiently for an entire week.
	FindByDateRange(ctx context.Context, from, to timezone.Date) ([]*ActivityException, error)

	// Generic query helpers promoted from the embedded base repository.
	// Used by the timetable retention cleanup.
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)
	OldestBefore(ctx context.Context, dateColumn string, cutoff *timezone.Date) (*timezone.Date, error)
	DeleteOlderThan(ctx context.Context, dateColumn string, cutoff timezone.Date) (int64, error)
}
