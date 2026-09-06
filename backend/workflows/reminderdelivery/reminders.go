// Package reminderdelivery exposes reminder queries and the scheduler command.
package reminderdelivery

import (
	"context"
	"errors"
	"time"
)

var ErrNotLinkedToStaff = errors.New("not linked to staff")

// Command queues guardian reminders for occurrences starting in [from, to).
// The caller supplies a tenant context and the configured reminder window.
// The result counts queued emails; delivery failures remain visible as errors.
type Command interface {
	EnqueueDueAppointmentReminders(ctx context.Context, from, to time.Time) (int, error)
}

// Capability is the single owner facade used by inbound and worker composition.
type Capability interface {
	Query
	Command
}

type Module struct {
	Query
	Command
}

// CallerQuery resolves staff identity before computing a caller's reminders.
// EffectiveAdmin must come from the authenticated principal's permissions.
type CallerQuery interface {
	ComputeForCaller(ctx context.Context, effectiveAdmin bool) (*Result, error)
}

// Reminder event types.
const (
	TypePickupUpcoming  = "pickup_upcoming"
	TypePickupOverdue   = "pickup_overdue"
	TypeActivityStart   = "activity_start"
	TypeActivityOverdue = "activity_overdue"
)

// Reminder is a single visual reminder shown to staff.
type Reminder struct {
	Type string `json:"type"`
	// Entity IDs are serialized as strings to honor the repo's int64→string API
	// contract (JS loses precision on int64 as a JSON number). Exactly one is set
	// according to Type and gives frontend lists a stable reconciliation key.
	StudentID          *string `json:"student_id,omitempty"`
	ActivityInstanceID *string `json:"activity_instance_id,omitempty"`
	Title              string  `json:"title"`              // student name OR activity title
	Subtitle           string  `json:"subtitle,omitempty"` // school class or room — optional
	DueTime            string  `json:"due_time"`           // "HH:MM" of the relevant time
	MinutesAway        int     `json:"minutes_away"`       // negative when overdue
}

// Result is the computed reminder list plus a convenience count for the badge.
// Enabled reports whether the tenant has switched on at least one reminder
// type — the header bell uses it to show/hide itself (and the /reminders page
// link it exposes) regardless of whether anything is currently due.
type Result struct {
	Reminders []Reminder `json:"reminders"`
	Count     int        `json:"count"`
	Enabled   bool       `json:"enabled"`
	// NextChangeAt is the wall-clock "HH:MM" of the soonest future moment at
	// which this list would change purely by time passing — the next pickup or
	// activity entering/leaving its window. The frontend schedules a timer to
	// exactly this time to refetch, so a reminder appears on its threshold
	// instead of only on the next fixed poll. Omitted when nothing time-based is
	// pending (the poll then remains the only cadence). Data-driven changes
	// (check-ins, edits) are still delivered via the SSE-stale event, not this.
	NextChangeAt string `json:"next_change_at,omitempty"`
	// AssignedActivityInstanceIDs names the activity instances this person is
	// personally planned on, keyed exactly like Reminder.ActivityInstanceID.
	//
	// Set by ComputeBatch only and never serialized: the /reminders page shows
	// one activity list and does not care where an entry came from. A consumer
	// that addresses people individually does care — "an activity in the room
	// you are watching" and "the slot you have to show up for" are two
	// different messages, and a person may want to hear one and not the other.
	AssignedActivityInstanceIDs map[string]struct{} `json:"-"`
}

// Scope describes whose reminders to compute. Admins see all present children
// and all activities; caregivers see only the children currently in the rooms
// they supervise and the activities in those rooms (issue #1457 decision).
type Scope struct {
	IsAdmin bool
	StaffID int64
}

// Service is the reminder computation entry point for one caller.
type Service interface {
	Compute(ctx context.Context, scope Scope) (*Result, error)
}

// BatchComputer computes personally-scoped reminder lists for many staff
// members with a number of queries that does not grow with the number of staff.
// It exists for callers with no authenticated user (the scheduler), which is
// also why it takes staff IDs rather than deriving the person from the context.
//
// It is deliberately separate from Service: Service is implemented by several
// test doubles that have no business growing a batch method.
type BatchComputer interface {
	// ComputeBatch returns one result per requested scope, keyed by ResultKey
	// when set and otherwise by StaffID. Every requested scope gets an entry, so
	// callers may index the map without checking. Any read failure fails the
	// whole batch, matching Compute's all-or-nothing contract.
	ComputeBatch(ctx context.Context, scopes []BatchScope) (map[int64]*Result, error)
}

// Query is the full surface the concrete service provides. It is assignable
// to Service, so consumers that only need Compute keep their narrower type.
type Query interface {
	Service
	BatchComputer
	CallerQuery
}

// BatchScope is one recipient of a batch computation. IsAdmin is the caller's
// assertion, exactly as with Scope — the service does not verify it.
type BatchScope struct {
	Scope
	// ResultKey lets callers address an effective admin who has no staff row.
	// It is an opaque, non-zero map key; ordinary staff scopes leave it unset
	// and remain keyed by StaffID.
	ResultKey int64
	// IncludeAssignedActivityStart enables upcoming reminders for activity
	// instances this person is planned on. Unlike room-scoped activity
	// reminders, this personal type has no tenant-level reminder gate.
	IncludeAssignedActivityStart bool
}
