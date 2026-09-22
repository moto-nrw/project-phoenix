package domain

import (
	"errors"
	"time"
)

var (
	ErrActivityInstanceNotFound = errors.New("activity instance not found")
	// ErrActivityInstanceStarted refuses a planning-status change of a block
	// that has a session; the execution must end first.
	ErrActivityInstanceStarted = errors.New("activity instance has started")
)

// Planning states. Execution state is Student Presence's activity session.
const (
	InstanceStatusPlanned   = "planned"
	InstanceStatusCancelled = "cancelled"
)

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
	ListKind               *string
	IsSpontaneous          bool
	UnderstaffedAck        bool
	UnderstaffedNote       *string
	CancelReason           *string
	Notes                  *string
	IdempotencyKey         *string
	IdempotencyFingerprint *string
	CreatedBy              *int64
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
	ListKind               *string
	IsSpontaneous          bool
	UnderstaffedAck        bool
	UnderstaffedNote       *string
	CancelReason           *string
	Notes                  *string
	IdempotencyKey         *string
	IdempotencyFingerprint *string
	CreatedBy              *int64
}

type ActivityInstanceFilter struct {
	IDs                 []int64
	Date                *string
	Dates               []string
	FromDate            *string
	ToDate              *string
	ActivityGroupID     *int64
	ActivityGroupIDs    []int64
	Status              string
	IsSpontaneous       *bool
	IdempotencyKey      string
	MaterializedPlanned bool
	OrderByDateAndTime  bool
	Limit               int
	Offset              int
}
