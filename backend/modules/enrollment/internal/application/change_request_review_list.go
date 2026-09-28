package application

import (
	"context"
	"fmt"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Anmeldungsänderungen in the request module (#2435): the review rows in the
// shared display format of the Eltern tab — who filed what for which child,
// and who decided it when.

// reviewerUnknown names a decider whose account is gone.
const reviewerUnknown = "Unbekannt"

// ListForReview serves the Eltern tab: open requests ordered by submission,
// decided ones by decision, keyset-paginated.
func (s *ChangeRequests) ListForReview(ctx context.Context, query enrollment.ChangeRequestReviewQuery) ([]*enrollment.ChangeRequestReviewItem, *enrollment.ChangeRequestReviewCursor, error) {
	items, next, err := s.listForReview(ctx, query)
	return items, next, publicError(err)
}

// CountOpenForReview is the number of Anmeldungsänderungen still waiting for
// a decision, counted in the database rather than by one page's length.
func (s *ChangeRequests) CountOpenForReview(ctx context.Context, statuses []string) (int, error) {
	count, err := s.countOpenForReview(ctx, statuses)
	return count, publicError(err)
}

func (s *ChangeRequests) countOpenForReview(ctx context.Context, statuses []string) (int, error) {
	count, err := s.deps.Requests.CountChangeRequestsForReview(ctx, statuses)
	if err != nil {
		return 0, fmt.Errorf("change request review count: %w", err)
	}
	return count, nil
}

func (s *ChangeRequests) listForReview(ctx context.Context, query enrollment.ChangeRequestReviewQuery) ([]*enrollment.ChangeRequestReviewItem, *enrollment.ChangeRequestReviewCursor, error) {
	if query.Limit <= 0 || len(query.Statuses) == 0 {
		return []*enrollment.ChangeRequestReviewItem{}, nil, nil
	}
	// limit+1 probes for a further page without a second count query.
	rows, err := changeRequestsFromOwner(s.deps.Requests.ChangeRequestsForReview(ctx, enrollment.ChangeRequestReviewFilters{
		Statuses: query.Statuses, Search: strings.TrimSpace(query.Search), History: query.History,
		From: query.From, To: query.To, BeforeInstant: query.BeforeInstant, BeforeID: query.BeforeID, Limit: query.Limit + 1,
	}))
	if err != nil {
		return nil, nil, fmt.Errorf("change request review list: %w", err)
	}
	var next *enrollment.ChangeRequestReviewCursor
	if len(rows) > query.Limit {
		rows = rows[:query.Limit]
		last := rows[len(rows)-1]
		position := last.CreatedAt
		if query.History {
			position = last.decisionInstant()
		}
		next = &enrollment.ChangeRequestReviewCursor{UpdatedAt: position, ID: last.ID}
	}
	if len(rows) == 0 {
		return []*enrollment.ChangeRequestReviewItem{}, nil, nil
	}
	requests, children, reviewers, err := s.reviewListLookups(ctx, rows)
	if err != nil {
		return nil, nil, err
	}
	items := make([]*enrollment.ChangeRequestReviewItem, 0, len(rows))
	for _, row := range rows {
		item, err := reviewItem(row, requests[row.RequestID], children[row.RequestID], reviewers)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, item)
	}
	return items, next, nil
}

func reviewItem(row *ChangeRequest, req *enrollmentModels.Request, children []*RequestChild, reviewers map[int64]string) (*enrollment.ChangeRequestReviewItem, error) {
	value, err := changeRequestToOwner(row)
	if err != nil {
		return nil, err
	}
	affected := affectedReviewChildren(children, row.RequestChildID)
	item := &enrollment.ChangeRequestReviewItem{
		ChangeRequest: value, ChildNames: reviewChildNames(affected), ChildIDs: reviewStudentIDs(affected),
		Children: affected, ReviewerName: reviewerDisplayName(reviewers, row.ReviewedByAccountID),
	}
	if req != nil {
		item.GuardianName = strings.TrimSpace(req.GuardianFirstName + " " + req.GuardianLastName)
	}
	return item, nil
}

// reviewerDisplayName is empty while undecided and "Unbekannt" when the
// deciding account is gone.
func reviewerDisplayName(reviewers map[int64]string, reviewedBy *int64) string {
	if reviewedBy == nil || *reviewedBy <= 0 {
		return ""
	}
	if name, ok := reviewers[*reviewedBy]; ok {
		return name
	}
	return reviewerUnknown
}

func affectedReviewChildren(children []*RequestChild, pinnedChildID *int64) []enrollment.ChangeRequestReviewChild {
	result := make([]enrollment.ChangeRequestReviewChild, 0, len(children))
	for _, child := range children {
		if pinnedChildID != nil && child.ID != *pinnedChildID {
			continue
		}
		studentID := child.CreatedStudentID
		if studentID == nil {
			studentID = child.MatchedStudentID
		}
		result = append(result, enrollment.ChangeRequestReviewChild{
			RequestChildID: child.ID, StudentID: studentID, Name: strings.TrimSpace(child.FirstName + " " + child.LastName),
		})
	}
	return result
}

// reviewListLookups batch-loads everything the page needs: one read per
// kind, never one per row.
func (s *ChangeRequests) reviewListLookups(ctx context.Context, rows []*ChangeRequest) (map[int64]*enrollmentModels.Request, map[int64][]*RequestChild, map[int64]string, error) {
	requestIDs, reviewerIDs := reviewListIDs(rows)
	requestRows, err := decodedRequestsByID(ctx, s.deps.Requests, requestIDs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("change request review list: load enrollments: %w", err)
	}
	requests := make(map[int64]*enrollmentModels.Request, len(requestRows))
	for _, req := range requestRows {
		requests[req.ID] = req
	}
	childRows, err := decodedChildrenOfRequests(ctx, s.deps.Children, requestIDs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("change request review list: load children: %w", err)
	}
	children := make(map[int64][]*RequestChild, len(requestIDs))
	for _, child := range childRows {
		children[child.RequestID] = append(children[child.RequestID], child)
	}
	// Without the reviewer names every decided row reads "Unbekannt" —
	// misleading, but better than refusing the history over a display name.
	reviewers := map[int64]string{}
	if s.deps.Reviewers != nil && len(reviewerIDs) > 0 {
		if reviewers, err = s.deps.Reviewers.ReviewerNames(ctx, reviewerIDs); err != nil {
			return nil, nil, nil, fmt.Errorf("change request review list: load reviewers: %w", err)
		}
	}
	return requests, children, reviewers, nil
}

func reviewListIDs(rows []*ChangeRequest) ([]int64, []int64) {
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
	return requestIDs, reviewerIDs
}

func reviewChildNames(children []enrollment.ChangeRequestReviewChild) []string {
	names := make([]string, 0, len(children))
	for _, child := range children {
		names = append(names, child.Name)
	}
	return names
}

func reviewStudentIDs(children []enrollment.ChangeRequestReviewChild) []int64 {
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		if child.StudentID != nil {
			ids = append(ids, *child.StudentID)
		}
	}
	return ids
}
