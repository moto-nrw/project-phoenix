package enrollment

import "time"

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

// ChangeRequest is the legacy service value for a proposed enrollment correction.
// Persistence belongs to the Enrollment capability.
type ChangeRequest struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	RequestID         int64          `json:"request_id"`
	RequestChildID    *int64         `json:"request_child_id,omitempty"`
	Origin            string         `json:"origin"`
	Status            string         `json:"status"`
	ParentNote        *string        `json:"parent_note,omitempty"`
	AdminDecisionNote *string        `json:"admin_decision_note,omitempty"`
	BaseSnapshot      map[string]any `json:"base_snapshot"`
	ProposedSnapshot  map[string]any `json:"proposed_snapshot"`
	Diff              map[string]any `json:"diff"`
	// CareOfferingsEnabledAtCreation pins the form capability used to validate
	// and apply this proposal. It is internal workflow state, not API data.
	CareOfferingsEnabledAtCreation bool       `json:"-"`
	CreatedByAccountID             *int64     `json:"created_by_account_id,omitempty"`
	ReviewedByAccountID            *int64     `json:"reviewed_by_account_id,omitempty"`
	ReviewedAt                     *time.Time `json:"reviewed_at,omitempty"`
}

// DecisionInstant is when a terminal change request was decided. Older rows
// may not carry reviewed_at, so their last status update is the decision time.
func (r *ChangeRequest) DecisionInstant() time.Time {
	if r.ReviewedAt != nil {
		return *r.ReviewedAt
	}
	return r.UpdatedAt
}
