package enrollment

import (
	"context"
	"fmt"
	"strings"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Anmeldungsänderungen in the request module (#2435): the same rows the
// enrollment review surface has always shown, projected into the shared
// display format of the Eltern tab — who filed what for which child, and who
// decided it when. Until now this system had no history at all: a decided
// change request simply left the list.

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

type ChangeRequestIntakeRequests interface {
	ChangeRequestByID(context.Context, int64) (*capability.ChangeRequest, error)
	ChangeRequestByIDForUpdate(context.Context, int64) (*capability.ChangeRequest, error)
	ChangeRequestsForRequest(context.Context, int64) ([]*capability.ChangeRequest, error)
	OpenChangeRequestsForRequestForUpdate(context.Context, int64) ([]*capability.ChangeRequest, error)
	ListChangeRequests(context.Context, capability.ChangeRequestListFilters) ([]*capability.ChangeRequest, error)
	ChangeRequestsForReview(context.Context, capability.ChangeRequestReviewFilters) ([]*capability.ChangeRequest, error)
	IntakeRequests
	InsertChangeRequest(context.Context, *capability.ChangeRequest) error
	ChangeRequestMessages(context.Context, []int64, bool) ([]*capability.ChangeRequestMessage, error)
	InsertChangeRequestMessage(context.Context, *capability.ChangeRequestMessage) error
	SetChangeRequestStatus(context.Context, int64, string) error
	MarkChangeRequestReviewed(context.Context, int64, string, *string, int64, time.Time) error
	CountChangeRequestsForReview(context.Context, []string) (int, error)
}

// ChangeRequestReviewItem is one change request plus the names the list shows.
// The row itself stays untouched, so the detail view keeps its own contract.
type ChangeRequestReviewItem struct {
	ChangeRequest *enrollmentModels.ChangeRequest
	// ChildNames are the affected children: the pinned one when the request
	// targets a single child, every child of the enrollment otherwise.
	ChildNames []string
	// ChildIDs contains linked Phoenix student IDs. A one-child change can join
	// the child's other requests without relying on a non-unique display name.
	ChildIDs []int64
	// Children carries one stable case identity per affected enrollment child.
	// RequestChildID keeps unlinked applications groupable across follow-up
	// change requests; StudentID joins an already imported child to other kinds.
	Children []ChangeRequestReviewChild
	// GuardianName is the person who filed the enrollment.
	GuardianName string
	// ReviewerName is who decided; empty while undecided, "Unbekannt" when the
	// deciding account is gone.
	ReviewerName string
}

type ChangeRequestReviewChild struct {
	RequestChildID int64
	StudentID      *int64
	Name           string
}

func (s *changeRequestService) ListForReview(
	ctx context.Context,
	query ChangeRequestReviewQuery,
) ([]*ChangeRequestReviewItem, *usersService.HistoryCursor, error) {
	if query.Limit <= 0 || len(query.Statuses) == 0 {
		return []*ChangeRequestReviewItem{}, nil, nil
	}
	// limit+1 probes for a further page without a second count query.
	rows, err := readChangeRequestsForReview(ctx, s.Requests, capability.ChangeRequestReviewFilters{
		Statuses:      query.Statuses,
		Search:        strings.TrimSpace(query.Search),
		History:       query.History,
		From:          query.From,
		To:            query.To,
		BeforeInstant: query.BeforeInstant,
		BeforeID:      query.BeforeID,
		Limit:         query.Limit + 1,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("change request review list: %w", err)
	}
	var next *usersService.HistoryCursor
	if len(rows) > query.Limit {
		rows = rows[:query.Limit]
		last := rows[len(rows)-1]
		position := last.CreatedAt
		if query.History {
			position = last.DecisionInstant()
		}
		next = &usersService.HistoryCursor{UpdatedAt: position, ID: last.ID}
	}
	if len(rows) == 0 {
		return []*ChangeRequestReviewItem{}, nil, nil
	}

	requests, children, reviewers, err := s.reviewListLookups(ctx, rows)
	if err != nil {
		return nil, nil, err
	}

	items := make([]*ChangeRequestReviewItem, 0, len(rows))
	for _, row := range rows {
		affectedChildren := affectedReviewChildren(children[row.RequestID], row.RequestChildID)
		item := &ChangeRequestReviewItem{
			ChangeRequest: row,
			ChildNames:    reviewChildNames(affectedChildren),
			ChildIDs:      reviewStudentIDs(affectedChildren),
			Children:      affectedChildren,
			ReviewerName:  usersService.ReviewerDisplayName(reviewers, row.ReviewedByAccountID),
		}
		if req := requests[row.RequestID]; req != nil {
			item.GuardianName = strings.TrimSpace(req.GuardianFirstName + " " + req.GuardianLastName)
		}
		items = append(items, item)
	}
	return items, next, nil
}

func affectedReviewChildren(children []*enrollmentModels.RequestChild, pinnedChildID *int64) []ChangeRequestReviewChild {
	result := make([]ChangeRequestReviewChild, 0, len(children))
	for _, child := range children {
		if pinnedChildID != nil && child.ID != *pinnedChildID {
			continue
		}
		var studentID *int64
		if child.CreatedStudentID != nil {
			studentID = child.CreatedStudentID
		} else if child.MatchedStudentID != nil {
			studentID = child.MatchedStudentID
		}
		result = append(result, ChangeRequestReviewChild{
			RequestChildID: child.ID,
			StudentID:      studentID,
			Name:           strings.TrimSpace(child.FirstName + " " + child.LastName),
		})
	}
	return result
}

// CountOpenForReview is the number of Anmeldungsänderungen still waiting for a
// decision — the badge's number, counted in the database instead of by the
// length of one page.
func (s *changeRequestService) CountOpenForReview(ctx context.Context, statuses []string) (int, error) {
	count, err := s.Requests.CountChangeRequestsForReview(ctx, statuses)
	if err != nil {
		return 0, fmt.Errorf("change request review count: %w", err)
	}
	return count, nil
}

// reviewListLookups batch-loads everything the page needs: one query per kind,
// never one per row.
func (s *changeRequestService) reviewListLookups(
	ctx context.Context,
	rows []*enrollmentModels.ChangeRequest,
) (
	map[int64]*enrollmentModels.Request,
	map[int64][]*enrollmentModels.RequestChild,
	map[int64]*userModels.Person,
	error,
) {
	requestIDs := make([]int64, 0, len(rows))
	reviewerIDs := make([]int64, 0, len(rows))
	seenRequest := make(map[int64]struct{}, len(rows))
	seenReviewer := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if _, ok := seenRequest[row.RequestID]; !ok {
			seenRequest[row.RequestID] = struct{}{}
			requestIDs = append(requestIDs, row.RequestID)
		}
		if row.ReviewedByAccountID == nil || *row.ReviewedByAccountID <= 0 {
			continue
		}
		if _, ok := seenReviewer[*row.ReviewedByAccountID]; !ok {
			seenReviewer[*row.ReviewedByAccountID] = struct{}{}
			reviewerIDs = append(reviewerIDs, *row.ReviewedByAccountID)
		}
	}

	requestRows, err := intakeRequestsByID(ctx, s.Requests, requestIDs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("change request review list: load enrollments: %w", err)
	}
	requests := make(map[int64]*enrollmentModels.Request, len(requestRows))
	for _, req := range requestRows {
		requests[req.ID] = req
	}

	childRows, err := listIntakeChildrenForRequests(ctx, s.Children, requestIDs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("change request review list: load children: %w", err)
	}
	children := make(map[int64][]*enrollmentModels.RequestChild, len(requestIDs))
	for _, child := range childRows {
		children[child.RequestID] = append(children[child.RequestID], child)
	}

	// The factory always wires PersonRepo; only narrow test setups leave it
	// nil. Without it every decided row reads "Unbekannt" (the name reserved
	// for deleted accounts), which is misleading but still better than
	// refusing the whole history over a display name.
	reviewers := map[int64]*userModels.Person{}
	if s.PersonRepo != nil && len(reviewerIDs) > 0 {
		reviewers, err = s.PersonRepo.FindByAccountIDs(ctx, reviewerIDs)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("change request review list: load reviewers: %w", err)
		}
	}
	return requests, children, reviewers, nil
}

// affectedChildNames lists the children a change request concerns, in the
// enrollment's own order.
func reviewChildNames(children []ChangeRequestReviewChild) []string {
	names := make([]string, 0, len(children))
	for _, child := range children {
		names = append(names, child.Name)
	}
	return names
}

func reviewStudentIDs(children []ChangeRequestReviewChild) []int64 {
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		if child.StudentID != nil {
			ids = append(ids, *child.StudentID)
		}
	}
	return ids
}
