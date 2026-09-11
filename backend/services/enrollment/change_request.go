package enrollment

import "time"

// ChangeRequest is the application value of an enrollment change request: the
// owner's row with its snapshots decoded, because the review workflow compares
// and applies them field by field. Persistence stays with the Enrollment owner.
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
