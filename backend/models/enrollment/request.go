package enrollment

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Request is a row in enrollment.requests — one parent submission. Each
// row carries the immutable schema_id pinning it to the form-schema
// version that was active at submission time.
//
// Status is derived from child statuses (see RequestStatus values) and
// computed by the request service, not stored.
type Request struct {
	base.Model `bun:"schema:enrollment,table:requests"`
	base.TenantModel
	SchemaID           int64          `bun:"schema_id,notnull" json:"schema_id"`
	PhaseID            int64          `bun:"phase_id,notnull" json:"phase_id"`
	GuardianFirstName  string         `bun:"guardian_first_name,notnull" json:"guardian_first_name"`
	GuardianLastName   string         `bun:"guardian_last_name,notnull" json:"guardian_last_name"`
	GuardianEmail      string         `bun:"guardian_email,notnull" json:"guardian_email"`
	GuardianPhone      *string        `bun:"guardian_phone" json:"guardian_phone,omitempty"`
	GuardianAccountID  *int64         `bun:"guardian_account_id" json:"guardian_account_id,omitempty"`
	ConsentFlags       map[string]any `bun:"consent_flags,type:jsonb,notnull,default:'{}'" json:"consent_flags"`
	CustomData         map[string]any `bun:"custom_data,type:jsonb,notnull,default:'{}'" json:"custom_data"`
	StatusToken        string         `bun:"status_token,notnull,unique" json:"status_token"`
	StatusTokenExpires *time.Time     `bun:"status_token_expires" json:"status_token_expires,omitempty"`
	SubmittedAt        time.Time      `bun:"submitted_at,notnull,default:current_timestamp" json:"submitted_at"`
	WithdrawnAt        *time.Time     `bun:"withdrawn_at" json:"withdrawn_at,omitempty"`
}

func (r *Request) TableName() string {
	return "enrollment.requests"
}

// RequestStatus values — derived, not stored. Documented here as the
// canonical source for the derivation logic the request service applies.
const (
	RequestStatusSubmitted   = "submitted"    // every child still submitted
	RequestStatusUnderReview = "under_review" // at least one child under_review, no decisions yet
	RequestStatusPartial     = "partial"      // some children decided, others pending
	RequestStatusFinalized   = "finalized"    // all children in a terminal status
	RequestStatusWithdrawn   = "withdrawn"    // request withdrawn (withdrawn_at set)
)

// RequestRepository describes the DB operations PR 5/7/8 need. PR 5
// only implements + tests Create/FindByID/FindByStatusToken; PR 7 + 8
// fill in the rest.
type RequestRepository interface {
	Create(ctx context.Context, req *Request) error
	FindByID(ctx context.Context, id int64) (*Request, error)
	FindByStatusToken(ctx context.Context, token string) (*Request, error)
}
