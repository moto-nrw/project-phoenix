package timetable

import (
	"context"
	"time"
)

const (
	InstanceStatusPlanned   = "planned"
	InstanceStatusActive    = "active"
	InstanceStatusCompleted = "completed"
	InstanceStatusCancelled = "cancelled"

	ActivityInstanceTitleMaxLength          = 255
	ActivityInstanceIdempotencyKeyMaxLength = 128
)

type ActivityInstance struct {
	ID                     int64     `json:"id"`
	TenantID               int64     `json:"tenant_id"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
	Date                   string
	ActivityGroupID        *int64
	CalendarPeriodID       *int64
	Title                  string
	Description            *string
	StartTime              string
	EndTime                string
	RoomID                 int64
	RequiredStaff          *int
	Status                 string
	ActiveGroupID          *int64
	ListKind               *string
	IsSpontaneous          bool
	UnderstaffedAck        bool
	UnderstaffedNote       *string
	CancelReason           *string
	Notes                  *string
	IdempotencyKey         *string
	IdempotencyFingerprint *string
	CreatedBy              *int64
	StartedBy              *int64
	StartedAt              *time.Time
	CompletedAt            *time.Time
	CompletedBy            *int64
	ReopenUntil            *time.Time
	CompletionSnapshot     []byte
}

type ActivityInstanceInput struct {
	Date                   string
	ActivityGroupID        *int64
	CalendarPeriodID       *int64
	Title                  string
	Description            *string
	StartTime              string
	EndTime                string
	RoomID                 int64
	RequiredStaff          *int
	Status                 string
	ActiveGroupID          *int64
	ListKind               *string
	IsSpontaneous          bool
	UnderstaffedAck        bool
	UnderstaffedNote       *string
	CancelReason           *string
	Notes                  *string
	IdempotencyKey         *string
	IdempotencyFingerprint *string
	CreatedBy              *int64
	StartedBy              *int64
	StartedAt              *time.Time
	CompletedAt            *time.Time
	CompletedBy            *int64
	ReopenUntil            *time.Time
	CompletionSnapshot     []byte
}

type ActivityInstanceFilter struct {
	IDs                 []int64
	Date                *string
	Dates               []string
	FromDate            *string
	ToDate              *string
	ActivityGroupID     *int64
	ActivityGroupIDs    []int64
	ActiveGroupID       *int64
	ActiveGroupIDs      []int64
	Status              string
	IsSpontaneous       *bool
	IdempotencyKey      string
	MaterializedPlanned bool
	OrderByDateAndTime  bool
	Limit               int
	Offset              int
}

type ActivityInstanceQuery interface {
	FindActivityInstance(context.Context, int64) (ActivityInstance, error)
	ListActivityInstances(context.Context, ActivityInstanceFilter) ([]ActivityInstance, error)
	MaxActivityInstanceID(context.Context) (int64, error)
	CountActivityInstances(context.Context, *string) (int, error)
	OldestActivityInstanceBefore(context.Context, *string) (*string, error)
}

type ActivityInstanceCommand interface {
	CreateActivityInstance(context.Context, ActivityInstanceInput) (ActivityInstance, error)
	CreateTemplateBackedActivityInstanceIfAbsent(context.Context, ActivityInstanceInput) (ActivityInstance, bool, error)
	CreateIdempotentActivityInstance(context.Context, ActivityInstanceInput) (ActivityInstance, bool, error)
	UpdateActivityInstance(context.Context, int64, ActivityInstanceInput) (ActivityInstance, error)
	PatchActivityInstance(context.Context, int64, ActivityInstanceInput, []string) (int64, error)
	DeleteActivityInstance(context.Context, int64) error
	MarkActivityInstanceCompleted(context.Context, int64, time.Time) error
	CompleteActiveActivityInstances(context.Context, []int64, time.Time) (int64, error)
	DeletePlannedActivityInstances(context.Context, string, *string, *int64, bool) (int64, error)
	DeleteRemovedWeekendActivityInstances(context.Context, int64, []int) (int64, error)
	PropagateActivityInstanceListKind(context.Context, int64, *string, *string, string) (int64, error)
	DeleteActivityInstancesBefore(context.Context, string) (int64, error)
}

type ActivityInstanceCapability interface {
	ActivityInstanceQuery
	ActivityInstanceCommand
}
