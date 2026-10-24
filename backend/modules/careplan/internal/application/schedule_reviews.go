package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type ScheduleReviewSchedules interface {
	ListPickupSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error)
	ListArrivalSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
}

type ScheduleReviewDependencies struct {
	RunInTx               func(context.Context, func(context.Context) error) error
	Requests              careplan.CareScheduleRequestQuery
	Schedules             ScheduleReviewSchedules
	People                careplan.ScheduleReviewDirectory
	Bookings              careplan.ReviewBookings
	Classes               careplan.ReviewClassTimes
	Scope                 careplan.ReviewScopeResolver
	BookingsAuthoritative func(context.Context) (bool, error)
	Today                 func() careplan.Date
	Fingerprint           func([]byte) string
	Blocks                careplan.PickupReviewBlocks
	Logger                *slog.Logger
}

type ScheduleReviews struct{ deps ScheduleReviewDependencies }

func NewScheduleReviews(deps ScheduleReviewDependencies) careplan.CareScheduleReviewQuery {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &scheduleReviewCommands{&ScheduleReviews{deps: deps}}
}

func (s *scheduleReviewCommands) ListPending(ctx context.Context, filter careplan.RequestQueueFilter) (value0 []*careplan.CareScheduleReviewItem, value1 *careplan.RequestCursor, err error) {
	err = s.commands.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, value1, err = s.commands.ListPending(txCtx, filter)
		return err
	})
	return
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
	plans := reviewPlanCache{plans: map[int64]reviewPlanFacts{}, errs: map[int64]error{}}
	for i := range rows {
		row := &rows[i]
		student, found := students[row.StudentID]
		if !found || !scope.Allows(&student) || student.Alumnus || student.CareEndedOn(today) {
			continue
		}
		name := names[student.PersonID]
		item := &careplan.CareScheduleReviewItem{Request: row, FirstName: name.FirstName, LastName: name.LastName}
		if err := s.careRequestDiff(ctx, row, student, item, today, plans); err != nil {
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

// reviewPlanCache holds one weekly plan, or its load error, per student so a
// student with several requests is read once.
type reviewPlanCache struct {
	plans map[int64]reviewPlanFacts
	errs  map[int64]error
}

func (s *ScheduleReviews) cachedPlan(ctx context.Context, cache reviewPlanCache, student careplan.ReviewStudent, today careplan.Date) (reviewPlanFacts, error) {
	plan, loaded := cache.plans[student.ID]
	planErr, failed := cache.errs[student.ID]
	if loaded || failed {
		return plan, planErr
	}
	plan, planErr = s.plan(ctx, student, today, true)
	if planErr != nil {
		cache.errs[student.ID] = planErr
	} else {
		cache.plans[student.ID] = plan
	}
	return plan, planErr
}

// careRequestDiff sets the item's diff: a pickup change against the stored
// pickups, any other request against the student's current weekly plan.
func (s *ScheduleReviews) careRequestDiff(ctx context.Context, row *careplan.CareScheduleChangeRequest, student careplan.ReviewStudent, item *careplan.CareScheduleReviewItem, today careplan.Date, plans reviewPlanCache) error {
	if row.RequestKind == "pickup_change" {
		var err error
		item.Diff, err = s.pickupDiff(ctx, row, student, item)
		return err
	}
	plan, err := s.cachedPlan(ctx, plans, student, today)
	if err != nil {
		return err
	}
	var payload careplan.CareWeeklyChange
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		return err
	}
	current, err := plan.weekly(today)
	if err != nil {
		return err
	}
	item.Diff = careplan.WeeklyCareDiff(payload.Weekdays, current)
	return nil
}

func (s *ScheduleReviews) people(ctx context.Context, rows []careplan.CareScheduleChangeRequest, operation string) (map[int64]careplan.ReviewStudent, map[int64]careplan.PersonName, error) {
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

func (s *scheduleReviewCommands) ListHistory(ctx context.Context, filter careplan.RequestQueueFilter) (value0 []*careplan.CareScheduleHistoryItem, value1 *careplan.RequestCursor, err error) {
	err = s.commands.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, value1, err = s.commands.ListHistory(txCtx, filter)
		return err
	})
	return
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

func (s *ScheduleReviews) plan(ctx context.Context, student careplan.ReviewStudent, date careplan.Date, weekly bool) (reviewPlanFacts, error) {
	facts := reviewPlanFacts{}
	filter := careplan.StudentScheduleFilter{StudentIDs: []int64{student.ID}}
	var err error
	facts.pickups, err = s.deps.Schedules.ListPickupSchedules(ctx, filter)
	if err != nil {
		return facts, err
	}
	from, to := date, date
	if weekly {
		from, to = reviewWeekRange(date)
		if err := s.weeklyPlanFacts(ctx, student, filter, &facts); err != nil {
			return facts, err
		}
	}
	facts.authoritative, err = s.deps.BookingsAuthoritative(ctx)
	if err != nil {
		return facts, err
	}
	facts.bookings, err = s.planBookings(ctx, student.ID, from, to)
	return facts, err
}

// reviewWeekRange spans Monday to Friday of the date's ISO week, extended to
// the date itself when it falls on the weekend.
func reviewWeekRange(date careplan.Date) (careplan.Date, careplan.Date) {
	from := date.StartOfISOWeek()
	to := from.AddDays(4)
	if date.After(to) {
		to = date
	}
	return from, to
}

// weeklyPlanFacts loads the stored arrivals, class arrival times and
// departure modes a weekly plan needs.
func (s *ScheduleReviews) weeklyPlanFacts(ctx context.Context, student careplan.ReviewStudent, filter careplan.StudentScheduleFilter, facts *reviewPlanFacts) error {
	var err error
	facts.arrivals, err = s.deps.Schedules.ListArrivalSchedules(ctx, filter)
	if err != nil {
		return err
	}
	classes, err := s.deps.Classes.ListClassArrivalTimes(ctx, []string{student.SchoolClass})
	if err != nil {
		return err
	}
	facts.classTimes = classes[strings.ToLower(strings.TrimSpace(student.SchoolClass))]
	modes, err := s.deps.People.ListStudentDepartureModes(ctx, []int64{student.ID})
	if err != nil {
		return err
	}
	facts.departureModes = modes[student.ID]
	return nil
}

// planBookings resolves the student's approved bookings in the range to their
// offerings; bookings of an unknown offering are dropped.
func (s *ScheduleReviews) planBookings(ctx context.Context, studentID int64, from, to careplan.Date) ([]reviewPlanSelection, error) {
	bookings, err := s.deps.Bookings.ApprovedForStudents(ctx, []int64{studentID}, from, to)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(bookings))
	for _, booking := range bookings {
		if booking.StudentID == studentID {
			ids = append(ids, booking.OfferingID)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	offerings, err := s.deps.Schedules.ListCareOfferings(ctx, careplan.CareOfferingFilter{IDs: ids})
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]careplan.CareOffering, len(offerings))
	for _, row := range offerings {
		byID[row.ID] = careplan.CareOffering{ID: row.ID, IsActive: row.IsActive, CountsAsCare: row.CountsAsCare, DaysOfWeekMode: row.DaysOfWeekMode, AvailableDays: row.AvailableDays, PickupTimes: row.PickupTimes}
	}
	var selections []reviewPlanSelection
	for _, booking := range bookings {
		if booking.StudentID != studentID {
			continue
		}
		if offering, found := byID[booking.OfferingID]; found {
			selections = append(selections, reviewPlanSelection{offering: offering, selectedDays: booking.SelectedDays, from: booking.ValidFrom, until: booking.ValidUntil})
		}
	}
	return selections, nil
}

type scheduleReviewCommands struct{ commands *ScheduleReviews }
