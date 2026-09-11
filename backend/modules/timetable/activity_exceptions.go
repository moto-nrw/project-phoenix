package timetable

import (
	"context"
	"time"
)

const ActivityExceptionReasonMaxLength = 500

const (
	ActivityExceptionCancelled = "cancelled"
	ActivityExceptionModified  = "modified"
)

type ActivityException struct {
	ID              int64     `json:"id"`
	TenantID        int64     `json:"tenant_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	ActivityGroupID int64     `json:"activity_group_id"`
	ExceptionDate   string    `json:"exception_date"`
	ExceptionType   string    `json:"exception_type"`
	StartTime       *string   `json:"start_time,omitempty"`
	EndTime         *string   `json:"end_time,omitempty"`
	RoomID          *int64    `json:"room_id,omitempty"`
	Reason          *string   `json:"reason,omitempty"`
	CreatedBy       *int64    `json:"created_by,omitempty"`
}

type ActivityExceptionInput struct {
	ActivityGroupID int64
	ExceptionDate   string
	ExceptionType   string
	StartTime       *string
	EndTime         *string
	RoomID          *int64
	Reason          *string
	CreatedBy       *int64
}

type ActivityExceptionFilter struct {
	ActivityGroupID *int64
	ExceptionDate   *string
	FromDate        *string
	ToDate          *string
	BeforeDate      *string
	OrderByDate     bool
	Limit           int
	Offset          int
}

type ActivityExceptionQuery interface {
	FindActivityException(context.Context, int64) (ActivityException, error)
	ListActivityExceptions(context.Context, ActivityExceptionFilter) ([]ActivityException, error)
	CountActivityExceptions(context.Context, *string) (int, error)
	OldestActivityExceptionBefore(context.Context, *string) (*string, error)
}

type ActivityExceptionCommand interface {
	CreateActivityException(context.Context, ActivityExceptionInput) (ActivityException, error)
	UpdateActivityException(context.Context, int64, ActivityExceptionInput) (ActivityException, error)
	DeleteActivityException(context.Context, int64) error
	DeleteActivityExceptionsBefore(context.Context, string) (int64, error)
}

type ActivityExceptionCapability interface {
	ActivityExceptionQuery
	ActivityExceptionCommand
}
