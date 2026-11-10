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

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// holidayAllocateDetails is the audit entry of one decision of the Büro.
type holidayAllocateDetails struct {
	ChildID   int64  `json:"child_id"`
	WeekID    int64  `json:"week_id"`
	Placement string `json:"placement"`
	Proposal  string `json:"proposal"`
	Reason    string `json:"reason,omitempty"`
}

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
	if a.review.placement.mail == nil {
		return enrollment.HolidayAllocationBoard{}, errHolidayPlaceMailRequired
	}
	period, err := a.criteriaPeriod(ctx, reviewer, periodID, true)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	organizationID := reviewer.OrganizationID
	store := a.review.store
	pending, err := store.ReadHolidayPendingBookings(ctx, organizationID, periodID)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, fmt.Errorf("enrollment: read pending holiday bookings: %w", err)
	}
	byBooking := map[int64]enrollment.HolidayPendingBooking{}
	for _, booking := range pending {
		byBooking[booking.BookingID] = booking
	}
	byWeek := map[int64][]enrollment.HolidayAllocationChoice{}
	for _, choice := range choices {
		booking, found := byBooking[choice.BookingID]
		if !found {
			return enrollment.HolidayAllocationBoard{}, enrollment.ErrHolidayAllocationChanged
		}
		byWeek[booking.WeekID] = append(byWeek[booking.WeekID], choice)
	}
	locked, err := store.LockHolidayWeeks(ctx, organizationID, periodID, slices.Sorted(maps.Keys(byWeek)))
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, fmt.Errorf("enrollment: lock holiday weeks: %w", err)
	}
	if len(locked) != len(byWeek) {
		return enrollment.HolidayAllocationBoard{}, enrollment.ErrHolidayAllocationChanged
	}
	criteria, err := store.ReadHolidayCriteria(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, fmt.Errorf("enrollment: read holiday criteria: %w", err)
	}
	answers, err := store.ReadHolidayCriterionAnswers(ctx, organizationID, periodID, 0)
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, fmt.Errorf("enrollment: read holiday criterion answers: %w", err)
	}
	active := activeHolidayCriteria(criteria)
	values := holidayCriterionValues(active, answers)
	weeks := map[int64]enrollment.HolidayWeek{}
	for _, week := range period.Weeks {
		weeks[week.ID] = week
	}
	var decisions []enrollment.HolidayAllocationDecision
	for _, weekID := range locked {
		decided, err := a.decideWeek(ctx, organizationID, weeks[weekID], byWeek[weekID], byBooking, func(booking enrollment.HolidayPendingBooking) []string {
			return holidayCriteriaMet(active, values(booking.RegistrationID, booking.ChildID))
		})
		if err != nil {
			return enrollment.HolidayAllocationBoard{}, err
		}
		decisions = append(decisions, decided...)
	}
	if err := a.record(ctx, reviewer, period, decisions); err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	return a.board(ctx, organizationID, period)
}

// decideWeek checks the choices of one locked week against its proposal
// and writes them.
func (a *HolidayAllocation) decideWeek(ctx context.Context, organizationID int64, week enrollment.HolidayWeek, choices []enrollment.HolidayAllocationChoice,
	pending map[int64]enrollment.HolidayPendingBooking, met func(enrollment.HolidayPendingBooking) []string,
) ([]enrollment.HolidayAllocationDecision, error) {
	queue, found, err := a.review.store.ReadHolidayWeekQueue(ctx, organizationID, week.ID)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday week queue: %w", err)
	}
	if !found {
		return nil, enrollment.ErrHolidayAllocationChanged
	}
	careDays := holidayAllocationCareDays(queue.StartsOn, queue.EndsOn)
	daysOf := func(booking enrollment.HolidayQueuedBooking) []calendar.Date {
		if len(booking.Days) > 0 {
			return booking.Days
		}
		return careDays
	}
	taken := map[calendar.Date]int{}
	var waiting []enrollment.HolidayQueuedBooking
	for _, booking := range queue.Bookings {
		switch {
		case booking.Placed:
			holidayTakeDays(taken, daysOf(booking))
		case booking.Pending:
			waiting = append(waiting, booking)
		}
	}
	days := make([][]calendar.Date, len(waiting))
	for i, booking := range waiting {
		days[i] = booking.Days
	}
	proposal := holidayProposal(queue.Capacity, taken, careDays, days)
	chosen := map[int64]enrollment.HolidayAllocationChoice{}
	for _, choice := range choices {
		chosen[choice.BookingID] = choice
	}
	var placed, waitlisted []int64
	var decisions []enrollment.HolidayAllocationDecision
	for i, booking := range waiting {
		choice, ok := chosen[booking.ID]
		if !ok {
			continue
		}
		if choice.Proposal != holidayPlacementOf(proposal[i]) {
			return nil, enrollment.ErrHolidayAllocationChanged
		}
		if choice.Placement == enrollment.HolidayPlacementPlaced {
			if !holidayDaysFree(queue.Capacity, taken, daysOf(booking)) {
				return nil, &enrollment.HolidayAllocationFullRefusal{WeekID: week.ID}
			}
			holidayTakeDays(taken, daysOf(booking))
			placed = append(placed, booking.ID)
		} else {
			waitlisted = append(waitlisted, booking.ID)
		}
		read := pending[booking.ID]
		decisions = append(decisions, enrollment.HolidayAllocationDecision{
			WeekID: week.ID, WeekStartsOn: week.StartsOn, WeekEndsOn: week.EndsOn,
			RegistrationID: booking.RegistrationID, ChildID: booking.ChildID,
			ChildFirstName: read.ChildFirstName, ChildLastName: read.ChildLastName,
			Placement: choice.Placement, Proposal: choice.Proposal, Rank: i + 1, CriteriaMet: met(read), Reason: choice.Reason,
			ID: booking.ID,
		})
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

// record stores the decisions, writes each into its registration's trail
// and tells the parents of each registration in one mail. A decision's ID
// carries its booking until it is stored.
func (a *HolidayAllocation) record(ctx context.Context, reviewer enrollment.HolidayReviewer, period enrollment.HolidayPeriod, decisions []enrollment.HolidayAllocationDecision) error {
	bookings := make([]int64, len(decisions))
	stored := make([]enrollment.HolidayAllocationDecision, len(decisions))
	for i, decision := range decisions {
		bookings[i] = decision.ID
		decision.ID = 0
		stored[i] = decision
	}
	if err := a.review.store.InsertHolidayAllocationDecisions(ctx, reviewer.OrganizationID, period.ID, stored, reviewer.AccountID); err != nil {
		return err
	}
	byRegistration := map[int64][]int{}
	var order []int64
	for i, decision := range stored {
		if _, seen := byRegistration[decision.RegistrationID]; !seen {
			order = append(order, decision.RegistrationID)
		}
		byRegistration[decision.RegistrationID] = append(byRegistration[decision.RegistrationID], i)
		if err := a.review.audit(ctx, reviewer, period.ID, decision.RegistrationID, enrollment.HolidayAuditAllocate, holidayAllocateDetails{
			ChildID: decision.ChildID, WeekID: decision.WeekID, Placement: decision.Placement, Proposal: decision.Proposal, Reason: decision.Reason,
		}); err != nil {
			return err
		}
	}
	for _, registrationID := range order {
		var mine []enrollment.HolidayAllocationDecision
		var keys []string
		for _, i := range byRegistration[registrationID] {
			mine = append(mine, stored[i])
			keys = append(keys, strconv.FormatInt(bookings[i], 10)+":"+stored[i].Placement)
		}
		if err := a.announce(ctx, reviewer.OrganizationID, period, registrationID, mine, keys); err != nil {
			return err
		}
	}
	return nil
}

// announce tells the parents of a registration the places and the waiting
// list the Büro decided on, in the mail of a granted place (#4699) marked
// as an allocation.
func (a *HolidayAllocation) announce(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, registrationID int64,
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
	lines := holidayWeekLines(enrollment.HolidayTraeger{}, period)
	children := []enrollment.HolidayRegistrationMailChild{}
	for _, child := range registration.Children {
		mailChild := enrollment.HolidayRegistrationMailChild{Name: child.FirstName + " " + child.LastName, Guest: child.HomeSchoolID == nil, Weeks: []string{}}
		for _, booking := range child.Bookings {
			index := slices.IndexFunc(decisions, func(decision enrollment.HolidayAllocationDecision) bool {
				return decision.ChildID == child.ID && decision.WeekID == booking.WeekID
			})
			if index < 0 {
				continue
			}
			line := lines(booking.WeekID, booking.TimeModelID, booking.Days)
			if decisions[index].Placement == enrollment.HolidayPlacementPlaced {
				mailChild.Weeks = append(mailChild.Weeks, line)
			} else {
				mailChild.WaitlistWeeks = append(mailChild.WaitlistWeeks, line)
			}
		}
		if len(mailChild.Weeks)+len(mailChild.WaitlistWeeks) > 0 {
			children = append(children, mailChild)
		}
	}
	slices.Sort(keys)
	payload, err := json.Marshal(enrollment.HolidayPlaceGrantedMail{
		Recipient: registration.GuardianEmail, GuardianName: registration.GuardianFirstName + " " + registration.GuardianLastName,
		PeriodName: period.Name, Binding: period.Binding, Allocated: true, Children: children,
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
		return enrollment.HolidayAuditCriteriaCheck, map[string]any{"checks": checks}, nil
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
