package audit

import (
	"context"
	"time"
)

// EnrollmentRestoration is an append-only, data-minimal audit event written
// when an admin restores a withdrawn enrollment request (#2157). It stores
// identifiers only; no names or contact data. ChildIDs carries the
// request_children rows that were flipped back from withdrawn to submitted.
type EnrollmentRestoration struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	TenantModel
	RequestID      int64     `bun:"request_id,notnull" json:"request_id"`
	ChildIDs       []int64   `bun:"child_ids,type:jsonb,notnull" json:"child_ids"`
	ActorAccountID *int64    `bun:"actor_account_id" json:"actor_account_id,omitempty"`
	RestoredAt     time.Time `bun:"restored_at,notnull,default:now()" json:"restored_at"`
}

type EnrollmentRestorationRepository interface {
	Create(ctx context.Context, event *EnrollmentRestoration) error
}
