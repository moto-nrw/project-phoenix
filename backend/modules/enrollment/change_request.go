package enrollment

import (
	"encoding/json"
	"time"
)

const (
	ChangeRequestStatusPendingReview       = "pending_review"
	ChangeRequestStatusNeedsParentResponse = "needs_parent_response"
	ChangeRequestStatusApproved            = "approved"
	ChangeRequestStatusRejected            = "rejected"
	// ChangeRequestStatusCancelled is allowed by the column constraint but
	// written by nothing today. Readers still have to account for it: a row
	// that exists must never fall out of every list.
	ChangeRequestStatusCancelled = "cancelled"
)

const (
	ChangeRequestOriginParent = "parent"
	ChangeRequestOriginAdmin  = "admin"
)

const (
	ChangeRequestMessageAuthorParent = "parent"
	ChangeRequestMessageAuthorStaff  = "staff"
)

// ChangeRequest is a proposed correction to an enrollment request, filed by
// the family over the status token or recorded by staff as a direct
// correction. Snapshots and diff stay raw JSON at the owner boundary.
type ChangeRequest struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	RequestID         int64           `json:"request_id"`
	RequestChildID    *int64          `json:"request_child_id,omitempty"`
	Origin            string          `json:"origin"`
	Status            string          `json:"status"`
	ParentNote        *string         `json:"parent_note,omitempty"`
	AdminDecisionNote *string         `json:"admin_decision_note,omitempty"`
	BaseSnapshot      json.RawMessage `json:"base_snapshot"`
	ProposedSnapshot  json.RawMessage `json:"proposed_snapshot"`
	Diff              json.RawMessage `json:"diff"`
	// CareOfferingsEnabledAtCreation pins the form capability used to validate
	// and apply this proposal. It is internal workflow state, not API data.
	CareOfferingsEnabledAtCreation bool       `json:"-"`
	CreatedByAccountID             *int64     `json:"created_by_account_id,omitempty"`
	ReviewedByAccountID            *int64     `json:"reviewed_by_account_id,omitempty"`
	ReviewedAt                     *time.Time `json:"reviewed_at,omitempty"`
}
