package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// DecideHolidayAllocation decides on pending bookings of a period by
// criteria. It holds the period, then the weeks of the bookings in
// ascending ID, recomputes each week's proposal under the locks and refuses
// the whole decision when a booking is no longer pending, its proposal
// changed since the Büro read it, a deviation lacks its reason or a week
// has no free place for a booking decided into it. Every decision is
// recorded with the criteria met, in the registration's trail and as a
// decision, and the parents get one mail per registration.
func (a *HolidayAllocation) DecideHolidayAllocation(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, choices []enrollment.HolidayAllocationChoice) (enrollment.HolidayAllocationBoard, error) {
	choices, err := normalizeHolidayAllocationChoices(choices)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	if a.review.placement.mail == nil || a.review.placement.traeger == nil {
		return enrollment.HolidayAllocationBoard{}, errHolidayPlaceMailRequired
	}
	period, err := a.criteriaPeriod(ctx, reviewer, periodID, true)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	batch, err := a.lockChoices(ctx, reviewer.OrganizationID, period, choices)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	weeks := map[int64]enrollment.HolidayWeek{}
	for _, week := range period.Weeks {
		weeks[week.ID] = week
	}
	var decisions []enrollment.HolidayAllocationDecision
	for _, weekID := range batch.weeks {
		decided, err := a.decideWeek(ctx, reviewer.OrganizationID, weeks[weekID], batch, batch.byWeek[weekID])
		if err != nil {
			return enrollment.HolidayAllocationBoard{}, err
		}
		decisions = append(decisions, decided...)
	}
	if err := a.record(ctx, reviewer, period, decisions); err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	// The office's new waitlist decisions stay waiting during this command.
	// Older waitlisted bookings may move up when the last pending hold ends.
	placement := a.review.placement
	placement.excludedBookingIDs = make(map[int64]bool, len(choices))
	for _, choice := range choices {
		placement.excludedBookingIDs[choice.BookingID] = true
	}
	_, promoted, err := placement.place(ctx, reviewer.OrganizationID, period, batch.weeks, 0, reviewer.AccountID)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	board, err := a.board(ctx, reviewer.OrganizationID, period)
	board.PlacedRegistrations = holidayPlacedRegistrations(decisions, promoted)
	return board, err
}

// holidayPlacedRegistrations identifies every registration whose new place
// can change its confirmed siblings' contribution, once per command.
func holidayPlacedRegistrations(decisions []enrollment.HolidayAllocationDecision, promoted []enrollment.HolidayPromotion) []int64 {
	placed := map[int64]bool{}
	for _, decision := range decisions {
		if decision.Placement == enrollment.HolidayPlacementPlaced {
			placed[decision.RegistrationID] = true
		}
	}
	for _, promotion := range promoted {
		placed[promotion.RegistrationID] = true
	}
	return slices.Sorted(maps.Keys(placed))
}

// holidayAllocationBatch are the choices of one decision under the locks:
// the pending bookings they name, by week, the locked weeks in ascending ID
// and the criteria met of each booking.
type holidayAllocationBatch struct {
	pending map[int64]enrollment.HolidayPendingBooking
	byWeek  map[int64][]enrollment.HolidayAllocationChoice
	weeks   []int64
	met     func(enrollment.HolidayPendingBooking) []string
}

// lockChoices finds the pending booking of each choice, locks their weeks in
// ascending ID and reads the answers that count; a booking that is no longer
// pending is refused.
func (a *HolidayAllocation) lockChoices(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, choices []enrollment.HolidayAllocationChoice) (holidayAllocationBatch, error) {
	store := a.review.store
	if err := store.LockHolidayCriteriaCatalog(ctx, organizationID); err != nil {
		return holidayAllocationBatch{}, err
	}
	pending, err := store.ReadHolidayPendingBookings(ctx, organizationID, period.ID)
	if err != nil {
		return holidayAllocationBatch{}, fmt.Errorf("enrollment: read pending holiday bookings: %w", err)
	}
	batch := holidayAllocationBatch{pending: map[int64]enrollment.HolidayPendingBooking{}, byWeek: map[int64][]enrollment.HolidayAllocationChoice{}}
	for _, booking := range pending {
		batch.pending[booking.BookingID] = booking
	}
	for _, choice := range choices {
		booking, found := batch.pending[choice.BookingID]
		if !found {
			return holidayAllocationBatch{}, enrollment.ErrHolidayAllocationChanged
		}
		batch.byWeek[booking.WeekID] = append(batch.byWeek[booking.WeekID], choice)
	}
	if batch.weeks, err = store.LockHolidayWeeks(ctx, organizationID, period.ID, slices.Sorted(maps.Keys(batch.byWeek))); err != nil {
		return holidayAllocationBatch{}, fmt.Errorf("enrollment: lock holiday weeks: %w", err)
	}
	if len(batch.weeks) != len(batch.byWeek) {
		return holidayAllocationBatch{}, enrollment.ErrHolidayAllocationChanged
	}
	criteria, err := store.ReadHolidayCriteria(ctx, organizationID)
	if err != nil {
		return holidayAllocationBatch{}, fmt.Errorf("enrollment: read holiday criteria: %w", err)
	}
	answers, err := store.ReadHolidayCriterionAnswers(ctx, organizationID, period.ID, 0)
	if err != nil {
		return holidayAllocationBatch{}, fmt.Errorf("enrollment: read holiday criterion answers: %w", err)
	}
	active := activeHolidayCriteria(criteria)
	values := holidayAllocationValues(period, active, answers)
	names := []string{}
	if period.HolidayWishRule {
		names = append(names, enrollment.HolidayWishCriterionName)
	}
	for _, criterion := range active {
		names = append(names, criterion.Name)
	}
	batch.met = func(booking enrollment.HolidayPendingBooking) []string {
		return holidayCriteriaMet(names, values(booking))
	}
	return batch, nil
}

// holidayWeekProposal is the proposal of a locked week as its queue stands:
// the pending bookings in their order, a place for each one that fits, the
// days taken by the placed bookings and the days each booking takes.
type holidayWeekProposal struct {
	queue    enrollment.HolidayWeekQueue
	waiting  []enrollment.HolidayQueuedBooking
	proposal []bool
	taken    map[calendar.Date]int
	careDays []calendar.Date
}

func newHolidayWeekProposal(queue enrollment.HolidayWeekQueue) holidayWeekProposal {
	w := holidayWeekProposal{queue: queue, taken: map[calendar.Date]int{}, careDays: holidayAllocationCareDays(queue.StartsOn, queue.EndsOn)}
	var days [][]calendar.Date
	for _, booking := range queue.Bookings {
		switch {
		case booking.Placed:
			holidayTakeDays(w.taken, w.daysOf(booking))
		case booking.Pending:
			w.waiting = append(w.waiting, booking)
			days = append(days, booking.Days)
		}
	}
	w.proposal = holidayProposal(queue.Capacity, w.taken, w.careDays, days)
	return w
}

// daysOf are the days a booking takes: its days, or every care day.
func (w holidayWeekProposal) daysOf(booking enrollment.HolidayQueuedBooking) []calendar.Date {
	if len(booking.Days) > 0 {
		return booking.Days
	}
	return w.careDays
}

// take gives booking a place when each of its days has a free one.
func (w holidayWeekProposal) take(booking enrollment.HolidayQueuedBooking) bool {
	if !holidayDaysFree(w.queue.Capacity, w.taken, w.daysOf(booking)) {
		return false
	}
	holidayTakeDays(w.taken, w.daysOf(booking))
	return true
}

// decideWeek checks the choices of one locked week against its proposal and
// its free places, in the order of the proposal, and writes them.
func (a *HolidayAllocation) decideWeek(ctx context.Context, organizationID int64, week enrollment.HolidayWeek, batch holidayAllocationBatch, choices []enrollment.HolidayAllocationChoice) ([]enrollment.HolidayAllocationDecision, error) {
	queue, found, err := a.review.store.ReadHolidayWeekQueue(ctx, organizationID, week.ID)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday week queue: %w", err)
	}
	if !found {
		return nil, enrollment.ErrHolidayAllocationChanged
	}
	proposal := newHolidayWeekProposal(queue)
	chosen := map[int64]enrollment.HolidayAllocationChoice{}
	for _, choice := range choices {
		chosen[choice.BookingID] = choice
	}
	var placed, waitlisted []int64
	var decisions []enrollment.HolidayAllocationDecision
	for i, booking := range proposal.waiting {
		choice, ok := chosen[booking.ID]
		if !ok {
			continue
		}
		if choice.Proposal != holidayPlacementOf(proposal.proposal[i]) {
			return nil, enrollment.ErrHolidayAllocationChanged
		}
		switch {
		case choice.Placement == enrollment.HolidayPlacementWaitlisted:
			waitlisted = append(waitlisted, booking.ID)
		case proposal.take(booking):
			placed = append(placed, booking.ID)
		default:
			return nil, &enrollment.HolidayAllocationFullRefusal{WeekID: week.ID}
		}
		decisions = append(decisions, holidayDecisionOf(week, booking, batch, choice, i+1))
	}
	if len(decisions) != len(choices) {
		return nil, enrollment.ErrHolidayAllocationChanged
	}
	changed, err := a.review.store.DecideHolidayBookings(ctx, organizationID, week.ID, placed, waitlisted)
	if err != nil {
		return nil, err
	}
	if len(changed) != len(decisions) {
		return nil, enrollment.ErrHolidayAllocationChanged
	}
	return decisions, nil
}

// holidayDecisionOf is the record of one decision; its ID carries the
// booking until the decision is stored.
func holidayDecisionOf(week enrollment.HolidayWeek, booking enrollment.HolidayQueuedBooking, batch holidayAllocationBatch, choice enrollment.HolidayAllocationChoice, rank int) enrollment.HolidayAllocationDecision {
	read := batch.pending[booking.ID]
	return enrollment.HolidayAllocationDecision{
		ID: booking.ID, WeekID: week.ID, WeekStartsOn: week.StartsOn, WeekEndsOn: week.EndsOn,
		RegistrationID: booking.RegistrationID, ChildID: booking.ChildID,
		ChildFirstName: read.ChildFirstName, ChildLastName: read.ChildLastName,
		Placement: choice.Placement, Proposal: choice.Proposal, Rank: rank, CriteriaMet: batch.met(read), Reason: choice.Reason,
	}
}

// record stores the decisions, writes each into its registration's trail
// and tells the parents of each registration in one mail. A decision's ID
// carries its booking until it is stored.
func (a *HolidayAllocation) record(ctx context.Context, reviewer enrollment.HolidayReviewer, period enrollment.HolidayPeriod, decisions []enrollment.HolidayAllocationDecision) error {
	stored := make([]enrollment.HolidayAllocationDecision, len(decisions))
	for i, decision := range decisions {
		decision.ID = 0
		stored[i] = decision
	}
	ids, err := a.review.store.InsertHolidayAllocationDecisions(ctx, reviewer.OrganizationID, period.ID, stored, reviewer.AccountID)
	if err != nil {
		return err
	}
	byRegistration := map[int64][]int{}
	var order []int64
	for i, decision := range stored {
		if _, seen := byRegistration[decision.RegistrationID]; !seen {
			order = append(order, decision.RegistrationID)
		}
		byRegistration[decision.RegistrationID] = append(byRegistration[decision.RegistrationID], i)
		if err := a.review.audit(ctx, reviewer, period.ID, decision.RegistrationID, enrollment.HolidayAuditAllocate, enrollment.HolidayRegistrationEventDetails{
			ChildID: decision.ChildID, WeekIDs: []int64{decision.WeekID}, Placement: decision.Placement, Proposal: decision.Proposal, Reason: decision.Reason,
		}); err != nil {
			return err
		}
	}
	// The Träger and its host schools name the weeks in the mails, as in the
	// mail of a booking that moved up (#6437).
	traeger, err := a.review.placement.traeger.HolidayTraeger(ctx, reviewer.OrganizationID)
	if err != nil {
		return fmt.Errorf("enrollment: read the Träger of the holiday allocation mail: %w", err)
	}
	for _, registrationID := range order {
		var mine []enrollment.HolidayAllocationDecision
		var keys []string
		for _, i := range byRegistration[registrationID] {
			mine = append(mine, stored[i])
			keys = append(keys, strconv.FormatInt(ids[i], 10))
		}
		if err := a.announce(ctx, reviewer.OrganizationID, traeger, period, registrationID, mine, keys); err != nil {
			return err
		}
	}
	return nil
}

// announce tells the parents of a registration the places and the waiting
// list the Büro decided on, in the mail of a granted place (#4699) marked
// as an allocation.
func (a *HolidayAllocation) announce(ctx context.Context, organizationID int64, traeger enrollment.HolidayTraeger, period enrollment.HolidayPeriod, registrationID int64,
	decisions []enrollment.HolidayAllocationDecision, keys []string,
) error {
	read, _, err := a.review.store.ReadHolidayReview(ctx, organizationID, period.ID, registrationID)
	if err != nil {
		return fmt.Errorf("enrollment: read holiday registration: %w", err)
	}
	if len(read) == 0 {
		return enrollment.ErrHolidayRegistrationEntryNotFound
	}
	registration := read[0]
	lines := holidayWeekLines(traeger, period)
	children := []enrollment.HolidayRegistrationMailChild{}
	for _, child := range registration.Children {
		if mailChild := holidayAllocationMailChild(child, decisions, lines); len(mailChild.Weeks)+len(mailChild.WaitlistWeeks) > 0 {
			children = append(children, mailChild)
		}
	}
	slices.Sort(keys)
	payload, err := json.Marshal(enrollment.HolidayPlaceGrantedMail{
		Recipient: registration.GuardianEmail, GuardianName: registration.GuardianFirstName + " " + registration.GuardianLastName,
		TraegerName: traeger.Name, PeriodName: period.Name, Binding: period.Binding, Allocated: true, Children: children,
	})
	if err != nil {
		return fmt.Errorf("enrollment: encode holiday allocation mail: %w", err)
	}
	if err := a.review.placement.mail.EnqueueTraegerMail(ctx, enrollment.TraegerMail{
		OrganizationID: organizationID, Kind: enrollment.MailKindHolidayPlaceGranted,
		Recipient: registration.GuardianEmail, Payload: payload,
		RelatedType: enrollment.MailRelatedHolidayRegistration, RelatedID: registrationID,
		IdempotencyKey: fmt.Sprintf("holiday-place-allocated:%d:%s", registrationID, strings.Join(keys, ",")),
	}); err != nil {
		return fmt.Errorf("enrollment: enqueue holiday allocation mail: %w", err)
	}
	return nil
}

// holidayAllocationMailChild names the weeks of a child the Büro decided on:
// those with a place and those on the waiting list.
func holidayAllocationMailChild(child enrollment.HolidayRegistrationChild, decisions []enrollment.HolidayAllocationDecision,
	lines func(weekID, timeModelID int64, days []calendar.Date, lunch bool) string,
) enrollment.HolidayRegistrationMailChild {
	mailChild := enrollment.HolidayRegistrationMailChild{Name: child.FirstName + " " + child.LastName, Guest: child.HomeSchoolID == nil, Weeks: []string{}}
	for _, booking := range child.Bookings {
		index := slices.IndexFunc(decisions, func(decision enrollment.HolidayAllocationDecision) bool {
			return decision.ChildID == child.ID && decision.WeekID == booking.WeekID
		})
		if index < 0 {
			continue
		}
		line := lines(booking.WeekID, booking.TimeModelID, booking.Days, booking.Lunch)
		if decisions[index].Placement == enrollment.HolidayPlacementPlaced {
			mailChild.Weeks = append(mailChild.Weeks, line)
		} else {
			mailChild.WaitlistWeeks = append(mailChild.WaitlistWeeks, line)
		}
	}
	return mailChild
}

// normalizeHolidayAllocationChoices trims the reasons and checks the
// choices: one per booking, a placement and a proposal each, a reason for
// every deviation.
func normalizeHolidayAllocationChoices(choices []enrollment.HolidayAllocationChoice) ([]enrollment.HolidayAllocationChoice, error) {
	if len(choices) == 0 || len(choices) > enrollment.HolidayAllocationChoicesMax {
		return nil, validation.Errors{"decisions": fmt.Errorf("between 1 and %d decisions", enrollment.HolidayAllocationChoicesMax)}
	}
	valid := func(placement string) bool {
		return placement == enrollment.HolidayPlacementPlaced || placement == enrollment.HolidayPlacementWaitlisted
	}
	seen := map[int64]bool{}
	normalized := make([]enrollment.HolidayAllocationChoice, len(choices))
	for i, choice := range choices {
		choice.Reason = strings.TrimSpace(choice.Reason)
		if choice.BookingID <= 0 || seen[choice.BookingID] || !valid(choice.Placement) || !valid(choice.Proposal) {
			return nil, validation.Errors{fmt.Sprintf("decisions.%d", i): errors.New("needs a booking once, a placement and the proposal")}
		}
		if utf8.RuneCountInString(choice.Reason) > enrollment.HolidayAllocationReasonMaxLen {
			return nil, validation.Errors{fmt.Sprintf("decisions.%d.reason", i): fmt.Errorf("at most %d characters", enrollment.HolidayAllocationReasonMaxLen)}
		}
		if choice.Placement != choice.Proposal && choice.Reason == "" {
			return nil, &enrollment.HolidayAllocationReasonRefusal{Index: i}
		}
		if choice.Placement == choice.Proposal {
			choice.Reason = ""
		}
		seen[choice.BookingID] = true
		normalized[i] = choice
	}
	return normalized, nil
}

// CheckHolidayCriteria records the Büro's check of answers of a
// registration under the lock of its period and its row.
func (a *HolidayAllocation) CheckHolidayCriteria(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, checks []enrollment.HolidayCriterionCheck) (enrollment.HolidayRegistrationDetail, error) {
	if len(checks) == 0 || len(checks) > enrollment.HolidayCriteriaMax*(enrollment.HolidayRegistrationMaxChildren+1) {
		return enrollment.HolidayRegistrationDetail{}, validation.Errors{"checks": errors.New("between one check and one per criterion and child")}
	}
	return a.review.change(ctx, reviewer, periodID, registrationID, func(period enrollment.HolidayPeriod, current holidayRegistrationState) (string, any, error) {
		if period.AllocationMode != enrollment.HolidayAllocationCriteria {
			return "", nil, enrollment.ErrHolidayAllocationOff
		}
		if current.Status == enrollment.HolidayRegistrationCancelled {
			return "", nil, enrollment.ErrHolidayRegistrationStateConflict
		}
		for i, check := range checks {
			if err := a.check(ctx, reviewer, period.ID, registrationID, i, check); err != nil {
				return "", nil, err
			}
		}
		return enrollment.HolidayAuditCriteriaCheck, enrollment.HolidayRegistrationEventDetails{Checks: checks}, nil
	})
}

func (a *HolidayAllocation) check(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, index int, check enrollment.HolidayCriterionCheck) error {
	field := fmt.Sprintf("checks.%d", index)
	var checked *bool
	if check.Checked != nil {
		switch *check.Checked {
		case enrollment.HolidayCriterionYes:
			checked = new(bool)
			*checked = true
		case enrollment.HolidayCriterionNo:
			checked = new(bool)
		default:
			return validation.Errors{field + ".checked": errors.New("must be yes, no or null")}
		}
	}
	childID := int64(0)
	if check.ChildID != nil {
		childID = *check.ChildID
	}
	if check.CriterionID <= 0 || childID < 0 {
		return validation.Errors{field: errors.New("names no criterion of the registration")}
	}
	found, err := a.review.store.CheckHolidayCriterion(ctx, reviewer.OrganizationID, periodID, registrationID, check.CriterionID, childID, checked, reviewer.AccountID)
	if err != nil {
		return err
	}
	if !found {
		return validation.Errors{field: errors.New("names no criterion of the registration")}
	}
	return nil
}
