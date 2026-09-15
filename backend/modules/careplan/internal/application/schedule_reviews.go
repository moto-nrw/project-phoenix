package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type ScheduleReviewDependencies struct {
	Requests              careplan.CareScheduleRequestQuery
	Schedules             *Service
	People                ports.ScheduleReviewDirectory
	Bookings              ports.ReviewBookings
	Classes               ports.ReviewClassTimes
	Scope                 ports.ReviewScopeResolver
	BookingsAuthoritative func(context.Context) (bool, error)
	Today                 func() careplan.Date
	Blocks                ports.PickupReviewBlocks
	Logger                *slog.Logger
}

type ScheduleReviews struct{ deps ScheduleReviewDependencies }

func NewScheduleReviews(deps ScheduleReviewDependencies) *ScheduleReviews {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &ScheduleReviews{deps: deps}
}

func (s *ScheduleReviews) ListPending(ctx context.Context, filter careplan.RequestQueueFilter) ([]*careplan.CareScheduleReviewItem, *careplan.RequestCursor, error) {
	probe := probeLimit(filter)
	rows, err := s.deps.Requests.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{Statuses: []string{"pending"}, Queue: &probe})
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: list pending care requests: %w", err)
	}
	rows, next := nextCursor(rows, filter.Limit, func(row careplan.CareScheduleChangeRequest) (time.Time, int64) { return row.CreatedAt, row.ID })
	items := make([]*careplan.CareScheduleReviewItem, 0, len(rows))
	if len(rows) == 0 {
		return items, next, nil
	}
	students, names, err := s.people(ctx, rows, "care requests")
	if err != nil {
		return nil, nil, err
	}
	scope, err := s.deps.Scope(ctx)
	if err != nil {
		return nil, nil, err
	}
	today, err := careplan.ParseDate(filter.UrgentDate)
	if err != nil {
		today = s.deps.Today()
	}
	plans := map[int64]reviewPlanFacts{}
	planErrors := map[int64]error{}
	for i := range rows {
		row := &rows[i]
		student, found := students[row.StudentID]
		if !found || !scope.Allows(&student) || student.Alumnus || student.CareEndedOn(today) {
			continue
		}
		name := names[student.PersonID]
		item := &careplan.CareScheduleReviewItem{Request: row, FirstName: name.FirstName, LastName: name.LastName}
		if row.RequestKind == "pickup_change" {
			item.Diff, err = s.pickupDiff(ctx, row, student, item)
		} else {
			plan, loaded := plans[student.ID]
			planErr, failed := planErrors[student.ID]
			if !loaded && !failed {
				plan, planErr = s.plan(ctx, student, today, true)
				if planErr != nil {
					planErrors[student.ID] = planErr
				} else {
					plans[student.ID] = plan
				}
			}
			err = planErr
			if err == nil {
				var payload careplan.CareWeeklyChange
				err = json.Unmarshal(row.Payload, &payload)
				if err == nil {
					var current careplan.CareWeeklyPlan
					current, err = plan.weekly(today)
					if err == nil {
						item.Diff = careplan.WeeklyCareDiff(payload.Weekdays, current)
					}
				}
			}
		}
		if err != nil {
			s.deps.Logger.Warn("schedule: build care request diff failed",
				"request_id", row.ID,
				"error", err.Error(),
			)
			item.Diff = requestedCareSummary(row.Payload)
		}
		items = append(items, item)
	}
	return items, next, nil
}

func (s *ScheduleReviews) people(ctx context.Context, rows []careplan.CareScheduleChangeRequest, operation string) (map[int64]ports.ReviewStudent, map[int64]ports.PersonName, error) {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
	}
	students, err := s.deps.People.FindStudents(ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: load students for %s: %w", operation, err)
	}
	ids = ids[:0]
	for _, student := range students {
		ids = append(ids, student.PersonID)
	}
	names, err := s.deps.People.PersonNames(ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: load persons for %s: %w", operation, err)
	}
	return students, names, nil
}

func (s *ScheduleReviews) ListHistory(ctx context.Context, filter careplan.RequestQueueFilter) ([]*careplan.CareScheduleHistoryItem, *careplan.RequestCursor, error) {
	probe := probeLimit(filter)
	rows, err := s.deps.Requests.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{Statuses: []string{"approved", "rejected", "withdrawn"}, Queue: &probe})
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: list decided care requests: %w", err)
	}
	rows, next := nextCursor(rows, filter.Limit, func(row careplan.CareScheduleChangeRequest) (time.Time, int64) { return row.UpdatedAt, row.ID })
	items := make([]*careplan.CareScheduleHistoryItem, 0, len(rows))
	if len(rows) == 0 {
		return items, next, nil
	}
	students, names, err := s.people(ctx, rows, "care request history")
	if err != nil {
		return nil, nil, err
	}
	reviewerIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.ReviewedBy != nil && *row.ReviewedBy > 0 {
			reviewerIDs = append(reviewerIDs, *row.ReviewedBy)
		}
	}
	reviewers, err := s.deps.People.ReviewerNames(ctx, reviewerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("schedule: load reviewers for care request history: %w", err)
	}
	scope, err := s.deps.Scope(ctx)
	if err != nil {
		return nil, nil, err
	}
	for i := range rows {
		student, found := students[rows[i].StudentID]
		if !found || !scope.Allows(&student) || student.Alumnus {
			continue
		}
		name := names[student.PersonID]
		items = append(items, &careplan.CareScheduleHistoryItem{
			Request: &rows[i], FirstName: name.FirstName, LastName: name.LastName,
			ReviewerName: reviewerDisplayName(reviewers, rows[i].ReviewedBy),
			Requested:    requestedCareSummary(rows[i].Payload), Diff: careReviewSnapshot(rows[i].DecisionSnapshot),
		})
	}
	return items, next, nil
}

func (s *ScheduleReviews) plan(ctx context.Context, student ports.ReviewStudent, date careplan.Date, weekly bool) (reviewPlanFacts, error) {
	facts := reviewPlanFacts{}
	filter := careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}}
	var err error
	facts.pickups, err = s.deps.Schedules.ListPickupSchedules(ctx, filter)
	if err != nil {
		return facts, err
	}
	from, to := date, date
	if weekly {
		from = date.StartOfISOWeek()
		to = from.AddDays(4)
		if date.After(to) {
			to = date
		}
		facts.arrivals, err = s.deps.Schedules.ListArrivalSchedules(ctx, filter)
		if err != nil {
			return facts, err
		}
		classes, readErr := s.deps.Classes.ListClassArrivalTimes(ctx, []string{student.SchoolClass})
		if readErr != nil {
			return facts, readErr
		}
		facts.classTimes = classes[strings.ToLower(strings.TrimSpace(student.SchoolClass))]
		modes, readErr := s.deps.People.DepartureModes(ctx, []int64{student.ID})
		if readErr != nil {
			return facts, readErr
		}
		facts.departureModes = modes[student.ID]
	}
	facts.authoritative, err = s.deps.BookingsAuthoritative(ctx)
	if err != nil {
		return facts, err
	}
	bookings, err := s.deps.Bookings.ApprovedForStudents(ctx, []int64{student.ID}, from, to)
	if err != nil {
		return facts, err
	}
	ids := make([]int64, 0, len(bookings))
	for _, booking := range bookings {
		if booking.StudentID == student.ID {
			ids = append(ids, booking.OfferingID)
		}
	}
	if len(ids) == 0 {
		return facts, nil
	}
	offerings, err := s.deps.Schedules.ListCareOfferings(ctx, domain.CareOfferingFilter{IDs: ids})
	if err != nil {
		return facts, err
	}
	byID := make(map[int64]careplan.CareOffering, len(offerings))
	for _, row := range offerings {
		byID[row.ID] = careplan.CareOffering{ID: row.ID, IsActive: row.IsActive, CountsAsCare: row.CountsAsCare, DaysOfWeekMode: row.DaysOfWeekMode, AvailableDays: row.AvailableDays, PickupTimes: row.PickupTimes}
	}
	for _, booking := range bookings {
		if booking.StudentID != student.ID {
			continue
		}
		offering, found := byID[booking.OfferingID]
		if found {
			facts.bookings = append(facts.bookings, reviewPlanSelection{offering: offering, selectedDays: booking.SelectedDays, from: booking.ValidFrom, until: booking.ValidUntil})
		}
	}
	return facts, nil
}
