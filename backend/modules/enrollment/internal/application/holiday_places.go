package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Places of the holiday weeks (#4699, decision E-e); the rules are on
// enrollment.HolidayPlaces. The lock order is: the registration a
// submission inserts, or the period and the registration the Träger-Büro
// locks, then the weeks in ascending ID. Only a holder of the week's lock
// gives a booking a place.

// HolidayPlaceStore persists the places of the weeks.
type HolidayPlaceStore interface {
	// LockHolidayWeeks holds the weeks of the period in ascending ID until
	// the transaction ends and returns those it locked.
	LockHolidayWeeks(ctx context.Context, organizationID, periodID int64, weekIDs []int64) ([]int64, error)
	// ReadHolidayWeekQueue reads a locked week with the bookings that take
	// or wait for its places, in the order of the waiting list; found is
	// false for a week of another Träger.
	ReadHolidayWeekQueue(ctx context.Context, organizationID, weekID int64) (enrollment.HolidayWeekQueue, bool, error)
	// PlaceHolidayBookings gives the named waiting bookings of a locked week
	// their place and returns them; those of initialIDs are initial
	// placements, every other one moved up.
	PlaceHolidayBookings(ctx context.Context, organizationID, weekID int64, bookingIDs, initialIDs []int64) ([]enrollment.HolidayPromotion, error)
	SetHolidayWeekCapacity(ctx context.Context, organizationID, periodID, weekID int64, capacity *int) error
	ReadHolidayWeekPlaces(ctx context.Context, organizationID, periodID int64) ([]enrollment.HolidayWeekPlaces, error)
	ReadHolidayReview(ctx context.Context, organizationID, periodID, registrationID int64) ([]enrollment.HolidayRegistrationDetail, map[int64]string, error)
	AppendHolidayAudit(ctx context.Context, organizationID, periodID, registrationID int64, action string, details any, actorAccountID int64) error
	// ReadHolidayChildBookings reads, in one statement, the bookings the
	// children keys names hold in the period over all their registrations
	// that count, except those of the child row exceptChildID, by child
	// (#6446). The caller holds the period.
	ReadHolidayChildBookings(ctx context.Context, organizationID, periodID int64, keys []enrollment.HolidayChildKey, exceptChildID int64) (map[enrollment.HolidayChildKey][]enrollment.HolidayBookingInput, error)
}

// errHolidayPlaceMailRequired reports a move up the waiting list without the
// mail outbox to tell the parents or the Träger directory that names the
// Träger and the host schools in that mail.
var errHolidayPlaceMailRequired = errors.New("enrollment: moving up the holiday waiting list needs the mail outbox and the Träger directory")

// holidayPlacement gives places and moves bookings up. mail tells the
// parents of a booking that moved up; without it a move up fails, so no
// child moves up unnoticed. traeger names the Träger and the host schools in
// that mail, in the caller's transaction: the public submission reads them
// with its link, the Träger-Büro in its Träger transaction (#6437). Without
// it a move up fails too, so no mail leaves the parents guessing who sent it.
type holidayPlacement struct {
	store   HolidayPlaceStore
	traeger enrollment.HolidayTraegerDirectory
	mail    enrollment.TraegerMailOutbox
	// initialChildID restricts an initial placement to the rebooked child.
	// Zero places every child of a newly submitted registration initially.
	initialChildID int64
	// excludedBookingIDs are decisions made in the current allocation. They
	// may move up after a later capacity change, cancellation or rebooking.
	excludedBookingIDs map[int64]bool
}

// holidayPromotionDetails is the audit entry of a booking that moved up.
type holidayPromotionDetails struct {
	ChildID int64   `json:"child_id"`
	WeekIDs []int64 `json:"week_ids"`
}

// place locks the weeks in ascending ID and gives their free places to their
// waiting lists. The initial bookings of submittedID, restricted to
// initialChildID for a direct rebooking, come back as own; every other booking that got a place moved up: it is
// recorded in its registration's trail and its parents get a mail, in this
// transaction.
func (p holidayPlacement) place(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, weekIDs []int64, submittedID, actorAccountID int64) (own, promoted []enrollment.HolidayPromotion, err error) {
	weeks := slices.Compact(slices.Sorted(slices.Values(weekIDs)))
	if len(weeks) == 0 {
		return nil, nil, nil
	}
	locked, err := p.store.LockHolidayWeeks(ctx, organizationID, period.ID, weeks)
	if err != nil {
		return nil, nil, fmt.Errorf("enrollment: lock holiday weeks: %w", err)
	}
	for _, weekID := range locked {
		placed, err := p.placeWeek(ctx, organizationID, weekID, submittedID)
		if err != nil {
			return nil, nil, fmt.Errorf("enrollment: place holiday waiting list: %w", err)
		}
		for _, booking := range placed {
			if booking.RegistrationID == submittedID && (p.initialChildID == 0 || booking.ChildID == p.initialChildID) {
				own = append(own, booking)
			} else {
				promoted = append(promoted, booking)
			}
		}
	}
	if err := p.announce(ctx, organizationID, period, promoted, actorAccountID); err != nil {
		return nil, nil, err
	}
	return own, promoted, nil
}

// placeWeek gives the free places of a locked week to its waiting list
// (#4699): those of submittedID, restricted to initialChildID when nonzero,
// are initial placements. A week with a booking that waits for the
// decision of the Büro gives no place (#4700): its free places are the
// Büro's to give, in the order of the criteria.
func (p holidayPlacement) placeWeek(ctx context.Context, organizationID, weekID, submittedID int64) ([]enrollment.HolidayPromotion, error) {
	queue, found, err := p.store.ReadHolidayWeekQueue(ctx, organizationID, weekID)
	if err != nil || !found {
		return nil, err
	}
	if slices.ContainsFunc(queue.Bookings, func(booking enrollment.HolidayQueuedBooking) bool { return booking.Pending }) {
		return nil, nil
	}
	queue.Bookings = slices.DeleteFunc(slices.Clone(queue.Bookings), func(booking enrollment.HolidayQueuedBooking) bool {
		return !booking.Placed && p.excludedBookingIDs[booking.ID]
	})
	var place, initial []int64
	for _, booking := range holidayFirstFit(queue) {
		place = append(place, booking.ID)
		if booking.RegistrationID == submittedID && (p.initialChildID == 0 || booking.ChildID == p.initialChildID) {
			initial = append(initial, booking.ID)
		}
	}
	return p.store.PlaceHolidayBookings(ctx, organizationID, weekID, place, initial)
}

// holidayFirstFit chooses the waiting bookings of a week that get a place,
// in the order of the waiting list (#4699, #4701). The capacity holds for
// each care day: a booking gets its place only when every one of its days,
// all care days of the week for a whole week, has a free place after the
// placed bookings and those chosen before it; otherwise it keeps waiting and
// a later booking of other days may pass it. Without a capacity every
// booking gets its place. For a week booked as a whole every booking needs
// every day, so this is the first free places in order, as before.
func holidayFirstFit(queue enrollment.HolidayWeekQueue) []enrollment.HolidayQueuedBooking {
	careDays := enrollment.HolidayWeekCareDays(queue.StartsOn, queue.EndsOn)
	if len(careDays) == 0 {
		// Whole-week offers can cover only a weekend. Count each booking
		// against one shared date so their capacity still holds per child.
		careDays = []calendar.Date{queue.StartsOn}
	}
	daysOf := func(booking enrollment.HolidayQueuedBooking) []calendar.Date {
		if len(booking.Days) > 0 {
			return booking.Days
		}
		return careDays
	}
	taken := map[calendar.Date]int{}
	for _, booking := range queue.Bookings {
		if booking.Placed {
			holidayTakeDays(taken, daysOf(booking))
		}
	}
	var chosen []enrollment.HolidayQueuedBooking
	for _, booking := range queue.Bookings {
		if booking.Placed || booking.Pending || !holidayDaysFree(queue.Capacity, taken, daysOf(booking)) {
			continue
		}
		holidayTakeDays(taken, daysOf(booking))
		chosen = append(chosen, booking)
	}
	return chosen
}

// holidayTakeDays counts one more place on each of days.
func holidayTakeDays(taken map[calendar.Date]int, days []calendar.Date) {
	for _, day := range days {
		taken[day]++
	}
}

// holidayDaysFree reports whether each of days has a free place; without a
// capacity every day has one.
func holidayDaysFree(capacity *int, taken map[calendar.Date]int, days []calendar.Date) bool {
	if capacity == nil {
		return true
	}
	return *capacity > 0 && !slices.ContainsFunc(days, func(day calendar.Date) bool { return taken[day] >= *capacity })
}

// announce records each registration's moves up in its trail and enqueues
// one mail to its parents.
func (p holidayPlacement) announce(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, promoted []enrollment.HolidayPromotion, actorAccountID int64) error {
	if len(promoted) == 0 {
		return nil
	}
	if p.mail == nil || p.traeger == nil {
		return errHolidayPlaceMailRequired
	}
	traeger, err := p.traeger.HolidayTraeger(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("enrollment: read the Träger of the holiday place mail: %w", err)
	}
	byRegistration := map[int64][]enrollment.HolidayPromotion{}
	var order []int64
	for _, booking := range promoted {
		if _, seen := byRegistration[booking.RegistrationID]; !seen {
			order = append(order, booking.RegistrationID)
		}
		byRegistration[booking.RegistrationID] = append(byRegistration[booking.RegistrationID], booking)
	}
	for _, registrationID := range order {
		if err := p.announceRegistration(ctx, organizationID, traeger, period, registrationID, byRegistration[registrationID], actorAccountID); err != nil {
			return err
		}
	}
	return nil
}

func (p holidayPlacement) announceRegistration(ctx context.Context, organizationID int64, traeger enrollment.HolidayTraeger, period enrollment.HolidayPeriod,
	registrationID int64, promoted []enrollment.HolidayPromotion, actorAccountID int64,
) error {
	read, _, err := p.store.ReadHolidayReview(ctx, organizationID, period.ID, registrationID)
	if err != nil {
		return fmt.Errorf("enrollment: read holiday registration: %w", err)
	}
	if len(read) == 0 {
		return enrollment.ErrHolidayRegistrationEntryNotFound
	}
	registration := read[0]
	weeks := map[int64][]int64{}
	keys := make([]string, 0, len(promoted))
	for _, booking := range promoted {
		keys = append(keys, strconv.FormatInt(booking.BookingID, 10))
		weeks[booking.ChildID] = append(weeks[booking.ChildID], booking.WeekID)
	}
	lines := holidayWeekLines(traeger, period)
	children := []enrollment.HolidayRegistrationMailChild{}
	for _, child := range registration.Children {
		moved := weeks[child.ID]
		if len(moved) == 0 {
			continue
		}
		details := holidayPromotionDetails{ChildID: child.ID, WeekIDs: moved}
		if err := p.store.AppendHolidayAudit(ctx, organizationID, period.ID, registrationID, enrollment.HolidayAuditPromote, details, actorAccountID); err != nil {
			return fmt.Errorf("enrollment: record holiday promotion: %w", err)
		}
		mailChild := enrollment.HolidayRegistrationMailChild{Name: child.FirstName + " " + child.LastName, Guest: child.HomeSchoolID == nil}
		for _, booking := range child.Bookings {
			if slices.Contains(moved, booking.WeekID) {
				mailChild.Weeks = append(mailChild.Weeks, lines(booking.WeekID, booking.TimeModelID, booking.Days))
			}
		}
		children = append(children, mailChild)
	}
	slices.Sort(keys)
	payload, err := json.Marshal(enrollment.HolidayPlaceGrantedMail{
		Recipient: registration.GuardianEmail, GuardianName: registration.GuardianFirstName + " " + registration.GuardianLastName,
		TraegerName: traeger.Name, PeriodName: period.Name, Binding: period.Binding, Children: children,
	})
	if err != nil {
		return fmt.Errorf("enrollment: encode holiday place mail: %w", err)
	}
	if err := p.mail.EnqueueTraegerMail(ctx, enrollment.TraegerMail{
		OrganizationID: organizationID, Kind: enrollment.MailKindHolidayPlaceGranted,
		Recipient: registration.GuardianEmail, Payload: payload,
		RelatedType: enrollment.MailRelatedHolidayRegistration, RelatedID: registrationID,
		IdempotencyKey: fmt.Sprintf("holiday-place-granted:%d:%s", registrationID, strings.Join(keys, ",")),
	}); err != nil {
		return fmt.Errorf("enrollment: enqueue holiday place mail: %w", err)
	}
	return nil
}

// holidayWeekLines names a booked week of the period in a mail: dates, host
// school and time model, and the booked days of a booking of days (#4701).
func holidayWeekLines(traeger enrollment.HolidayTraeger, period enrollment.HolidayPeriod) func(weekID, timeModelID int64, days []calendar.Date) string {
	schools := map[int64]string{}
	for _, school := range traeger.Schools {
		schools[school.ID] = school.Name
	}
	weeks := map[int64]enrollment.HolidayWeek{}
	models := map[int64]enrollment.HolidayTimeModel{}
	for _, week := range period.Weeks {
		weeks[week.ID] = week
		for _, model := range week.TimeModels {
			models[model.ID] = model
		}
	}
	return func(weekID, timeModelID int64, days []calendar.Date) string {
		week, model := weeks[weekID], models[timeModelID]
		place := model.Name
		if school := schools[week.HostSchoolID]; school != "" {
			place = school + ", " + model.Name
		}
		line := fmt.Sprintf("%s bis %s: %s (%s bis %s Uhr)", week.StartsOn.Format("02.01.2006"),
			week.EndsOn.Format("02.01.2006"), place, model.StartsAt, model.EndsAt)
		if len(days) > 0 {
			line += ", Tage: " + enrollment.HolidayDayNames(days)
		}
		return line
	}
}

// moveUp gives the places of the weeks to their waiting lists after a
// change of the locked registration.
func (h *HolidayRegistrationReview) moveUp(ctx context.Context, reviewer enrollment.HolidayReviewer, period enrollment.HolidayPeriod, weekIDs []int64) ([]enrollment.HolidayPromotion, error) {
	_, promoted, err := h.placement.place(ctx, reviewer.OrganizationID, period, weekIDs, 0, reviewer.AccountID)
	return promoted, err
}

// holidayBookedWeeks are the weeks of the bookings of read registrations.
func holidayBookedWeeks(read []enrollment.HolidayRegistrationDetail) []int64 {
	var weeks []int64
	for _, registration := range read {
		for _, child := range registration.Children {
			for _, booking := range child.Bookings {
				weeks = append(weeks, booking.WeekID)
			}
		}
	}
	return weeks
}

// HolidayPlaces serves enrollment.HolidayPlaces.
type HolidayPlaces struct {
	periods   HolidayPeriodStore
	store     HolidayPlaceStore
	placement holidayPlacement
}

var _ enrollment.HolidayPlaces = (*HolidayPlaces)(nil)

// NewHolidayPlaces builds the places over store; mail tells the parents of a
// booking that moved up, traeger names the Träger and the host schools in it.
// Without both a capacity that moves a booking up fails; the read works.
func NewHolidayPlaces(store HolidayPlaceStore, periods HolidayPeriodStore, traeger enrollment.HolidayTraegerDirectory, mail enrollment.TraegerMailOutbox) *HolidayPlaces {
	if store == nil || periods == nil {
		panic("enrollment: the holiday place store and the period store are required")
	}
	return &HolidayPlaces{periods: periods, store: store, placement: holidayPlacement{store: store, traeger: traeger, mail: mail}}
}

// HolidayWeekPlaces lists the places of every week of the period with its
// waiting list and records the read in the period's trail.
func (h *HolidayPlaces) HolidayWeekPlaces(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64) ([]enrollment.HolidayWeekPlaces, error) {
	if err := holidayReviewKey(reviewer, periodID, 1); err != nil {
		return nil, err
	}
	if _, err := foundHolidayPeriod(h.periods.FindHolidayPeriod(ctx, reviewer.OrganizationID, periodID, false)); err != nil {
		return nil, err
	}
	places, err := h.store.ReadHolidayWeekPlaces(ctx, reviewer.OrganizationID, periodID)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday week places: %w", err)
	}
	waiting := 0
	for _, week := range places {
		waiting += len(week.Waitlist)
	}
	if err := h.store.AppendHolidayAudit(ctx, reviewer.OrganizationID, periodID, 0, enrollment.HolidayAuditPlaces,
		map[string]any{"weeks": len(places), "waitlisted": waiting}, reviewer.AccountID); err != nil {
		return nil, fmt.Errorf("enrollment: record holiday places read: %w", err)
	}
	return places, nil
}

// holidayCapacityDetails is the audit entry of a changed capacity; nil is
// unlimited.
type holidayCapacityDetails struct {
	WeekID int64 `json:"week_id"`
	Before *int  `json:"before"`
	After  *int  `json:"after"`
}

// SetHolidayWeekCapacity locks the period and the week, writes the capacity
// and moves bookings up into the places it frees.
func (h *HolidayPlaces) SetHolidayWeekCapacity(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, weekID int64, capacity *int) (enrollment.HolidayCapacityChange, error) {
	if err := holidayReviewKey(reviewer, periodID, 1); err != nil {
		return enrollment.HolidayCapacityChange{}, err
	}
	if capacity != nil && (*capacity < 0 || *capacity > enrollment.HolidayWeekCapacityMax) {
		return enrollment.HolidayCapacityChange{}, validation.Errors{
			"capacity": fmt.Errorf("must be between 0 and %d", enrollment.HolidayWeekCapacityMax),
		}
	}
	period, err := foundHolidayPeriod(h.periods.FindHolidayPeriod(ctx, reviewer.OrganizationID, periodID, true))
	if err != nil {
		return enrollment.HolidayCapacityChange{}, err
	}
	index := slices.IndexFunc(period.Weeks, func(week enrollment.HolidayWeek) bool { return week.ID == weekID })
	if index < 0 {
		return enrollment.HolidayCapacityChange{}, enrollment.ErrHolidayWeekNotFound
	}
	if _, err := h.store.LockHolidayWeeks(ctx, reviewer.OrganizationID, periodID, []int64{weekID}); err != nil {
		return enrollment.HolidayCapacityChange{}, fmt.Errorf("enrollment: lock holiday week: %w", err)
	}
	if err := h.store.SetHolidayWeekCapacity(ctx, reviewer.OrganizationID, periodID, weekID, capacity); err != nil {
		return enrollment.HolidayCapacityChange{}, err
	}
	if err := h.store.AppendHolidayAudit(ctx, reviewer.OrganizationID, periodID, 0, enrollment.HolidayAuditCapacity,
		holidayCapacityDetails{WeekID: weekID, Before: period.Weeks[index].Capacity, After: capacity}, reviewer.AccountID); err != nil {
		return enrollment.HolidayCapacityChange{}, fmt.Errorf("enrollment: record holiday capacity: %w", err)
	}
	_, promoted, err := h.placement.place(ctx, reviewer.OrganizationID, period, []int64{weekID}, 0, reviewer.AccountID)
	if err != nil {
		return enrollment.HolidayCapacityChange{}, err
	}
	places, err := h.store.ReadHolidayWeekPlaces(ctx, reviewer.OrganizationID, periodID)
	if err != nil {
		return enrollment.HolidayCapacityChange{}, fmt.Errorf("enrollment: read holiday week places: %w", err)
	}
	change := enrollment.HolidayCapacityChange{Promotions: promoted}
	if change.Promotions == nil {
		change.Promotions = []enrollment.HolidayPromotion{}
	}
	for _, week := range places {
		if week.WeekID == weekID {
			change.Week = week
		}
	}
	return change, nil
}
