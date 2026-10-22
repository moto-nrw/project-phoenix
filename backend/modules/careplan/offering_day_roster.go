package careplan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareOfferingOption names one active care offering that counts as care, as
// a selection field lists it.
type CareOfferingOption struct {
	ID   int64
	Name string
}

// OfferingDayRoster answers which children a care offering holds on one day
// (#4571): the selection "Betreuungsangebot" of a spontaneous activity.
type OfferingDayRoster interface {
	// ActiveCareOfferings lists the school's active offerings that count as
	// care, in catalog order.
	ActiveCareOfferings(ctx context.Context) ([]CareOfferingOption, error)
	// ChildrenOnOfferingDay keeps those of studentIDs, in their order, whose
	// approved booking of the offering is valid on date and includes its
	// weekday, and who are not signed off for date. Signed off means a
	// status day, a day exception without a time on either leg ("Kommt heute
	// nicht", as ResolveDayPlanning reads it), or a pickup exception earlier
	// than the offering's pickup time of that weekday. A regular pickup time
	// a person entered is no sign-off. An unknown, inactive or not-care
	// offering holds no child.
	ChildrenOnOfferingDay(ctx context.Context, offeringID int64, studentIDs []int64, date calendar.Date) ([]int64, error)
}

// OfferingDayRosterRecords are the owner's offerings and the day facts that
// sign a child off for one day.
type OfferingDayRosterRecords interface {
	ListCareOfferings(context.Context, CareOfferingFilter) ([]CareOffering, error)
	ListPickupExceptions(context.Context, StudentScheduleFilter) ([]PickupException, error)
	ListArrivalExceptions(context.Context, StudentScheduleFilter) ([]ArrivalException, error)
	ListStudentStatusDays(context.Context, StudentStatusDayFilter) ([]StudentStatusDay, error)
}

// NewOfferingDayRoster binds the roster to the owner's records and the
// approved bookings. Every read is one batch over the asked children, so the
// cost does not grow with them.
func NewOfferingDayRoster(records OfferingDayRosterRecords, bookings ApprovedBookingReader) (OfferingDayRoster, error) {
	if records == nil || bookings == nil {
		return nil, errors.New("offering day roster: records and approved bookings are required")
	}
	return offeringDayRoster{records: records, bookings: bookings}, nil
}

type offeringDayRoster struct {
	records  OfferingDayRosterRecords
	bookings ApprovedBookingReader
}

func (q offeringDayRoster) ActiveCareOfferings(ctx context.Context) ([]CareOfferingOption, error) {
	rows, err := q.records.ListCareOfferings(ctx, CareOfferingFilter{ActiveOnly: true, Order: OfferingOrderCatalog})
	if err != nil {
		return nil, fmt.Errorf("list active care offerings: %w", err)
	}
	out := make([]CareOfferingOption, 0, len(rows))
	for _, row := range rows {
		if row.IsActive && row.CountsAsCare {
			out = append(out, CareOfferingOption{ID: row.ID, Name: row.Name})
		}
	}
	return out, nil
}

func (q offeringDayRoster) ChildrenOnOfferingDay(ctx context.Context, offeringID int64, studentIDs []int64, date calendar.Date) ([]int64, error) {
	if offeringID <= 0 || len(studentIDs) == 0 {
		return []int64{}, nil
	}
	offerings, err := q.records.ListCareOfferings(ctx, CareOfferingFilter{IDs: []int64{offeringID}})
	if err != nil {
		return nil, fmt.Errorf("offering day roster: load offering: %w", err)
	}
	if len(offerings) == 0 {
		return []int64{}, nil
	}
	facts, err := q.dayFacts(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	return childrenOnOfferingDay(&offerings[0], studentIDs, date, facts), nil
}

// offeringDayFacts are the day facts of the asked children on one date:
// their approved bookings and what signs them off for that date.
type offeringDayFacts struct {
	bookings          []*ApprovedBooking
	pickupExceptions  []PickupException
	arrivalExceptions []ArrivalException
	statusDays        []StudentStatusDay
}

func (q offeringDayRoster) dayFacts(ctx context.Context, studentIDs []int64, date calendar.Date) (offeringDayFacts, error) {
	var facts offeringDayFacts
	var err error
	if facts.bookings, err = q.bookings.ListApprovedByStudentIDsInRange(ctx, studentIDs, date, date); err != nil {
		return facts, fmt.Errorf("offering day roster: load bookings: %w", err)
	}
	day := StudentScheduleFilter{StudentIDs: studentIDs, Date: Date(date)}
	if facts.pickupExceptions, err = q.records.ListPickupExceptions(ctx, day); err != nil {
		return facts, fmt.Errorf("offering day roster: load pickup exceptions: %w", err)
	}
	if facts.arrivalExceptions, err = q.records.ListArrivalExceptions(ctx, day); err != nil {
		return facts, fmt.Errorf("offering day roster: load arrival exceptions: %w", err)
	}
	statusDays := StudentStatusDayFilter{StudentIDs: studentIDs, Date: Date(date), ActiveOnly: true}
	if facts.statusDays, err = q.records.ListStudentStatusDays(ctx, statusDays); err != nil {
		return facts, fmt.Errorf("offering day roster: load status days: %w", err)
	}
	return facts, nil
}

// childrenOnOfferingDay decides the roster from the loaded facts. A pickup
// exception without an offering time to compare against signs nobody off by
// its time alone.
func childrenOnOfferingDay(offering *CareOffering, studentIDs []int64, date calendar.Date, facts offeringDayFacts) []int64 {
	if offering == nil || !offering.IsActive || !offering.CountsAsCare {
		return []int64{}
	}
	booked := bookedOnOfferingDay(offering, date, facts.bookings)
	offeringPickup, hasOfferingPickup := offeringPickupOn(offering, date)
	signedOff := signedOffOnDay(facts, offeringPickup, hasOfferingPickup)
	out := make([]int64, 0, len(booked))
	for _, id := range studentIDs {
		if booked[id] && !signedOff[id] {
			out = append(out, id)
			booked[id] = false
		}
	}
	return out
}

var rosterDayKeys = [...]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// bookedOnOfferingDay is the set of children whose booking of the offering
// is valid on date and includes its weekday; valid_until is exclusive. A
// booking without its own days of a fixed offering takes the offering's.
func bookedOnOfferingDay(offering *CareOffering, date calendar.Date, bookings []*ApprovedBooking) map[int64]bool {
	key := rosterDayKeys[baselineWeekday(date)-1]
	out := make(map[int64]bool, len(bookings))
	for _, booking := range bookings {
		link := bookingLinkOf(booking)
		if link == nil || link.CareOfferingID != offering.ID || !bookingCovers(link, date) {
			continue
		}
		days := link.SelectedDays
		if len(days) == 0 && offering.DaysOfWeekMode == "fixed" {
			days = offering.AvailableDays
		}
		for _, day := range days {
			if strings.ToLower(strings.TrimSpace(day)) == key {
				out[booking.StudentID] = true
			}
		}
	}
	return out
}

func bookingLinkOf(booking *ApprovedBooking) *BookingSelection {
	if booking == nil {
		return nil
	}
	return booking.Link
}

func bookingCovers(link *BookingSelection, date calendar.Date) bool {
	return (link.ValidFrom == nil || !date.Before(*link.ValidFrom)) &&
		(link.ValidUntil == nil || date.Before(*link.ValidUntil))
}

// offeringPickupOn is the offering's pickup time of date's weekday.
func offeringPickupOn(offering *CareOffering, date calendar.Date) (time.Time, bool) {
	key := rosterDayKeys[baselineWeekday(date)-1]
	for day, hhmm := range offering.PickupTimes {
		if strings.ToLower(strings.TrimSpace(day)) != key {
			continue
		}
		parsed, err := time.Parse("15:04", strings.TrimSpace(hhmm))
		if err != nil {
			return time.Time{}, false
		}
		return calendar.NormalizeWallClock(parsed), true
	}
	return time.Time{}, false
}

func signedOffOnDay(facts offeringDayFacts, offeringPickup time.Time, hasOfferingPickup bool) map[int64]bool {
	out := make(map[int64]bool)
	for _, day := range facts.statusDays {
		out[day.StudentID] = true
	}
	for _, exception := range facts.arrivalExceptions {
		if exception.ExpectedArrival == nil {
			out[exception.StudentID] = true
		}
	}
	for _, exception := range facts.pickupExceptions {
		if exception.PickupTime == nil {
			out[exception.StudentID] = true
			continue
		}
		if hasOfferingPickup && calendar.NormalizeWallClock(*exception.PickupTime).Before(offeringPickup) {
			out[exception.StudentID] = true
		}
	}
	return out
}
