package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Event types of the parent-request ledger. They mirror the CHECK constraint
// in migration 1.15.348 — adding one here without the migration inserts
// nothing.
const (
	ParentRequestEventSubmitted     = "submitted"
	ParentRequestEventGuardianEdit  = "guardian_edited"
	ParentRequestEventShared        = "shared"
	ParentRequestEventDecided       = "decided"
	ParentRequestEventCorrected     = "corrected"
	ParentRequestEventMarkedDone    = "marked_done"
	ParentRequestTypeMasterData     = "master_data"
	ParentRequestTypeCareSchedule   = "care_schedule"
	ParentRequestTypePickupChange   = "pickup_change"
	ParentRequestTypeOffering       = "offering"
	ParentRequestTypeExcusedAbsence = "excused"
)

// ParentRequestEvent is one immutable step in a request's life. The four
// request tables carry only their current state, so this ledger is what lets
// staff and guardians see that a request was edited before it was decided, or
// that a decision was later corrected.
type ParentRequestEvent struct {
	base.Model `bun:"schema:users,table:parent_request_events"`
	base.TenantModel
	StudentID   int64  `bun:"student_id,notnull" json:"student_id"`
	RequestType string `bun:"request_type,notnull" json:"request_type"`
	RequestID   int64  `bun:"request_id,notnull" json:"request_id"`
	EventType   string `bun:"event_type,notnull" json:"event_type"`
	// ActorAccountID is the account that caused the event: the guardian on a
	// submit or edit, the reviewer on a decision. Nil for system events.
	ActorAccountID *int64 `bun:"actor_account_id" json:"actor_account_id,omitempty"`
	// Version is the request's expected_version AFTER the event, so a client
	// can tell which version a decision was taken on.
	Version string         `bun:"version,notnull" json:"version"`
	Payload map[string]any `bun:"payload,type:jsonb,notnull" json:"payload"`
}

// ParentRequestEventRepository is append-only by design: the table grants the
// tenant role SELECT and INSERT only.
type ParentRequestEventRepository interface {
	Create(context.Context, *ParentRequestEvent) error
	ListForRequest(ctx context.Context, requestType string, requestID int64) ([]*ParentRequestEvent, error)
	ListForStudent(ctx context.Context, studentID int64, limit int) ([]*ParentRequestEvent, error)
}
