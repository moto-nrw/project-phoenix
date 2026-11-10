package application

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Allocation of holiday places by criteria (#4700); the rules are on
// enrollment.HolidayAllocation. A decision locks the period, which every
// submission, change of a registration and change of a capacity locks
// first, then the weeks of the decided bookings in ascending ID, the order
// of #4699. The proposal is computed under these locks from the same order
// the list shows; a decision against a proposal the Büro did not see is
// refused.

// HolidayAllocationStore persists the criteria, the answers and the
// decisions of the allocation.
type HolidayAllocationStore interface {
	ReadHolidayCriteria(ctx context.Context, organizationID int64) ([]enrollment.HolidayCriterion, error)
	// WriteHolidayCriteria makes criteria the criteria of the Träger in
	// their order; one left out is archived.
	WriteHolidayCriteria(ctx context.Context, organizationID int64, criteria []enrollment.HolidayCriterionInput, actorAccountID int64) error
	// ReadHolidayCriterionAnswers reads the answers of the registrations of
	// a period, or of the one registrationID names.
	ReadHolidayCriterionAnswers(ctx context.Context, organizationID, periodID, registrationID int64) ([]enrollment.HolidayCriterionAnswer, error)
	// CheckHolidayCriterion writes or, with nil, takes back the Büro's check
	// of one answer; found is false outside the registration.
	CheckHolidayCriterion(ctx context.Context, organizationID, periodID, registrationID, criterionID, childID int64, checked *bool, actorAccountID int64) (bool, error)
	// ReadHolidayPendingBookings reads the pending bookings of the period
	// by week in the order of the proposal.
	ReadHolidayPendingBookings(ctx context.Context, organizationID, periodID int64) ([]enrollment.HolidayPendingBooking, error)
	// DecideHolidayBookings decides on pending bookings of a locked week and
	// returns those it changed.
	DecideHolidayBookings(ctx context.Context, organizationID, weekID int64, placed, waitlisted []int64) ([]int64, error)
	InsertHolidayAllocationDecisions(ctx context.Context, organizationID, periodID int64, decisions []enrollment.HolidayAllocationDecision, actorAccountID int64) error
	ReadHolidayAllocationDecisions(ctx context.Context, organizationID, periodID int64) ([]enrollment.HolidayAllocationDecision, error)
}

// HolidayAllocation serves enrollment.HolidayAllocation over the review it
// shares its store, its locks and its mail with.
type HolidayAllocation struct {
	review *HolidayRegistrationReview
}

var _ enrollment.HolidayAllocation = (*HolidayAllocation)(nil)

// NewHolidayAllocation builds the allocation over review; without the mail
// outbox of review no decision is made, so no parent misses one.
func NewHolidayAllocation(review *HolidayRegistrationReview) *HolidayAllocation {
	if review == nil {
		panic("enrollment: the holiday registration review is required")
	}
	return &HolidayAllocation{review: review}
}

// HolidayCriteria lists the criteria of the Träger by priority with the
// periods that allocate by them.
func (a *HolidayAllocation) HolidayCriteria(ctx context.Context, reviewer enrollment.HolidayReviewer) (enrollment.HolidayCriteriaCatalog, error) {
	if err := holidayReviewKey(reviewer, 1, 1); err != nil {
		return enrollment.HolidayCriteriaCatalog{}, err
	}
	criteria, err := a.review.store.ReadHolidayCriteria(ctx, reviewer.OrganizationID)
	if err != nil {
		return enrollment.HolidayCriteriaCatalog{}, fmt.Errorf("enrollment: read holiday criteria: %w", err)
	}
	periods, err := a.review.periods.ListHolidayPeriods(ctx, reviewer.OrganizationID)
	if err != nil {
		return enrollment.HolidayCriteriaCatalog{}, fmt.Errorf("enrollment: list holiday periods: %w", err)
	}
	return holidayCriteriaCatalog(criteria, periods), nil
}

// SaveHolidayCriteria checks and writes the criteria of the Träger in their
// order and returns them as stored.
func (a *HolidayAllocation) SaveHolidayCriteria(ctx context.Context, reviewer enrollment.HolidayReviewer, input []enrollment.HolidayCriterionInput) (enrollment.HolidayCriteriaCatalog, error) {
	if err := holidayReviewKey(reviewer, 1, 1); err != nil {
		return enrollment.HolidayCriteriaCatalog{}, err
	}
	input, err := normalizeHolidayCriteria(input)
	if err != nil {
		return enrollment.HolidayCriteriaCatalog{}, err
	}
	if err := a.review.store.WriteHolidayCriteria(ctx, reviewer.OrganizationID, input, reviewer.AccountID); err != nil {
		return enrollment.HolidayCriteriaCatalog{}, err
	}
	criteria, err := a.review.store.ReadHolidayCriteria(ctx, reviewer.OrganizationID)
	if err != nil {
		return enrollment.HolidayCriteriaCatalog{}, fmt.Errorf("enrollment: read holiday criteria: %w", err)
	}
	periods, err := a.review.periods.ListHolidayPeriods(ctx, reviewer.OrganizationID)
	if err != nil {
		return enrollment.HolidayCriteriaCatalog{}, fmt.Errorf("enrollment: list holiday periods: %w", err)
	}
	return holidayCriteriaCatalog(criteria, periods), nil
}

// holidayCriteriaCatalog names the criteria with the periods by criteria.
func holidayCriteriaCatalog(criteria []enrollment.HolidayCriterion, periods []enrollment.HolidayPeriod) enrollment.HolidayCriteriaCatalog {
	catalog := enrollment.HolidayCriteriaCatalog{Criteria: criteria, Periods: []enrollment.HolidayCriterionPeriod{}}
	for _, period := range periods {
		if period.AllocationMode == enrollment.HolidayAllocationCriteria {
			catalog.Periods = append(catalog.Periods, enrollment.HolidayCriterionPeriod{ID: period.ID, Name: period.Name})
		}
	}
	return catalog
}

// normalizeHolidayCriteria trims the texts and checks the list: at most
// HolidayCriteriaMax criteria with a name, a question and a scope, each ID
// once.
func normalizeHolidayCriteria(criteria []enrollment.HolidayCriterionInput) ([]enrollment.HolidayCriterionInput, error) {
	if len(criteria) > enrollment.HolidayCriteriaMax {
		return nil, validation.Errors{"criteria": fmt.Errorf("at most %d criteria", enrollment.HolidayCriteriaMax)}
	}
	errs := validation.Errors{}
	seen := map[int64]bool{}
	normalized := make([]enrollment.HolidayCriterionInput, len(criteria))
	for i, criterion := range criteria {
		criterion.Name = strings.TrimSpace(criterion.Name)
		criterion.Question = strings.TrimSpace(criterion.Question)
		validateHolidayCriterion(errs, i, criterion, seen)
		seen[criterion.ID] = true
		normalized[i] = criterion
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return normalized, nil
}

// validateHolidayCriterion checks one criterion of the list at index.
func validateHolidayCriterion(errs validation.Errors, index int, criterion enrollment.HolidayCriterionInput, seen map[int64]bool) {
	field := func(name string) string { return fmt.Sprintf("criteria.%d.%s", index, name) }
	if criterion.Name == "" || utf8.RuneCountInString(criterion.Name) > enrollment.HolidayCriterionNameMaxLength {
		errs[field("name")] = fmt.Errorf("a name of 1 to %d characters is required", enrollment.HolidayCriterionNameMaxLength)
	}
	if criterion.Question == "" || utf8.RuneCountInString(criterion.Question) > enrollment.HolidayCriterionQuestionMaxLen {
		errs[field("question")] = fmt.Errorf("a question of 1 to %d characters is required", enrollment.HolidayCriterionQuestionMaxLen)
	}
	if criterion.Scope != enrollment.HolidayCriterionFamily && criterion.Scope != enrollment.HolidayCriterionChild {
		errs[field("scope")] = errors.New("must be family or child")
	}
	if criterion.ID < 0 || (criterion.ID > 0 && seen[criterion.ID]) {
		errs[field("id")] = errors.New("names a criterion twice")
	}
}

// HolidayAllocationBoard reads the allocation list of a period by criteria
// and records the read in the period's trail.
func (a *HolidayAllocation) HolidayAllocationBoard(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64) (enrollment.HolidayAllocationBoard, error) {
	period, err := a.criteriaPeriod(ctx, reviewer, periodID, false)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	board, err := a.board(ctx, reviewer.OrganizationID, period)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	pending := 0
	for _, week := range board.Weeks {
		pending += len(week.Pending)
	}
	if err := a.review.store.AppendHolidayAudit(ctx, reviewer.OrganizationID, periodID, 0, enrollment.HolidayAuditAllocation,
		map[string]any{"weeks": len(board.Weeks), "pending": pending, "decisions": len(board.Decisions)}, reviewer.AccountID); err != nil {
		return enrollment.HolidayAllocationBoard{}, fmt.Errorf("enrollment: record holiday allocation read: %w", err)
	}
	return board, nil
}

// criteriaPeriod reads, or with lock holds, a period of the Träger that
// gives its places by criteria.
func (a *HolidayAllocation) criteriaPeriod(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, lock bool) (enrollment.HolidayPeriod, error) {
	if err := holidayReviewKey(reviewer, periodID, 1); err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	period, err := foundHolidayPeriod(a.review.periods.FindHolidayPeriod(ctx, reviewer.OrganizationID, periodID, lock))
	if err != nil {
		return enrollment.HolidayPeriod{}, err
	}
	if period.AllocationMode != enrollment.HolidayAllocationCriteria {
		return enrollment.HolidayPeriod{}, enrollment.ErrHolidayAllocationOff
	}
	return period, nil
}

// board reads the allocation list: five statements for any number of
// weeks and bookings.
func (a *HolidayAllocation) board(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod) (enrollment.HolidayAllocationBoard, error) {
	store := a.review.store
	criteria, err := store.ReadHolidayCriteria(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, fmt.Errorf("enrollment: read holiday criteria: %w", err)
	}
	places, err := store.ReadHolidayWeekPlaces(ctx, organizationID, period.ID)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, fmt.Errorf("enrollment: read holiday week places: %w", err)
	}
	pending, err := store.ReadHolidayPendingBookings(ctx, organizationID, period.ID)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, fmt.Errorf("enrollment: read pending holiday bookings: %w", err)
	}
	answers, err := store.ReadHolidayCriterionAnswers(ctx, organizationID, period.ID, 0)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, fmt.Errorf("enrollment: read holiday criterion answers: %w", err)
	}
	decisions, err := store.ReadHolidayAllocationDecisions(ctx, organizationID, period.ID)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, fmt.Errorf("enrollment: read holiday allocation decisions: %w", err)
	}
	active := activeHolidayCriteria(criteria)
	values := holidayAllocationValues(period, active, answers)
	board := enrollment.HolidayAllocationBoard{
		AllocationMode: period.AllocationMode, Criteria: []enrollment.HolidayCriterionRef{},
		Weeks: make([]enrollment.HolidayAllocationWeek, len(places)), Decisions: decisions,
	}
	if period.HolidayWishRule {
		board.Criteria = append(board.Criteria, enrollment.HolidayCriterionRef{
			ID: enrollment.HolidayWishCriterionID, Name: enrollment.HolidayWishCriterionName, Scope: enrollment.HolidayCriterionChild, Position: 1,
		})
	}
	for _, criterion := range active {
		board.Criteria = append(board.Criteria, enrollment.HolidayCriterionRef{ID: criterion.ID, Name: criterion.Name, Scope: criterion.Scope, Position: len(board.Criteria) + 1})
	}
	for i, week := range places {
		board.Weeks[i] = holidayAllocationWeek(week, pending, values)
	}
	return board, nil
}

// holidayAllocationWeek is a week of the list with its pending bookings in
// the order of the proposal and the proposal of each.
func holidayAllocationWeek(week enrollment.HolidayWeekPlaces, pending []enrollment.HolidayPendingBooking, values func(enrollment.HolidayPendingBooking) []enrollment.HolidayCriterionValue) enrollment.HolidayAllocationWeek {
	careDays := holidayAllocationCareDays(week.StartsOn, week.EndsOn)
	taken := map[calendar.Date]int{}
	if week.BookingUnit == enrollment.HolidayBookingUnitDay {
		for _, day := range week.Days {
			taken[day.Date] = day.Placed
		}
	} else {
		for _, day := range careDays {
			taken[day] = week.Placed
		}
	}
	allocation := enrollment.HolidayAllocationWeek{
		WeekID: week.WeekID, StartsOn: week.StartsOn, EndsOn: week.EndsOn, HostSchoolID: week.HostSchoolID,
		BookingUnit: week.BookingUnit, Capacity: week.Capacity, Placed: week.Placed,
		Free: holidayFreePlaces(week.Capacity, taken, careDays), Pending: []enrollment.HolidayAllocationCandidate{},
	}
	var days [][]calendar.Date
	for _, booking := range pending {
		if booking.WeekID != week.WeekID {
			continue
		}
		days = append(days, booking.Days)
		allocation.Pending = append(allocation.Pending, enrollment.HolidayAllocationCandidate{
			BookingID: booking.BookingID, Rank: booking.Rank, RegistrationID: booking.RegistrationID, SubmittedAt: booking.SubmittedAt,
			ChildID: booking.ChildID, ChildFirstName: booking.ChildFirstName, ChildLastName: booking.ChildLastName,
			TimeModelName: booking.TimeModelName, Days: booking.Days, Criteria: values(booking),
		})
	}
	for i, placed := range holidayProposal(week.Capacity, taken, careDays, days) {
		allocation.Pending[i].Proposal = holidayPlacementOf(placed)
	}
	return allocation
}

// holidayAllocationCareDays are the days a whole-week booking takes, as in
// holidayFirstFit: Monday to Friday, or the first day of a week that covers
// only a weekend.
func holidayAllocationCareDays(startsOn, endsOn calendar.Date) []calendar.Date {
	if days := enrollment.HolidayWeekCareDays(startsOn, endsOn); len(days) > 0 {
		return days
	}
	return []calendar.Date{startsOn}
}

// holidayProposal walks pending bookings in their order and proposes a
// place for each one while every one of its days, all care days for a
// whole week, has a free place after taken and those proposed before it:
// the first fit of #4701 over the pending bookings.
func holidayProposal(capacity *int, taken map[calendar.Date]int, careDays []calendar.Date, pending [][]calendar.Date) []bool {
	taken = maps.Clone(taken)
	if taken == nil {
		taken = map[calendar.Date]int{}
	}
	proposal := make([]bool, len(pending))
	for i, days := range pending {
		if len(days) == 0 {
			days = careDays
		}
		if holidayDaysFree(capacity, taken, days) {
			holidayTakeDays(taken, days)
			proposal[i] = true
		}
	}
	return proposal
}

// holidayFreePlaces is the number of free places of the fullest care day,
// nil without a capacity.
func holidayFreePlaces(capacity *int, taken map[calendar.Date]int, careDays []calendar.Date) *int {
	if capacity == nil {
		return nil
	}
	free := *capacity
	for _, day := range careDays {
		free = min(free, *capacity-taken[day])
	}
	free = max(free, 0)
	return &free
}

func holidayPlacementOf(placed bool) string {
	if placed {
		return enrollment.HolidayPlacementPlaced
	}
	return enrollment.HolidayPlacementWaitlisted
}

// activeHolidayCriteria are the active criteria in their priority.
func activeHolidayCriteria(criteria []enrollment.HolidayCriterion) []enrollment.HolidayCriterion {
	return slices.DeleteFunc(slices.Clone(criteria), func(criterion enrollment.HolidayCriterion) bool { return !criterion.Active })
}

// holidayCriterionLookup names the values of the active criteria for the
// booking of a child of a registration, in their priority.
type holidayCriterionLookup func(registrationID, childID int64) []enrollment.HolidayCriterionValue

// holidayCriterionValues looks the answers up: a family criterion by the
// registration, a child criterion by the child. The Büro's check counts
// before the parents' statement; without either the value is unknown.
func holidayCriterionValues(active []enrollment.HolidayCriterion, answers []enrollment.HolidayCriterionAnswer) holidayCriterionLookup {
	type key struct{ registration, child, criterion int64 }
	byKey := map[key]enrollment.HolidayCriterionAnswer{}
	for _, answer := range answers {
		child := int64(0)
		if answer.ChildID != nil {
			child = *answer.ChildID
		}
		byKey[key{answer.RegistrationID, child, answer.CriterionID}] = answer
	}
	return func(registrationID, childID int64) []enrollment.HolidayCriterionValue {
		values := make([]enrollment.HolidayCriterionValue, len(active))
		for i, criterion := range active {
			child := int64(0)
			if criterion.Scope == enrollment.HolidayCriterionChild {
				child = childID
			}
			answer, found := byKey[key{registrationID, child, criterion.ID}]
			values[i] = enrollment.HolidayCriterionValue{CriterionID: criterion.ID, Value: enrollment.HolidayCriterionUnknown, Source: enrollment.HolidayCriterionByParent}
			if !found {
				continue
			}
			value, source := holidayAnswerValue(answer)
			values[i].Value, values[i].Source = value, source
		}
		return values
	}
}

// holidayAnswerValue is the value that counts of a stored answer and its
// source.
func holidayAnswerValue(answer enrollment.HolidayCriterionAnswer) (string, string) {
	if answer.Checked != nil {
		return holidayYesNo(*answer.Checked), enrollment.HolidayCriterionByOffice
	}
	if answer.Stated != nil {
		return holidayYesNo(*answer.Stated), enrollment.HolidayCriterionByParent
	}
	return enrollment.HolidayCriterionUnknown, enrollment.HolidayCriterionByParent
}

func holidayYesNo(value bool) string {
	if value {
		return enrollment.HolidayCriterionYes
	}
	return enrollment.HolidayCriterionNo
}

// holidayAllocationValues names the values of a pending booking in the
// order of the list: the holiday care wish first when the period takes it
// into account (#4703), then the active criteria of the Träger. A wish of
// the office counts as checked; an unknown wish is unknown, as in the
// order of the waiting list, where only a known "no" waits behind.
func holidayAllocationValues(period enrollment.HolidayPeriod, active []enrollment.HolidayCriterion, answers []enrollment.HolidayCriterionAnswer) func(enrollment.HolidayPendingBooking) []enrollment.HolidayCriterionValue {
	values := holidayCriterionValues(active, answers)
	return func(booking enrollment.HolidayPendingBooking) []enrollment.HolidayCriterionValue {
		criteria := values(booking.RegistrationID, booking.ChildID)
		if !period.HolidayWishRule {
			return criteria
		}
		wish := enrollment.HolidayCriterionValue{CriterionID: enrollment.HolidayWishCriterionID, Value: enrollment.HolidayCriterionUnknown, Source: enrollment.HolidayCriterionByParent}
		if booking.HolidayWish == enrollment.HolidayWishYes || booking.HolidayWish == enrollment.HolidayWishNo {
			wish.Value = booking.HolidayWish
		}
		if booking.HolidayWishSource == enrollment.HolidayWishSourceOffice {
			wish.Source = enrollment.HolidayCriterionByOffice
		}
		return append([]enrollment.HolidayCriterionValue{wish}, criteria...)
	}
}

// holidayCriteriaMet names the criteria a booking meets, of names in the
// order of values.
func holidayCriteriaMet(names []string, values []enrollment.HolidayCriterionValue) []string {
	met := []string{}
	for i, value := range values {
		if value.Value == enrollment.HolidayCriterionYes {
			met = append(met, names[i])
		}
	}
	return met
}
