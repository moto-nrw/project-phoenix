package timetable

import (
	"context"
	"time"
)

// Planning states of an activity instance (#2762). Whether a block is running
// or has ended is Student Presence's activity session, not a planning state.
const (
	InstanceStatusPlanned   = "planned"
	InstanceStatusCancelled = "cancelled"

	ActivityInstanceTitleMaxLength          = 255
	ActivityInstanceIdempotencyKeyMaxLength = 128
)

// ActivityInstance is one planned occurrence of a block: its date, times,
// room, template link and planning annotations. Its execution (start, live
// group, completion) lives in Student Presence's activity session.
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
	IDs              []int64
	Date             *string
	Dates            []string
	FromDate         *string
	ToDate           *string
	ActivityGroupID  *int64
	ActivityGroupIDs []int64
	Status           string
	IsSpontaneous    *bool
	IdempotencyKey   string
	// MaterializedPlanned narrows to template-backed, period-bound, planned
	// occurrences that have not started: what a replan may replace.
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
	// UpdateActivityInstance replaces the planning fields; the planning
	// status changes only through PatchActivityInstance.
	UpdateActivityInstance(context.Context, int64, ActivityInstanceInput) (ActivityInstance, error)
	// PatchActivityInstance writes the named planning columns. A status
	// change is refused with ErrActivityInstanceStarted while the block has a
	// session; callers end the session first.
	PatchActivityInstance(context.Context, int64, ActivityInstanceInput, []string) (int64, error)
	DeleteActivityInstance(context.Context, int64) error
	// DeletePlannedActivityInstances removes the template-backed occurrences
	// of the window that have not started; a started or ended block stays.
	DeletePlannedActivityInstances(context.Context, string, *string, *int64, bool) (int64, error)
	DeleteRemovedWeekendActivityInstances(context.Context, int64, []int) (int64, error)
	PropagateActivityInstanceListKind(context.Context, int64, *string, *string, string) (int64, error)
	DeleteActivityInstancesBefore(context.Context, string) (int64, error)
}

type ActivityInstanceCapability interface {
	ActivityInstanceQuery
	ActivityInstanceCommand
}

// SessionFacts is the consumer-owned port to the Student Presence owner of
// activity sessions and session attendance. Timetable asks it which of its
// planned rows already carry an execution it must not replan, remove or
// revive; it never reads the owner's tables itself.
type SessionFacts interface {
	// StartedInstanceIDs returns which of the instances have a session,
	// running or ended.
	StartedInstanceIDs(context.Context, []int64) ([]int64, error)
	// CompletedInstanceIDs returns which of the instances have ended.
	CompletedInstanceIDs(context.Context, []int64) ([]int64, error)
	// ObservedParticipantIDs returns which of the participants carry observed
	// presence: a check-in or a checkout.
	ObservedParticipantIDs(context.Context, []int64) ([]int64, error)
	// NotScheduledParticipantIDs returns which of the participants carry the
	// frozen non-booking marker.
	NotScheduledParticipantIDs(context.Context, []int64) ([]int64, error)
	// ExecutionFacts answers both questions of a block choice in one read:
	// which of the instances ended, which of the participants carry the
	// frozen non-booking marker.
	ExecutionFacts(ctx context.Context, instanceIDs, participantIDs []int64) (completed, notScheduled []int64, err error)
}
