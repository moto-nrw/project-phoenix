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
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type reviewPickupPayload struct {
	Date               string `json:"date"`
	PickupTime         string `json:"pickup_time"`
	PreviousPickupTime string `json:"previous_pickup_time"`
	Reason             string `json:"reason"`
}

func (p reviewPickupPayload) terms() (careplan.Date, time.Time, error) {
	date, err := careplan.ParseDate(p.Date)
	if err != nil {
		return "", time.Time{}, err
	}
	clock, err := time.Parse("15:04", p.PickupTime)
	if err != nil {
		return "", time.Time{}, err
	}
	return date, time.Date(1, time.January, 1, clock.Hour(), clock.Minute(), 0, 0, time.UTC), nil
}

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

func (s *ScheduleReviews) pickupDiff(ctx context.Context, row *careplan.CareScheduleChangeRequest, student ports.ReviewStudent, item *careplan.CareScheduleReviewItem) ([]careplan.CareRequestDiffEntry, error) {
	var payload reviewPickupPayload
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return nil, err
	}
	if reason := strings.TrimSpace(payload.Reason); reason != "" {
		item.Reason = &reason
	}
	date, clock, err := payload.terms()
	if err != nil {
		return nil, err
	}
	if item.Reason == nil || utf8.RuneCountInString(*item.Reason) > 255 {
		return nil, fmt.Errorf("invalid care request payload")
	}
	if s.deps.Blocks != nil {
		blocks, previewErr := s.pickupBlocks(ctx, student, date, clock)
		if previewErr != nil {
			s.deps.Logger.Warn("schedule: preview pickup change blocks failed",
				"request_id", row.ID,
				"error", previewErr.Error(),
			)
		} else {
			item.AffectedBlocks, item.ImpactAvailable = blocks, true
			item.ImpactToken = s.deps.Fingerprint(carerequests.PickupImpactContent(blocks))
		}
	}
	old := payload.PreviousPickupTime
	if old == "" {
		exceptions, readErr := s.deps.Schedules.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}, Date: date})
		if readErr != nil {
			return nil, readErr
		}
		for _, exception := range exceptions {
			if exception.PickupTime != nil {
				old = exception.PickupTime.Format("15:04")
				break
			}
		}
	}
	if old == "" {
		facts, readErr := s.plan(ctx, student, date, false)
		if readErr != nil {
			return nil, readErr
		}
		times, readErr := facts.pickupWeek(date)
		if readErr != nil {
			return nil, readErr
		}
		old = times[int(date.Weekday())]
	}
	return []careplan.CareRequestDiffEntry{{Label: date.Format("02.01.2006") + " · Abholzeit", Old: old, New: clock.Format("15:04"), CareKind: "pickup"}}, nil
}

func (s *ScheduleReviews) pickupBlocks(ctx context.Context, student ports.ReviewStudent, date careplan.Date, clock time.Time) ([]careplan.CareReviewBlock, error) {
	result := []careplan.CareReviewBlock{}
	weekday := int(date.Weekday())
	if weekday < 1 || weekday > 5 {
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
	input := ports.PickupReviewImpact{StudentID: student.ID, Date: date, From: clock, Enrolled: !student.Alumnus}
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
