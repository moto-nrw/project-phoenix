package domain

import (
	"errors"
	"time"
)

var ErrActivityInstanceNotFound = errors.New("activity instance not found")

type ActivityInstance struct {
	ID                     int64
	TenantID               int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
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

type ActivityInstanceFields struct {
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
