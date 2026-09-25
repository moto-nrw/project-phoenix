package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Refusals of the change requests (#3565). Handlers render err.Error().
var (
	ErrChangeRequestNotFound      = errors.New("enrollment change request not found")
	ErrChangeRequestNotAllowed    = errors.New("enrollment change request is not allowed")
	ErrChangeRequestInvalidStatus = errors.New("enrollment change request has invalid status")
	ErrChangeRequestInvalidData   = errors.New("enrollment change request data is invalid")
	ErrChangeRequestConflict      = errors.New("enrollment change request conflicts with newer enrollment data")
	// ErrChangeRequestChildLocked refuses a change to a child that is
	// already taken over into care; those changes run through the parent
	// app (ADR 0003).
	ErrChangeRequestChildLocked = errors.New("enrollment change request cannot change a child that is already in care")
)

// CreateChangeRequestInput is a family's proposed correction of its request.
type CreateChangeRequestInput struct {
	Submission         SubmitRequest
	ParentNote         string
	CreatedByAccountID *int64
}

// ChangeRequestMessageInput is one message of the question-and-answer thread.
type ChangeRequestMessageInput struct {
	Body           string
	ActorAccountID int64
}

// ReviewChangeRequestInput is a staff decision on a change request.
type ReviewChangeRequestInput struct {
	Note           string
	ActorAccountID int64
	ActorRole      string
}

// CorrectApprovedChildDataInput is a staff correction of an approved child's
// identity, applied to the enrollment record and the student it created.
type CorrectApprovedChildDataInput struct {
	RequestID         int64
	ChildID           int64
	FirstName         string
	LastName          string
	DateOfBirth       calendar.Date
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	Reason            string
	ActorAccountID    int64
}

// ChangeRequestCase is a change request with the request, children, messages
// and phase it concerns.
type ChangeRequestCase struct {
	ChangeRequest *ChangeRequest
	Request       *Request
	Children      []*RequestChild
	Messages      []*ChangeRequestMessage
	Phase         *Phase
}

// ChangeRequestFilters narrow the admin list. Zero values are ignored.
type ChangeRequestFilters struct {
	RequestID int64
	Status    string
	Limit     int
}

// ChangeRequestReviewQuery is the parsed request of the review list.
type ChangeRequestReviewQuery struct {
	// Statuses is the exact status set to return. Empty returns nothing.
	Statuses []string
	Search   string
	// History switches from the open working list (ordered by submission) to
	// the decided history (ordered by decision).
	History bool
	// From and To bound the decision instant. History only, zero = unbounded.
	From, To time.Time
	// BeforeInstant/BeforeID continue a previous page; zero starts at the top.
	BeforeInstant time.Time
	BeforeID      int64
	Limit         int
}

// ChangeRequestReviewItem is one change request plus the names the review
// list shows.
type ChangeRequestReviewItem struct {
	ChangeRequest *ChangeRequest
	// ChildNames are the affected children: the pinned one when the request
	// targets a single child, every child of the enrollment otherwise.
	ChildNames []string
	// ChildIDs contains the linked student IDs of the affected children.
	ChildIDs []int64
	// Children carries one stable case identity per affected child.
	Children []ChangeRequestReviewChild
	// GuardianName is the person who filed the enrollment.
	GuardianName string
	// ReviewerName is who decided; empty while undecided, "Unbekannt" when
	// the deciding account is gone.
	ReviewerName string
}

// ChangeRequestReviewChild is one affected child of a review list entry.
// StudentID joins an already imported child to the other request kinds.
type ChangeRequestReviewChild struct {
	RequestChildID int64
	StudentID      *int64
	Name           string
}

// ChangeRequestReviewCursor continues the review list after the last item
// of a page.
type ChangeRequestReviewCursor struct {
	UpdatedAt time.Time
	ID        int64
}

// DecisionInstant is when a decided change request was decided. Older rows
// may lack reviewed_at, so their last update is the decision time.
func (r *ChangeRequest) DecisionInstant() time.Time {
	if r.ReviewedAt != nil {
		return *r.ReviewedAt
	}
	return r.UpdatedAt
}

// ChangeRequests runs the family's change requests and staff's direct
// corrections of an enrollment. The family's calls resolve the status token
// themselves; staff calls run in the caller's tenant transaction.
type ChangeRequests interface {
	Create(ctx context.Context, token string, input CreateChangeRequestInput) (*ChangeRequestCase, error)
	ListPublic(ctx context.Context, token string) ([]*ChangeRequestCase, error)
	ParentReply(ctx context.Context, token string, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestCase, error)
	ListAdmin(ctx context.Context, filters ChangeRequestFilters) ([]*ChangeRequestCase, error)
	GetAdmin(ctx context.Context, changeRequestID int64) (*ChangeRequestCase, error)
	AskQuestion(ctx context.Context, changeRequestID int64, input ChangeRequestMessageInput) (*ChangeRequestCase, error)
	Reject(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestCase, error)
	Approve(ctx context.Context, changeRequestID int64, input ReviewChangeRequestInput) (*ChangeRequestCase, error)
	CorrectApprovedChildData(ctx context.Context, input CorrectApprovedChildDataInput) (*ChangeRequestCase, error)
	// ListForReview serves the request module's Eltern tab: open requests
	// ordered by submission, decided ones by decision, keyset-paginated.
	ListForReview(ctx context.Context, query ChangeRequestReviewQuery) ([]*ChangeRequestReviewItem, *ChangeRequestReviewCursor, error)
	// CountOpenForReview counts the rows in the given statuses.
	CountOpenForReview(ctx context.Context, statuses []string) (int, error)
}
