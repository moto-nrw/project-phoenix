package enrollment

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

const (
	ChangeRequestStatusPendingReview       = "pending_review"
	ChangeRequestStatusNeedsParentResponse = "needs_parent_response"
	ChangeRequestStatusApproved            = "approved"
	ChangeRequestStatusRejected            = "rejected"
	ChangeRequestStatusCancelled           = "cancelled"
)

const (
	ChangeRequestMessageAuthorParent = "parent"
	ChangeRequestMessageAuthorStaff  = "staff"
	ChangeRequestMessageAuthorSystem = "system"
)

// ChangeRequest stores a parent-proposed full-form correction for an
// enrollment request after the direct edit window has closed.
type ChangeRequest struct {
	base.Model `bun:"schema:enrollment,table:change_requests"`
	base.TenantModel

	RequestID           int64          `bun:"request_id,notnull" json:"request_id"`
	RequestChildID      *int64         `bun:"request_child_id" json:"request_child_id,omitempty"`
	Status              string         `bun:"status,notnull,default:'pending_review'" json:"status"`
	ParentNote          *string        `bun:"parent_note" json:"parent_note,omitempty"`
	AdminDecisionNote   *string        `bun:"admin_decision_note" json:"admin_decision_note,omitempty"`
	BaseSnapshot        map[string]any `bun:"base_snapshot,type:jsonb,notnull,default:'{}'" json:"base_snapshot"`
	ProposedSnapshot    map[string]any `bun:"proposed_snapshot,type:jsonb,notnull,default:'{}'" json:"proposed_snapshot"`
	Diff                map[string]any `bun:"diff_json,type:jsonb,notnull,default:'{}'" json:"diff"`
	CreatedByAccountID  *int64         `bun:"created_by_account_id" json:"created_by_account_id,omitempty"`
	ReviewedByAccountID *int64         `bun:"reviewed_by_account_id" json:"reviewed_by_account_id,omitempty"`
	ReviewedAt          *time.Time     `bun:"reviewed_at" json:"reviewed_at,omitempty"`
}

func (c *ChangeRequest) TableName() string {
	return "enrollment.change_requests"
}

// ChangeRequestMessage is one parent/staff/system message in the public
// conversation around a change request.
type ChangeRequestMessage struct {
	base.Model `bun:"schema:enrollment,table:change_request_messages"`
	base.TenantModel

	ChangeRequestID int64  `bun:"change_request_id,notnull" json:"change_request_id"`
	AuthorType      string `bun:"author_type,notnull" json:"author_type"`
	AuthorAccountID *int64 `bun:"author_account_id" json:"author_account_id,omitempty"`
	Body            string `bun:"body,notnull" json:"body"`
	InternalOnly    bool   `bun:"internal_only,notnull,default:false" json:"internal_only"`
}

func (m *ChangeRequestMessage) TableName() string {
	return "enrollment.change_request_messages"
}

type ChangeRequestListFilters struct {
	RequestID int64
	Status    string
	Limit     int
}

type ChangeRequestRepository interface {
	Create(ctx context.Context, row *ChangeRequest) error
	FindByID(ctx context.Context, id int64) (*ChangeRequest, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*ChangeRequest, error)
	ListByRequestID(ctx context.Context, requestID int64) ([]*ChangeRequest, error)
	ListAdmin(ctx context.Context, filters ChangeRequestListFilters) ([]*ChangeRequest, error)
	MarkReviewed(ctx context.Context, id int64, status string, note *string, reviewerAccountID int64, reviewedAt time.Time) error
	SetStatus(ctx context.Context, id int64, status string) error
}

type ChangeRequestMessageRepository interface {
	Create(ctx context.Context, row *ChangeRequestMessage) error
	ListByChangeRequestID(ctx context.Context, changeRequestID int64, includeInternal bool) ([]*ChangeRequestMessage, error)
	ListByChangeRequestIDs(ctx context.Context, changeRequestIDs []int64, includeInternal bool) ([]*ChangeRequestMessage, error)
}
