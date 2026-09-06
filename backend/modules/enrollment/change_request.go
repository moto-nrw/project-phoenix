package enrollment

import (
	"encoding/json"
	"time"
)

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
