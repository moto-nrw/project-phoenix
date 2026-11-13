package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/carerequests"
)

// reviewDiff sets the diff the queue shows, falling back to the requested
// summary when the live plan cannot be read. A weekly request also carries
// the token of exactly that diff, so its approval can refuse a plan that
// changed after the reviewer saw it (#6843).
func (s *ScheduleReviews) reviewDiff(ctx context.Context, row *careplan.CareScheduleChangeRequest, student careplan.ReviewStudent, item *careplan.CareScheduleReviewItem, today careplan.Date, plans reviewPlanCache) {
	if err := s.careRequestDiff(ctx, row, student, item, today, plans); err != nil {
		s.deps.Logger.Warn("schedule: build care request diff failed",
			"request_id", row.ID,
			"error", err.Error(),
		)
		item.Diff = requestedCareSummary(row.Payload)
	}
	if row.RequestKind != carerequests.KindPickupChange && row.RequestKind != carerequests.KindPickupPermission {
		item.ImpactToken = s.deps.Fingerprint(carerequests.WeeklyImpactContent(item.Diff))
	}
}

// WeeklyImpactToken recomputes the token the pending queue shows for request,
// from the same live plan and clock. It runs inside the deciding unit, which
// already authorized and locked the child, and opens no unit of its own.
func (s *scheduleReviewCommands) WeeklyImpactToken(ctx context.Context, request carerequests.Request) (string, error) {
	reviews := s.commands
	students, err := reviews.deps.People.FindStudents(ctx, []int64{request.StudentID})
	if err != nil {
		return "", fmt.Errorf("schedule: load student for weekly impact: %w", err)
	}
	student, found := students[request.StudentID]
	if !found {
		return "", careplan.ErrCareScheduleRequestNotFound
	}
	item := &careplan.CareScheduleReviewItem{Request: &request}
	plans := reviewPlanCache{plans: map[int64]reviewPlanFacts{}, errs: map[int64]error{}}
	reviews.reviewDiff(ctx, &request, student, item, reviews.deps.Today(), plans)
	return item.ImpactToken, nil
}
