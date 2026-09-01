package audit

import (
	"context"
	"encoding/json"
	"time"
)

// Sources of an offering adjustment. A direct correction is the office
// changing a booking on its own; a request-applied row is the side effect of
// approving a parent request, which the central history already shows as the
// decided request (#2436).
const (
	OfferingAdjustmentSourceDirect  = "direct"
	OfferingAdjustmentSourceRequest = "request"
	// Unknown marks adjustments recorded before source provenance existed.
	// They remain available in the child detail but are intentionally excluded
	// from the central direct-correction history.
	OfferingAdjustmentSourceUnknown = "unknown"
)

// EnrollmentOfferingAdjustment records an admin correction to one approved
// child's care-offering selection. Rows are append-only.
type EnrollmentOfferingAdjustment struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	TenantModel
	RequestID                   int64           `bun:"request_id,notnull" json:"request_id"`
	RequestChildID              int64           `bun:"request_child_id,notnull" json:"request_child_id"`
	StudentID                   int64           `bun:"student_id,notnull" json:"student_id"`
	ActorAccountID              int64           `bun:"actor_account_id,notnull" json:"actor_account_id"`
	ActorRole                   string          `bun:"actor_role,notnull" json:"actor_role"`
	ActorNameSnapshot           *string         `bun:"actor_name_snapshot" json:"actor_name_snapshot,omitempty"`
	ActorEmailSnapshot          *string         `bun:"actor_email_snapshot" json:"actor_email_snapshot,omitempty"`
	Reason                      string          `bun:"reason,notnull" json:"reason"`
	Source                      string          `bun:"source,notnull" json:"source"`
	Before                      json.RawMessage `bun:"before_json,type:jsonb,notnull" json:"before"`
	After                       json.RawMessage `bun:"after_json,type:jsonb,notnull" json:"after"`
	CompleteWithdrawalConfirmed bool            `bun:"complete_withdrawal_confirmed,notnull" json:"complete_withdrawal_confirmed"`
	ChangedAt                   time.Time       `bun:"changed_at,notnull,default:now()" json:"changed_at"`
}

type EnrollmentOfferingAdjustmentRepository interface {
	Create(ctx context.Context, entry *EnrollmentOfferingAdjustment) error
	CountForDeletion(ctx context.Context, requestID int64, requestChildID *int64) (int, error)
	ListByRequestChildID(ctx context.Context, requestChildID int64) ([]*EnrollmentOfferingAdjustment, error)
	// ListDirectForTenant returns the tenant's direct corrections, newest
	// change first, keyset paginated on (changed_at, id). A zero BeforeInstant
	// starts at the top; Limit is taken literally, so callers probing for a next
	// page ask for limit+1.
	ListDirectForTenant(ctx context.Context, filters DirectAdjustmentFilter) ([]*EnrollmentOfferingAdjustment, error)
}

type DirectAdjustmentFilter struct {
	BeforeInstant time.Time
	BeforeID      int64
	Limit         int
}
