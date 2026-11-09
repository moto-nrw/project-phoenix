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
	// PlaceHolidayWaitlist gives the free places of a locked week to its
	// waiting list in its order and returns the bookings that got one;
	// those of submittedID are the registration being sent.
	PlaceHolidayWaitlist(ctx context.Context, organizationID, weekID, submittedID int64) ([]enrollment.HolidayPromotion, error)
	SetHolidayWeekCapacity(ctx context.Context, organizationID, periodID, weekID int64, capacity *int) error
	ReadHolidayWeekPlaces(ctx context.Context, organizationID, periodID int64) ([]enrollment.HolidayWeekPlaces, error)
	ReadHolidayReview(ctx context.Context, organizationID, periodID, registrationID int64) ([]enrollment.HolidayRegistrationDetail, map[int64]string, error)
	AppendHolidayAudit(ctx context.Context, organizationID, periodID, registrationID int64, action string, details any, actorAccountID int64) error
}

// errHolidayPlaceMailRequired reports a move up the waiting list without
// the Träger directory and the mail outbox to tell the parents.
var errHolidayPlaceMailRequired = errors.New("enrollment: moving up the holiday waiting list needs the Träger directory and the mail outbox")

// holidayPlacement gives places and moves bookings up. traeger and mail tell
// the parents of a booking that moved up; without them a move up fails, so
// no child moves up unnoticed.
type holidayPlacement struct {
	store   HolidayPlaceStore
	traeger enrollment.HolidayTraegerDirectory
	mail    enrollment.TraegerMailOutbox
}

// holidayPromotionDetails is the audit entry of a booking that moved up.
type holidayPromotionDetails struct {
	ChildID int64   `json:"child_id"`
	WeekIDs []int64 `json:"week_ids"`
}

// place locks the weeks in ascending ID and gives their free places to their
// waiting lists. The bookings of submittedID, the registration being sent,
// come back as own; every other booking that got a place moved up: it is
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
		placed, err := p.store.PlaceHolidayWaitlist(ctx, organizationID, weekID, submittedID)
		if err != nil {
			return nil, nil, fmt.Errorf("enrollment: place holiday waiting list: %w", err)
		}
		for _, booking := range placed {
			if booking.RegistrationID == submittedID {
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

// announce records each registration's moves up in its trail and enqueues
// one mail to its parents.
func (p holidayPlacement) announce(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, promoted []enrollment.HolidayPromotion, actorAccountID int64) error {
	if len(promoted) == 0 {
		return nil
	}
	if p.traeger == nil || p.mail == nil {
		return errHolidayPlaceMailRequired
	}
	traeger, err := p.traeger.HolidayTraeger(ctx, organizationID)
	if err != nil {
		return err
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
	for _, booking := range promoted {
		weeks[booking.ChildID] = append(weeks[booking.ChildID], booking.WeekID)
	}
	lines := holidayWeekLines(traeger, period)
	children := []enrollment.HolidayRegistrationMailChild{}
	keys := []string{}
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
				mailChild.Weeks = append(mailChild.Weeks, lines(booking.WeekID, booking.TimeModelID))
				keys = append(keys, strconv.FormatInt(child.ID, 10)+"-"+strconv.FormatInt(booking.WeekID, 10))
			}
		}
		children = append(children, mailChild)
	}
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
// school and time model.
func holidayWeekLines(traeger enrollment.HolidayTraeger, period enrollment.HolidayPeriod) func(weekID, timeModelID int64) string {
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
	return func(weekID, timeModelID int64) string {
		week, model := weeks[weekID], models[timeModelID]
		return fmt.Sprintf("%s bis %s: %s, %s (%s bis %s Uhr)", week.StartsOn.Format("02.01.2006"),
			week.EndsOn.Format("02.01.2006"), schools[week.HostSchoolID], model.Name, model.StartsAt, model.EndsAt)
	}
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

// NewHolidayPlaces builds the places over store; traeger and mail tell the
// parents of a booking that moved up. Without them only the read works.
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
