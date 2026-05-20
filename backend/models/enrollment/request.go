package enrollment

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Request is a row in enrollment.requests - one parent submission. Each
// row carries the immutable schema_id pinning it to the form-schema
// version that was active at submission time.
//
// Status is derived from child statuses (see RequestStatus values) and
// computed by the request service, not stored.
type Request struct {
	base.Model `bun:"schema:enrollment,table:requests"`
	base.TenantModel
	// SchemaID is nullable: a phase set to "Basis" with no tenant-
	// active schema submits without one. The form_schemas FK still
	// applies when set, so admins can audit which schema version a
	// non-trivial submission was bound to.
	SchemaID           *int64         `bun:"schema_id" json:"schema_id,omitempty"`
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

// RequestStatus values - derived, not stored. Documented here as the
// canonical source for the derivation logic the request service applies.
const (
	RequestStatusSubmitted   = "submitted"    // every child still submitted
	RequestStatusUnderReview = "under_review" // at least one child under_review, no decisions yet
	RequestStatusPartial     = "partial"      // some children decided, others pending
	RequestStatusFinalized   = "finalized"    // all children in a terminal status
	RequestStatusWithdrawn   = "withdrawn"    // request withdrawn (withdrawn_at set)
)

// RequestListFilters narrows the admin list query. Zero-value fields
// are ignored. ChildStatus matches when ANY child of the request
// carries the given status - handy for "show me everything still
// awaiting a decision".
type RequestListFilters struct {
	PhaseID     int64
	ChildStatus string
}

// DuplicateChildKey identifies one (first_name, last_name) pair the
// caller wants to dedup against existing enrollments in a phase.
// Comparison is case-insensitive, with leading/trailing whitespace
// trimmed at the SQL layer.
type DuplicateChildKey struct {
	FirstName string
	LastName  string
}

// RequestRepository describes the DB operations PR 5/7/8 need.
type RequestRepository interface {
	Create(ctx context.Context, req *Request) error
	FindByID(ctx context.Context, id int64) (*Request, error)
	FindByStatusToken(ctx context.Context, token string) (*Request, error)

	// ListAdmin returns every request matching the filters, newest
	// first. PR 8's admin review UI consumes this; the parent-facing
	// flows never call it.
	ListAdmin(ctx context.Context, filters RequestListFilters) ([]*Request, error)

	// FindActiveDuplicate returns the names of any children for which a
	// non-terminal-rejected/withdrawn enrollment already exists for the
	// same (phase_id, guardian_email). Empty result means safe to
	// proceed. Used by the submit flow to block accidental
	// double-submits without affecting different parents or different
	// child names.
	FindActiveDuplicate(ctx context.Context, phaseID int64, guardianEmail string, children []DuplicateChildKey) ([]DuplicateChildKey, error)

	// ExistsByPhaseID reports whether any request row references the
	// given phase. The phase delete path uses this to refuse a destructive
	// delete and surface "deaktivieren statt löschen" to the admin.
	ExistsByPhaseID(ctx context.Context, phaseID int64) (bool, error)
}
