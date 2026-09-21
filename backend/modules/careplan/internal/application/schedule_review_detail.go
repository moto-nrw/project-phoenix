package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// GetForReview authorizes the child before loading display identities. It
// serves every request status and never reads today's schedule.
func (s *ScheduleReviews) GetForReview(ctx context.Context, id int64) (*carerequests.HistoryItem, error) {
	if id <= 0 {
		return nil, careplan.ErrCareScheduleRequestNotFound
	}
	request, err := s.deps.Requests.FindCareScheduleRequest(ctx, id, false)
	if err != nil {
		return nil, fmt.Errorf("schedule: load care request: %w", err)
	}
	students, err := s.deps.People.FindStudents(ctx, []int64{request.StudentID})
	if err != nil {
		return nil, fmt.Errorf("schedule: load student for care request: %w", err)
	}
	student, found := students[request.StudentID]
	if !found {
		return nil, careplan.ErrCareScheduleRequestNotFound
	}
	scope, err := s.deps.Scope(ctx)
	if err != nil {
		return nil, fmt.Errorf("schedule: resolve request reviewer scope: %w", err)
	}
	if !scope.Allows(&student) {
		return nil, carerequests.ErrCareRequestForbidden
	}
	item := storedCareRequestDetail(&request)
	names, err := s.deps.People.PersonNames(ctx, []int64{student.PersonID})
	if err != nil {
		return nil, fmt.Errorf("schedule: load person for care request: %w", err)
	}
	name := names[student.PersonID]
	item.FirstName, item.LastName = name.FirstName, name.LastName
	if request.ReviewedBy != nil && *request.ReviewedBy > 0 {
		reviewers, readErr := s.deps.People.ReviewerNames(ctx, []int64{*request.ReviewedBy})
		if readErr != nil {
			return nil, fmt.Errorf("schedule: load reviewer for care request: %w", readErr)
		}
		item.ReviewerName = reviewerDisplayName(reviewers, request.ReviewedBy)
	}
	return item, nil
}

func storedCareRequestDetail(request *carerequests.Request) *carerequests.HistoryItem {
	item := &carerequests.HistoryItem{Request: request,
		Requested: requestedCareSummary(request.Payload), Diff: careReviewSnapshot(request.DecisionSnapshot)}
	var payload map[string]any
	if json.Unmarshal(request.Payload, &payload) != nil {
		return item
	}
	if reason, ok := payload["reason"].(string); ok && strings.TrimSpace(reason) != "" {
		trimmed := strings.TrimSpace(reason)
		item.RequestReason = &trimmed
	}
	if request.RequestKind == "pickup_change" {
		item.PickupChange = carerequests.StoredPickupTerms(request.Payload)
	}
	return item
}
