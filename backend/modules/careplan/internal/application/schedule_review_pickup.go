package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func requestedCareSummary(raw json.RawMessage) []careplan.CareRequestDiffEntry {
	return carerequests.RequestedSummary(raw)
}

func careReviewSnapshot(raw json.RawMessage) []careplan.CareRequestDiffEntry {
	var snapshot *carerequests.DecisionSnapshot
	if json.Unmarshal(raw, &snapshot) != nil {
		return nil
	}
	return snapshot.Entries()
}

func (s *ScheduleReviews) pickupDiff(ctx context.Context, row *careplan.CareScheduleChangeRequest, student careplan.ReviewStudent, item *careplan.CareScheduleReviewItem) ([]careplan.CareRequestDiffEntry, error) {
	var payload map[string]any
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return nil, err
	}
	if reason, _ := payload["reason"].(string); strings.TrimSpace(reason) != "" {
		trimmed := strings.TrimSpace(reason)
		item.Reason = &trimmed
	}
	terms := carerequests.StoredPickupTerms(row.Payload)
	if terms == nil {
		return nil, fmt.Errorf("invalid care request payload")
	}
	if item.Reason == nil || utf8.RuneCountInString(*item.Reason) > 255 {
		return nil, fmt.Errorf("invalid care request payload")
	}
	change := carerequests.PickupChange{Date: terms.Date, DepartureMode: terms.DepartureMode}
	if terms.PickupPersonName != "" {
		change.PickupPerson = &carerequests.PickupPerson{Name: terms.PickupPersonName}
	}
	old := ""
	if terms.PickupTime != "" {
		clock, err := time.Parse("15:04", terms.PickupTime)
		if err != nil {
			return nil, err
		}
		clock = time.Date(1, time.January, 1, clock.Hour(), clock.Minute(), 0, 0, time.UTC)
		change.PickupTime = &clock
		s.previewPickupImpact(ctx, row, student, careplan.Date(terms.Date), clock, item)
		if old, err = s.oldReviewPickup(ctx, student, careplan.Date(terms.Date), terms.PreviousPickupTime); err != nil {
			return nil, err
		}
	}
	// An Abholart-only request touches no block, so it carries no impact.
	return carerequests.PickupChangeEntries(change, old, terms.PreviousDepartureModes), nil
}

// permissionDiff is the diff of a pickup permission request (#4780): the
// person's days before (recorded at submission) and the days asked for.
func permissionDiff(row *careplan.CareScheduleChangeRequest, item *careplan.CareScheduleReviewItem) ([]careplan.CareRequestDiffEntry, error) {
	permission, err := carerequests.ParsePickupPermission(row.Payload)
	if err != nil {
		return nil, err
	}
	if reason := strings.TrimSpace(permission.Reason); reason != "" {
		item.Reason = &reason
	}
	var payload struct {
		Previous string `json:"previous_label"`
	}
	_ = json.Unmarshal(row.Payload, &payload)
	return carerequests.PickupPermissionEntries(permission, payload.Previous), nil
}

func (s *ScheduleReviews) previewPickupImpact(ctx context.Context, row *careplan.CareScheduleChangeRequest, student careplan.ReviewStudent, date careplan.Date, clock time.Time, item *careplan.CareScheduleReviewItem) {
	if s.deps.Blocks == nil {
		return
	}
	blocks, previewErr := s.pickupBlocks(ctx, student, date, clock)
	if previewErr != nil {
		s.deps.Logger.Warn("schedule: preview pickup change blocks failed",
			"request_id", row.ID,
			"error", previewErr.Error(),
		)
		return
	}
	item.AffectedBlocks, item.ImpactAvailable = blocks, true
	item.ImpactToken = s.deps.Fingerprint(carerequests.PickupImpactContent(blocks))
}

func (s *ScheduleReviews) oldReviewPickup(ctx context.Context, student careplan.ReviewStudent, date careplan.Date, old string) (string, error) {
	if old != "" {
		return old, nil
	}
	exceptions, err := s.deps.Schedules.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}, Date: date})
	if err != nil {
		return "", err
	}
	for _, exception := range exceptions {
		if exception.PickupTime != nil {
			return exception.PickupTime.Format("15:04"), nil
		}
	}
	return s.weeklyPickup(ctx, student, date)
}

// weeklyPickup is the weekly pickup time that applies on date: Friday's on a
// weekend that follows Friday's plan (#3921).
func (s *ScheduleReviews) weeklyPickup(ctx context.Context, student careplan.ReviewStudent, date careplan.Date) (string, error) {
	facts, err := s.plan(ctx, student, date, false)
	if err != nil {
		return "", err
	}
	times, err := facts.pickupWeek(date)
	if err != nil {
		return "", err
	}
	weekday, err := calendar.PlanWeekday(ctx, calendar.Date(date))
	if err != nil {
		return "", err
	}
	return times[weekday], nil
}

func (s *ScheduleReviews) pickupBlocks(ctx context.Context, student careplan.ReviewStudent, date careplan.Date, clock time.Time) ([]careplan.CareReviewBlock, error) {
	result := []careplan.CareReviewBlock{}
	weekday, err := calendar.PlanWeekday(ctx, calendar.Date(date))
	if err != nil {
		return nil, err
	}
	if weekday > 5 {
		return result, nil
	}
	facts, err := s.plan(ctx, student, date, false)
	if err != nil {
		return nil, err
	}
	times, err := facts.pickupWeek(date)
	if err != nil {
		return nil, err
	}
	if baseline := times[weekday]; baseline == "" || clock.Format("15:04") >= baseline {
		return result, nil
	}
	exceptions, err := s.deps.Schedules.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}, Date: date})
	if err != nil {
		return nil, err
	}
	input := careplan.PickupReviewImpact{StudentID: student.ID, Date: date, From: clock, Enrolled: !student.Alumnus}
	for _, exception := range exceptions {
		if exception.ExcusedAuto {
			input.AutoExceptionIDs = append(input.AutoExceptionIDs, exception.ID)
		}
	}
	blocks, err := s.deps.Blocks.PreviewPickupBlocks(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("auto excusal: preview affected blocks: %w", err)
	}
	for _, block := range blocks {
		result = append(result, careplan.CareReviewBlock{ID: block.ID, Title: block.Title, StartTime: block.StartTime, EndTime: block.EndTime})
	}
	return result, nil
}
