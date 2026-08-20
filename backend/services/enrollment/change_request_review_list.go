package enrollment

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// ChangeRequestReviewItem is one change request plus the names the list shows.
// The row itself stays untouched, so the detail view keeps its own contract.
type ChangeRequestReviewItem struct {
	ChangeRequest *enrollmentModels.ChangeRequest
	// ChildNames are the affected children: the pinned one when the request
	// targets a single child, every child of the enrollment otherwise.
	ChildNames []string
	// GuardianName is the person who filed the enrollment.
	GuardianName string
	// ReviewerName is who decided; empty while undecided, "Unbekannt" when the
	// deciding account is gone.
	ReviewerName string
}

func (s *changeRequestService) ListForReview(
	ctx context.Context,
	query ChangeRequestReviewQuery,
) ([]*ChangeRequestReviewItem, *usersService.HistoryCursor, error) {
	if query.Limit <= 0 || len(query.Statuses) == 0 {
		return []*ChangeRequestReviewItem{}, nil, nil
	}
	// limit+1 probes for a further page without a second count query.
	rows, err := s.ChangeRequestRepo.ListForReview(ctx, enrollmentModels.ChangeRequestReviewFilters{
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
		item := &ChangeRequestReviewItem{
			ChangeRequest: row,
			ChildNames:    affectedChildNames(children[row.RequestID], row.RequestChildID),
			ReviewerName:  usersService.ReviewerDisplayName(reviewers, row.ReviewedByAccountID),
		}
		if req := requests[row.RequestID]; req != nil {
			item.GuardianName = strings.TrimSpace(req.GuardianFirstName + " " + req.GuardianLastName)
		}
		items = append(items, item)
	}
	return items, next, nil
}

// CountOpenForReview is the number of Anmeldungsänderungen still waiting for a
// decision — the badge's number, counted in the database instead of by the
// length of one page.
func (s *changeRequestService) CountOpenForReview(ctx context.Context, statuses []string) (int, error) {
	count, err := s.ChangeRequestRepo.CountForReview(ctx, statuses)
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

	requestRows, err := s.RequestRepo.ListByIDs(ctx, requestIDs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("change request review list: load enrollments: %w", err)
	}
	requests := make(map[int64]*enrollmentModels.Request, len(requestRows))
	for _, req := range requestRows {
		requests[req.ID] = req
	}

	childRows, err := s.RequestChildRepo.ListByRequestIDs(ctx, requestIDs)
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
func affectedChildNames(children []*enrollmentModels.RequestChild, pinnedChildID *int64) []string {
	names := make([]string, 0, len(children))
	for _, child := range children {
		if pinnedChildID != nil && child.ID != *pinnedChildID {
			continue
		}
		names = append(names, strings.TrimSpace(child.FirstName+" "+child.LastName))
	}
	return names
}
