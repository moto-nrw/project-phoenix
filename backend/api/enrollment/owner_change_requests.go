package enrollment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// The change requests as these routes speak them: the owner's cases with
// the snapshots and answers decoded.

// Change-request values the owner's contract carries unchanged.
type (
	ChangeRequestMessageInput     = capability.ChangeRequestMessageInput
	ReviewChangeRequestInput      = capability.ReviewChangeRequestInput
	CorrectApprovedChildDataInput = capability.CorrectApprovedChildDataInput
	ChangeRequestFilters          = capability.ChangeRequestFilters
	ChangeRequestReviewQuery      = capability.ChangeRequestReviewQuery
	ChangeRequestReviewChild      = capability.ChangeRequestReviewChild
)

// ChangeRequest is a change request with its snapshots decoded, because the
// review UI compares them field by field.
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

// ChangeRequestAggregate is a change request with the request, children,
// messages and phase it concerns, decoded.
type ChangeRequestAggregate struct {
	ChangeRequest *ChangeRequest
	Request       *enrollmentModels.Request
	Children      []*RequestChild
	Messages      []*capability.ChangeRequestMessage
	Phase         *capability.Phase
}

// CreateChangeRequestInput is a family's proposed correction of its request.
type CreateChangeRequestInput struct {
	Submission         SubmitRequest
	ParentNote         string
	CreatedByAccountID *int64
}

// ChangeRequestReviewRow is one change request of the review list plus the
// names the list shows, decoded.
type ChangeRequestReviewRow struct {
	ChangeRequest *ChangeRequest
	ChildNames    []string
	ChildIDs      []int64
	Children      []ChangeRequestReviewChild
	GuardianName  string
	ReviewerName  string
}

// ChangeRequestService is the change-request flow these routes call.
type ChangeRequestService interface {
	Create(ctx context.Context, token string, input CreateChangeRequestInput) (*ChangeRequestAggregate, error)
	ListPublic(ctx context.Context, token string) ([]*ChangeRequestAggregate, error)
	ParentReply(ctx context.Context, token string, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error)
	ListAdmin(ctx context.Context, filters ChangeRequestFilters) ([]*ChangeRequestAggregate, error)
	GetAdmin(ctx context.Context, changeRequestID int64) (*ChangeRequestAggregate, error)
	AskQuestion(ctx context.Context, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error)
	Reject(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error)
	Approve(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error)
	CorrectApprovedChildData(ctx context.Context, input CorrectApprovedChildDataInput) (*ChangeRequestAggregate, error)
	ListForReview(ctx context.Context, query ChangeRequestReviewQuery) ([]*ChangeRequestReviewRow, *usersService.HistoryCursor, error)
	CountOpenForReview(ctx context.Context, statuses []string) (int, error)
}

// NewChangeRequestService decodes the owner's change requests for these
// routes. A nil owner yields nil.
func NewChangeRequestService(owner capability.ChangeRequests) ChangeRequestService {
	if owner == nil {
		return nil
	}
	return changeRequestService{owner: owner}
}

type changeRequestService struct{ owner capability.ChangeRequests }

func changeRequestValue(value *capability.ChangeRequest) (*ChangeRequest, error) {
	if value == nil {
		return nil, nil
	}
	row := &ChangeRequest{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		RequestID: value.RequestID, RequestChildID: value.RequestChildID, Origin: value.Origin,
		Status: value.Status, ParentNote: value.ParentNote, AdminDecisionNote: value.AdminDecisionNote,
		CareOfferingsEnabledAtCreation: value.CareOfferingsEnabledAtCreation,
		CreatedByAccountID:             value.CreatedByAccountID, ReviewedByAccountID: value.ReviewedByAccountID,
		ReviewedAt: value.ReviewedAt,
	}
	for _, field := range []struct {
		name string
		raw  json.RawMessage
		into *map[string]any
	}{
		{"BaseSnapshot", value.BaseSnapshot, &row.BaseSnapshot},
		{"ProposedSnapshot", value.ProposedSnapshot, &row.ProposedSnapshot},
		{"Diff", value.Diff, &row.Diff},
	} {
		if err := decodeJSONObject(field.raw, field.into); err != nil {
			return nil, fmt.Errorf("decode change request %s: %w", field.name, err)
		}
	}
	return row, nil
}

func changeRequestCase(value *capability.ChangeRequestCase, err error) (*ChangeRequestAggregate, error) {
	if err != nil || value == nil {
		return nil, err
	}
	changeRequest, err := changeRequestValue(value.ChangeRequest)
	if err != nil {
		return nil, err
	}
	request, err := requestValue(value.Request)
	if err != nil {
		return nil, err
	}
	children, err := childValues(value.Children)
	if err != nil {
		return nil, err
	}
	return &ChangeRequestAggregate{
		ChangeRequest: changeRequest, Request: request, Children: children,
		Messages: value.Messages, Phase: value.Phase,
	}, nil
}

func changeRequestCases(values []*capability.ChangeRequestCase, err error) ([]*ChangeRequestAggregate, error) {
	if err != nil {
		return nil, err
	}
	out := make([]*ChangeRequestAggregate, 0, len(values))
	for _, value := range values {
		converted, err := changeRequestCase(value, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}

func (s changeRequestService) Create(ctx context.Context, token string, input CreateChangeRequestInput) (*ChangeRequestAggregate, error) {
	submission, err := encodeSubmitRequest(input.Submission)
	if err != nil {
		return nil, err
	}
	return changeRequestCase(s.owner.Create(ctx, token, capability.CreateChangeRequestInput{
		Submission: submission, ParentNote: input.ParentNote, CreatedByAccountID: input.CreatedByAccountID,
	}))
}

func (s changeRequestService) ListPublic(ctx context.Context, token string) ([]*ChangeRequestAggregate, error) {
	return changeRequestCases(s.owner.ListPublic(ctx, token))
}

func (s changeRequestService) ParentReply(ctx context.Context, token string, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error) {
	return changeRequestCase(s.owner.ParentReply(ctx, token, changeRequestID, input))
}

func (s changeRequestService) ListAdmin(ctx context.Context, filters ChangeRequestFilters) ([]*ChangeRequestAggregate, error) {
	return changeRequestCases(s.owner.ListAdmin(ctx, filters))
}

func (s changeRequestService) GetAdmin(ctx context.Context, changeRequestID int64) (*ChangeRequestAggregate, error) {
	return changeRequestCase(s.owner.GetAdmin(ctx, changeRequestID))
}

func (s changeRequestService) AskQuestion(ctx context.Context, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestAggregate, error) {
	return changeRequestCase(s.owner.AskQuestion(ctx, changeRequestID, input))
}

func (s changeRequestService) Reject(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error) {
	return changeRequestCase(s.owner.Reject(ctx, changeRequestID, input))
}

func (s changeRequestService) Approve(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestAggregate, error) {
	return changeRequestCase(s.owner.Approve(ctx, changeRequestID, input))
}

func (s changeRequestService) CorrectApprovedChildData(ctx context.Context, input CorrectApprovedChildDataInput) (*ChangeRequestAggregate, error) {
	return changeRequestCase(s.owner.CorrectApprovedChildData(ctx, input))
}

func (s changeRequestService) ListForReview(ctx context.Context, query ChangeRequestReviewQuery) ([]*ChangeRequestReviewRow, *usersService.HistoryCursor, error) {
	items, cursor, err := s.owner.ListForReview(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	rows := make([]*ChangeRequestReviewRow, 0, len(items))
	for _, item := range items {
		changeRequest, err := changeRequestValue(item.ChangeRequest)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, &ChangeRequestReviewRow{
			ChangeRequest: changeRequest, ChildNames: item.ChildNames, ChildIDs: item.ChildIDs,
			Children: item.Children, GuardianName: item.GuardianName, ReviewerName: item.ReviewerName,
		})
	}
	var next *usersService.HistoryCursor
	if cursor != nil {
		next = &usersService.HistoryCursor{UpdatedAt: cursor.UpdatedAt, ID: cursor.ID}
	}
	return rows, next, nil
}

func (s changeRequestService) CountOpenForReview(ctx context.Context, statuses []string) (int, error) {
	return s.owner.CountOpenForReview(ctx, statuses)
}
